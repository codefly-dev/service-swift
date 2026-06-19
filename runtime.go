package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	basev0 "github.com/codefly-dev/core/generated/go/codefly/base/v0"

	"github.com/codefly-dev/core/agents/helpers/code"
	"github.com/codefly-dev/core/agents/services"
	"github.com/codefly-dev/core/resources"
	runners "github.com/codefly-dev/core/runners/base"
	"github.com/codefly-dev/core/wool"

	runtimev0 "github.com/codefly-dev/core/generated/go/codefly/services/runtime/v0"
)

type Runtime struct {
	services.RuntimeServer

	*Service

	// Native runner environment for swift processes
	nativeEnv *runners.NativeEnvironment

	// Running swift process
	runner runners.Proc

	// Port the service is bound to (for readiness check)
	port uint32
}

func NewRuntime() *Runtime {
	return &Runtime{
		Service: NewService(),
	}
}

func (s *Runtime) Load(ctx context.Context, req *runtimev0.LoadRequest) (*runtimev0.LoadResponse, error) {
	err := s.Base.Load(ctx, req.Identity, s.Settings)
	if err != nil {
		return s.Runtime.LoadErrorf(err, "loading base")
	}

	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	if req.DisableCatch {
		s.Wool.DisableCatch()
	}

	s.Runtime.SetEnvironment(req.Environment)

	s.sourceLocation, err = s.LocalDirCreate(ctx, "%s", s.Settings.SwiftSourceDir())
	if err != nil {
		return s.Runtime.LoadErrorf(err, "creating source location")
	}

	if s.Watcher != nil {
		s.Watcher.Pause()
	}

	s.Endpoints, err = s.Base.Service.LoadEndpoints(ctx)
	if err != nil {
		return s.Runtime.LoadErrorf(err, "loading endpoints")
	}

	s.RestEndpoint, err = resources.FindRestEndpoint(ctx, s.Endpoints)
	if err != nil {
		// Fall back to HTTP endpoint
		s.RestEndpoint, err = resources.FindHTTPEndpoint(ctx, s.Endpoints)
		if err != nil {
			return s.Runtime.LoadErrorf(err, "finding rest/http endpoint")
		}
	}

	return s.Runtime.LoadResponse()
}

func (s *Runtime) Init(ctx context.Context, req *runtimev0.InitRequest) (*runtimev0.InitResponse, error) {
	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	s.Runtime.LogInitRequest(req)

	s.Runtime.RuntimeContext = req.RuntimeContext

	s.Wool.Forwardf("starting Swift service in %s mode", s.Runtime.RuntimeContext.Kind)

	s.EnvironmentVariables.SetRuntimeContext(s.Runtime.RuntimeContext)

	s.NetworkMappings = req.ProposedNetworkMappings

	// Project configurations
	err := s.EnvironmentVariables.AddConfigurations(ctx, req.WorkspaceConfigurations...)
	if err != nil {
		return s.Runtime.InitError(err)
	}

	// Dependency configurations
	confs := resources.FilterConfigurations(req.DependenciesConfigurations, s.Runtime.RuntimeContext)
	s.Wool.Trace("adding configurations", wool.Field("configurations", resources.MakeManyConfigurationSummary(confs)))
	err = s.EnvironmentVariables.AddConfigurations(ctx, confs...)
	if err != nil {
		return s.Runtime.InitError(err)
	}

	// Networking: find our assigned port
	net, err := resources.FindNetworkInstanceInNetworkMappings(ctx, s.NetworkMappings, s.RestEndpoint, resources.NewNativeNetworkAccess())
	if err != nil {
		return s.Runtime.InitError(err)
	}

	s.port = net.Port
	s.Infof("HTTP will run on %s", net.Address)

	nm, err := resources.FindNetworkMapping(ctx, s.NetworkMappings, s.RestEndpoint)
	if err != nil {
		return s.Runtime.InitError(err)
	}
	err = s.EnvironmentVariables.AddEndpoints(ctx, []*basev0.NetworkMapping{nm}, resources.NewNativeNetworkAccess())
	if err != nil {
		return s.Runtime.InitError(err)
	}

	// Hot-reload setup
	if s.Settings.HotReload {
		s.Wool.Trace("starting hot reload")
		dependencies := requirements.Clone()
		dependencies.Localize(s.Location)
		s.Wool.Trace("setting up code watcher", wool.Field("dep", dependencies.All()))
		conf := services.NewWatchConfiguration(dependencies)
		err = s.SetupWatcher(ctx, conf, s.EventHandler)
		if err != nil {
			s.Wool.Warn("error in watcher", wool.ErrField(err))
		}
	} else {
		s.Wool.Trace("hot-reload disabled")
	}

	if s.Watcher != nil {
		s.Watcher.Resume()
	}

	// Create native runner environment
	s.nativeEnv, err = runners.NewNativeEnvironment(ctx, s.sourceLocation)
	if err != nil {
		return s.Runtime.InitErrorf(err, "cannot create native environment")
	}

	err = s.nativeEnv.Init(ctx)
	if err != nil {
		return s.Runtime.InitErrorf(err, "cannot init native environment")
	}

	s.Wool.Info("successful init of Swift runner")

	return s.Runtime.InitResponse()
}

func (s *Runtime) Start(ctx context.Context, req *runtimev0.StartRequest) (*runtimev0.StartResponse, error) {
	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	s.Wool.Forwardf("starting Swift service...")

	// Stop before replacing the runner
	if s.runner != nil {
		err := s.runner.Stop(ctx)
		if err != nil {
			return s.Runtime.StartError(err)
		}
	}

	// Add DependenciesNetworkMappings
	err := s.EnvironmentVariables.AddEndpoints(ctx, req.DependenciesNetworkMappings, resources.NetworkAccessFromRuntimeContext(s.Runtime.RuntimeContext))
	if err != nil {
		return s.Runtime.StartError(err)
	}

	// Add Fixture
	s.EnvironmentVariables.SetFixture(req.Fixture)

	// Add per-service runtime overrides (--set <service>:KEY=VAL)
	s.EnvironmentVariables.AddOverrides(req.GetOverrides())

	runningContext := s.Wool.Inject(context.Background())

	// Create swift run process
	proc, err := s.nativeEnv.NewProcess("swift", "run")
	if err != nil {
		return s.Runtime.StartErrorf(err, "creating swift run process")
	}

	startEnvs, err := s.EnvironmentVariables.All()
	if err != nil {
		return s.Runtime.StartErrorf(err, "getting environment variables")
	}
	proc.WithEnvironmentVariables(ctx, startEnvs...)

	// Pass the port via environment variable so Swift code can read it
	proc.WithEnvironmentVariables(ctx, &resources.EnvironmentVariable{
		Key:   "PORT",
		Value: fmt.Sprintf("%d", s.port),
	})

	proc.WithOutput(s.Logger)

	s.runner = proc
	err = s.runner.Start(runningContext)
	if err != nil {
		if s.Settings.HotReload {
			s.Wool.Info("compile error, waiting for hot-reload")
			return s.Runtime.StartResponse()
		}
		return s.Runtime.StartErrorf(err, "starting swift run")
	}

	// Wait for readiness
	s.Wool.Trace("waiting for readiness")
	err = s.WaitForReady(ctx)
	if err != nil {
		s.Wool.Warn("readiness check failed", wool.ErrField(err))
	}

	s.Wool.Forwardf("Swift service started and running")

	return s.Runtime.StartResponse()
}

// WaitForReady polls the health endpoint until the service responds or timeout.
func (s *Runtime) WaitForReady(ctx context.Context) error {
	addr := fmt.Sprintf("http://localhost:%d/health", s.port)
	timeout := time.After(30 * time.Second)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("service not ready after 30s")
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			resp, err := http.Get(addr)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
	}
}

func (s *Runtime) Test(ctx context.Context, req *runtimev0.TestRequest) (*runtimev0.TestResponse, error) {
	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	s.Infof("running swift test")

	proc, err := s.nativeEnv.NewProcess("swift", "test")
	if err != nil {
		return s.Runtime.TestErrorf(err, "creating swift test process")
	}

	testEnvs, err := s.EnvironmentVariables.All()
	if err != nil {
		return s.Runtime.TestErrorf(err, "getting environment variables")
	}
	proc.WithEnvironmentVariables(ctx, testEnvs...)
	proc.WithOutput(s.Logger)

	runErr := proc.Run(ctx)
	if runErr != nil {
		s.Wool.Forwardf("Tests: FAILED")
		return s.Runtime.TestResponseWithResults(1, 0, 1, 0, 0, []string{runErr.Error()}, runErr)
	}

	s.Wool.Forwardf("Tests: PASSED")
	return s.Runtime.TestResponseWithResults(1, 1, 0, 0, 0, nil, nil)
}

func (s *Runtime) Information(ctx context.Context, req *runtimev0.InformationRequest) (*runtimev0.InformationResponse, error) {
	return s.Runtime.InformationResponse(ctx, req)
}

func (s *Runtime) Stop(ctx context.Context, req *runtimev0.StopRequest) (*runtimev0.StopResponse, error) {
	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	s.Wool.Trace("stopping service")
	if s.runner != nil {
		s.Wool.Trace("stopping runner")
		err := s.runner.Stop(ctx)
		if err != nil {
			return s.Runtime.StopError(err)
		}
		s.runner = nil
		s.Wool.Trace("runner stopped")
	}

	// Stop the file watcher to prevent CPU spin on orphaned processes
	if s.Watcher != nil {
		s.Watcher.Pause()
	}
	// Close events channel to unblock the handler goroutine
	if s.Events != nil {
		close(s.Events)
		s.Events = nil
	}

	s.Wool.Trace("base stopped")
	return s.Runtime.StopResponse()
}

func (s *Runtime) Destroy(ctx context.Context, req *runtimev0.DestroyRequest) (*runtimev0.DestroyResponse, error) {
	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	s.Wool.Trace("destroying service")
	// Clean up .build directory if present
	_ = os.RemoveAll(s.sourceLocation + "/.build")
	return s.Runtime.DestroyResponse()
}

/* Details */

func (s *Runtime) EventHandler(event code.Change) error {
	if strings.HasSuffix(event.Path, ".swift") {
		s.Wool.Info("detected Swift source change", wool.Field("path", event.Path))
		s.Runtime.DesiredStart()
		return nil
	}
	if strings.HasSuffix(event.Path, "Package.swift") {
		s.Wool.Info("detected Package.swift change", wool.Field("path", event.Path))
		s.Runtime.DesiredStart()
		return nil
	}
	return nil
}
