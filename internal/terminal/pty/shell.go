package pty

import (
	"errors"
	"io"
)

var ErrLocalShellUnsupported = errors.New("local shell is unsupported on this operating system")
var ErrShellNotFound = errors.New("local shell executable not found")

type Session interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
	Wait() error
}

type Adapter interface {
	Start(cols, rows int) (Session, error)
}

func resolveShell(envShell string, exists func(string) bool) string {
	if envShell != "" && exists(envShell) {
		return envShell
	}
	for _, candidate := range []string{"/bin/bash", "/bin/sh"} {
		if exists(candidate) {
			return candidate
		}
	}
	return ""
}
