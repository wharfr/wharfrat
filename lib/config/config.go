package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/shibukawa/configdir"
)

type notFound string

func (nf notFound) Error() string {
	return string(nf)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func find(start, name string) (string, error) {
	fp, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(fp, name)
		if exists(path) {
			return path, nil
		}
		if fp == "/" {
			break
		}
		fp = filepath.Dir(fp)
	}
	return "", NotFound
}

func configDir() *configdir.Config {
	configDirs := configdir.New("", "wharfrat")

	folders := configDirs.QueryFolders(configdir.Global)
	log.Printf("CONFIG FOLDERS: %v", folders)
	return folders[0]
}

func save(filename string, data interface{}) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return configDir().WriteFile(filename, content)
}

func load(filename string, data interface{}) error {
	content, err := configDir().ReadFile(filename)
	if err != nil {
		return err
	}

	return json.Unmarshal(content, data)
}
