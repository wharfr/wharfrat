package docker

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"

	"github.com/creack/multio"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/moby/term"
	"golang.org/x/sys/unix"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/fds"
	"wharfr.at/wharfrat/lib/rpc/proxy"
)

func (c *Connection) ExecCmd2(id string, cmd []string, crate *config.Crate, user, workdir string) (ret int, retErr error) {
	defer func() {
		log.Printf("ExecCmd2: %d %v", ret, retErr)
	}()

	container, err := c.c.ContainerInspect(c.ctx, crate.ContainerName())
	if err != nil {
		return -1, err
	}

	cmds := []string(container.Config.Entrypoint)
	cmds = append(cmds, cmd...)

	log.Printf("CMD: %v", cmds)

	inFd, inTerm := term.GetFdInfo(os.Stdin)
	outFd, outTerm := term.GetFdInfo(os.Stdout)
	errFd, _ := term.GetFdInfo(os.Stderr)
	tty := inTerm && outTerm

	if user == "" {
		user = fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	}

	if workdir == "" {
		workdir, err = c.calcWorkdir(id, user, crate.WorkingDir, crate)
		if err != nil {
			return -1, fmt.Errorf("failed to set working directory: %w", err)
		}
	}

	log.Printf("User: %s, Workdir: %s, TTY: %v", user, workdir, tty)

	if config.Debug {
		cmds = append([]string{"/sbin/wr-init", "exec", "--debug"}, cmds...)
	} else {
		cmds = append([]string{"/sbin/wr-init", "exec"}, cmds...)
	}
	log.Printf("CMD: %s", cmds)

	env, err := buildEnv(id, crate)
	if err != nil {
		return 0, err
	}

	config := types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          cmds,
		Env:          env,
		User:         user,
		WorkingDir:   workdir,
	}

	resp, err := c.c.ContainerExecCreate(c.ctx, id, config)
	if err != nil {
		return -1, err
	}

	execID := resp.ID
	if execID == "" {
		return -1, fmt.Errorf("got empty exec ID")
	}

	log.Printf("EXEC: ID=%s", execID)

	startCheck := types.ExecStartCheck{
		Tty: config.Tty,
	}
	attach, err := c.c.ContainerExecAttach(c.ctx, execID, startCheck)
	if err != nil {
		return -1, fmt.Errorf("failed to exec container: %w", err)
	}
	defer func() {
		log.Printf("!! close attach !!")
		attach.Close()
		log.Printf("attach closed")
	}()

	// TODO: this is horrible and hacky, and needs a better approach
	pr, pw := io.Pipe()
	go func() {
		_, err := stdcopy.StdCopy(pw, os.Stderr, attach.Reader)
		pw.Close()
		if err != nil {
			// need to do something better with this error ...
			log.Printf("ERROR: StdCopy failed: %s", err)
		}
	}()

	log.Printf("CREATE MUX")
	// m := mux.New("client", pr, attach.Conn)
	m, err := multio.NewMultiplexer(pr, attach.Conn)
	if err != nil {
		return 0, fmt.Errorf("failed to create mux: %w", err)
	}
	defer func() {
		log.Printf("!! close mux !!")
		m.Close()
		log.Printf("MUX CLOSED")
	}()

	processCh := make(chan error, 1)
	// go func() {
	// 	log.Printf("START mux.Process")
	// 	processCh <- m.Process()
	// 	log.Printf("END mux.Process")
	// }()

	log.Printf("SETUP 0 & 1")
	go func() {
		r := m.NewReader(1)
		io.Copy(os.Stderr, r)
		r.Close()
		processCh <- nil
	}()
	// m.Recv(1, os.Stderr)
	ctrl := proxy.NewClient(m.NewReadWriter(0))

	log.Printf("SETUP stdin")
	if err := ctrl.Input(2, inFd); err != nil {
		log.Printf("FAILED TO SETUP stdin: %s", err)
		return -1, fmt.Errorf("failed to setup stdin: %w", err)
	}
	go func() {
		w := m.NewWriter(2)
		io.Copy(w, os.Stdin)
		w.Close()
	}()

	log.Printf("SETUP stdout")
	if err := ctrl.Output(3, outFd); err != nil {
		return -1, fmt.Errorf("failed to setup stdout: %w", err)
	}
	// m.Recv(3, rewrite(cmd[0], os.Stdout, id, crate))
	go func() {
		r := m.NewReader(3)
		io.Copy(rewrite(cmd[0], os.Stdout, id, crate), r)
		r.Close()
	}()

	log.Printf("SETUP stderr")
	if err := ctrl.Output(4, errFd); err != nil {
		return -1, fmt.Errorf("failed to setup stderr: %w", err)
	}
	// m.Recv(4, os.Stderr)
	go func() {
		r := m.NewReader(4)
		io.Copy(os.Stderr, r)
		r.Close()
	}()

	log.Printf("GET extra fds")
	extra, err := fds.ExtraOpen()
	if err != nil {
		return 0, fmt.Errorf("failed to setup extra fds: %w", err)
	}

	log.Printf("SETUP extra fds: %v", extra)
	for i, fd := range extra {
		id := uint32(5 + i)
		if err := ctrl.IO(id, fd); err != nil {
			return -1, fmt.Errorf("failed to create mux channel: %w", err)
		}
		f := os.NewFile(fd, fmt.Sprintf("/dev/fd/%d", fd))
		if f == nil {
			return -1, fmt.Errorf("failed to create file from FD: %d", fd)
		}
		defer func() {
			f.Close()
			log.Printf("FD %d CLOSED", fd)
		}()
		conn := m.NewReadWriter(int(id))
		go func() {
			_, err := io.Copy(conn, f)
			log.Printf("f -> conn: %s %T %T %v", err, err, errors.Unwrap(err), errors.Is(err, unix.EBADF))
			conn.WriteCloser.Close()
		}()
		go func() {
			_, err := io.Copy(f, conn)
			log.Printf("conn -> f: %s %T %T %v", err, err, errors.Unwrap(err), errors.Is(err, unix.EBADF))
			conn.ReadCloser.Close()
		}()
	}

	log.Printf("START process")
	if err := ctrl.Start(); err != nil {
		return -1, fmt.Errorf("failed to start process: %w", err)
	}

	// Start forwarding signals
	log.Printf("SETUP SIGNALS")
	sig := make(chan os.Signal, 10)
	signal.Notify(sig)
	go func() {
		log.Printf("START SIGNALS")
		for s := range sig {
			log.Printf("FORWARD SIGNAL: %s", s)
			if err := ctrl.Signal(s); err != nil {
				log.Printf("Error sending signal %d: %s", s, err)
			}
		}
		log.Printf("STOPPED SIGNALS")
	}()

	// Wait for m.Process to finish
	log.Printf("WAIT FOR PROCESS")
	if err := <-processCh; err != nil {
		log.Printf("PROCESS ERROR: %s", err)
		return -1, fmt.Errorf("error copying output: %w", err)
	}

	// Stop forwarding signals
	log.Printf("STOP SIGNALS")
	signal.Stop(sig)
	close(sig)

	log.Printf("EXEC INSPECT")
	inspect, err := c.c.ContainerExecInspect(c.ctx, execID)
	if err != nil {
		return -1, fmt.Errorf("failed to get exec response: %w", err)
	}
	log.Printf("running: %v exit: %d", inspect.Running, inspect.ExitCode)

	if inspect.Running {
		return -1, fmt.Errorf("command still running")
	}

	return inspect.ExitCode, nil
}
