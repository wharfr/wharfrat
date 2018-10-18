package docker

import (
	"fmt"
	"log"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker/label"
)

func (c *Connection) EnsureRunning(crate *config.Crate, force bool) (string, error) {
	container, err := c.GetContainer(crate.ContainerName())
	if err != nil {
		return "", fmt.Errorf("Failed to get docker container: %s", err)
	}

	if container == nil {
		return c.Create(crate)
	}

	log.Printf("FOUND %s %s", container.ID, container.State)

	oldJson := container.Config.Labels[label.Config]
	if oldJson != crate.Json() && !force {
		return "", fmt.Errorf("Container built from old config")
	}

	image, err := c.GetImage(crate.Image)
	if err != nil {
		return "", err
	}

	if container.Image != image.ID && !force {
		log.Printf("CONTAINER IMAGE: wanted \"%s\", got \"%s\"", image.ID, container.Image)
		return "", fmt.Errorf("Container built from wrong (old?) image")
	}

	switch container.State.Status {
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

	switch container.State.Status {
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

	return c.Remove(name, true)
}
