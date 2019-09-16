package ssh

import (
	"os/exec"
)

func setTerminalEcho(flag bool) {
	if flag {
		// disable input buffering
		exec.Command("stty", "-f", "/dev/tty", "cbreak", "min", "1").Run()
		// do not display entered characters on the screen
		exec.Command("stty", "-f", "/dev/tty", "-echo").Run()
	} else {
		exec.Command("stty", "-f", "/dev/tty", "echo").Run()
		// exec.Command("stty", "-f", "/dev/tty", "-cbreak", "min", "1024").Run()
	}
}
