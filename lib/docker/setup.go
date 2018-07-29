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

	"git.qur.me/qur/wharf_rat/lib/config"
	"github.com/docker/docker/api/types"
)

func (c *Connection) setupUser(id string, crate *config.Crate) error {
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("Failed to get user information: %s", err)
	}

	group, err := user.LookupGroupId(usr.Gid)
	if err != nil {
		return fmt.Errorf("Failed to get group information: %s", err)
	}

	cmd := []string{
		"/sbin/wr-init", "setup", "--debug",
		"--user", usr.Username, "--uid", usr.Uid, "--name", usr.Name,
		"--group", group.Name, "--gid", group.Gid,
	}

	for _, group := range crate.Groups {
		cmd = append(cmd, "--extra-group", group)
	}

	buf := &bytes.Buffer{}

	exitCode, err := c.run(id, cmd, nil, nil, buf)
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
	cmd.Dir = filepath.Dir(path)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Setup prep script failed: %s", err)
	}

	return nil
}

func (c *Connection) runScript(id, label, script string, args ...string) error {
	if script == "" {
		return nil
	}

	cmd := append([]string{"/bin/bash", "-s"}, args...)
	stdin := strings.NewReader(script)

	exitCode, err := c.run(id, cmd, stdin, os.Stdout, os.Stderr)
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

func (c *Connection) doSteps(id, base, prep, pre, post string, tarballs map[string]string, args ...string) error {
	if err := c.setupPrep(id, prep, base, args...); err != nil {
		return err
	}

	if err := c.runScript(id, "pre", pre); err != nil {
		return err
	}

	if err := c.setupTarballs(id, base, tarballs); err != nil {
		return err
	}

	if err := c.runScript(id, "post", post); err != nil {
		return err
	}

	return nil
}

func (c *Connection) setup(id string, crate *config.Crate) error {
	projectPath := filepath.Dir(crate.ProjectPath())

	if err := c.setupUser(id, crate); err != nil {
		return err
	}

	if err := c.doSteps(id, projectPath, crate.SetupPrep, crate.SetupPre, crate.SetupPost, crate.Tarballs, projectPath, crate.Name()); err != nil {
		return err
	}

	local := config.Local()
	localPath := local.Path()

	if err := c.doSteps(id, localPath, local.SetupPrep, local.SetupPre, local.SetupPost, local.Tarballs, projectPath, crate.Name()); err != nil {
		return err
	}

	return nil
}
