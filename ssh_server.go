package ssh

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	logging "github.com/ipfs/go-log"
	"github.com/kr/pty"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/wanyvic/ssh"
)

var log = logging.Logger("ssh")

const ID = "/ssh/1.0.0"

type SSHService struct {
	Host         host.Host
	ServerConfig ssh.ServerConfig
}

func NewSSHService(h host.Host, config ssh.ServerConfig) *SSHService {
	ss := &SSHService{h, config}
	h.SetStreamHandler(ID, ss.SSHandler)
	return ss
}

func (ss *SSHService) SSHandler(s network.Stream) {
	sshConn, chans, reqs, err := ssh.NewServerConn(s, &ss.ServerConfig)
	if err != nil {
		log.Error("Failed to handshake ", err)
	}
	log.Debug("New SSH connection from ", sshConn.RemoteMultiaddr(), sshConn.ClientVersion())
	// Discard all global out-of-band Requests
	go ssh.DiscardRequests(reqs)
	// Accept all channels
	handleChannels(chans)
	fmt.Println("exit")
	// for newChannel := range chans {
	// 	// Channels have a type, depending on the application level
	// 	// protocol intended. In the case of a shell, the type is
	// 	// "session" and ServerShell may be used to present a simple
	// 	// terminal interface.
	// 	if newChannel.ChannelType() != "session" {
	// 		newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
	// 		continue
	// 	}
	// 	channel, requests, err := newChannel.Accept()
	// 	if err != nil {
	// 		log.Fatalf("Could not accept channel: %v", err)
	// 	}
	// 	// Sessions have out-of-band requests such as "shell",
	// 	// "pty-req" and "env".  Here we handle only the
	// 	// "shell" request.
	// 	go func(in <-chan *ssh.Request) {
	// 		for req := range in {
	// 			req.Reply(req.Type == "shell", nil)
	// 		}
	// 	}(requests)
	// 	term := terminal.NewTerminal(channel, "> ")
	// 	go func() {
	// 		defer channel.Close()
	// 		for {
	// 			line, err := term.ReadLine()
	// 			if err != nil {
	// 				break
	// 			}
	// 			fmt.Println(line)
	// 		}
	// 	}()
	// }
}
func AddHostKey(config *ssh.ServerConfig, keyPath string) (*ssh.ServerConfig, error) {
	// You can generate a keypair with 'ssh-keygen -t rsa'
	privateBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		log.Fatal("Failed to load private key ", err)
		return config, err
	}
	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key")
		return config, err
	}
	config.AddHostKey(private)
	return config, nil
}
func handleChannels(chans <-chan ssh.NewChannel) {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		go handleChannel(newChannel)
	}
}

func handleChannel(newChannel ssh.NewChannel) {
	// Since we're handling a shell, we expect a
	// channel type of "session". The also describes
	// "x11", "direct-tcpip" and "forwarded-tcpip"
	// channel types.
	if t := newChannel.ChannelType(); t != "session" {
		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
		return
	}

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		fmt.Printf("Could not accept channel (%s)", err)
		return
	}

	// Fire up bash for this session
	bash := exec.Command("bash")

	// Prepare teardown function
	close := func() {
		connection.Close()
		_, err := bash.Process.Wait()
		if err != nil {
			fmt.Printf("Failed to exit bash (%s)", err)
		}
		fmt.Printf("Session closed")
	}

	// Allocate a terminal for this channel
	fmt.Println("Creating pty...")
	bashf, err := pty.Start(bash)
	if err != nil {
		fmt.Printf("Could not start pty (%s)", err)
		close()
		return
	}

	//pipe session to bash and visa-versa
	var once sync.Once
	go func() {
		io.Copy(connection, bashf)
		once.Do(close)
	}()
	go func() {
		io.Copy(bashf, connection)
		once.Do(close)
	}()

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	go func() {
		for req := range requests {
			switch req.Type {
			case "shell":
				// We only accept the default shell
				// (i.e. no command in the Payload)
				if len(req.Payload) == 0 {
					req.Reply(true, nil)
				}
				PrintMOTD(connection)
			case "pty-req":
				termLen := req.Payload[3]
				w, h := parseDims(req.Payload[termLen+4:])
				SetWinsize(bashf.Fd(), w, h)
				// Responding true (OK) here will let the client
				// know we have a pty ready for input
				req.Reply(true, nil)
			case "window-change":
				w, h := parseDims(req.Payload)
				SetWinsize(bashf.Fd(), w, h)
			}
		}
	}()
}

// parseDims extracts terminal dimensions (width x height) from the provided buffer.
func parseDims(b []byte) (uint32, uint32) {
	w := binary.BigEndian.Uint32(b)
	h := binary.BigEndian.Uint32(b[4:])
	return w, h
}

// Winsize stores the Height and Width of a terminal.
type Winsize struct {
	Height uint16
	Width  uint16
	x      uint16 // unused
	y      uint16 // unused
}

// SetWinsize sets the size of the given pty.
func SetWinsize(fd uintptr, w, h uint32) {
	ws := &Winsize{Width: uint16(w), Height: uint16(h)}
	syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}
