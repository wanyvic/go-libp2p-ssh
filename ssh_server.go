package ssh

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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
	ServerConfig ssh.ServerConfig
}

//DefaultServerConfig use
//privateDirectory $HOME/.ssh/
//checkPasswd from /etc/shadow
func DefaultServerConfig() (config ssh.ServerConfig, err error) {
	var home string
	if home = os.Getenv("HOME"); home == "" {
		return config, errors.New("user not found")
	}
	config = ssh.ServerConfig{
		//Define a function to run when a client attempts a password login
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if err := checkPasswd(c.User(), pass); err != nil {
				// Should use constant-time compare (or better, salt+hash) in a production setting.
				return nil, err
			}
			return nil, nil
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			authorizedKeysBytes, err := ioutil.ReadFile(fmt.Sprintf("/home/%s/.ssh/authorized_keys", c.User()))
			if err != nil {
				log.Fatalf("Failed to load authorized_keys, err: %v", err)
				return nil, errors.New("Failed to load authorized_keys")
			}
			authorizedKeysMap := map[string]bool{}
			for len(authorizedKeysBytes) > 0 {
				pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
				if err != nil {
					log.Fatal(err)
				}
				authorizedKeysMap[string(pubKey.Marshal())] = true
				authorizedKeysBytes = rest
			}
			if authorizedKeysMap[string(pubKey.Marshal())] {
				return &ssh.Permissions{
					// Record the public key used for authentication.
					Extensions: map[string]string{
						"pubkey-fp": ssh.FingerprintSHA256(pubKey),
					},
				}, nil
			}
			return nil, fmt.Errorf("unknown public key for %q", c.User())
		},
		// You may also explicitly allow anonymous client authentication, though anon bash
		// sessions may not be a wise idea
		// NoClientAuth: true,
	}
	privateBytes, err := ioutil.ReadFile(home + "/.ssh/id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key ", err)
		return config, err
	}
	config, err = AddHostKey(&config, privateBytes)
	if err != nil {
		return config, err
	}
	return config, nil
}

//NewSSHService Create a Default ssh service
func NewSSHService(h host.Host) (*SSHService, error) {
	config, err := DefaultServerConfig()
	if err != nil {
		return nil, err
	}
	ss := &SSHService{config}
	h.SetStreamHandler(ID, ss.handler)
	return ss, nil
}

//NewSSHServiceWithConfig Create a ssh service with server config
func NewSSHServiceWithConfig(h host.Host, config ssh.ServerConfig) *SSHService {
	ss := &SSHService{config}
	h.SetStreamHandler(ID, ss.handler)
	return ss
}

//AddHostKey add ssh private key to Host(.ssh/id_rsa)
func AddHostKey(config *ssh.ServerConfig, privateBytes []byte) (ssh.ServerConfig, error) {
	// You can generate a keypair with 'ssh-keygen -t rsa'
	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key")
		return *config, err
	}
	config.AddHostKey(private)
	return *config, nil
}

func (ss *SSHService) handler(s network.Stream) {
	sshConn, chans, reqs, err := ssh.NewServerConn(s, &ss.ServerConfig)
	if err != nil {
		log.Error("Failed to handshake ", err)
		return
	}
	log.Info("New SSH connection from ", sshConn.RemoteMultiaddr(), sshConn.ClientVersion(), sshConn.User())
	// Discard all global out-of-band Requests
	go ssh.DiscardRequests(reqs)
	// Accept all channels
	handleChannels(chans, sshConn.User())
}
func handleChannels(chans <-chan ssh.NewChannel, user string) {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		go handleChannel(newChannel, user)
	}
}

func handleChannel(newChannel ssh.NewChannel, user string) {
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
		log.Errorf("Could not accept channel (%s)", err)
		return
	}
	var bash *exec.Cmd
	// Fire up bash for this session
	bash = exec.Command("login", "-f", user)

	// Prepare teardown function
	close := func() {
		connection.Close()
		_, err := bash.Process.Wait()
		if err != nil {
			log.Errorf("Failed to exit bash (%s)", err)
		}
	}

	// Allocate a terminal for this channel
	log.Info("Creating pty...")
	bashf, err := pty.Start(bash)
	if err != nil {
		log.Errorf("Could not start pty (%s)", err)
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
