package wharfrat

import (
	"fmt"

	"wharfr.at/wharfrat/lib/config"
)

type Logout struct {
	Args logoutArgs `positional-args:"true" required:"true"`
}

type logoutArgs struct {
	Server string `positional-arg-name:"server" required:"true"`
}

func (l *Logout) Execute(args []string) error {
	auth, err := config.LoadAuth()
	if err != nil {
		return err
	}

	if _, ok := auth[l.Args.Server]; ok {
		auth.Clear(l.Args.Server)
		fmt.Printf("Removed credentials for %s\n", l.Args.Server)
		return auth.Save()
	}

	return nil
}
