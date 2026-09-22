package main

import (
	"path/filepath"
	"strings"
	"testing"

	basev0 "github.com/codefly-dev/core/generated/go/codefly/base/v0"
	builderv0 "github.com/codefly-dev/core/generated/go/codefly/services/builder/v0"
	"github.com/codefly-dev/core/resources"
)

func TestCreationLoadsEmbeddedGettingStartedTemplate(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	service := &resources.Service{Name: "subject", Version: "0.0.0"}
	if err := service.SaveAtDir(ctx, filepath.Join(root, "app", "subject")); err != nil {
		t.Fatal(err)
	}
	builder := NewBuilder(NewService())
	response, err := builder.Load(ctx, &builderv0.LoadRequest{
		DisableCatch: true,
		Identity: &basev0.ServiceIdentity{
			Name: "subject", Module: "app", Workspace: "fixture",
			WorkspacePath: root, RelativeToWorkspace: "app/subject",
		},
		CreationMode: &builderv0.CreationMode{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetState().GetState() != builderv0.LoadStatus_READY {
		t.Fatalf("creation load failed: %v", response.GetState())
	}
	if !strings.Contains(builder.Builder.GettingStarted, "# Swift Service") {
		t.Fatal("creation did not load the embedded Swift guidance")
	}
}
