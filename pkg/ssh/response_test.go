package ssh

import (
	"strings"
	"testing"
)

func TestCheckResponse(t *testing.T) {
	if err := checkResponse(strings.NewReader("\x00")); err != nil {
		t.Errorf("expected nil for Ok response, got %v", err)
	}

	err := checkResponse(strings.NewReader("\x01scp: permission denied\n"))
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected error containing remote message, got %v", err)
	}

	if err := checkResponse(strings.NewReader("")); err == nil {
		t.Error("expected error on empty reader")
	}
}
