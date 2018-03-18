package vc

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

func Branch(path string) (string, error) {
	log.Printf("VC BRANCH: %s", path)
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := exec.Command("git", "-C", path, "symbolic-ref", "--short", "HEAD")
	cmd.Stdout = buf
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git failed (%s): %s", err, errBuf)
	}
	return strings.TrimSpace(buf.String()), nil
}
