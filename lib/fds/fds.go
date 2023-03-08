package fds

import (
	"os"

	"golang.org/x/sys/unix"
)

func openInRange(s, e int, ignore map[uintptr]bool) ([]uintptr, error) {
	open := make([]uintptr, 0, 3)
	fds := make([]unix.PollFd, e-s)
	for i := range fds {
		fds[i].Fd = int32(s + i)
		fds[i].Events = unix.POLLIN | unix.POLLOUT
	}
	if _, err := unix.Poll(fds, 0); err != nil {
		return nil, err
	}
	for i := range fds {
		fd := uintptr(fds[i].Fd)
		if fds[i].Revents&unix.POLLNVAL == 0 && !ignore[fd] {
			open = append(open, fd)
		}
	}
	return open, nil
}

func extraOpen() ([]uintptr, error) {
	ignore := map[uintptr]bool{
		os.Stdin.Fd():  true,
		os.Stderr.Fd(): true,
		os.Stdout.Fd(): true,
	}
	open := make([]uintptr, 0, 3)
	for s, e := 0, 1000; e < 65536; s, e = e, e+1000 {
		//log.Printf("open: s:%d e:%d", s, e)
		r, err := openInRange(s, e, ignore)
		if err != nil {
			return nil, err
		}
		open = append(open, r...)
	}
	return open, nil
}

var (
	extra    []uintptr
	extraErr error
)

func ExtraOpen() ([]uintptr, error) {
	return extra, extraErr
}

func init() {
	extra, extraErr = extraOpen()
}
