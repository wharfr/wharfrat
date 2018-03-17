package internal

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

type args struct {
	Dir string `positional-arg-name:"dir" required:"true"`
	Cmd string `positional-arg-name:"cmd" required:"true"`
	//	Args []string `positional-arg-name:"args" required:"0"`
}

type Proxy struct {
	//	Args    args `positional-args:"true" required:"true"`
}

func (p *Proxy) Execute(args []string) error {
	log.Printf("PROXY: %s %#v", args, p)
	if len(args) < 2 {
		log.Fatalf("Need at least 2 args for proxy")
	}
	dir := args[0]
	argv := args[1:]
	log.Printf("PROXY: dir: %s, cmd: %v", dir, argv)

	if err := os.Chdir(dir); err != nil {
		log.Fatalf("Failed to change directory to %s: %s", dir, err)
	}

	cmd, err := exec.LookPath(args[1])
	if err != nil {
		log.Fatalf("Failed to find %s: %s", args[1], err)
	}

	if err := syscall.Exec(cmd, argv, os.Environ()); err != nil {
		log.Fatalf("Failed to exec %s: %s", err)
	}
	return nil
}
