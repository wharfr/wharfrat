package config

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"os"

	"github.com/docker/docker/api/types/registry"
)

const authFilename = "auth.json"

type Auth map[string]string

func LoadAuth() (Auth, error) {
	auth := Auth{}

	if err := load(authFilename, &auth); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return auth, nil
}

func (a Auth) Set(authConfig *registry.AuthConfig) error {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}
	addr := authConfig.ServerAddress
	a[addr] = base64.URLEncoding.EncodeToString(buf)
	log.Printf("AUTH SET: Addr: %s, Encoded: %s\n", addr, a[addr])
	return nil
}

func (a Auth) Clear(name string) {
	delete(a, name)
}

func (a Auth) Save() error {
	f, err := configDir().Create(authFilename)
	if err != nil {
		return err
	}
	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(a)
}
