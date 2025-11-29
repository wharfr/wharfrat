package config

import (
	"log"
	"os"
	"regexp"
	"sync"

	"github.com/BurntSushi/toml"
)

type LocalSetup struct {
	Project   string            `toml:"project"`
	Crate     string            `toml:"crate"`
	SetupPrep string            `toml:"setup-prep"`
	SetupPre  string            `toml:"setup-pre"`
	SetupPost string            `toml:"setup-post"`
	Tarballs  map[string]string `toml:"tarballs"`
	Env       map[string]string `toml:"env"`
	project   *regexp.Regexp
	crate     *regexp.Regexp
}

type LocalConfig struct {
	DockerURL string       `toml:"docker-url"`
	AutoClean bool         `toml:"auto-clean"`
	Setups    []LocalSetup `toml:"setups"`
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
		log.Printf("Failed to open local config: %s", err)
		return
	}
	defer f.Close()

	md, err := toml.NewDecoder(f).Decode(&localConfig)
	if err != nil {
		log.Printf("Failed to load local config: %s", err)
		return
	}

	localConfig.path = f.Name()

	log.Printf("Unknown config keys: %s", md.Undecoded())
}

func Local() *LocalConfig {
	localOnce.Do(loadLocal)
	return &localConfig
}

func (l *LocalConfig) Setup(crate *Crate) ([]*LocalSetup, error) {
	projects := make([]*LocalSetup, 0, len(l.Setups))
	project := crate.ProjectPath()

	for i, setup := range l.Setups {
		if setup.project == nil {
			if setup.Project == "" {
				setup.Project = ".*"
			}

			r, err := regexp.Compile(setup.Project)
			if err != nil {
				return nil, err
			}

			setup.project = r
		}

		if setup.crate == nil {
			if setup.Crate == "" {
				setup.Crate = ".*"
			}

			r, err := regexp.Compile(setup.Crate)
			if err != nil {
				return nil, err
			}

			setup.crate = r
		}

		if !setup.project.MatchString(project) {
			continue
		}

		if !setup.crate.MatchString(crate.Name()) {
			continue
		}

		projects = append(projects, &l.Setups[i])
	}

	return projects, nil
}

func (l *LocalConfig) Path() string {
	return l.path
}
