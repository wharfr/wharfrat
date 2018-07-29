package internal

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/docker/docker/pkg/term"
)

type args struct {
	Dir string `positional-arg-name:"dir" required:"true"`
	Cmd string `positional-arg-name:"cmd" required:"true"`
	//	Args []string `positional-arg-name:"args" required:"0"`
}

type Proxy struct {
	Sync    bool   `long:"sync"`
	Workdir string `long:"workdir"`
	//	Args    args `positional-args:"true" required:"true"`
}

func (p *Proxy) Wait() error {
	// 1. setup terminal (raw & disable echo)
	inFd, inTerm := term.GetFdInfo(os.Stdin)
	if inTerm {
		inState, err := term.SetRawTerminal(inFd)
		if err != nil {
			return fmt.Errorf("Failed to set raw terminal mode: %s", err)
		}
		if err := term.DisableEcho(inFd, inState); err != nil {
			return fmt.Errorf("Failed to disable terminal echo: %s", err)
		}
		defer term.RestoreTerminal(inFd, inState)
	}

	// 2. tell client we are ready
	os.Stdout.Write([]byte("PROXY READY\n"))

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

func (p *Proxy) Execute(args []string) error {
	log.Printf("PROXY: %s %#v", args, p)
	if len(args) < 1 {
		return fmt.Errorf("Need at least 1 argument for proxy")
	}
	log.Printf("PROXY: sync: %v, dir: %s, cmd: %v", p.Sync, p.Workdir, args)

	if p.Sync {
		log.Printf("PROXY WAIT ...\n")
		if err := p.Wait(); err != nil {
			return err
		}
		log.Printf("PROXY RUN ...\n")
	}

	if p.Workdir != "" {
		if err := os.Chdir(p.Workdir); err != nil {
			return fmt.Errorf("Failed to change directory to %s: %s", p.Workdir, err)
		}
	}

	cmd, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("Failed to find %s: %s", args[0], err)
	}

	if err := syscall.Exec(cmd, args, os.Environ()); err != nil {
		return fmt.Errorf("Failed to exec %s: %s", err)
	}
	return nil
}
