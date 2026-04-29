//go:build windows

package pty

type LocalAdapter struct{}

func NewLocalAdapter() Adapter {
	return LocalAdapter{}
}

func (LocalAdapter) Start(cols, rows int) (Session, error) {
	return nil, ErrLocalShellUnsupported
}
