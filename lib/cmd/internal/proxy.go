package internal

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/term"
	"golang.org/x/sys/unix"
)

type args struct {
	Dir string `positional-arg-name:"dir" required:"true"`
	Cmd string `positional-arg-name:"cmd" required:"true"`
	//	Args []string `positional-arg-name:"args" required:"0"`
}

type Proxy struct {
	Sync        bool     `long:"sync"`
	Workdir     string   `long:"workdir"`
	Groups      []string `long:"group"`
	PathAppend  []string `long:"append-path"`
	PathPrepend []string `long:"prepend-path"`
	//	Args    args `positional-args:"true" required:"true"`
}

func (p *Proxy) Wait(logOut io.Writer) error {
	// 1. setup terminal (raw & disable echo)
	inFd, inTerm := term.GetFdInfo(os.Stdin)
	if inTerm {
		inState, err := term.SetRawTerminal(inFd)
		if err != nil {
			return fmt.Errorf("Failed to set raw terminal mode: %w", err)
		}
		if err := term.DisableEcho(inFd, inState); err != nil {
			return fmt.Errorf("Failed to disable terminal echo: %w", err)
		}
		defer term.RestoreTerminal(inFd, inState)
	}

	// 2. tell client we are ready
	os.Stdout.Write([]byte("PROXY READY\n"))

	// 2b. we can enable logging now if requested
	log.SetOutput(logOut)

	// 3. wait for client to tell us to continue
	cmd := []byte{}
	for {
		buf := make([]byte, 1)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return err
		}
		if buf[0] == '\n' {
			break
		}
		cmd = append(cmd, buf[:n]...)
	}
	log.Printf("READ: %s\n", cmd)

	// 4. all done, restore terminal and continue
	return nil
}

func (p *Proxy) updatePath() {
	path := os.Getenv("PATH")
	parts := append(p.PathPrepend, filepath.SplitList(path)...)
	parts = append(parts, p.PathAppend...)
	path = strings.Join(parts, ":")
	os.Setenv("PATH", path)
	log.Printf("PROXY: update path: %s", path)
}

func (p *Proxy) Execute(args []string) error {
	// Make sure that we control things as we expect
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()

	// If sync is enabled then we can't output anything before the "PROXY READY"
	// message ...
	logOut := log.Writer()
	if p.Sync {
		log.SetOutput(io.Discard)
	}

	log.Printf("PROXY: %s %#v", args, p)
	if len(args) < 1 {
		log.SetOutput(logOut)
		return fmt.Errorf("Need at least 1 argument for proxy")
	}
	log.Printf("PROXY: sync: %v, dir: %s, cmd: %v", p.Sync, p.Workdir, args)

	if p.Sync {
		log.Printf("PROXY WAIT ...\n")
		if err := p.Wait(logOut); err != nil {
			log.SetOutput(logOut)
			return err
		}
		log.Printf("PROXY RUN ...\n")
	}

	if p.Workdir != "" {
		if err := os.Chdir(p.Workdir); err != nil {
			return fmt.Errorf("Failed to change directory to %s: %w", p.Workdir, err)
		}
	}

	u, err := user.LookupId(strconv.Itoa(os.Getuid()))
	if err != nil {
		return fmt.Errorf("Failed to lookup current user: %w", err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("Invalid GID '%s': %w", u.Gid, err)
	}
	groups := []int{gid}
	for _, name := range p.Groups {
		group, err := user.LookupGroup(name)
		if err != nil {
			log.Printf("Failed to lookup group '%s': %s", name, err)
			continue
		}
		gid, err := strconv.Atoi(group.Gid)
		if err != nil {
			log.Printf("Invalid GID '%s' for group '%s': %s", u.Gid, name, err)
			continue
		}
		groups = append(groups, gid)
	}
	if err := unix.Setgroups(groups); err != nil {
		fmt.Printf("Failed to set groups: %s\n", err)
	}

	if err := unix.Setreuid(os.Getuid(), os.Getuid()); err != nil {
		return fmt.Errorf("Failed to set UID: %w", err)
	}

	p.updatePath()

	env := []string{"USER=" + u.Username}
	env = append(env, os.Environ()...)
	log.Printf("PROXY: ENV: %v", env)

	cmd, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("Failed to find %s: %w", args[0], err)
	}

	log.Printf("PROXY: EXEC %s %v", cmd, args)

	if err := syscall.Exec(cmd, args, env); err != nil {
		return fmt.Errorf("Failed to exec %s: %w", cmd, err)
	}
	return nil
}
