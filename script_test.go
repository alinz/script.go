package script

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunLocal(t *testing.T) {
	r := &runner{}
	workspace := t.TempDir()

	t.Run("expands workspace and supports shell features", func(t *testing.T) {
		out := filepath.Join(workspace, "out.txt")
		err := r.RunLocal(workspace, `echo "hello world" > ${workspace}/out.txt`)
		if err != nil {
			t.Fatalf("RunLocal failed: %v", err)
		}

		content, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("output file not written: %v", err)
		}
		if string(content) != "hello world\n" {
			t.Errorf("unexpected content: %q", content)
		}
	})

	t.Run("expands environment variables", func(t *testing.T) {
		t.Setenv("SCRIPT_GO_TEST_MSG", "from-env")

		out := filepath.Join(workspace, "env.txt")
		err := r.RunLocal(workspace, `printf %s ${SCRIPT_GO_TEST_MSG} > ${workspace}/env.txt`)
		if err != nil {
			t.Fatalf("RunLocal failed: %v", err)
		}

		content, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("output file not written: %v", err)
		}
		if string(content) != "from-env" {
			t.Errorf("unexpected content: %q", content)
		}
	})

	t.Run("propagates failure", func(t *testing.T) {
		if err := r.RunLocal(workspace, "exit 3"); err == nil {
			t.Error("expected error from failing command")
		}
	})

	t.Run("stops at first failure", func(t *testing.T) {
		marker := filepath.Join(workspace, "marker.txt")
		_ = r.RunLocal(workspace, "false", "touch ${workspace}/marker.txt")
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Error("command after a failure should not run")
		}
	})
}

func TestNewRunnerValidation(t *testing.T) {
	if _, err := NewRunner(nil); err == nil {
		t.Error("expected error for nil config")
	}
	if _, err := NewRunner(&Config{User: "deploy"}); err == nil {
		t.Error("expected error for missing host")
	}
	if _, err := NewRunner(&Config{Host: "example.com"}); err == nil {
		t.Error("expected error for missing user")
	}
}
