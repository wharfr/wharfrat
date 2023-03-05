package proxy

import (
	"errors"
	"io"
	"log"
	"net/rpc"
	"os/exec"
	"syscall"

	"wharfr.at/wharfrat/lib/mux"
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
	mux  *mux.Mux
	stop chan struct{}
	bg   []func()
}

func New(cmd *exec.Cmd, m *mux.Mux) *Proxy {
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
		in, err := p.cmd.StdinPipe()
		if err != nil {
			return err
		}
		p.mux.Recv(args.ID, in)
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
		p.cmd.Stdout = p.mux.Send(args.ID)
		return nil
	case 2:
		p.cmd.Stderr = p.mux.Send(args.ID)
		return nil
	default:
		return errors.New("not implemented")
	}
}

func (p *Proxy) IO(args ProxyFDArgs, _ *ProxyNone) error {
	if p.running() {
		return errors.New("proxy running")
	}
	log.Printf("PROXY IO: id:%d fd:%d", args.ID, args.FD)
	return errors.New("not implemented")
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
