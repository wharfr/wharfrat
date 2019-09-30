package version

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/docker/docker/api"
)

var versionString = "eyJ2ZXJzaW9uIjoidjAuOS4xIiwiY29tbWl0IjoiYTI1YWRiNWNhYjMyYzcxOGI4NjhmZDAyNjUxZmJmYTA1ZjBhZDFlYyIsImJ1aWxkdGltZSI6IjIwMTktMDktMzBUMDA6Mzg6NDBaIn0K"

type versionInfo struct {
	Version   string
	Commit    string
	BuildTime time.Time
}

var version versionInfo

func ShowVersion() error {
	fmt.Printf("Version: %s\n", version.Version)
	fmt.Printf("Git Commit: %s\n", version.Commit)
	fmt.Printf("Docker API Version: %s\n", api.DefaultVersion)
	fmt.Printf("Go Version: %s\n", runtime.Version())
	fmt.Printf("Built: %s\n", version.BuildTime.Local())
	return nil
}

func Commit() string {
	return version.Commit
}

func init() {
	verData, err := base64.StdEncoding.DecodeString(versionString)
	if err != nil {
		panic("Failed to decode version info: " + err.Error())
	}
	err = json.Unmarshal(verData, &version)
	if err != nil {
		panic("Failed to parse version info: " + err.Error())
	}
}
