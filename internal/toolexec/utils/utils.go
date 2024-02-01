package utils

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

// ExitIfError calls os.Exit(1) if err is not nil
func ExitIfError(err error) {
	if err != nil {
		log.Printf("%v", err)
		os.Exit(1)
	}
}

// GoBuild builds in provided dir and returns the work directory's true path
func GoBuild(dir string, args ...string) (string, error) {
	args = append([]string{"build", "-work", "-a", "-p", "1"}, args...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	log.Println(string(out))
	if err != nil {
		return "", err
	}

	// Extract work dir from output
	wDir := strings.Split(string(out), "=")[1]
	wDir = strings.TrimSuffix(wDir, "\n")

	return wDir, nil
}