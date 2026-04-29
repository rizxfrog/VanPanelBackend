//go:build !windows

package pty

import (
	"io"
	"os"
	"os/exec"

	creackpty "github.com/creack/pty/v2"
)

type LocalAdapter struct{}

func NewLocalAdapter() Adapter {
	return LocalAdapter{}
}

func (LocalAdapter) Start(cols, rows int) (Session, error) {
	shell := resolveShell(os.Getenv("SHELL"), func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && !info.IsDir()
	})
	if shell == "" {
		return nil, ErrShellNotFound
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	cmd := exec.Command(shell)
	file, err := creackpty.StartWithSize(cmd, &creackpty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	return &localSession{cmd: cmd, file: file}, nil
}

type localSession struct {
	cmd  *exec.Cmd
	file *os.File
}

func (s *localSession) Read(p []byte) (int, error)  { return s.file.Read(p) }
func (s *localSession) Write(p []byte) (int, error) { return s.file.Write(p) }
func (s *localSession) Close() error                { return s.file.Close() }
func (s *localSession) Wait() error                 { return s.cmd.Wait() }

func (s *localSession) Resize(cols, rows int) error {
	return creackpty.Setsize(s.file, &creackpty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

var _ io.ReadWriteCloser = (*localSession)(nil)
