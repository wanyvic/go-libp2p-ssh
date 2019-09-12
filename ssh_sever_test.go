package ssh

import (
	"context"
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-tcp-transport"
)

//go test -v -count=1 ssh_test.go ssh_client.go ssh_server.go terminal_linux.go
func Server_Test(t *testing.T) {
	fmt.Println("server")
	err := server()
	if err != nil {
		t.Error(err)
	}
	fmt.Println("server exit")
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
	config, err := DefaultServerConfig()
	if err != nil {
		return err
	}

	NewSSHService(host, config)
	fmt.Printf("Your PeerID is :%s\nListen:%s\n", host.ID().String(), host.Addrs())

	select {}
}
