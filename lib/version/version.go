package version

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"wharfr.at/wharfrat/lib/docker"
)

var versionString = "eyJ2ZXJzaW9uIjoidjAuOC4yIiwiY29tbWl0IjoiZTIwNjc5MmI4MDMxMjIwNjNiY2ExZGQzN2E5YjlkMjA4NWVkMGI0YyIsImJ1aWxkdGltZSI6IjIwMTgtMTItMDlUMDE6NDY6MjJaIn0K"

type versionInfo struct {
	Version   string
	Commit    string
	BuildTime time.Time
}

var version versionInfo

func ShowVersion() error {
	fmt.Printf("Version: %s\n", version.Version)
	fmt.Printf("Git Commit: %s\n", version.Commit)
	fmt.Printf("Docker API Version: %s\n", docker.Version())
	fmt.Printf("Go Version: %s\n", runtime.Version())
	fmt.Printf("Built: %s\n", version.BuildTime.Local())
	return nil
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
