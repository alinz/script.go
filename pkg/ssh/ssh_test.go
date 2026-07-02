package ssh

import (
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
