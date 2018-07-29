package config

import (
	"log"
	"os"
	"sync"

	"github.com/burntsushi/toml"
)

type LocalConfig struct {
	DockerURL string `toml:"docker-url"`
	SetupPrep string `toml:"setup-prep"`
	SetupPre  string `toml:"setup-pre"`
	SetupPost string `toml:"setup-post"`
	Tarballs  map[string]string
	Env       map[string]string
	path      string
}

const localName = "config.toml"

var (
	localConfig LocalConfig
	localOnce   sync.Once
)

func loadLocal() {
	f, err := configDir().Open(localName)
	if os.IsNotExist(err) {
		return
	} else if err != nil {
		log.Printf("Failed to open local config: ", err)
		return
	}
	defer f.Close()
	md, err := toml.DecodeReader(f, &localConfig)
	if err != nil {
		log.Printf("Failed to load local config: ", err)
		return
	}
	localConfig.path = f.Name()
	log.Printf("Unknown config keys: %s", md.Undecoded())
}

func Local() *LocalConfig {
	localOnce.Do(loadLocal)
	return &localConfig
}

func (l *LocalConfig) Path() string {
	return l.path
}
