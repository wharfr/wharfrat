package wharfrat

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"

	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/registry"
)

type Login struct {
	Args loginArgs `positional-args:"true"`
	User string    `short:"u" long:"user" description:"Username"`
	Pass string    `short:"p" long:"password" description:"Password"`
}

type loginArgs struct {
	Server string `positional-arg-name:"server"`
}

func getInput(prompt string) string {
	fmt.Printf("%s: ", prompt)
	buf := bufio.NewReader(os.Stdin)
	v, _, err := buf.ReadLine()
	if err != nil {
		panic("Failed to read from stdin: " + err.Error())
	}
	return string(v)
}

func getPass(prompt string) string {
	fd, _ := term.GetFdInfo(os.Stdin)
	state, err := term.SaveState(fd)
	if err != nil {
		panic("Failed to save terminal state: " + err.Error())
	}
	defer func() {
		fmt.Printf("\n")
		term.RestoreTerminal(fd, state)
	}()
	term.DisableEcho(fd, state)
	return getInput(prompt)
}

func (l *Login) Execute(args []string) error {
	client, err := docker.Connect()
	if err != nil {
		return err
	}
	defer client.Close()

	server := l.Args.Server

	if server == "" {
		info, err := client.Info()
		if err != nil {
			return err
		}
		server = info.IndexServerAddress
	}

	server = registry.ConvertToHostname(server)

	log.Printf("LOGIN: %s", server)

	if server == "" {
		return fmt.Errorf("Server required")
	}

	username := l.User
	password := l.Pass

	_, inTerm := term.GetFdInfo(os.Stdin)

	if password == "" && !inTerm {
		return fmt.Errorf("Unable to request password without terminal")
	}

	if username == "" {
		username = getInput("Username")
	}

	if password == "" {
		password = getPass("Password")
	}

	if username == "" {
		return fmt.Errorf("Username required")
	}

	if password == "" {
		return fmt.Errorf("Password required")
	}

	log.Printf("LOGIN: user=%s, server=%s", username, server)

	authConfig, err := client.Login(server, username, password)
	if err != nil {
		return err
	}

	auth, err := config.LoadAuth()
	if err != nil {
		return err
	}

	if err := auth.Set(authConfig); err != nil {
		return err
	}

	return auth.Save()
}
