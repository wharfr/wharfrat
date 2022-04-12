package self

import (
	"errors"
)

func GetLinux() ([]byte, error) {
	return linuxData, nil
}

func Native() (string, error) {
	return "", errors.New("not implemented")
}
