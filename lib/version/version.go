package version

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"git.qur.me/qur/wharf_rat/lib/docker"
)

var versionString = "eyJ2ZXJzaW9uIjoidW5rbm93biIsImdvdmVyc2lvbiI6InVua25vd24ifQo="

type versionInfo struct {
	Version   string
	GoVersion string
	BuildTime time.Time
}

var version versionInfo

func ShowVersion() error {
	fmt.Printf("Version: %s\n", version.Version)
	fmt.Printf("Docker API Version: %s\n", docker.Version())
	fmt.Printf("Go Version: %s\n", version.GoVersion)
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
