package ssh

import (
	"context"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/wanyvic/ssh"
)

type SSHClient struct {
	Host         host.Host
	ClientConfig ssh.ClientConfig
	Stdout       io.Writer
	Stderr       io.Writer
	Stdin        io.Reader
}

func NewSSHClient(h host.Host, config ssh.ClientConfig) *SSHClient {
	ss := &SSHClient{h, config, nil, nil, nil}
	return ss
}
func (sc *SSHClient) Connect(ctx context.Context, p peer.ID) error {
	stream, err := sc.Host.NewStream(ctx, p, ID)
	if err != nil {
		return err
	}
	defer stream.Close()
	c, chans, reqs, err := ssh.NewClientConn(stream, p.String(), &sc.ClientConfig)
	if err != nil {
		return err
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()
	// create session
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	// excute command
	session.Stdout = sc.Stdout
	session.Stderr = sc.Stderr
	session.Stdin = sc.Stdin

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for _ = range signalChan {
			os.Stdin.Write([]byte{3, '\n'})
		}
	}()
	// Set up terminal width, height
	width, height, err := getTerminalSize()
	if err != nil {
		return err
	}

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // enable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	// Request pseudo terminal
	if err := session.RequestPty("xterm-256color", height, width, modes); err != nil {
		return err

	}
	ctx, cancel := context.WithCancel(context.Background())
	go WindowChange(ctx, session)
	defer cancel()
	SetTerminalEcho(true)
	defer SetTerminalEcho(false)
	if err = session.Shell(); err != nil {
		return err
	}
	session.Wait()
	return nil
}
func WindowChange(ctx context.Context, session *ssh.Session) {
	width, height, err := getTerminalSize()
	if err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Millisecond):
			w, h, _ := getTerminalSize()
			if w != width || h != height {
				session.WindowChange(h, w)
				width = w
				height = h
			}
		}
	}
}
