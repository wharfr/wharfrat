package docker

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	shellwords "github.com/mattn/go-shellwords"
	"wharfr.at/wharfrat/lib/config"
)

func (c *Connection) setupUser(id string, crate *config.Crate, usr *user.User, group *user.Group) error {
	cmd := []string{
		"/sbin/wr-init", "setup", "--debug",
		"--user", usr.Username, "--uid", usr.Uid, "--name", usr.Name,
		"--group", group.Name, "--gid", group.Gid,
	}

	for _, name := range crate.CopyGroups {
		group, err := user.LookupGroup(name)
		if err != nil {
			return fmt.Errorf("Failed to get group information for '%s': %s", name, err)
		}
		cmd = append(cmd, "--create-group", fmt.Sprintf("%s=%s", name, group.Gid))
	}

	for _, group := range crate.Groups {
		cmd = append(cmd, "--extra-group", group)
	}

	if !crate.MountHome {
		cmd = append(cmd, "--mkhome")
	}

	buf := &bytes.Buffer{}

	exitCode, err := c.run(id, cmd, nil, nil, nil, buf)
	if err != nil {
		return err
	}

	log.Printf("Setup stderr: %s", buf)

	if exitCode != 0 {
		return fmt.Errorf("Setup command failed (%d): %s", exitCode, buf)
	}

	return nil
}

func (c *Connection) setupPrep(id, prep, path string, args ...string) error {
	if prep == "" {
		return nil
	}

	args = append([]string{"-s"}, args...)
	log.Printf("PREP ARGS: %v", args)

	cmd := exec.Command("/bin/bash", args...)
	cmd.Stdin = strings.NewReader(prep)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = path

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Setup prep script failed: %s", err)
	}

	return nil
}

func (c *Connection) runScript(id, label, script string, env map[string]string) error {
	if script == "" {
		return nil
	}

	cmd := []string{"/bin/sh"}
	if strings.HasPrefix(strings.TrimSpace(script), "#!") {
		parts := strings.SplitN(strings.TrimSpace(script), "\n", 2)
		items, err := shellwords.Parse(parts[0][2:])
		if err != nil {
			return err
		}
		cmd = items
	}

	log.Printf("RUN SCRIPT CMD: %#v", cmd)

	stdin := strings.NewReader(script)

	exitCode, err := c.run(id, cmd, env, stdin, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("Setup %s script failed: %s", label, err)
	}

	log.Printf("SETUP %s: %d", label, exitCode)

	if exitCode != 0 {
		return fmt.Errorf("Setup %s script failed: exit status %d", label, exitCode)
	}

	return nil
}

func (c *Connection) installTarball(id string, base, src, dst string) error {
	if !filepath.IsAbs(src) {
		src = filepath.Join(base, src)
	}

	if !filepath.IsAbs(dst) {
		return fmt.Errorf("Tarball dest '%s' should be absolute path", dst)
	}

	log.Printf("INSTALL TARBALL: %s -> %s", src, dst)

	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	options := types.CopyToContainerOptions{}
	return c.c.CopyToContainer(c.ctx, id, dst, f, options)
}

func (c *Connection) setupTarballs(id, base string, tarballs map[string]string) error {
	for src, dst := range tarballs {
		if err := c.installTarball(id, base, src, dst); err != nil {
			return err
		}
	}

	return nil
}

func (c *Connection) doSteps(id, base, prep, pre, post string, tarballs, env map[string]string, args ...string) error {
	if err := c.setupPrep(id, prep, base, args...); err != nil {
		return err
	}

	if err := c.runScript(id, "pre", pre, env); err != nil {
		return err
	}

	if err := c.setupTarballs(id, base, tarballs); err != nil {
		return err
	}

	if err := c.runScript(id, "post", post, env); err != nil {
		return err
	}

	return nil
}

func (c *Connection) setup(id string, crate *config.Crate) error {
	projectPath := filepath.Dir(crate.ProjectPath())

	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("Failed to get user information: %s", err)
	}

	group, err := user.LookupGroupId(usr.Gid)
	if err != nil {
		return fmt.Errorf("Failed to get group information: %s", err)
	}

	if err := c.setupUser(id, crate, usr, group); err != nil {
		return err
	}

	local := config.Local()
	localPath := filepath.Dir(local.Path())

	env := map[string]string{
		"WR_EXT_USER":    usr.Username,
		"WR_EXT_GROUP":   group.Name,
		"WR_EXT_PROJECT": projectPath,
		"WR_EXT_CONFIG":  localPath,
		"WR_CRATE":       crate.Name(),
	}

	if err := c.doSteps(id, projectPath, crate.SetupPrep, crate.SetupPre, crate.SetupPost, crate.Tarballs, env, projectPath, crate.Name()); err != nil {
		return err
	}

	if err := c.doSteps(id, localPath, local.SetupPrep, local.SetupPre, local.SetupPost, local.Tarballs, env, projectPath, crate.Name()); err != nil {
		return err
	}

	return nil
}
