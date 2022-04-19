package self

import (
	"fmt"
	"io/ioutil"
	"os"
)

func GetLinux() ([]byte, error) {
	self, err := os.Open("/proc/self/exe")
	if err != nil {
		return nil, fmt.Errorf("failed to get self: %w", err)
	}
	defer self.Close()
	selfData, err := ioutil.ReadAll(self)
	if err != nil {
		return nil, fmt.Errorf("failed to read self: %w", err)
	}
	return selfData, nil
}

var HomeMount = []string{
	"/home:/home",
}
