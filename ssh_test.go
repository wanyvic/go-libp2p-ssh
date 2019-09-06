package ssh

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-tcp-transport"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/wanyvic/ssh"
)

//go test -v -count=1 ssh_test.go ssh_client.go ssh_server.go terminal_linux.go
func Test(t *testing.T) {
	fmt.Println(len(os.Args))
	if len(os.Args) > 3 {
		fmt.Println("client")
		err := client(os.Args[3])
		if err != nil {
			t.Error(err)
		}
	} else {
		err := server()
		if err != nil {
			t.Error(err)
		}
	}
	fmt.Println("test exit")
}
func client(pid string) error {
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

	// get auth method
	auth := make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password("0815"))

	clientConfig := &ssh.ClientConfig{
		User:    "wany",
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
	err = clients.Connect(context.Background(), peerinfo.ID)
	if err != nil {
		return err
	}
	fmt.Println("exit")
	return nil
}
func server() error {
	transports := libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
	)
	listenAddrs := libp2p.ListenAddrStrings(
		"/ip4/0.0.0.0/tcp/9000",
	)

	host, err := libp2p.New(context.Background(), transports, listenAddrs, libp2p.NATPortMap())
	if err != nil {
		return err
	}
	config := &ssh.ServerConfig{
		//Define a function to run when a client attempts a password login
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// Should use constant-time compare (or better, salt+hash) in a production setting.
			return nil, nil
		},
		// You may also explicitly allow anonymous client authentication, though anon bash
		// sessions may not be a wise idea
		// NoClientAuth: true,
	}
	config, err = AddHostKey(config, "/home/wany/.ssh/id_rsa")

	if err != nil {
		return err
	}

	NewSSHService(host, *config)
	fmt.Printf("Your PeerID is :%s\nListen:%s\n", host.ID().String(), host.Addrs())

	select {}
	return nil
}
