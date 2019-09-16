package ssh

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/sha512_crypt"
	"golang.org/x/crypto/ssh/terminal"
)

//getTerminalSize get terminal size
func getTerminalSize() (int, int, error) {
	fd := int(0)
	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		fd = int(os.Stdin.Fd())
	} else {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return 0, 0, errors.New(err.Error() + "error allocating terminal")
		}
		defer tty.Close()
		fd = int(tty.Fd())
	}
	oldState, err := terminal.MakeRaw(fd)
	if err != nil {
		return 0, 0, err
	}
	defer terminal.Restore(fd, oldState)

	termWidth, termHeight, err := terminal.GetSize(fd)
	if err != nil {
		return 0, 0, err
	}
	return termWidth, termHeight, nil
}

//checkPasswd check password from /etc/shadow
func checkPasswd(user string, passwd []byte) error {
	var secretStr string
	var soltHash string
	shadow, err := ioutil.ReadFile("/etc/shadow")
	if err != nil {
		return err
	}
	lines := strings.Split(string(shadow), "\n")
	for _, line := range lines {
		if strings.Index(line, user) == 0 {
			userStr := strings.Split(line, ":")
			secretStr = userStr[1]
			soltHash = secretStr[:strings.LastIndex(secretStr, "$")]
			break
		}
	}
	if secretStr == "" || soltHash == "" {
		return errors.New("user not found")
	}
	crypt := crypt.SHA512.New()
	ret, err := crypt.Generate(passwd, []byte(soltHash))
	if err != nil {
		return err
	}
	if ret != secretStr {
		return errors.New("mismatch error")
	}
	return nil
}
