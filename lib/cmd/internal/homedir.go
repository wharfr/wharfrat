package internal

import (
	"fmt"
	"log"
	"os/user"
	"strconv"
)

type Homedir struct {
	Args homeDirArgs `positional-args:"true"`
}

type homeDirArgs struct {
	User string `positional-arg-name:"user" required:"true"`
}

func (h *Homedir) getUser() (*user.User, error) {
	u, err := user.Lookup(h.Args.User)
	log.Printf("LOOKUP USER: [%p] [%s]", u, err)
	if err == nil {
		return u, nil
	}
	if _, ok := err.(user.UnknownUserError); !ok {
		return nil, err
	}

	_, err = strconv.Atoi(h.Args.User)
	if err != nil {
		return nil, fmt.Errorf("unknown user: %s", h.Args.User)
	}

	u, err = user.LookupId(h.Args.User)
	log.Printf("LOOKUP USER ID: [%p] [%s]", u, err)
	if err == nil {
		return u, nil
	}
	if _, ok := err.(user.UnknownUserIdError); !ok {
		return nil, err
	}

	return nil, fmt.Errorf("unknown user: %s", h.Args.User)
}

func (h *Homedir) Execute(args []string) error {
	user, err := h.getUser()
	if err != nil {
		return err
	}

	fmt.Println(user.HomeDir)

	return nil
}
