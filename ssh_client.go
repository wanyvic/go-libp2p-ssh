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
	host         host.Host
	ClientConfig ssh.ClientConfig
	Stdout       io.Writer
	Stderr       io.Writer
	Stdin        io.Reader
}

//NewSSHClientWithConfig Create a new ssh client with config
func NewSSHClientWithConfig(h host.Host, config ssh.ClientConfig) *SSHClient {
	sc := &SSHClient{h, config, nil, nil, nil}
	return sc
}

//Connect connect to ssh server
func (sc *SSHClient) Connect(p peer.ID) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := sc.host.NewStream(ctx, p, ID)
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
	}
	defer session.Close()
	// excute command
	session.Stdout = sc.Stdout
	session.Stderr = sc.Stderr

	w, err := session.StdinPipe()
	if err != nil {
		return err
	}
	go io.Copy(w, sc.Stdin)

	// Notify os.Interrupt
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				signal.Stop(signalChan)
				return
			case <-signalChan:
				w.Write([]byte{3})
			}
		}
	}(ctx)
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
	// change window size
	go windowChange(ctx, session)

	// no buffering
	setTerminalEcho(true)
	defer setTerminalEcho(false)

	if err = session.Shell(); err != nil {
		return err
	}
	session.Wait()
	return nil
}

// windowChange change window size when the terminal resize
func windowChange(ctx context.Context, session *ssh.Session) {
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
