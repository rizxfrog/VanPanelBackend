package service

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	terminalmodel "github.com/GoSimplicity/AI-CloudOps/internal/terminal/model"
	"github.com/GoSimplicity/AI-CloudOps/internal/terminal/pty"
	"go.uber.org/zap"
)

type fakeLocalAdapter struct {
	stream pty.Session
	err    error
}

func (f fakeLocalAdapter) Start(cols, rows int) (pty.Session, error) {
	return f.stream, f.err
}

type fakeStream struct {
	closed bool
}

type failingSSHAdapter struct{}

func (failingSSHAdapter) ListTargets(context.Context) ([]terminalmodel.Target, error) {
	return nil, errors.New("database unavailable")
}

func (failingSSHAdapter) Start(context.Context, string, int, int) (Stream, terminalmodel.Target, error) {
	return nil, terminalmodel.Target{}, errors.New("not implemented")
}

func (f *fakeStream) Read(_ []byte) (int, error)  { return 0, io.EOF }
func (f *fakeStream) Write(p []byte) (int, error) { return len(p), nil }
func (f *fakeStream) Close() error {
	f.closed = true
	return nil
}
func (f *fakeStream) Resize(cols, rows int) error { return nil }
func (f *fakeStream) Wait() error                 { return nil }

func TestListTargetsIncludesLocal(t *testing.T) {
	svc := NewTerminalService(zap.NewNop(), nil, nil, Config{IdleTimeout: time.Minute})
	targets, err := svc.ListTargets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) == 0 || targets[0].Type != terminalmodel.TargetTypeLocal {
		t.Fatalf("targets = %+v", targets)
	}
}

func TestListTargetsKeepsLocalWhenSSHListFails(t *testing.T) {
	svc := NewTerminalService(zap.NewNop(), nil, failingSSHAdapter{}, Config{IdleTimeout: time.Minute})
	targets, err := svc.ListTargets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Type != terminalmodel.TargetTypeLocal {
		t.Fatalf("targets = %+v, want local fallback", targets)
	}
}

func TestStartRejectsUnknownTargetType(t *testing.T) {
	svc := NewTerminalService(zap.NewNop(), nil, nil, Config{IdleTimeout: time.Minute})
	_, err := svc.Start(context.Background(), StartRequest{
		UserID:     1,
		Username:   "admin",
		SourceIP:   "127.0.0.1",
		TargetType: "bad",
		TargetID:   "bad",
		Cols:       80,
		Rows:       24,
	})
	if !errors.Is(err, terminalmodel.ErrInvalidTarget) {
		t.Fatalf("Start error = %v, want ErrInvalidTarget", err)
	}
}

func TestStartAndCloseLocalSession(t *testing.T) {
	stream := &fakeStream{}
	svc := NewTerminalService(zap.NewNop(), fakeLocalAdapter{stream: stream}, nil, Config{IdleTimeout: time.Minute})
	session, err := svc.Start(context.Background(), StartRequest{
		UserID:     1,
		Username:   "admin",
		SourceIP:   "127.0.0.1",
		TargetType: terminalmodel.TargetTypeLocal,
		TargetID:   terminalmodel.LocalTargetID,
		Cols:       80,
		Rows:       24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.ID == "" || session.TargetName == "" {
		t.Fatalf("session = %+v", session)
	}
	if _, ok := svc.Get(session.ID); !ok {
		t.Fatal("session not stored")
	}
	if err := svc.Close(session.ID, "test_closed"); err != nil {
		t.Fatal(err)
	}
	if !stream.closed {
		t.Fatal("stream was not closed")
	}
}
