package expand_test

import (
	"testing"

	"github.com/alinz/script.go/v2/internal/expand"
)

func TestVars(t *testing.T) {
	lookup := func(key string) (string, bool) {
		values := map[string]string{
			"workspace": "/home/runner/work",
			"TOKEN":     "abc==def", // values containing '=' must survive
		}
		v, ok := values[key]
		return v, ok
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "cd ${workspace}", "cd /home/runner/work"},
		{"multiple", "${workspace}/${workspace}", "/home/runner/work//home/runner/work"},
		{"value with equals", "token=${TOKEN}", "token=abc==def"},
		{"unknown preserved", "echo ${UNKNOWN_VAR}", "echo ${UNKNOWN_VAR}"},
		{"bare dollar untouched", "echo $workspace", "echo $workspace"},
		{"no vars", "ls -la", "ls -la"},
		{"empty", "", ""},
		{"invalid name untouched", "echo ${1BAD}", "echo ${1BAD}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expand.Vars(tt.in, lookup); got != tt.want {
				t.Errorf("Vars(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestOSLookup(t *testing.T) {
	t.Setenv("SCRIPT_GO_TEST_VAR", "from-env")
	t.Setenv("SCRIPT_GO_SHADOWED", "from-env")

	lookup := expand.OSLookup(map[string]string{
		"workspace":          "/ws",
		"SCRIPT_GO_SHADOWED": "from-extra",
	})

	if v, ok := lookup("workspace"); !ok || v != "/ws" {
		t.Errorf("lookup(workspace) = %q, %v", v, ok)
	}
	if v, ok := lookup("SCRIPT_GO_TEST_VAR"); !ok || v != "from-env" {
		t.Errorf("lookup(SCRIPT_GO_TEST_VAR) = %q, %v", v, ok)
	}
	if v, ok := lookup("SCRIPT_GO_SHADOWED"); !ok || v != "from-extra" {
		t.Errorf("extra map should take precedence, got %q, %v", v, ok)
	}
	if _, ok := lookup("SCRIPT_GO_DEFINITELY_NOT_SET"); ok {
		t.Error("expected miss for unset var")
	}
}
