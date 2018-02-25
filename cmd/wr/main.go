package main

import (
	"io"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/term"
	"golang.org/x/net/context"
)

func main() {
	c, err := client.NewEnvClient()
	if err != nil {
		log.Fatalf("Failed to create docker client: %s", err)
	}

	ctx := context.Background()

	config := &container.Config{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		OpenStdin:    true,
		Cmd:          os.Args[1:],
		Image:        "centos:6.8",
	}
	hostConfig := &container.HostConfig{
	//AutoRemove: true,
	}
	networkingConfig := &network.NetworkingConfig{}

	create, err := c.ContainerCreate(ctx, config, hostConfig, networkingConfig, "")
	if err != nil {
		log.Fatalf("Unable to create containter: %s", err)
	}
	cid := create.ID

	log.Printf("ID: %s, Warnings: %s", cid, create.Warnings)

	attachOptions := types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}

	attach, err := c.ContainerAttach(ctx, cid, attachOptions)
	if err != nil {
		log.Fatalf("Unable to attach container: %s", err)
	}

	defer attach.Close()

	inFd, inTerm := term.GetFdInfo(os.Stdin)
	log.Printf("IN: fd=%d, term=%v", inFd, inTerm)

	outFd, outTerm := term.GetFdInfo(os.Stdout)
	log.Printf("OUT: fd=%d, term=%v", outFd, outTerm)

	inState, err := term.SetRawTerminal(inFd)
	if err != nil {
		log.Fatalf("Failed to set raw terminal mode: %s", err)
	}
	defer term.RestoreTerminal(inFd, inState)

	outState, err := term.SetRawTerminal(outFd)
	if err != nil {
		log.Fatalf("Failed to set raw terminal mode: %s", err)
	}
	defer term.RestoreTerminal(outFd, outState)

	go io.Copy(attach.Conn, os.Stdin)
	go io.Copy(os.Stdout, attach.Reader)

	err = c.ContainerStart(ctx, cid, types.ContainerStartOptions{})
	if err != nil {
		log.Fatalf("Failed to start container: %s", err)
	}

	wait, errChan := c.ContainerWait(ctx, cid, container.WaitConditionNextExit)
	select {
	case result := <-wait:
		if result.Error != nil {
			log.Fatalf("Failed to wait for container: %s", result.Error)
		}
		log.Printf("Status: %d", result.StatusCode)
	case err := <-errChan:
		log.Fatalf("Failed to wait for container: %s", err)
	}
}
