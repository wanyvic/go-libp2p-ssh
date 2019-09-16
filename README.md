### go-libp2p-ssh
a implementation ssh client server for libp2p

## server
server will check password used by reading /etc/shadow, please running with sudo!
```go
package main

import (
    "context"
    "fmt"

    "github.com/libp2p/go-libp2p"
    lssh "github.com/wanyvic/go-libp2p-ssh"
)

func main() {
    listenAddrs := libp2p.ListenAddrStrings(
        "/ip4/0.0.0.0/tcp/9001",
    )
    host, err := libp2p.New(context.Background(), listenAddrs)
    if err != nil {
        fmt.Println(err)
    }
    fmt.Printf("Your PeerID is :%s\nListen:%s\n", host.ID().String(), host.Addrs())
    /*Your PeerID is :QmZ8zzzFhZAxWHzWecrj6x1r4UH9TnD35f6hBom3TbRGpu
Listen:[/ip4/127.0.0.1/tcp/9000 /ip4/192.168.3.131/tcp/9000 /ip4/192.168.0.133/tcp/9000 /ip4/172.17.0.1/tcp/9000]*/
    lssh.NewSSHService(host)
    select {}   //hold on
}
```

## client
```go
package main

import (
    "context"
    "fmt"
    "io/ioutil"
    "os"
    "time"

    "github.com/libp2p/go-libp2p"
    "github.com/libp2p/go-libp2p-core/peer"
    ma "github.com/multiformats/go-multiaddr"
    lssh "github.com/wanyvic/go-libp2p-ssh"
    "github.com/wanyvic/ssh"
)

func main() {
    host, err := libp2p.New(context.Background())
    if err != nil {
        fmt.Println(err)
    }
    fmt.Printf("Your PeerID is :%s\nListen:%s\n", host.ID().String(), host.Addrs())
    //pid: /ip4/127.0.0.1/tcp/9000/p2p/QmZ8zzzFhZAxWHzWecrj6x1r4UH9TnD35f6hBom3TbRGpu

    maddr, err := ma.NewMultiaddr(pid)
    if err != nil {
        fmt.Println(err)
    }
    peerinfo, _ := peer.AddrInfoFromP2pAddr(maddr)
    if err := host.Connect(context.Background(), *peerinfo); err != nil {
        fmt.Println(err)
    }

    //auth
    auth := make([]ssh.AuthMethod, 0)
    // password authentication
    auth = append(auth, ssh.Password("xxxx")) //your os password

    // public key authentication
    home := os.Getenv("HOME")

    privateBytes, err := ioutil.ReadFile(home + "/.ssh/id_rsa")
    if err != nil {
        fmt.Println(err)
    }
    Signer, err := ssh.ParsePrivateKey(privateBytes)
    if err != nil {
        fmt.Println(err)
    }
    auth = append(auth, ssh.PublicKeys(Signer))

    //create clientConfig
    clientConfig := &ssh.ClientConfig{
        User:    "wany", // username which you want to login with
        Auth:    auth,
        Timeout: 30 * time.Second,
        HostKeyCallback: func(hostname string, remote ma.Multiaddr, key ssh.PublicKey) error {
            return nil
        },
    }

    clients := lssh.NewSSHClientWithConfig(host, *clientConfig)

    //bind reader writer
    clients.Stdout = os.Stdout
    clients.Stderr = os.Stderr
    clients.Stdin = os.Stdin

    clients.Connect(peerinfo.ID)
}
```
more [GoDoc](https://godoc.org/github.com/wanyvic/go-libp2p-ssh#SetWinsize)