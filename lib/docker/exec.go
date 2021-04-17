package docker

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/output"

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
func buildEnv(id string, crate *config.Crate) ([]string, error) {
	env := []string{
		"WHARFRAT_ID=" + id,
		"WHARFRAT_NAME=" + crate.ContainerName(),
		"WHARFRAT_CRATE=" + crate.Name(),
		"WHARFRAT_PROJECT=" + crate.ProjectPath(),
		"WHARFRAT_PROJECT_DIR=" + filepath.Dir(crate.ProjectPath()),
	}

	log.Printf("CRATE ENV: %v", crate.Env)
	for name, value := range crate.Env {
		switch name {
		case "WHARFRAT_ID", "WHARFRAT_NAME", "WHARFRAT_CRATE", "WHARFRAT_PROJECT", "WHARFRAT_PROJECT_DIR":
			log.Printf("Ignoring attempt to change %s", name)
		default:
			env = append(env, name+"="+value)
		}
	}

	locals, err := config.Local().Setup(crate)
	if err != nil {
		return nil, err
	}

	for _, local := range locals {
		log.Printf("LOCAL ENV: %v", local.Env)
		for name, value := range local.Env {
			switch name {
			case "WHARFRAT_ID", "WHARFRAT_NAME", "WHARFRAT_CRATE", "WHARFRAT_PROJECT", "WHARFRAT_PROJECT_DIR":
				log.Printf("Ignoring attempt to change %s", name)
			default:
				env = append(env, name+"="+value)
			}
		}
	}

	blacklist := map[string]bool{
		// Blacklist basic environment setup that shouldn't be inherited
		"HOSTNAME": true,
		"PATH":     true,
		"SHELL":    true,
		"HOST":     true,
		"USER":     true,
		"HOME":     true,

		// Blacklist prompt variables
		"PS0": true,
		"PS1": true,
		"PS2": true,
		"PS3": true,
		"PS4": true,

		// Blacklist VTE variables
		"VTE_VERSION": true,

		// Blacklist Konsole DBUS variables
		"KONSOLE_DBUS_SESSION": true,
		"KONSOLE_DBUS_WINDOW":  true,
		"KONSOLE_DBUS_SERVICE": true,

		// Blacklist desktop session variables
		"DBUS_SESSION_BUS_ADDRESS": true,
		"XDG_SESSION_PATH":         true,
		"KDE_FULL_SESSION":         true,
		"SESSION_MANAGER":          true,
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

	return env, nil
}

func wrGetenv(id string, crate *config.Crate) func(string) string {
	return func(key string) string {
		if strings.HasPrefix(key, "WHARFRAT_") {
			switch key {
			case "WHARFRAT_ID":
				return id
			case "WHARFRAT_NAME":
				return crate.ContainerName()
			case "WHARFRAT_CRATE":
				return crate.Name()
			case "WHARFRAT_PROJECT":
				return crate.ProjectPath()
			case "WHARFRAT_PROJECT_DIR":
				return filepath.Dir(crate.ProjectPath())
			default:
				log.Printf("wrGetenv: unknown WHARFRAT_ variable: %s", key)
				return ""
			}
		}
		if value, found := crate.Env[key]; found {
			return value
		}
		return os.Getenv(key)
	}
}

func rewrite(cmd string, out io.Writer, id string, crate *config.Crate) io.Writer {
	getenv := wrGetenv(id, crate)

	if cfg, found := crate.CmdReplace[cmd]; found {
		match := os.Expand(cfg.Match, getenv)
		replace := os.Expand(cfg.Replace, getenv)
		log.Printf("REPLACE (%s): %s -> %s", cmd, match, replace)
		return output.NewRewriter(out, []byte(match), []byte(replace))
	}

	binary := filepath.Base(cmd)
	if cfg, found := crate.CmdReplace[binary]; found {
		match := os.Expand(cfg.Match, getenv)
		replace := os.Expand(cfg.Replace, getenv)
		log.Printf("REPLACE (%s): %s -> %s", binary, match, replace)
		return output.NewRewriter(out, []byte(match), []byte(replace))
	}

	return out
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
			return -1, fmt.Errorf("failed to set working directory: %w", err)
		}
	}

	log.Printf("User: %s, Workdir: %s", user, workdir)

	oldAPI := versions.LessThan(c.c.ClientVersion(), "1.35")
	if oldAPI || tty || len(crate.Groups) > 0 || len(crate.PathAppend) > 0 || len(crate.PathPrepend) > 0 {
		proxy := []string{"/sbin/wr-init", "proxy"}
		if config.Debug {
			proxy = append(proxy, "-d")
		}
		if tty {
			proxy = append(proxy, "--sync")
		}
		if oldAPI {
			proxy = append(proxy, "--workdir", workdir)
		}
		for _, group := range crate.Groups {
			proxy = append(proxy, "--group", group)
		}
		for _, path := range crate.PathPrepend {
			proxy = append(proxy, "--prepend-path", path)
		}
		for _, path := range crate.PathAppend {
			proxy = append(proxy, "--append-path", path)
		}
		log.Printf("USE PROXY (workdir / terminal sync workaround): %s", proxy)
		cmds = append(proxy, cmds...)
	}

	env, err := buildEnv(id, crate)
	if err != nil {
		return 0, err
	}

	config := types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          tty,
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

	outChan := make(chan error)

	stdout := rewrite(cmd[0], os.Stdout, id, crate)

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
		cmd = bytes.TrimSpace(cmd)
		log.Printf("READ: %s\n", cmd)

		if string(cmd) != "PROXY READY" {
			return -1, fmt.Errorf("failed to get proxy ready, got: %s", cmd)
		}

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
			return -1, fmt.Errorf("failed to set raw terminal mode: %w", err)
		}
		defer term.RestoreTerminal(inFd, inState)

		outState, err := term.SetRawTerminal(outFd)
		if err != nil {
			return -1, fmt.Errorf("failed to set raw terminal mode: %w", err)
		}
		defer term.RestoreTerminal(outFd, outState)

		attach.Conn.Write([]byte("PROXY RUN\n"))

		go func() {
			_, err := io.Copy(stdout, attach.Reader)
			outChan <- err
		}()
	} else {
		go func() {
			_, err := stdcopy.StdCopy(stdout, os.Stderr, attach.Reader)
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
		return -1, fmt.Errorf("error copying output: %w", err)
	}

	inspect, err := c.c.ContainerExecInspect(c.ctx, execID)
	if err != nil {
		return -1, fmt.Errorf("failed to get exec response: %w", err)
	}

	if inspect.Running {
		return -1, fmt.Errorf("command still running!")
	}

	return inspect.ExitCode, nil
}

func (c *Connection) GetOutput(id string, cmd []string, crate *config.Crate, user string) ([]byte, []byte, error) {
	_, err := c.c.ContainerInspect(c.ctx, crate.ContainerName())
	if err != nil {
		return nil, nil, err
	}

	if user == "" {
		user = fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	}

	log.Printf("GET OUTPUT (%s): %v", user, cmd)

	env, err := buildEnv(id, crate)
	if err != nil {
		return nil, nil, err
	}

	config := types.ExecConfig{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          cmd,
		Env:          env,
		User:         user,
		WorkingDir:   "/",
	}

	resp, err := c.c.ContainerExecCreate(c.ctx, id, config)
	if err != nil {
		return nil, nil, err
	}

	execID := resp.ID
	if execID == "" {
		return nil, nil, fmt.Errorf("got empty exec ID")
	}

	log.Printf("EXEC: ID=%s", execID)

	startCheck := types.ExecStartCheck{}
	attach, err := c.c.ContainerExecAttach(c.ctx, execID, startCheck)
	if err != nil {
		return nil, nil, err
	}
	defer attach.Close()

	outChan := make(chan error)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	go func() {
		_, err := stdcopy.StdCopy(stdout, stderr, attach.Reader)
		log.Printf("Copy done")
		outChan <- err
	}()

	// Wait for copies to finish
	if err = <-outChan; err != nil {
		return nil, nil, fmt.Errorf("error copying output: %w", err)
	}

	inspect, err := c.c.ContainerExecInspect(c.ctx, execID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get exec response: %w", err)
	}

	if inspect.Running {
		return nil, nil, fmt.Errorf("command still running!")
	}

	if inspect.ExitCode != 0 {
		return nil, nil, fmt.Errorf("command exited with status %d", inspect.ExitCode)
	}

	return stdout.Bytes(), stderr.Bytes(), nil
}
