package main

import (
	"context"
	"embed"

	"github.com/codefly-dev/core/builders"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/codefly-dev/core/templates"

	"github.com/codefly-dev/core/agents"
	"github.com/codefly-dev/core/agents/services"
	basev0 "github.com/codefly-dev/core/generated/go/codefly/base/v0"
	agentv0 "github.com/codefly-dev/core/generated/go/codefly/services/agent/v0"
	configurations "github.com/codefly-dev/core/resources"
	"github.com/codefly-dev/core/shared"
)

// Agent version
var agent = shared.Must(configurations.LoadFromFs[configurations.Agent](shared.Embed(infoFS)))

var requirements = builders.NewDependencies(agent.Name,
	builders.NewDependency("service.codefly.yaml"),
	builders.NewDependency("code").WithPathSelect(shared.NewSelect("*.swift")),
)

// Settings holds agent-level configuration stored in service.codefly.yaml.
type Settings struct {
	HotReload bool   `yaml:"hot-reload"`
	SourceDir string `yaml:"source-dir"`
	Framework string `yaml:"framework"`
}

func (s *Settings) SwiftSourceDir() string {
	if s.SourceDir != "" {
		return s.SourceDir
	}
	return "code"
}

func (s *Settings) SwiftFramework() string {
	if s.Framework != "" {
		return s.Framework
	}
	return "vapor"
}

const HotReloadSetting = "hot-reload"
const FrameworkSetting = "framework"

// Service holds shared state across Runtime and Builder.
type Service struct {
	*services.Base

	// Endpoints
	RestEndpoint *basev0.Endpoint

	// Settings
	*Settings

	sourceLocation string
}

func (s *Service) GetAgentInformation(ctx context.Context, _ *agentv0.AgentInformationRequest) (*agentv0.AgentInformation, error) {
	defer s.Wool.Catch()

	// Information may be nil if GetAgentInformation is called before Load.
	info := s.Information
	if info == nil {
		info = &services.Information{}
	}
	readme, err := templates.ApplyTemplateFrom(ctx, shared.Embed(readmeFS), "templates/agent/README.md", info)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &agentv0.AgentInformation{
		RuntimeRequirements: []*agentv0.Runtime{},
		Capabilities: []*agentv0.Capability{
			{Type: agentv0.Capability_BUILDER},
			{Type: agentv0.Capability_RUNTIME},
		},
		Languages: []*agentv0.Language{},
		Protocols: []*agentv0.Protocol{
			{Type: agentv0.Protocol_HTTP},
		},
		ReadMe: readme,
	}, nil
}

func NewService() *Service {
	return &Service{
		Base: services.NewServiceBase(context.Background(), agent),
		Settings: &Settings{
			SourceDir: "code",
			Framework: "vapor",
		},
	}
}

// SwiftVersion used in Docker builds
const SwiftVersion = "6.0"

// UbuntuVersion for the runtime stage
const UbuntuVersion = "22.04"

func main() {
	svc := NewService()
	agents.Serve(agents.PluginRegistration{
		Agent:   svc,
		Runtime: NewRuntime(),
		Builder: NewBuilder(),
	})
}

//go:embed agent.codefly.yaml
var infoFS embed.FS

//go:embed templates/agent
var readmeFS embed.FS
