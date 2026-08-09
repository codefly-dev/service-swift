package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	corecode "github.com/codefly-dev/core/code"
	basev0 "github.com/codefly-dev/core/generated/go/codefly/base/v0"
	codev0 "github.com/codefly-dev/core/generated/go/codefly/services/code/v0"
	toolingv0 "github.com/codefly-dev/core/generated/go/codefly/services/tooling/v0"
)

func TestSwiftToolingReportsSemanticCoverageHonestly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Example.swift"), []byte("struct Example {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService()
	service.sourceLocation = dir
	response, err := corecode.NewSourceTooling(NewCode(service)).GetSemanticIndex(t.Context(), &toolingv0.GetSemanticIndexRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetFailure() != nil || response.GetIndex().GetState() != basev0.SemanticIndexState_SEMANTIC_INDEX_STATE_NOT_ATTEMPTED || len(response.GetIndex().GetIssues()) != 1 || response.GetIndex().GetIssues()[0].GetCode() != "unsupported_source" {
		t.Fatalf("semantic response = %+v", response)
	}
}

func TestSwiftFixDryRunFormatsWithoutWriting(t *testing.T) {
	if _, err := exec.LookPath("swift"); err != nil {
		t.Skip("Swift toolchain not installed")
	}
	dir := t.TempDir()
	original := []byte("import Foundation\nstruct Example{let value:Int}\n")
	path := filepath.Join(dir, "Example.swift")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService()
	service.sourceLocation = dir
	response, err := NewCode(service).Execute(context.Background(), &codev0.CodeRequest{Operation: &codev0.CodeRequest_Fix{Fix: &codev0.FixRequest{
		File: "Example.swift", Mode: basev0.FixMode_FIX_MODE_SAFE, DryRun: true,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	fix := response.GetFix()
	if !fix.GetSuccess() || !fix.GetChanged() || fix.GetWrote() || !strings.Contains(fix.GetContent(), "struct Example { let value: Int }") {
		t.Fatalf("fix = %+v failure=%+v", fix, response.GetFailure())
	}
	written, err := os.ReadFile(path)
	if err != nil || string(written) != string(original) {
		t.Fatalf("dry-run changed source: err=%v content=%q", err, written)
	}
}
