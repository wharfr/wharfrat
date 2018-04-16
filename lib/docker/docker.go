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

	"git.qur.me/qur/wharf_rat/lib/config"
	"git.qur.me/qur/wharf_rat/lib/vc"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"golang.org/x/net/context"
)

type Connection struct {
	c   *client.Client
	ctx context.Context
}

func Version() string {
	return api.DefaultVersion
}

func Connect() (*Connection, error) {
	host := config.Local().DockerURL
	opts := []func(*client.Client) error{}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}
	c, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	before := c.ClientVersion()
	c.NegotiateAPIVersion(ctx)
	after := c.ClientVersion()
	log.Printf("API: before: %s, after: %s", before, after)
	return &Connection{
		c:   c,
		ctx: ctx,
	}, nil
}

func (c *Connection) Close() error {
	return c.c.Close()
}

func (c *Connection) List() ([]types.Container, error) {
	return c.c.ContainerList(c.ctx, types.ContainerListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", "me.qur.wharf-rat.project")),
	})
}

func (c *Connection) GetContainer(name string) (*types.Container, error) {
	containers, err := c.c.ContainerList(c.ctx, types.ContainerListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	if err != nil {
		return nil, err
	}
	for _, container := range containers {
		log.Printf("CONTAINER %s %v %s %v", container.ID, container.Names, container.Status, container.Labels)
	}
	if len(containers) == 0 {
		return nil, nil
	}
	if len(containers) > 1 {
		return nil, fmt.Errorf("Multiple containers with name %s", name)
	}
	return &containers[0], nil
}

func (c *Connection) Start(id string) error {
	return c.c.ContainerStart(c.ctx, id, types.ContainerStartOptions{})
}

func (c *Connection) Unpause(id string) error {
	return c.c.ContainerUnpause(c.ctx, id)
}

func (c *Connection) Stop(id string) error {
	return c.c.ContainerStop(c.ctx, id, nil)
}

func (c *Connection) Create(crate *config.Crate) (string, error) {
	self, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return "", fmt.Errorf("Failed to get self: %s", err)
	}

	labels := map[string]string{
		"me.qur.wharf-rat.project": crate.ProjectPath(),
		"me.qur.wharf-rat.crate":   crate.Name(),
		"me.qur.wharf-rat.config":  crate.Json(),
	}

	projectDir := filepath.Dir(crate.ProjectPath())
	if branch, err := vc.Branch(projectDir); err != nil {
		log.Printf("Failed to get vc branch: %s", err)
	} else {
		labels["me.qur.wharf-rat.branch"] = branch
	}

	config := &container.Config{
		User:     "root:root",
		Cmd:      []string{"/sbin/wr-init", "server", "--debug"},
		Image:    crate.Image,
		Hostname: crate.Hostname,
		Labels:   labels,
	}

	tmpfs := make(map[string]string)
	for _, entry := range crate.Tmpfs {
		if parts := strings.SplitN(entry, ":", 2); len(parts) > 1 {
			tmpfs[parts[0]] = parts[1]
		} else {
			tmpfs[parts[0]] = ""
		}
	}

	binds := []string{
		"/home:/home",
		"/tmp/.X11-unix:/tmp/.X11-unix",
		self + ":/sbin/wr-init:ro",
	}

	if crate.ProjectMount != "" {
		pdir := filepath.Dir(crate.ProjectPath())
		binds = append(binds, pdir+":"+crate.ProjectMount)
	}

	log.Printf("BINDS: %v", binds)

	hostConfig := &container.HostConfig{
		Binds: binds,
		Tmpfs: tmpfs,
	}

	networkingConfig := &network.NetworkingConfig{}

	var namedRef reference.Named

	ref, err := reference.ParseAnyReference(config.Image)
	if err != nil {
		return "", err
	}
	if named, ok := ref.(reference.Named); ok {
		namedRef = reference.TagNameOnly(named)
	}

	log.Printf("IMAGE: %s %s %s", config.Image, namedRef, reference.FamiliarString(namedRef))

	create, err := c.c.ContainerCreate(c.ctx, config, hostConfig, networkingConfig, crate.ContainerName())
	if client.IsErrNotFound(err) && namedRef != nil {
		fmt.Fprintf(os.Stderr, "Unable to find image '%s' locally\n", reference.FamiliarString(namedRef))

		if err := c.pullImage(config.Image); err != nil {
			return "", err
		}

		var retryErr error
		create, retryErr = c.c.ContainerCreate(c.ctx, config, hostConfig, networkingConfig, crate.ContainerName())
		if retryErr != nil {
			return "", retryErr
		}
	} else if err != nil {
		return "", err
	}
	cid := create.ID

	if err := c.c.ContainerStart(c.ctx, cid, types.ContainerStartOptions{}); err != nil {
		return "", err
	}

	if err := c.setup(cid, crate); err != nil {
		c.EnsureRemoved(crate.ContainerName())
		return "", err
	}

	return cid, nil
}

func (c *Connection) EnsureRunning(crate *config.Crate, force bool) (string, error) {
	container, err := c.GetContainer(crate.ContainerName())
	if err != nil {
		return "", fmt.Errorf("Failed to get docker container: %s", err)
	}

	if container == nil {
		return c.Create(crate)
	}

	log.Printf("FOUND %s %s", container.ID, container.State)

	oldJson := container.Labels["me.qur.wharf-rat.config"]
	if oldJson != crate.Json() && !force {
		return "", fmt.Errorf("Container built from old config")
	}

	switch container.State {
	case "created":
		return "", fmt.Errorf("State %s NOT IMPLEMENTED", container.State)
	case "running":
		log.Printf("RUNNING")
	case "paused":
		if err := c.Unpause(container.ID); err != nil {
			return "", fmt.Errorf("Failed to start container: %s", err)
		}
	case "restarting":
		return "", fmt.Errorf("State %s NOT IMPLEMENTED", container.State)
	case "removing":
		return "", fmt.Errorf("State %s NOT IMPLEMENTED", container.State)
	case "exited":
		if err := c.Start(container.ID); err != nil {
			return "", fmt.Errorf("Failed to start container: %s", err)
		}
	case "dead":
		return "", fmt.Errorf("State %s NOT IMPLEMENTED", container.State)
	default:
		return "", fmt.Errorf("Invalid container state: %s", container.State)
	}

	return container.ID, nil
}

func (c *Connection) EnsureStopped(name string) error {
	log.Printf("STOP %s", name)

	container, err := c.GetContainer(name)
	if err != nil {
		return fmt.Errorf("Failed to get docker container: %s", err)
	}

	if container == nil {
		return nil
	}

	log.Printf("FOUND %s %s", container.ID, container.State)

	// TODO(jp3): implement stopping the container

	switch container.State {
	case "created":
		log.Printf("CREATED")
	case "running":
		if err := c.Stop(container.ID); err != nil {
			return fmt.Errorf("Failed to stop container: %s", err)
		}
	case "paused":
		if err := c.Stop(container.ID); err != nil {
			return fmt.Errorf("Failed to stop container: %s", err)
		}
	case "restarting":
		if err := c.Stop(container.ID); err != nil {
			return fmt.Errorf("Failed to stop container: %s", err)
		}
	case "removing":
		log.Printf("REMOVING")
	case "exited":
		log.Printf("EXITED")
	case "dead":
		log.Printf("DEAD")
	default:
		return fmt.Errorf("Invalid container state: %s", container.State)
	}

	return nil
}

func (c *Connection) EnsureRemoved(name string) error {
	log.Printf("REMOVE %s", name)

	container, err := c.GetContainer(name)
	if err != nil {
		return fmt.Errorf("Failed to get docker container: %s", err)
	}

	if container == nil {
		return nil
	}

	log.Printf("FOUND %s %s", container.ID, container.State)

	return c.c.ContainerRemove(c.ctx, name, types.ContainerRemoveOptions{
		Force: true,
	})
}

func (c *Connection) calcWorkdir(id, user, workdir string, crate *config.Crate) (string, error) {
	if strings.HasPrefix(workdir, "/") {
		return workdir, nil
	}
	parts := strings.SplitN(workdir, ",", 2)
	workdir = strings.TrimSpace(parts[0])
	wd, err := c.calcWorkdirSingle(id, user, workdir, crate)
	if err == nil {
		return wd, nil
	}
	log.Printf("Calculate Working Dir: '%s' failed: %s", workdir, err)
	next := strings.TrimSpace(parts[1])
	if next == "" {
		return "", err
	}
	return c.calcWorkdir(id, user, next, crate)
}

func (c *Connection) calcWorkdirSingle(id, user, workdir string, crate *config.Crate) (string, error) {
	log.Printf("Calculate Working Dir: id=%s user=%s workdir=%s", id, user, workdir)
	switch workdir {
	case "", "match":
		return os.Getwd()
	case "project":
		if crate.ProjectMount == "" {
			return "", fmt.Errorf("project-mount not set")
		}
		project := filepath.Dir(crate.ProjectPath())
		local, err := os.Getwd()
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(project, local)
		if err != nil {
			return "", err
		}
		log.Printf("REL: %s %s", rel, local)
		if strings.HasPrefix(rel, "../") {
			return "", fmt.Errorf("Current path is not inside project")
		}
		return filepath.Join(crate.ProjectMount, rel), nil
	case "home":
		if idx := strings.Index(user, ":"); idx >= 0 {
			user = user[:idx]
		}
		cmd := []string{"/sbin/wr-init", "homedir", user}
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		exit, err := c.run(id, cmd, nil, stdout, stderr)
		if err != nil {
			return "", err
		}
		if exit != 0 {
			return "", fmt.Errorf("Failed to get home directory for %s: %s", user, stderr.String())
		}
		return string(bytes.TrimSpace(stdout.Bytes())), nil
	}
	return "", fmt.Errorf("Invalid working-dir: '%s'", workdir)
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
	outFd, _ := term.GetFdInfo(os.Stdout)

	env := []string{
		"WHARF_RAT_CRATE=" + crate.Name(),
		"WHARF_RAT_PROJECT=" + crate.ProjectPath(),
	}

	log.Printf("CRATE ENV: %v", crate.Env)
	for name, value := range crate.Env {
		switch name {
		case "WHARF_RAT_CRATE", "WHARF_RAT_PROJECT":
			log.Printf("Ignoring attempt to change %s", name)
		default:
			env = append(env, name+"="+value)
		}
	}

	local := config.Local()
	log.Printf("LOCAL ENV: %v", local.Env)
	for name, value := range local.Env {
		switch name {
		case "WHARF_RAT_CRATE", "WHARF_RAT_PROJECT":
			log.Printf("Ignoring attempt to change %s", name)
		default:
			env = append(env, name+"="+value)
		}
	}

	whitelist := map[string]bool{
		"DISPLAY":    true,
		"EDITOR":     true,
		"LESS":       true,
		"LS_COLORS":  true,
		"LS_OPTIONS": true,
		"MORE":       true,
		"PAGER":      true,
		"TERM":       true,
		"XAUTHORITY": true,
	}

	for _, entry := range os.Environ() {
		if parts := strings.SplitN(entry, "=", 2); whitelist[parts[0]] {
			env = append(env, entry)
		}
	}

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

	if versions.LessThan(c.c.ClientVersion(), "1.35") {
		log.Printf("WORKDIR WORKAROUND")
		cmds = append([]string{"/sbin/wr-init", "proxy", workdir}, cmds...)
	}

	config := types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          inTerm,
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
		return -1, fmt.Errorf("Got empty exec ID")
	}

	log.Printf("EXEC: ID=%s", execID)

	startCheck := types.ExecStartCheck{
		Tty: inTerm,
	}
	attach, err := c.c.ContainerExecAttach(c.ctx, execID, startCheck)
	if err != nil {
		return -1, err
	}
	defer attach.Close()

	if inTerm {
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
	}

	outChan := make(chan error)

	go func() {
		io.Copy(attach.Conn, os.Stdin)
		attach.CloseWrite()
	}()

	if inTerm {
		go func() {
			_, err := io.Copy(os.Stdout, attach.Reader)
			outChan <- err
		}()

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

		go func() {
			log.Printf("Initial Resize")
			for resizeTty() != nil {
			}
		}()

		go func() {
			sigchan := make(chan os.Signal, 1)
			signal.Notify(sigchan, syscall.SIGWINCH)
			for range sigchan {
				resizeTty()
			}
		}()
	} else {
		go func() {
			_, err := stdcopy.StdCopy(os.Stdout, os.Stderr, attach.Reader)
			log.Printf("Copy done")
			outChan <- err
		}()
	}

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

func (c *Connection) Login(addr, user, pass string) (*types.AuthConfig, error) {
	authConfig := types.AuthConfig{
		ServerAddress: addr,
		Username:      user,
		Password:      pass,
	}

	resp, err := c.c.RegistryLogin(c.ctx, authConfig)
	if err != nil {
		return nil, err
	}

	log.Printf("LOGIN: token=%s, status=%s", resp.IdentityToken, resp.Status)

	if resp.Status != "" {
		fmt.Println(resp.Status)
	}

	return &authConfig, nil
}

func (c *Connection) Info() (types.Info, error) {
	return c.c.Info(c.ctx)
}
