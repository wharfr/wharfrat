package docker

import (
	"fmt"
	"log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
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
