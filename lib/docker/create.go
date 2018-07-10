package docker

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/vc"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func getSelf() (*bytes.Buffer, error) {
	self, err := os.Open("/proc/self/exe")
	if err != nil {
		return nil, fmt.Errorf("Failed to get self: %s", err)
	}
	defer self.Close()
	selfData, err := ioutil.ReadAll(self)
	if err != nil {
		return nil, fmt.Errorf("Failed to read self: %s", err)
	}

	selfHdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "wr-init",
		Size:     int64(len(selfData)),
		Mode:     int64(os.ModeSetuid | os.ModeSetgid | 0755),
		Uid:      0,
		Gid:      0,
		Uname:    "root",
		Gname:    "root",
		ModTime:  time.Now(),
	}

	buf := &bytes.Buffer{}

	w := tar.NewWriter(buf)
	defer w.Close()

	if err := w.WriteHeader(selfHdr); err != nil {
		return nil, fmt.Errorf("Failed to build self archive (header): %s", err)
	}
	if _, err := w.Write(selfData); err != nil {
		return nil, fmt.Errorf("Failed to build self archive (data): %s", err)
	}

	return buf, nil
}

func (c *Connection) Create(crate *config.Crate) (string, error) {
	// self, err := os.Readlink("/proc/self/exe")
	// if err != nil {
	// 	return "", fmt.Errorf("Failed to get self: %s", err)
	// }
	self, err := getSelf()
	if err != nil {
		return "", fmt.Errorf("Failed to get self: %s", err)
	}

	labels := map[string]string{
		LabelProject: crate.ProjectPath(),
		LabelCrate:   crate.Name(),
		LabelConfig:  crate.Json(),
	}

	projectDir := filepath.Dir(crate.ProjectPath())
	if branch, err := vc.Branch(projectDir); err != nil {
		log.Printf("Failed to get vc branch: %s", err)
	} else {
		labels[LabelBranch] = branch
	}

	exposed, ports, err := nat.ParsePortSpecs(crate.Ports)
	if err != nil {
		return "", err
	}

	log.Printf("PORTS: %v %v", exposed, ports)

	config := &container.Config{
		ExposedPorts: exposed,
		User:         "root:root",
		Cmd:          []string{"/sbin/wr-init", "server", "--debug"},
		Image:        crate.Image,
		Hostname:     crate.Hostname,
		Labels:       labels,
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
		"/tmp/.X11-unix:/tmp/.X11-unix",
		//self + ":/sbin/wr-init:ro",
	}

	if crate.MountHome {
		binds = append(binds, "/home:/home")
	}

	if crate.ProjectMount != "" {
		pdir := filepath.Dir(crate.ProjectPath())
		binds = append(binds, pdir+":"+crate.ProjectMount)
	}

	if crate.Volumes != nil {
		binds = append(binds, crate.Volumes...)
	}

	log.Printf("BINDS: %v", binds)

	// apparently we shouldn't let the DNS... fields be nil?
	// See https://github.com/docker/docker/pull/17779
	hostConfig := &container.HostConfig{
		Binds:        binds,
		PortBindings: ports,
		Tmpfs:        tmpfs,
		CapAdd:       crate.CapAdd,
		CapDrop:      crate.CapDrop,
		DNS:          []string{},
		DNSSearch:    []string{},
		DNSOptions:   []string{},
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

	if err := c.c.CopyToContainer(c.ctx, cid, "/sbin", self, types.CopyToContainerOptions{CopyUIDGID: false}); err != nil {
		return "", err
	}

	if err := c.c.ContainerStart(c.ctx, cid, types.ContainerStartOptions{}); err != nil {
		return "", err
	}

	if err := c.setup(cid, crate); err != nil {
		c.EnsureRemoved(crate.ContainerName())
		return "", err
	}

	return cid, nil
}
