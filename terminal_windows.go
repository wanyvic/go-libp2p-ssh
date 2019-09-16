package ssh

import (
	"os"
	"syscall"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

var (
	dll            = syscall.MustLoadDLL("kernel32")
	setConsoleMode = dll.MustFindProc("SetConsoleMode")
	m              uint32
)

func init() {

	h := syscall.Handle(os.Stdin.Fd())
	if err := syscall.GetConsoleMode(h, &m); err != nil {
		logrus.Error(err)
	}
}

//getTerminalSize get terminal size
func getTerminalSize() (int, int, error) {
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err != nil {
		logrus.Error(err)
		return 0, 0, err
	} else {
		var info windows.ConsoleScreenBufferInfo
		if err := windows.GetConsoleScreenBufferInfo(h, &info); err != nil {
			logrus.Error(err)
			return 0, 0, err
		}
		width := info.Window.Right - info.Window.Left + 1
		height := info.Window.Bottom - info.Window.Top + 1
		return int(width), int(height), nil
	}
}

//setTerminalEcho cancel terminal echo
func setTerminalEcho(flag bool) {
	h := syscall.Handle(os.Stdin.Fd())
	if flag {
		if err := SetInputConsoleMode(h, 0); err != nil {
			logrus.Error(err)
		}
	} else {
		if err := SetInputConsoleMode(h, m); err != nil {
			logrus.Error(err)
		}
	}
}
