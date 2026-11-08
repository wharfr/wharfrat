package version

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/moby/moby/client"
)

var versionString = "eyJ2ZXJzaW9uIjoidjAuMTUuMC1kZXYiLCJjb21taXQiOiI0ZDA3YWI1MmUyNzNhMDZiOWViZjkyMGI4YmNhMzc1MjQ0MjgzMzFkIiwiYnVpbGR0aW1lIjoiMjAyNi0xMS0wOFQxNTo0NDo1MVoifQo="

type versionInfo struct {
	Version   string
	Commit    string
	BuildTime time.Time
}

var version versionInfo

func ShowVersion() error {
	fmt.Printf("Version: %s\n", version.Version)
	fmt.Printf("Git Commit: %s\n", version.Commit)
	fmt.Printf("Docker Max API Version: %s\n", client.MaxAPIVersion)
	fmt.Printf("Docker Min API Version: %s\n", client.MinAPIVersion)
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
