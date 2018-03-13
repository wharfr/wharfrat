package docker

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"git.qur.me/qur/wharf_rat/lib/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/term"
	"golang.org/x/net/context"
)

type Connection struct {
	c   *client.Client
	ctx context.Context
}

func Connect() (*Connection, error) {
	c, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return &Connection{
		c:   c,
		ctx: context.Background(),
	}, nil
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
		log.Printf("CONTAINER %s %v %s", container.ID, container.Names, container.Status)
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

func (c *Connection) Create(crate *config.Crate) (string, error) {
	self, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return "", fmt.Errorf("Failed to get self: %s", err)
	}

	config := &container.Config{
		Cmd:   []string{"/wr", "-s"},
		Image: "centos:6.8",
	}

	hostConfig := &container.HostConfig{
		Binds: []string{
			"/home:/home",
			self + ":/wr:ro",
		},
	}

	networkingConfig := &network.NetworkingConfig{}

	create, err := c.c.ContainerCreate(c.ctx, config, hostConfig, networkingConfig, crate.ContainerName())
	if err != nil {
		return "", err
	}
	cid := create.ID

	if err := c.c.ContainerStart(c.ctx, cid, types.ContainerStartOptions{}); err != nil {
		return "", err
	}

	return cid, nil
}

func (c *Connection) EnsureRunning(crate *config.Crate) (string, error) {
	container, err := c.GetContainer(crate.ContainerName())
	if err != nil {
		log.Fatalf("Failed to get docker container: %s", err)
	}

	if container == nil {
		return c.Create(crate)
	}

	log.Printf("FOUND %s %s", container.ID, container.State)

	switch container.State {
	case "created":
		if err := c.Start(container.ID); err != nil {
			return "", fmt.Errorf("Failed to start container: %s", err)
		}
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

func (c *Connection) ExecCmd(id string, cmd []string) (int, error) {
	inFd, inTerm := term.GetFdInfo(os.Stdin)
	log.Printf("IN: fd=%d, term=%v", inFd, inTerm)

	outFd, outTerm := term.GetFdInfo(os.Stdout)
	log.Printf("OUT: fd=%d, term=%v", outFd, outTerm)

	// size, err := term.GetWinsize(inFd)
	// if err != nil {
	// 	return -1, fmt.Errorf("Failed to get terminal size: %s", err)
	// }

	config := types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          cmd,
	}
	resp, err := c.c.ContainerExecCreate(c.ctx, id, config)
	if err != nil {
		return -1, err
	}
	execID := resp.ID
	if execID == "" {
		return -1, fmt.Errorf("Got empty exec ID")
	}

	startCheck := types.ExecStartCheck{
		Tty: true,
	}
	attach, err := c.c.ContainerExecAttach(c.ctx, execID, startCheck)
	if err != nil {
		return -1, err
	}
	defer attach.Close()

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

	outChan := make(chan error)

	go io.Copy(attach.Conn, os.Stdin)
	go func() {
		_, err := io.Copy(os.Stdout, attach.Reader)
		outChan <- err
	}()

	resizeTty := func() {
		size, err := term.GetWinsize(inFd)
		if (size.Height == 0 && size.Width == 0) || err != nil {
			return
		}
		c.c.ContainerExecResize(c.ctx, execID, types.ResizeOptions{
			Height: uint(size.Height),
			Width:  uint(size.Width),
		})
	}

	resizeTty()

	go func() {
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, syscall.SIGWINCH)
		for range sigchan {
			resizeTty()
		}
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
