package docker

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"wharfr.at/wharfrat/lib/config"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/registry"
)

func (c *Connection) run(id string, cmd []string, env map[string]string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	environ := make([]string, 0, len(env))
	for key, value := range env {
		environ = append(environ, key+"="+value)
	}

	config := types.ExecConfig{
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
		return 0, fmt.Errorf("Got empty exec ID")
	}

	startCheck := types.ExecStartCheck{
		Tty: false,
	}
	attach, err := c.c.ContainerExecAttach(c.ctx, execID, startCheck)
	if err != nil {
		return 0, err
	}
	defer attach.Close()

	if stdin != nil {
		go func() {
			io.Copy(attach.Conn, stdin)
			attach.CloseWrite()
		}()
	}

	if stdout == nil {
		stdout = ioutil.Discard
	}

	if stderr == nil {
		stderr = ioutil.Discard
	}

	outChan := make(chan error)
	go func() {
		_, err := stdcopy.StdCopy(stdout, stderr, attach.Reader)
		log.Printf("Copy done")
		outChan <- err
	}()

	// Wait for copies to finish
	if err = <-outChan; err != nil {
		return 0, fmt.Errorf("Error copying output: %s", err)
	}

	inspect, err := c.c.ContainerExecInspect(c.ctx, execID)
	if err != nil {
		return 0, fmt.Errorf("Failed to get exec response: %s", err)
	}

	if inspect.Running {
		return 0, fmt.Errorf("Container command (%s) still running", cmd[0])
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

	options := types.ImageCreateOptions{
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
