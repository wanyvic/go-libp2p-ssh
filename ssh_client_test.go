package ssh

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-tcp-transport"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/wanyvic/ssh"
)

//go test -v -count=1 ssh_test.go ssh_client.go ssh_server.go terminal_linux.go
func Test(t *testing.T) {
	fmt.Println(len(flag.Args()))
	if len(flag.Args()) < 1 {
		t.Error("argument require nodeid")
		return
	}
	fmt.Println("client")
	home := os.Getenv("HOME")
	// get auth method
	auth := make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password("xxxx"))
	err := connect(flag.Args()[0], "ubuntu", auth)
	if err != nil {
		t.Error(err)
	}

	auth = make([]ssh.AuthMethod, 0)
	privateBytes, err := parsePrivateKey(home + "/.ssh/id_rsa")
	if err != nil {
		t.Error(err)
	}
	Signer, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		t.Error(err)
	}
	auth = append(auth, ssh.PublicKeys(Signer))
	err = connect(flag.Args()[0], "ubuntu", auth)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("client exit")
}
func connect(pid string, username string, auth []ssh.AuthMethod) error {
	transports := libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
	)
	listenAddrs := libp2p.ListenAddrStrings(
		"/ip4/0.0.0.0/tcp/9001",
	)
	host, err := libp2p.New(context.Background(), transports, listenAddrs, libp2p.NATPortMap())
	if err != nil {
		return err
	}

	fmt.Printf("Your PeerID is :%s\nListen:%s\n", host.ID().String(), host.Addrs())
	fmt.Println(pid)
	maddr, err := ma.NewMultiaddr(pid)
	if err != nil {
		return err
	}
	peerinfo, _ := peer.AddrInfoFromP2pAddr(maddr)
	if err := host.Connect(context.Background(), *peerinfo); err != nil {
		return err
	}

	clientConfig := &ssh.ClientConfig{
		User:    username,
		Auth:    auth,
		Timeout: 30 * time.Second,
		HostKeyCallback: func(hostname string, remote ma.Multiaddr, key ssh.PublicKey) error {
			return nil
		},
	}

	clients := NewSSHClient(host, *clientConfig)
	clients.Stdout = os.Stdout
	clients.Stderr = os.Stderr
	clients.Stdin = os.Stdin

	ctx, cancel := context.WithCancel(context.Background())
	clients.Connect(ctx, peerinfo.ID)
	defer cancel()
	return nil
}
func parsePrivateKey(keyPath string) (private []byte, _ error) {
	privateBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return private, err
	}
	logrus.Debug("SSHPrivateKey: ", string(privateBytes))

	return privateBytes, nil
}
