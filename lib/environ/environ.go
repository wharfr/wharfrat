package environ

import (
	"fmt"
	"os"

	"wharfr.at/wharfrat/lib/config"
)

var (
	inContainer   bool
	containerName string
)

// InContainer returns true if wharfrat has been run inside a wharfrat
// container. This is intended to allow changing the behaviour when running
// inside a container.
func InContainer() bool {
	return inContainer
}

// ContainerName returns the name of the current container if we are inside a
// wharfrat container, or an empty string otherwise
func ContainerName() string {
	return containerName
}

// Exec runs the given command. If user is not empty, then the command will be
// run as that user, and if workdir is not empty then it will be run in that
// directory. ret is the return code of running the command, unless err != nil,
// in which case execution failed, and err contains the reason.
func Exec(args []string, crate *config.Crate, user, workdir string) (ret int, err error) {
	return 1, fmt.Errorf("environ.Exec Not Implemented")
}

func init() {
	inContainer = os.Getenv("WHARFRAT_ID") != ""

	containerName = os.Getenv("WHARFRAT_NAME")
}
