package main

import (
	"context"
	"embed"

	dockerhelpers "github.com/codefly-dev/core/agents/helpers/docker"
	"github.com/codefly-dev/core/agents/communicate"
	"github.com/codefly-dev/core/agents/services"
	basev0 "github.com/codefly-dev/core/generated/go/codefly/base/v0"
	agentv0 "github.com/codefly-dev/core/generated/go/codefly/services/agent/v0"
	builderv0 "github.com/codefly-dev/core/generated/go/codefly/services/builder/v0"
	"github.com/codefly-dev/core/resources"
	"github.com/codefly-dev/core/shared"
	"github.com/codefly-dev/core/standards"
	"github.com/codefly-dev/core/templates"
	"github.com/codefly-dev/core/wool"
)

type Builder struct {
	services.BuilderServer

	*Service

	// Answers from interactive Communicate stream (set during Create/Sync flows)
	answers map[string]*agentv0.Answer
}

func NewBuilder() *Builder {
	return &Builder{
		Service: NewService(),
	}
}

func (s *Builder) Load(ctx context.Context, req *builderv0.LoadRequest) (*builderv0.LoadResponse, error) {
	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	err := s.Builder.Load(ctx, req.Identity, s.Settings)
	if err != nil {
		return nil, err
	}

	s.sourceLocation = s.Local("%s", s.Settings.SwiftSourceDir())

	requirements.Localize(s.Location)

	if req.CreationMode != nil {
		s.Builder.CreationMode = req.CreationMode

		s.Builder.GettingStarted, err = templates.ApplyTemplateFrom(ctx, shared.Embed(factoryFS), "templates/factory/GETTING_STARTED.md", s.Information)
		if err != nil {
			return s.Builder.LoadError(err)
		}

		return s.Builder.LoadResponse()
	}

	s.Endpoints, err = s.Base.Service.LoadEndpoints(ctx)
	if err != nil {
		return s.Builder.LoadError(err)
	}

	s.RestEndpoint, err = resources.FindRestEndpoint(ctx, s.Endpoints)
	if err != nil {
		// Fall back to HTTP endpoint
		s.RestEndpoint, err = resources.FindHTTPEndpoint(ctx, s.Endpoints)
		if err != nil {
			return s.Builder.LoadError(err)
		}
	}

	return s.Builder.LoadResponse()
}

func (s *Builder) Init(ctx context.Context, req *builderv0.InitRequest) (*builderv0.InitResponse, error) {
	defer s.Wool.Catch()

	s.Builder.LogInitRequest(req)

	ctx = s.Wool.Inject(ctx)

	s.DependencyEndpoints = req.DependenciesEndpoints

	return s.Builder.InitResponse()
}

func (s *Builder) Update(ctx context.Context, req *builderv0.UpdateRequest) (*builderv0.UpdateResponse, error) {
	defer s.Wool.Catch()

	ctx = s.Wool.Inject(ctx)

	return &builderv0.UpdateResponse{}, nil
}

func (s *Builder) Sync(ctx context.Context, req *builderv0.SyncRequest) (*builderv0.SyncResponse, error) {
	defer s.Wool.Catch()

	ctx = s.Wool.Inject(ctx)

	// No proto generation for Swift — REST-only service
	return s.Builder.SyncResponse()
}

// Audit/Upgrade — Swift agent is WIP. Return Tool="missing" / NOOP so
// workspace-wide commands aggregate cleanly without erroring on swift.
func (s *Builder) Audit(ctx context.Context, _ *builderv0.AuditRequest) (*builderv0.AuditResponse, error) {
	defer s.Wool.Catch()
	return s.Builder.AuditResponse(nil, nil, "missing", "SWIFT")
}

func (s *Builder) Upgrade(ctx context.Context, _ *builderv0.UpgradeRequest) (*builderv0.UpgradeResponse, error) {
	defer s.Wool.Catch()
	return s.Builder.UpgradeResponse(nil, "")
}

// DockerTemplating holds template parameters for the Swift Dockerfile.
type DockerTemplating struct {
	SwiftVersion  string
	UbuntuVersion string
	Envs          []DockerEnv
}

// DockerEnv is a key-value pair for Docker environment variables.
type DockerEnv struct {
	Key   string
	Value string
}

func (s *Builder) Build(ctx context.Context, req *builderv0.BuildRequest) (*builderv0.BuildResponse, error) {
	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	w := s.Wool

	dockerRequest, err := s.Builder.DockerBuildRequest(ctx, req)
	if err != nil {
		return s.Builder.BuildError(err)
	}

	image := s.Builder.DockerImage(dockerRequest)
	w.Debug("building docker image", wool.Field("image", image.FullName()))

	if !dockerhelpers.IsValidDockerImageName(image.Name) {
		return s.Builder.BuildError(err)
	}

	docker := DockerTemplating{
		SwiftVersion:  SwiftVersion,
		UbuntuVersion: UbuntuVersion,
	}

	_ = shared.DeleteFile(ctx, s.Location+"/builder/Dockerfile")

	err = s.Builder.Templates(ctx, docker, services.WithBuilder(builderFS))
	if err != nil {
		return s.Builder.BuildError(err)
	}

	b, err := dockerhelpers.NewBuilder(dockerhelpers.BuilderConfiguration{
		Root:        s.Location,
		Dockerfile:  "builder/Dockerfile",
		Ignorefile:  "builder/dockerignore",
		Destination: image,
		Output:      w,
	})
	if err != nil {
		return s.Builder.BuildError(err)
	}

	_, err = b.Build(ctx)
	if err != nil {
		return s.Builder.BuildError(err)
	}

	s.Builder.WithDockerImages(image)

	return s.Builder.BuildResponse()
}

func (s *Builder) Deploy(ctx context.Context, req *builderv0.DeploymentRequest) (*builderv0.DeploymentResponse, error) {
	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	// Placeholder: deployment not yet implemented for Swift
	return s.Builder.DeployResponse()
}

func (s *Builder) Options() []*agentv0.Question {
	return []*agentv0.Question{
		communicate.NewConfirm(&agentv0.Message{Name: HotReloadSetting, Message: "Code hot-reload (Recommended)?", Description: "codefly can restart your Swift service when source files change"}, true),
	}
}

// CreateConfiguration is the data passed to factory templates during scaffolding.
type CreateConfiguration struct {
	*services.Information
	Framework string
	Envs      []string
}

func (s *Builder) Create(ctx context.Context, req *builderv0.CreateRequest) (*builderv0.CreateResponse, error) {
	defer s.Wool.Catch()

	ctx = s.Wool.Inject(ctx)

	if s.Builder.CreationMode != nil && s.Builder.CreationMode.Communicate && s.answers != nil {
		var err error
		s.Settings.HotReload, err = communicate.Confirm(s.answers, HotReloadSetting)
		if err != nil {
			return s.Builder.CreateError(err)
		}
	} else {
		// No interactive session -- use defaults
		options := s.Options()
		var err error
		s.Settings.HotReload, err = communicate.GetDefaultConfirm(options, HotReloadSetting)
		if err != nil {
			return s.Builder.CreateError(err)
		}
	}
	// Framework defaults to "vapor" from Settings initialization
	if s.Settings.Framework == "" {
		s.Settings.Framework = "vapor"
	}

	create := CreateConfiguration{
		Information: s.Information,
		Framework:   s.Settings.SwiftFramework(),
		Envs:        []string{},
	}

	err := s.Templates(ctx, create, services.WithFactory(factoryFS))
	if err != nil {
		return s.Builder.CreateError(err)
	}

	// Create REST endpoint
	err = s.CreateEndpoints(ctx)
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot create endpoints")
	}

	return s.Builder.CreateResponse(ctx, s.Settings)
}

func (s *Builder) CreateEndpoints(ctx context.Context) error {
	rest, err := resources.LoadRestAPI(ctx, s.Base.Service.LocalOrNil(ctx, standards.OpenAPIPath))
	if err != nil {
		// No OpenAPI spec yet is fine -- create endpoint with empty API
		rest = &basev0.RestAPI{}
	}

	endpoint := s.Base.BaseEndpoint(standards.REST)
	s.RestEndpoint, err = resources.NewAPI(ctx, endpoint, resources.ToRestAPI(rest))
	if err != nil {
		return s.Wool.Wrapf(err, "cannot create rest api")
	}

	s.Endpoints = append(s.Endpoints, s.RestEndpoint)
	return nil
}

func (s *Builder) Communicate(stream builderv0.Builder_CommunicateServer) error {
	asker := communicate.NewQuestionAsker(stream)
	answers, err := asker.RunSequence(s.Options())
	if err != nil {
		return err
	}
	s.answers = answers
	return nil
}

//go:embed templates/factory
var factoryFS embed.FS

//go:embed templates/builder
var builderFS embed.FS

//go:embed templates/deployment
var deploymentFS embed.FS
