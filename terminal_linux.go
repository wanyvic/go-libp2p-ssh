package ssh

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/wanyvic/ssh/terminal"
)

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
func SetTerminalEcho(flag bool) {
	if flag {
		// disable input buffering
		// exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
		// do not display entered characters on the screen
		exec.Command("stty", "-F", "/dev/tty", "-echo").Run()
	} else {
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
		// exec.Command("stty", "-F", "/dev/tty", "-cbreak", "min", "1024").Run()
	}
}

func PrintMOTD(w io.Writer) {
	out, err := exec.Command("cat", "/run/motd.dynamic").Output()
	if err == nil {
		fmt.Println(string(out))
		w.Write(out)
	}
}
