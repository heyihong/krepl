package command

import (
	"strings"
	"testing"
)

func TestNamespaceCommand_Set(t *testing.T) {
	env := makeTestEnv()
	cmd := newNamespaceCmd()

	out := captureStdout(t, func() {
		if err := cmd.Execute(env, []string{"my-ns"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if env.Namespace() != "my-ns" {
		t.Errorf("expected namespace %q, got %q", "my-ns", env.Namespace())
	}
	if !strings.Contains(out, "my-ns") {
		t.Errorf("expected confirmation output, got: %q", out)
	}
}

func TestNamespaceCommand_Clear(t *testing.T) {
	env := makeTestEnv()
	cmd := newNamespaceCmd()

	// Set first, then clear.
	_ = cmd.Execute(env, []string{"my-ns"})

	out := captureStdout(t, func() {
		if err := cmd.Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if env.Namespace() != "" {
		t.Errorf("expected namespace to be cleared, got %q", env.Namespace())
	}
	if !strings.Contains(out, "cleared") {
		t.Errorf("expected 'cleared' in output, got: %q", out)
	}
}

func TestNamespaceCommand_Invalid_Uppercase(t *testing.T) {
	env := makeTestEnv()
	cmd := newNamespaceCmd()
	err := cmd.Execute(env, []string{"MyNamespace"})
	if err == nil {
		t.Fatal("expected error for uppercase namespace name")
	}
	if env.Namespace() != "" {
		t.Errorf("namespace should not be set on invalid input, got %q", env.Namespace())
	}
}

func TestNamespaceCommand_Invalid_TooLong(t *testing.T) {
	env := makeTestEnv()
	cmd := newNamespaceCmd()
	long := strings.Repeat("a", 64)
	err := cmd.Execute(env, []string{long})
	if err == nil {
		t.Fatalf("expected error for %d-char namespace name", len(long))
	}
}

func TestNamespaceCommand_Invalid_LeadingHyphen(t *testing.T) {
	env := makeTestEnv()
	cmd := newNamespaceCmd()
	err := cmd.Execute(env, []string{"-bad"})
	if err == nil {
		t.Fatal("expected error for leading-hyphen namespace name")
	}
}

func TestNamespaceCommand_PromptUpdated(t *testing.T) {
	env := makeTestEnv()
	cmd := newNamespaceCmd()
	_ = captureStdout(t, func() {
		_ = cmd.Execute(env, []string{"kube-system"})
	})
	expected := "[ctx-a][kube-system][none] > "
	if env.Prompt() != expected {
		t.Errorf("expected prompt %q, got %q", expected, env.Prompt())
	}
}

func TestValidateRFC1123Label(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"default", false},
		{"my-namespace", false},
		{"ns123", false},
		{"a", false},
		{"", true},
		{"My-Namespace", true},
		{"-leading-hyphen", true},
		{"trailing-hyphen-", true},
		{"has spaces", true},
		{string(make([]byte, 64)), true},
		{"valid-63-" + string(make([]byte, 54)), true},
	}
	for _, tt := range tests {
		err := validateRFC1123Label(tt.input)
		if tt.wantErr && err == nil {
			t.Errorf("validateRFC1123Label(%q): expected error, got nil", tt.input)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("validateRFC1123Label(%q): unexpected error: %v", tt.input, err)
		}
	}
}
