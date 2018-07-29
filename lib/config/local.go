package config

import (
	"log"
	"os"
	"sync"

	"github.com/burntsushi/toml"
)

type LocalConfig struct {
	DockerURL string `toml:"docker-url"`
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
	log.Printf("Unknown config keys: %s", md.Undecoded())
}

func Local() *LocalConfig {
	localOnce.Do(loadLocal)
	return &localConfig
}
