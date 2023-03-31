package proxy

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/rpc"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/multio"
	"golang.org/x/sys/unix"
)

type Server struct {
	proxy  *Proxy
	server *rpc.Server
	conn   io.ReadWriteCloser
}

func NewServer(conn io.ReadWriteCloser, proxy *Proxy) (*Server, error) {
	server := rpc.NewServer()
	if err := server.Register(proxy); err != nil {
		return nil, err
	}
	return &Server{proxy: proxy, server: server, conn: conn}, nil
}

func (s *Server) Serve() {
	s.server.ServeConn(s.conn)
}

type ProxyFDArgs struct {
	ID uint32
	FD uintptr
}

type ProxySignalArgs struct {
	Signal int
}

type ProxyNone struct{}

type Proxy struct {
	cmd  *exec.Cmd
	mux  *multio.Multiplexer
	stop chan struct{}
	bg   []func()
}

func New(cmd *exec.Cmd, m *multio.Multiplexer) *Proxy {
	return &Proxy{cmd: cmd, mux: m, stop: make(chan struct{})}
}

func (p *Proxy) Wait() {
	<-p.stop
}

func (p *Proxy) running() bool {
	select {
	case <-p.stop:
		return true
	default:
		return false
	}
}

func (p *Proxy) Input(args ProxyFDArgs, _ *ProxyNone) error {
	if p.running() {
		return errors.New("proxy running")
	}
	log.Printf("PROXY INPUT: id:%d fd:%d", args.ID, args.FD)
	switch args.FD {
	case 0:
		p.cmd.Stdin = p.mux.NewReader(int(args.ID))
		return nil
	default:
		return errors.New("not implemented")
	}
}

func (p *Proxy) Output(args ProxyFDArgs, _ *ProxyNone) error {
	if p.running() {
		return errors.New("proxy running")
	}
	log.Printf("PROXY OUTPUT: id:%d fd:%d", args.ID, args.FD)
	switch args.FD {
	case 1:
		p.cmd.Stdout = p.mux.NewWriter(int(args.ID))
		return nil
	case 2:
		p.cmd.Stderr = p.mux.NewWriter(int(args.ID))
		return nil
	default:
		return errors.New("not implemented")
	}
}

func fdsToFile(fds [2]int) (a, b *os.File) {
	unix.CloseOnExec(fds[0])
	a = os.NewFile(uintptr(fds[0]), fmt.Sprintf("/dev/fd/%d", fds[0]))
	unix.CloseOnExec(fds[1])
	b = os.NewFile(uintptr(fds[1]), fmt.Sprintf("/dev/fd/%d", fds[1]))
	return
}

func (p *Proxy) IO(args ProxyFDArgs, _ *ProxyNone) error {
	if p.running() {
		return errors.New("proxy running")
	}
	log.Printf("PROXY IO: id:%d fd:%d", args.ID, args.FD)
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM, 0)
	if err != nil {
		return err
	}
	log.Printf("SOCKET PAIR: %v", fds)
	a, b := fdsToFile(fds)
	log.Printf("FILES: %p, %p", a, b)
	idx := int(args.FD - 3)
	if len(p.cmd.ExtraFiles) <= idx {
		p.cmd.ExtraFiles = append(p.cmd.ExtraFiles, make([]*os.File, idx+1-len(p.cmd.ExtraFiles))...)
	}
	p.cmd.ExtraFiles[idx] = a
	conn := p.mux.NewReadWriter(int(args.ID))
	go func() {
		io.Copy(b, conn)
		unix.Shutdown(int(b.Fd()), unix.SHUT_WR)
		conn.ReadCloser.Close()
	}()
	go func() {
		io.Copy(conn, b)
		unix.Shutdown(int(b.Fd()), unix.SHUT_RD)
		conn.WriteCloser.Close()
	}()
	return nil
}

func (p *Proxy) Start(ProxyNone, *ProxyNone) error {
	if p.running() {
		return errors.New("proxy running")
	}
	close(p.stop)
	return nil
}

func (p *Proxy) Signal(args ProxySignalArgs, _ *ProxyNone) error {
	if p.cmd.Process == nil {
		return errors.New("not running")
	}
	return p.cmd.Process.Signal(syscall.Signal(args.Signal))
}
