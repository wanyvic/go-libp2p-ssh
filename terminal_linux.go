package ssh

import (
	"os/exec"
)

//setTerminalEcho cancel terminal echo
func setTerminalEcho(flag bool) {
	if flag {
		// disable input buffering
		exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
		// do not display entered characters on the screen
		exec.Command("stty", "-F", "/dev/tty", "-echo").Run()
	} else {
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
		// exec.Command("stty", "-F", "/dev/tty", "-cbreak", "min", "1024").Run()
	}
}
