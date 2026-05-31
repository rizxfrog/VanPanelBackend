package ssh

import (
	"context"
	"io"
	"strconv"
	"sync"

	treemodel "github.com/rizxfrog/VanPanelBackend/internal/model"
	terminalmodel "github.com/rizxfrog/VanPanelBackend/internal/terminal/model"
	terminalservice "github.com/rizxfrog/VanPanelBackend/internal/terminal/service"
	treeservice "github.com/rizxfrog/VanPanelBackend/internal/tree/service"
	pkgssh "github.com/rizxfrog/VanPanelBackend/pkg/ssh"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
)

type Adapter struct {
	treeLocal treeservice.TreeLocalService
	logger    *zap.Logger
}

func NewAdapter(treeLocal treeservice.TreeLocalService, logger *zap.Logger) *Adapter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Adapter{treeLocal: treeLocal, logger: logger}
}

func (a *Adapter) ListTargets(ctx context.Context) ([]terminalmodel.Target, error) {
	list, err := a.treeLocal.GetTreeLocalList(ctx, &treemodel.GetTreeLocalResourceListReq{ListReq: treemodel.ListReq{Page: 1, Size: 100}})
	if err != nil {
		return nil, err
	}
	targets := make([]terminalmodel.Target, 0, len(list.Items))
	for _, item := range list.Items {
		targets = append(targets, terminalmodel.Target{
			Type: terminalmodel.TargetTypeSSH,
			ID:   strconv.Itoa(item.ID),
			Name: item.Name,
		})
	}
	return targets, nil
}

func (a *Adapter) Start(ctx context.Context, targetID string, cols, rows int) (terminalservice.Stream, terminalmodel.Target, error) {
	id, err := strconv.Atoi(targetID)
	if err != nil {
		return nil, terminalmodel.Target{}, terminalmodel.ErrInvalidTarget
	}
	host, err := a.treeLocal.GetTreeLocalForConnection(ctx, &treemodel.GetTreeLocalResourceDetailReq{ID: id})
	if err != nil {
		return nil, terminalmodel.Target{}, err
	}
	client := pkgssh.NewClient(a.logger)
	if err := client.Connect(&pkgssh.Config{
		Host:     host.IpAddr,
		Port:     host.Port,
		Username: host.Username,
		Password: host.Password,
		Key:      host.Key,
		Mode:     pkgssh.AuthMode(host.AuthMode),
		Timeout:  10,
	}); err != nil {
		return nil, terminalmodel.Target{}, err
	}

	session, err := client.RawSession()
	if err != nil {
		_ = client.Close()
		return nil, terminalmodel.Target{}, err
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	modes := gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm", rows, cols, modes); err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, terminalmodel.Target{}, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, terminalmodel.Target{}, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, terminalmodel.Target{}, err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, terminalmodel.Target{}, err
	}
	if err := session.Shell(); err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, terminalmodel.Target{}, err
	}

	reader, writer := io.Pipe()
	stream := &sshStream{
		client:  client,
		session: session,
		stdin:   stdin,
		output:  reader,
		writer:  writer,
	}
	go stream.copyOutput(stdout)
	go stream.copyOutput(stderr)

	return stream, terminalmodel.Target{
		Type: terminalmodel.TargetTypeSSH,
		ID:   targetID,
		Name: host.Name,
	}, nil
}

type sshStream struct {
	client  pkgssh.Client
	session *gossh.Session
	stdin   io.WriteCloser
	output  *io.PipeReader
	writer  *io.PipeWriter
	once    sync.Once
}

func (s *sshStream) Read(p []byte) (int, error)  { return s.output.Read(p) }
func (s *sshStream) Write(p []byte) (int, error) { return s.stdin.Write(p) }
func (s *sshStream) Resize(cols, rows int) error { return s.session.WindowChange(rows, cols) }
func (s *sshStream) Wait() error                 { return s.session.Wait() }

func (s *sshStream) Close() error {
	var err error
	s.once.Do(func() {
		_ = s.writer.Close()
		_ = s.output.Close()
		_ = s.stdin.Close()
		_ = s.session.Close()
		err = s.client.Close()
	})
	return err
}

func (s *sshStream) copyOutput(src io.Reader) {
	_, _ = io.Copy(s.writer, src)
}

var _ terminalservice.SSHAdapter = (*Adapter)(nil)
