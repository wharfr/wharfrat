package docker

import (
	"fmt"
	"io"
	"log"
	"os"

	"wharfr.at/wharfrat/lib/config"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/registry"
	"github.com/moby/term"
)

func (c *Connection) run(id string, cmd []string, env map[string]string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	environ := make([]string, 0, len(env))
	for key, value := range env {
		environ = append(environ, key+"="+value)
	}

	config := container.ExecOptions{
		AttachStdin:  stdin != nil,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		User:         "root:root",
		Cmd:          cmd,
		Env:          environ,
	}

	resp, err := c.c.ContainerExecCreate(c.ctx, id, config)
	if err != nil {
		return 0, err
	}

	execID := resp.ID
	if execID == "" {
		return 0, fmt.Errorf("got empty exec ID")
	}

	startCheck := container.ExecAttachOptions{
		Tty: false,
	}
	attach, err := c.c.ContainerExecAttach(c.ctx, execID, startCheck)
	if err != nil {
		return 0, err
	}
	defer attach.Close()

	if stdin != nil {
		go func() {
			_, _ = io.Copy(attach.Conn, stdin)
			_ = attach.CloseWrite()
		}()
	}

	if stdout == nil {
		stdout = io.Discard
	}

	if stderr == nil {
		stderr = io.Discard
	}

	outChan := make(chan error)
	go func() {
		_, err := stdcopy.StdCopy(stdout, stderr, attach.Reader)
		log.Printf("Copy done")
		outChan <- err
	}()

	// Wait for copies to finish
	if err = <-outChan; err != nil {
		return 0, fmt.Errorf("error copying output: %w", err)
	}

	inspect, err := c.c.ContainerExecInspect(c.ctx, execID)
	if err != nil {
		return 0, fmt.Errorf("failed to get exec response: %w", err)
	}

	if inspect.Running {
		return 0, fmt.Errorf("container command (%s) still running", cmd[0])
	}

	return inspect.ExitCode, nil
}

func (c *Connection) pullImage(name string) error {
	ref, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return err
	}

	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return err
	}

	authName := repoInfo.Index.Name
	if repoInfo.Index.Official {
		info, err := c.Info()
		if err != nil {
			return err
		}
		address := info.IndexServerAddress
		authName = registry.ConvertToHostname(address)
	}

	log.Printf("REF: %v, REG: %v, Name: %s", ref, repoInfo, authName)

	auth, err := config.LoadAuth()
	if err != nil {
		log.Printf("Failed to load saved auth: %s", err)
	}

	options := image.CreateOptions{
		RegistryAuth: auth[authName],
		Platform:     "linux",
	}

	resp, err := c.c.ImageCreate(c.ctx, name, options)
	if err != nil {
		return err
	}
	defer resp.Close()

	fd, term := term.GetFdInfo(os.Stderr)

	log.Printf("PULL: image:%s fd:%d term:%v", name, fd, term)

	return jsonmessage.DisplayJSONMessagesStream(resp, os.Stderr, fd, term, nil)
}
