package ui

import (
	"context"
	"os"
	"syscall"
	"unsafe"
)

func WatchEscape(ctx context.Context, onEscape func()) func() {
	file, err := os.Open("/dev/tty")
	if err != nil {
		return func() {}
	}
	fd := int(file.Fd())
	state, err := makeRaw(fd)
	if err != nil {
		_ = file.Close()
		return func() {}
	}
	done := make(chan struct{})
	stop := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1)
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			default:
			}
			n, err := file.Read(buf)
			if err != nil {
				return
			}
			if n == 0 {
				continue
			}
			if buf[0] == 27 {
				onEscape()
				return
			}
		}
	}()
	return func() {
		close(stop)
		restore(fd, state)
		_ = file.Close()
		<-done
	}
}

func makeRaw(fd int) (*syscall.Termios, error) {
	state, err := getTermios(fd)
	if err != nil {
		return nil, err
	}
	raw := *state
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	raw.Cc[syscall.VMIN] = 0
	raw.Cc[syscall.VTIME] = 1
	if err := setTermios(fd, &raw); err != nil {
		return nil, err
	}
	return state, nil
}

func restore(fd int, state *syscall.Termios) {
	if state != nil {
		_ = setTermios(fd, state)
	}
}

func getTermios(fd int) (*syscall.Termios, error) {
	termios := &syscall.Termios{}
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(termios)), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}
	return termios, nil
}

func setTermios(fd int, state *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(state)), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
