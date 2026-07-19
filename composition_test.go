package main

import (
	"context"
	"testing"

	agentv0 "github.com/codefly-dev/core/generated/go/codefly/services/agent/v0"
	"gopkg.in/yaml.v3"
)

func TestNewService_EmbedsBase(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.Base == nil {
		t.Fatal("Service.Base is nil — services.Base embedding broken")
	}
	if svc.Settings == nil {
		t.Fatal("Service.Settings is nil")
	}
}

func TestGetAgentInformationUsesEmbeddedReadmeTemplate(t *testing.T) {
	information, err := NewService().GetAgentInformation(context.Background(), &agentv0.AgentInformationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if information.GetReadMe() == "" {
		t.Fatal("agent README is empty")
	}
}

func TestSettings_YAMLRoundTrip(t *testing.T) {
	src := []byte(`
hot-reload: true
source-dir: Sources
framework: vapor
`)
	var s Settings
	if err := yaml.Unmarshal(src, &s); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	if !s.HotReload {
		t.Error("HotReload not populated")
	}
	if s.SourceDir != "Sources" {
		t.Errorf("SourceDir: got %q", s.SourceDir)
	}
	if s.Framework != "vapor" {
		t.Errorf("Framework: got %q", s.Framework)
	}
}
