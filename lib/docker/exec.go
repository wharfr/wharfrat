package docker

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"wharfr.at/wharfrat/lib/config"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
)

// buildEnv constructs the environment that we want to use inside the container.
// It combines values from the crate config, the local config and the current
// host environment to build the desired initial container environment. A
// blacklist is applies to the host environment to prevent the container
// environment from picking up settings that don't make sense (e.g. taking PATH
// into the container).
func buildEnv(id string, crate *config.Crate) []string {
	env := []string{
		"WHARFRAT_ID=" + id,
		"WHARFRAT_NAME=" + crate.ContainerName(),
		"WHARFRAT_CRATE=" + crate.Name(),
		"WHARFRAT_PROJECT=" + crate.ProjectPath(),
	}

	log.Printf("CRATE ENV: %v", crate.Env)
	for name, value := range crate.Env {
		switch name {
		case "WHARFRAT_ID", "WHARFRAT_NAME", "WHARFRAT_CRATE", "WHARFRAT_PROJECT":
			log.Printf("Ignoring attempt to change %s", name)
		default:
			env = append(env, name+"="+value)
		}
	}

	local := config.Local()
	log.Printf("LOCAL ENV: %v", local.Env)
	for name, value := range local.Env {
		switch name {
		case "WHARFRAT_ID", "WHARFRAT_NAME", "WHARFRAT_CRATE", "WHARFRAT_PROJECT":
			log.Printf("Ignoring attempt to change %s", name)
		default:
			env = append(env, name+"="+value)
		}
	}

	blacklist := map[string]bool{
		"HOSTNAME": true,
		"PATH":     true,
		"SHELL":    true,
		"HOST":     true,
		"USER":     true,
		"HOME":     true,
		"PS0":      true,
		"PS1":      true,
		"PS2":      true,
		"PS3":      true,
		"PS4":      true,
	}

	for _, name := range crate.EnvWhitelist {
		blacklist[name] = false
	}

	for _, name := range crate.EnvBlacklist {
		blacklist[name] = true
	}

	for _, entry := range os.Environ() {
		if parts := strings.SplitN(entry, "=", 2); !blacklist[parts[0]] {
			env = append(env, entry)
		}
	}

	return env
}

func (c *Connection) ExecCmd(id string, cmd []string, crate *config.Crate, user, workdir string) (int, error) {
	container, err := c.c.ContainerInspect(c.ctx, crate.ContainerName())
	if err != nil {
		return -1, err
	}

	cmds := []string(container.Config.Entrypoint)
	cmds = append(cmds, cmd...)

	log.Printf("CMD: %v", cmds)

	inFd, inTerm := term.GetFdInfo(os.Stdin)
	outFd, outTerm := term.GetFdInfo(os.Stdout)
	tty := inTerm && outTerm

	if user == "" {
		user = fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	}

	if workdir == "" {
		workdir, err = c.calcWorkdir(id, user, crate.WorkingDir, crate)
		if err != nil {
			return -1, fmt.Errorf("Failed to set working directory: %s", err)
		}
	}

	log.Printf("User: %s, Workdir: %s", user, workdir)

	oldAPI := versions.LessThan(c.c.ClientVersion(), "1.35")
	if oldAPI || tty || len(crate.Groups) > 0 {
		proxy := []string{"/sbin/wr-init", "proxy"}
		if tty {
			proxy = append(proxy, "--sync")
		}
		if oldAPI {
			proxy = append(proxy, "--workdir", workdir)
		}
		for _, group := range crate.Groups {
			proxy = append(proxy, "--group", group)
		}
		log.Printf("USE PROXY (workdir / terminal sync workaround): %s", proxy)
		cmds = append(proxy, cmds...)
	}

	config := types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          tty,
		Cmd:          cmds,
		Env:          buildEnv(id, crate),
		User:         user,
		WorkingDir:   workdir,
	}

	resp, err := c.c.ContainerExecCreate(c.ctx, id, config)
	if err != nil {
		return -1, err
	}

	execID := resp.ID
	if execID == "" {
		return -1, fmt.Errorf("Got empty exec ID")
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

	outChan := make(chan error)

	if config.Tty {
		resizeTty := func() error {
			size, err := term.GetWinsize(inFd)
			log.Printf("Resize: size=%v err=%s id=%s", size, err, execID)
			if err != nil {
				return err
			}
			err = c.c.ContainerExecResize(c.ctx, execID, types.ResizeOptions{
				Height: uint(size.Height),
				Width:  uint(size.Width),
			})
			log.Printf("Resize result: %s", err)
			return err
		}

		log.Printf("WAIT FOR PROXY READY ...")
		cmd := []byte{}
		for {
			buf := make([]byte, 1)
			n, err := attach.Reader.Read(buf)
			if err != nil {
				return -1, err
			}
			if buf[0] == '\n' {
				break
			}
			cmd = append(cmd, buf[:n]...)
		}
		log.Printf("READ: %s\n", cmd)

		log.Printf("Initial Resize")
		for resizeTty() != nil {
		}

		go func() {
			sigchan := make(chan os.Signal, 1)
			signal.Notify(sigchan, syscall.SIGWINCH)
			for range sigchan {
				resizeTty()
			}
		}()

		inState, err := term.SetRawTerminal(inFd)
		if err != nil {
			return -1, fmt.Errorf("Failed to set raw terminal mode: %s", err)
		}
		defer term.RestoreTerminal(inFd, inState)

		outState, err := term.SetRawTerminal(outFd)
		if err != nil {
			return -1, fmt.Errorf("Failed to set raw terminal mode: %s", err)
		}
		defer term.RestoreTerminal(outFd, outState)

		attach.Conn.Write([]byte("PROXY RUN\n"))

		go func() {
			_, err := io.Copy(os.Stdout, attach.Reader)
			outChan <- err
		}()
	} else {
		go func() {
			_, err := stdcopy.StdCopy(os.Stdout, os.Stderr, attach.Reader)
			log.Printf("Copy done")
			outChan <- err
		}()
	}

	go func() {
		io.Copy(attach.Conn, os.Stdin)
		attach.CloseWrite()
	}()

	// Wait for copies to finish
	if err = <-outChan; err != nil {
		return -1, fmt.Errorf("Error copying output: %s", err)
	}

	inspect, err := c.c.ContainerExecInspect(c.ctx, execID)
	if err != nil {
		return -1, fmt.Errorf("Failed to get exec response: %s", err)
	}

	if inspect.Running {
		return -1, fmt.Errorf("Command still running!")
	}

	return inspect.ExitCode, nil
}
