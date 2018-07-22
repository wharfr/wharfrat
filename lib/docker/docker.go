package docker

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker/label"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
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
	old, err := c.c.ContainerList(c.ctx, types.ContainerListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", label.OldProject)),
	})
	if err != nil {
		return nil, err
	}
	for _, c := range old {
		log.Printf("OLD: %s", c.ID)
		label.FixOld(c.Labels)
	}
	new, err := c.c.ContainerList(c.ctx, types.ContainerListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", label.Project)),
	})
	if err != nil {
		return nil, err
	}
	return append(old, new...), nil
}

func (c *Connection) GetContainer(name string) (*types.ContainerJSON, error) {
	container, err := c.c.ContainerInspect(c.ctx, name)
	if client.IsErrNotFound(err) {
		return nil, nil
	}
	if container.Config != nil {
		label.FixOld(container.Config.Labels)
	}
	return &container, err
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
		exit, err := c.run(id, cmd, nil, nil, stdout, stderr)
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

func (c *Connection) ImageLabels(name string) (map[string]string, error) {
	info, _, err := c.c.ImageInspectWithRaw(c.ctx, name)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	log.Printf("IMAGE LABELS (%s): %#v", name, info.ContainerConfig.Labels)
	return info.ContainerConfig.Labels, nil
}

func (c *Connection) GetImage(name string) (*types.ImageInspect, error) {
	info, _, err := c.c.ImageInspectWithRaw(c.ctx, name)
	if client.IsErrNotFound(err) {
		return nil, nil
	}
	return &info, err
}
