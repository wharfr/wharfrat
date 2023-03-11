package docker

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/moby/term"
	"golang.org/x/sys/unix"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/fds"
	"wharfr.at/wharfrat/lib/mux"
	"wharfr.at/wharfrat/lib/rpc/proxy"
)

func (c *Connection) ExecCmd2(id string, cmd []string, crate *config.Crate, user, workdir string) (int, error) {
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

	cmds = append([]string{"/sbin/wr-init", "exec"}, cmds...)

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
		return -1, err
	}
	defer attach.Close()

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
	m := mux.New("client", pr, attach.Conn)
	defer m.Close()

	log.Printf("SETUP 0 & 1")
	m.Recv(1, os.Stderr)
	ctrl := proxy.NewClient(m.Connect(0))

	processCh := make(chan error, 1)
	go func() {
		log.Printf("START mux.Process")
		processCh <- m.Process()
		log.Printf("END mux.Process")
	}()

	log.Printf("SETUP stdin")
	if err := ctrl.Input(2, inFd); err != nil {
		return -1, err
	}
	go func() {
		w := m.Send(2)
		io.Copy(w, os.Stdin)
		w.Close()
	}()

	log.Printf("SETUP stdout")
	if err := ctrl.Output(3, outFd); err != nil {
		return -1, err
	}
	m.Recv(3, rewrite(cmd[0], os.Stdout, id, crate))

	log.Printf("SETUP stderr")
	if err := ctrl.Output(4, errFd); err != nil {
		return -1, err
	}
	m.Recv(4, os.Stderr)

	log.Printf("GET extra fds")
	extra, err := fds.ExtraOpen()
	if err != nil {
		return 0, err
	}

	log.Printf("SETUP extra fds: %v", extra)
	for i, fd := range extra {
		id := uint32(5 + i)
		if err := ctrl.IO(id, fd); err != nil {
			return -1, err
		}
		f := os.NewFile(fd, fmt.Sprintf("/dev/fd/%d", fd))
		if f == nil {
			return -1, fmt.Errorf("failed to create file from FD: %d", fd)
		}
		defer f.Close()
		conn := m.Connect(id)
		go func() {
			_, err := io.Copy(conn, f)
			log.Printf("f -> conn: %s %T %T %v", err, err, errors.Unwrap(err), errors.Is(err, unix.EBADF))
			conn.CloseWrite()
		}()
		go func() {
			_, err := io.Copy(f, conn)
			log.Printf("conn -> f: %s %T %T %v", err, err, errors.Unwrap(err), errors.Is(err, unix.EBADF))
			conn.CloseRead()
		}()
	}

	log.Printf("START process")
	if err := ctrl.Start(); err != nil {
		return -1, err
	}
	m.Recv(4, os.Stderr)

	// Start forwarding signals
	sig := make(chan os.Signal, 10)
	signal.Notify(sig)
	go func() {
		for s := range sig {
			if err := ctrl.Signal(s); err != nil {
				log.Printf("Error sending signal: %s", err)
			}
		}
	}()

	// Wait for m.Process to finish
	if err := <-processCh; err != nil {
		return -1, fmt.Errorf("error copying output: %w", err)
	}

	// Stop forwarding signals
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
