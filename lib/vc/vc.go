package vc

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

func Branch(path string) (string, error) {
	log.Printf("VC BRANCH: %s", path)
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = path
	cmd.Stdout = buf
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git failed (%s): %s", err, errBuf)
	}
	return strings.TrimSpace(buf.String()), nil
}

func BranchedFile(path, branch string) (string, error) {
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)
	log.Printf("VC BRANCHED FILE: %s", path)
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := exec.Command("git", "show", branch+":./"+fileName)
	cmd.Dir = dirPath
	cmd.Stdout = buf
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git failed (%s): %s", err, errBuf)
	}
	return buf.String(), nil
}

func KnownFile(path, branch string) bool {
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)
	log.Printf("VC BRANCHED FILE: %s", path)
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := exec.Command("git", "cat-file", "-t", branch+":./"+fileName)
	cmd.Dir = dirPath
	cmd.Stdout = buf
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		log.Printf("git failed (%s): %s", err, errBuf)
	}
	return strings.TrimSpace(buf.String()) == "blob"
}
