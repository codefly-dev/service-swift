package main

import (
	"os"
	"path/filepath"
	"testing"

	agenttesting "github.com/codefly-dev/core/agents/testing"
	"gopkg.in/yaml.v3"
)

func TestDeploymentTemplates(t *testing.T) {
	agenttesting.AssertKustomizeTemplates(t, deploymentFS, nil)
}

// TestDeploymentMountsWritableTmpUnderReadOnlyRoot pins the invariant that a
// read-only root filesystem is paired with writable scratch space, so the
// Swift service can still write temporary files at runtime.
func TestDeploymentMountsWritableTmpUnderReadOnlyRoot(t *testing.T) {
	dir := agenttesting.AssertKustomizeTemplates(t, deploymentFS, nil)

	raw, err := os.ReadFile(filepath.Join(dir, "base", "deployment.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}

	podSpec := nested(t, doc, "spec", "template", "spec")
	containers, ok := podSpec["containers"].([]any)
	if !ok || len(containers) == 0 {
		t.Fatal("deployment declares no containers")
	}
	container := containers[0].(map[string]any)

	security, ok := container["securityContext"].(map[string]any)
	if !ok {
		t.Fatal("container has no securityContext")
	}
	if security["readOnlyRootFilesystem"] != true {
		t.Fatalf("readOnlyRootFilesystem = %v, want true", security["readOnlyRootFilesystem"])
	}

	mountName := ""
	for _, raw := range asSlice(container["volumeMounts"]) {
		mount := raw.(map[string]any)
		if mount["mountPath"] == "/tmp" {
			mountName, _ = mount["name"].(string)
		}
	}
	if mountName == "" {
		t.Fatal("read-only root filesystem has no writable volumeMount at /tmp")
	}

	for _, raw := range asSlice(podSpec["volumes"]) {
		volume := raw.(map[string]any)
		if volume["name"] != mountName {
			continue
		}
		if _, isEmptyDir := volume["emptyDir"]; !isEmptyDir {
			t.Fatalf("volume %q backing /tmp is not an emptyDir", mountName)
		}
		return
	}
	t.Fatalf("pod spec declares no volume named %q for the /tmp mount", mountName)
}

func nested(t *testing.T, value map[string]any, path ...string) map[string]any {
	t.Helper()
	for _, key := range path {
		next, ok := value[key].(map[string]any)
		if !ok {
			t.Fatalf("expected mapping at %q", key)
		}
		value = next
	}
	return value
}

func asSlice(value any) []any {
	slice, _ := value.([]any)
	return slice
}
