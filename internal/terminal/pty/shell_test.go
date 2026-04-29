package pty

import "testing"

func TestResolveShellPrefersEnvShell(t *testing.T) {
	got := resolveShell("/custom/sh", func(path string) bool {
		return path == "/custom/sh"
	})
	if got != "/custom/sh" {
		t.Fatalf("resolveShell = %q", got)
	}
}

func TestResolveShellFallsBackToBash(t *testing.T) {
	got := resolveShell("", func(path string) bool {
		return path == "/bin/bash"
	})
	if got != "/bin/bash" {
		t.Fatalf("resolveShell = %q", got)
	}
}

func TestResolveShellFallsBackToSh(t *testing.T) {
	got := resolveShell("", func(path string) bool {
		return path == "/bin/sh"
	})
	if got != "/bin/sh" {
		t.Fatalf("resolveShell = %q", got)
	}
}
