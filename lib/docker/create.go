package docker

import (
	"archive/tar"
	"bytes"
	"fmt"
	"log"
	"net/netip"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	specs "github.com/opencontainers/image-spec/specs-go/v1"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker/label"
	"wharfr.at/wharfrat/lib/self"
	"wharfr.at/wharfrat/lib/vc"
	"wharfr.at/wharfrat/lib/version"
)

type CreatedFunc func(c *Connection, id string, crate *config.Crate)

var created []CreatedFunc

func AfterCreate(f CreatedFunc) {
	created = append(created, f)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func getSelf() (*bytes.Buffer, error) {
	selfData, err := self.GetLinux()
	if err != nil {
		return nil, err
	}

	initHdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "/sbin/wr-init",
		Size:     int64(len(selfData)),
		Mode:     int64(os.ModeSetuid | os.ModeSetgid | 0755),
		Uid:      0,
		Gid:      0,
		Uname:    "root",
		Gname:    "root",
		ModTime:  time.Now(),
	}

	wharfratHdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "/usr/bin/wharfrat",
		Size:     int64(len(selfData)),
		Mode:     int64(0755),
		Uid:      0,
		Gid:      0,
		Uname:    "root",
		Gname:    "root",
		ModTime:  time.Now(),
	}

	wrHdr := &tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "/usr/bin/wr",
		Linkname: "wharfrat",
		Mode:     int64(0755),
		Uid:      0,
		Gid:      0,
		Uname:    "root",
		Gname:    "root",
		ModTime:  time.Now(),
	}

	buf := &bytes.Buffer{}

	w := tar.NewWriter(buf)
	defer w.Close()

	if err := w.WriteHeader(initHdr); err != nil {
		return nil, fmt.Errorf("failed to build self archive (init header): %w", err)
	}
	if _, err := w.Write(selfData); err != nil {
		return nil, fmt.Errorf("failed to build self archive (init data): %w", err)
	}

	if err := w.WriteHeader(wharfratHdr); err != nil {
		return nil, fmt.Errorf("failed to build self archive (wharfrat header): %w", err)
	}
	if _, err := w.Write(selfData); err != nil {
		return nil, fmt.Errorf("failed to build self archive (wharfrat data): %w", err)
	}

	if err := w.WriteHeader(wrHdr); err != nil {
		return nil, fmt.Errorf("failed to build self archive (wr header): %w", err)
	}

	return buf, nil
}

func parsePortSpecs(ports []string) (network.PortSet, network.PortMap, error) {
	natPorts, natBindings, err := nat.ParsePortSpecs(ports)
	if err != nil {
		return nil, nil, err
	}

	portSet := network.PortSet{}
	for port := range natPorts {
		p, err := network.ParsePort(string(port))
		if err != nil {
			return nil, nil, err
		}
		portSet[p] = struct{}{}
	}

	portMap := network.PortMap{}
	for port, bindings := range natBindings {
		p, err := network.ParsePort(string(port))
		if err != nil {
			return nil, nil, err
		}
		portMap[p] = []network.PortBinding{}
		for _, b := range bindings {
			binding := network.PortBinding{
				HostPort: b.HostPort,
			}
			if b.HostIP != "" {
				hostIP, err := netip.ParseAddr(b.HostIP)
				if err != nil {
					return nil, nil, err
				}
				binding.HostIP = hostIP
			}
			portMap[p] = append(portMap[p], binding)
		}
	}

	return portSet, portMap, nil
}

func (c *Connection) Create(crate *config.Crate) (string, error) {
	selfTar, err := getSelf()
	if err != nil {
		return "", fmt.Errorf("failed to get self: %w", err)
	}

	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get user information: %w", err)
	}

	labels := map[string]string{
		label.Project: crate.ProjectPath(),
		label.Crate:   crate.Name(),
		label.Commit:  version.Commit(),
		label.Config:  crate.Json(),
		label.User:    usr.Username,
	}

	projectDir := filepath.Dir(crate.ProjectPath())
	if branch, err := vc.Branch(projectDir); err != nil {
		log.Printf("Failed to get vc branch: %s", err)
	} else {
		labels[label.Branch] = branch
	}

	exposed, ports, err := parsePortSpecs(crate.Ports)
	if err != nil {
		return "", err
	}

	log.Printf("PORTS: %v %v", exposed, ports)

	config := &container.Config{
		ExposedPorts: exposed,
		User:         "root:root",
		Entrypoint:   []string{},
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

	binds := []string{}

	if exists("/tmp/.X11-unix") {
		binds = append(binds, "/tmp/.X11-unix:/tmp/.X11-unix")
	}

	if crate.MountHome {
		binds = append(binds, self.HomeMount...)
	}

	if crate.ProjectMount != "" {
		pDir := filepath.Dir(crate.ProjectPath())
		binds = append(binds, pDir+":"+crate.ProjectMount)
	}

	if crate.Volumes != nil {
		for _, volume := range crate.Volumes {
			binds = append(binds, os.Expand(volume, crate.Getenv))
		}
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
		DNS:          []netip.Addr{},
		DNSSearch:    []string{},
		DNSOptions:   []string{},
		NetworkMode:  container.NetworkMode(crate.Network),
	}

	networkingConfig := &network.NetworkingConfig{}

	// TODO: hard code the platform for now ...
	platform := &specs.Platform{
		Architecture: runtime.GOARCH,
		OS:           "linux",
	}

	var namedRef reference.Named

	ref, err := reference.ParseAnyReference(config.Image)
	if err != nil {
		return "", err
	}
	if named, ok := ref.(reference.Named); ok {
		namedRef = reference.TagNameOnly(named)
	}

	log.Printf("IMAGE: %s %s %s", config.Image, namedRef, reference.FamiliarString(namedRef))

	create, err := c.c.ContainerCreate(c.ctx, client.ContainerCreateOptions{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
		Platform:         platform,
		Name:             crate.ContainerName(),
	})
	if errdefs.IsNotFound(err) && namedRef != nil {
		fmt.Fprintf(os.Stderr, "Unable to find image '%s' locally\n", reference.FamiliarString(namedRef))

		if err := c.pullImage(config.Image); err != nil {
			return "", err
		}

		// Give crate another go at setting defaults now we have the image
		if err := crate.SetDefaults(c); err != nil {
			return "", err
		}
		// Crate config may have changed
		labels[label.Config] = crate.Json()

		var retryErr error
		create, retryErr = c.c.ContainerCreate(c.ctx, client.ContainerCreateOptions{
			Config:           config,
			HostConfig:       hostConfig,
			NetworkingConfig: networkingConfig,
			Platform:         platform,
			Name:             crate.ContainerName(),
		})
		if retryErr != nil {
			return "", retryErr
		}
	} else if err != nil {
		return "", err
	}
	cid := create.ID

	log.Printf("CREATE COMPLETE: %s", cid)

	if _, err := c.c.CopyToContainer(c.ctx, cid, client.CopyToContainerOptions{
		DestinationPath: "/",
		Content:         selfTar,
	}); err != nil {
		return "", err
	}

	log.Printf("SELF COPIED: %s", cid)

	if _, err := c.c.ContainerStart(c.ctx, cid, client.ContainerStartOptions{}); err != nil {
		return "", err
	}

	log.Printf("STARTED: %s", cid)

	if err := c.setup(cid, crate); err != nil {
		_ = c.EnsureRemoved(crate.ContainerName())
		return "", err
	}

	for _, f := range created {
		f(c, cid, crate)
	}

	log.Printf("CREATE COMPLETE: %s", cid)

	return cid, nil
}
