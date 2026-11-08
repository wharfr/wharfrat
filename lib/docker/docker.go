package docker

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/client"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker/label"
)

type Connection struct {
	c   *client.Client
	ctx context.Context
}

func Connect() (*Connection, error) {
	host := config.Local().DockerURL

	opts := []client.Opt{}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}

	c, err := client.New(opts...)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	return &Connection{
		c:   c,
		ctx: ctx,
	}, nil
}

func (c *Connection) Close() error {
	return c.c.Close()
}

func (c *Connection) List() ([]container.Summary, error) {
	all, err := c.c.ContainerList(c.ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("label", label.Project),
	})
	if err != nil {
		return nil, err
	}

	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	filtered := make([]container.Summary, 0, len(all.Items))
	for _, container := range all.Items {
		cUser := container.Labels[label.User]
		if cUser == usr.Username {
			filtered = append(filtered, container)
		}
	}

	return filtered, nil
}

func (c *Connection) GetContainer(name string) (*container.InspectResponse, error) {
	resp, err := c.c.ContainerInspect(c.ctx, name, client.ContainerInspectOptions{})
	if errdefs.IsNotFound(err) {
		return nil, nil
	}

	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	container := resp.Container

	if container.Config != nil && container.Config.Labels[label.User] != usr.Username {
		return nil, nil
	}

	return &container, err
}

func (c *Connection) Start(id string) error {
	_, err := c.c.ContainerStart(c.ctx, id, client.ContainerStartOptions{})
	return err
}

func (c *Connection) Unpause(id string) error {
	_, err := c.c.ContainerUnpause(c.ctx, id, client.ContainerUnpauseOptions{})
	return err
}

func (c *Connection) Stop(id string) error {
	_, err := c.c.ContainerStop(c.ctx, id, client.ContainerStopOptions{})
	return err
}

func (c *Connection) Remove(id string, force bool) error {
	_, err := c.c.ContainerRemove(c.ctx, id, client.ContainerRemoveOptions{
		Force: force,
	})
	return err
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
			return "", fmt.Errorf("current path is not inside project")
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
			return "", fmt.Errorf("failed to get home directory for %s: %s", user, stderr.String())
		}
		return string(bytes.TrimSpace(stdout.Bytes())), nil
	}

	return "", fmt.Errorf("invalid working-dir: '%s'", workdir)
}

func (c *Connection) Login(addr, user, pass string) (*registry.AuthConfig, error) {
	authConfig := registry.AuthConfig{
		ServerAddress: addr,
		Username:      user,
		Password:      pass,
	}

	resp, err := c.c.RegistryLogin(c.ctx, client.RegistryLoginOptions{
		Username:      authConfig.Username,
		Password:      authConfig.Password,
		ServerAddress: authConfig.ServerAddress,
		IdentityToken: authConfig.IdentityToken,
		RegistryToken: authConfig.RegistryToken,
	})
	if err != nil {
		return nil, err
	}

	log.Printf("LOGIN: token=%s, status=%s", resp.Auth.IdentityToken, resp.Auth.Status)

	if resp.Auth.Status != "" {
		fmt.Println(resp.Auth.Status)
	}

	return &authConfig, nil
}

func (c *Connection) Info() (system.Info, error) {
	resp, err := c.c.Info(c.ctx, client.InfoOptions{})
	return resp.Info, err
}

func (c *Connection) ImageLabels(name string) (map[string]string, error) {
	info, err := c.c.ImageInspect(c.ctx, name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	log.Printf("IMAGE LABELS (%s): %#v", name, info.Config.Labels)

	return info.Config.Labels, nil
}

func (c *Connection) GetImage(name string) (*image.InspectResponse, error) {
	info, err := c.c.ImageInspect(c.ctx, name)
	if errdefs.IsNotFound(err) {
		return nil, nil
	}
	return &info.InspectResponse, err
}
