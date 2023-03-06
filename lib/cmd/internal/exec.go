package internal

import (
	"log"
	"os"
	"os/exec"

	"wharfr.at/wharfrat/lib/mux"
	"wharfr.at/wharfrat/lib/rpc/proxy"
)

type Exec struct {
}

func (e *Exec) Execute(args []string) error {
	// Make sure that we control things as we expect
	// runtime.GOMAXPROCS(1)
	// runtime.LockOSThread()

	// Create a mux from stdin/stdout
	m := mux.New("server", os.Stdin, os.Stdout)
	defer m.Close()

	processCh := make(chan error, 1)
	go func() {
		err := m.Process()
		if err != nil {
			processCh <- err
		}
	}()

	// Setup logging to use stderr (for now anyway)
	log.SetOutput(os.Stderr)

	log.Printf("STARTING")

	cmd := exec.Command(args[0], args[1:]...)
	p := proxy.New(cmd, m)

	// Setup RPC server on channel 0
	log.Printf("START RPC SERVER")
	server, err := proxy.NewServer(m.Connect(0), p)
	if err != nil {
		return err
	}
	go server.Serve()

	log.Printf("WAIT FOR RPC START COMMAND")
	p.Wait()

	log.Printf("RUN COMMAND")
	cmdCh := make(chan error, 1)
	go func() {
		cmdCh <- cmd.Run()
	}()

	log.Printf("WAIT FOR SOMETHING TO HAPPEN")
	select {
	case err := <-cmdCh:
		log.Printf("COMMAND EXITED: %s", err)
		return err
	case err := <-processCh:
		log.Printf("PROCESS STOPPED: %s", err)
		return err
	}
}
