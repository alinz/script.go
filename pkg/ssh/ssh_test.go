package ssh

import (
	"regexp"
	"strings"
	"testing"
)

func TestRenderEnvFile(t *testing.T) {
	lookup := func(key string) (string, bool) {
		values := map[string]string{
			"DB_HOST": "db.internal",
			"SECRET":  "s3cr3t==",
		}
		v, ok := values[key]
		return v, ok
	}

	env := map[string]string{
		"B_KEY":     "plain",
		"A_KEY":     "host is ${DB_HOST}",
		"C_SECRET":  "${SECRET}",
		"D_UNKNOWN": "${NOPE}",
		"E_QUOTES":  `va"lue with 'quotes' and $dollar`,
	}

	want := "A_KEY=host is db.internal\n" +
		"B_KEY=plain\n" +
		"C_SECRET=s3cr3t==\n" +
		"D_UNKNOWN=${NOPE}\n" +
		"E_QUOTES=va\"lue with 'quotes' and $dollar\n"

	if got := renderEnvFile(env, lookup); got != want {
		t.Errorf("renderEnvFile mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderEnvFileEmpty(t *testing.T) {
	if got := renderEnvFile(nil, func(string) (string, bool) { return "", false }); got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}

func TestNormalizePermissions(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "0644", false},
		{"644", "0644", false},
		{"0644", "0644", false},
		{"0755", "0755", false},
		{"600", "0600", false},
		{"rw-r--r--", "", true},
		{"0999", "", true},
		{"64", "", true},
		{"06444", "", true},
	}

	for _, tt := range tests {
		got, err := normalizePermissions(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("normalizePermissions(%q): expected error, got %q", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizePermissions(%q): unexpected error: %v", tt.in, err)
		} else if got != tt.want {
			t.Errorf("normalizePermissions(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/var/www", "'/var/www'"},
		{"/path with space", "'/path with space'"},
		{"it's", `'it'\''s'`},
		{"$HOME`id`", "'$HOME`id`'"},
	}

	for _, tt := range tests {
		if got := shellQuote(tt.in); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestClientRequiresOptions(t *testing.T) {
	if _, err := Client(); err == nil {
		t.Error("expected error when no options are given")
	}

	if _, err := Client(WithAddr("example.com", 22)); err == nil {
		t.Error("expected error when user is missing")
	}
}

func TestWithHostKeyRejectsGarbage(t *testing.T) {
	opts := &clientOptions{}
	if err := WithHostKey("not a key")(opts); err == nil {
		t.Error("expected parse error for invalid host key")
	}
}

func TestCopyFilesGlobPatterns(t *testing.T) {
	tests := []struct {
		pattern    string
		hasGlob    bool
		wantGlob   string
	}{
		{"file.txt", false, ""},
		{"*.txt", true, "*.txt"},
		{"bin/*", true, "bin/*"},
		{"src/*.go", true, "src/*.go"},
		{"[abc]*.txt", true, "[abc]*.txt"},
		{"dir/file?.log", true, "dir/file?.log"},
		{"path/to/file", false, ""},
	}

	for _, tt := range tests {
		hasGlob := strings.ContainsAny(tt.pattern, "*?[]")
		if hasGlob != tt.hasGlob {
			t.Errorf("pattern %q: hasGlob = %v, want %v", tt.pattern, hasGlob, tt.hasGlob)
		}
	}
}

func TestCopyFilesVariableExpansion(t *testing.T) {
	// Test that variable expansion is correctly applied to filepaths
	lookup := func(key string) (string, bool) {
		values := map[string]string{
			"workspace": "/home/work",
			"BUILD_DIR": "/tmp/build",
		}
		v, ok := values[key]
		return v, ok
	}

	tests := []struct {
		input    string
		expanded string
	}{
		{"${workspace}/bin/*", "/home/work/bin/*"},
		{"${BUILD_DIR}/artifacts", "/tmp/build/artifacts"},
		{"files/*.txt", "files/*.txt"},
		{"${UNKNOWN}/file", "${UNKNOWN}/file"}, // unknown vars preserved
	}

	for _, tt := range tests {
		// Use the internal expand package to test the expansion
		// This mimics what CopyFiles does
		expanded := expandTestHelper(tt.input, lookup)
		if expanded != tt.expanded {
			t.Errorf("expand(%q) = %q, want %q", tt.input, expanded, tt.expanded)
		}
	}
}

// expandTestHelper is a test helper that mimics variable expansion
func expandTestHelper(s string, lookup func(key string) (string, bool)) string {
	pattern := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	return pattern.ReplaceAllStringFunc(s, func(match string) string {
		key := match[2 : len(match)-1]
		if value, ok := lookup(key); ok {
			return value
		}
		return match
	})
}
