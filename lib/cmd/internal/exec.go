package internal

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/creack/multio"
	"wharfr.at/wharfrat/lib/rpc/proxy"
)

type slipWriter struct {
	w io.Writer
}

func slip(w io.Writer) slipWriter {
	return slipWriter{w: w}
}

func (s slipWriter) Write(b []byte) (int, error) {
	_, _ = s.w.Write(b)
	return len(b), nil
}

type Exec struct {
}

func (e *Exec) Execute(args []string) error {
	// Make sure that we control things as we expect
	// runtime.GOMAXPROCS(1)
	// runtime.LockOSThread()

	f, err := os.OpenFile("/wr-init.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	log.SetOutput(io.MultiWriter(slip(os.Stderr), f))

	log.Printf("----------------------------------------------")

	// Create a mux from stdin/stdout
	// m := mux.New("server", os.Stdin, os.Stdout)
	// use io.MultiReader to force os.Stdin to ONLY be an io.Reader
	m, err := multio.NewMultiplexer(io.MultiReader(os.Stdin), os.Stdout)
	if err != nil {
		return fmt.Errorf("failed to create mux: %w", err)
	}
	defer func() {
		log.Printf("close mux")
		m.Close()
		log.Printf("mux closed")
	}()

	// processCh := make(chan error, 1)
	// go func() {
	// 	err := m.Process()
	// 	if err != nil {
	// 		processCh <- err
	// 	}
	// }()

	log.Printf("STARTING")

	cmd := exec.Command(args[0], args[1:]...)
	p := proxy.New(cmd, m)

	// Setup RPC server on channel 0
	log.Printf("START RPC SERVER")
	server, err := proxy.NewServer(m.NewReadWriter(0), p)
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
		log.Printf("COMMAND FINISHED: %s", err)
		return err
		// case err := <-processCh:
		// 	log.Printf("PROCESS STOPPED: %s", err)
		// 	if cmd.Process != nil {
		// 		// If the process is running, then kill it before we exit
		// 		cmd.Process.Kill()
		// 	}
		// 	return err
	}
}
