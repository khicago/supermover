package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"help"}, &stdout, &stderr)

	if got != 0 {
		t.Errorf("Run(%v) exit = %d, want %d", []string{"help"}, got, 0)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("Run(%v) stdout = %q, want usage text", []string{"help"}, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("Run(%v) stderr = %q, want empty", []string{"help"}, stderr.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"missing"}, &stdout, &stderr)

	if got != 2 {
		t.Errorf("Run(%v) exit = %d, want %d", []string{"missing"}, got, 2)
	}
	if !strings.Contains(stderr.String(), `unknown command "missing"`) {
		t.Errorf("Run(%v) stderr = %q, want unknown command message", []string{"missing"}, stderr.String())
	}
}
