package utils

import (
	"log"
	"os/exec"
	"strings"
)

// ExitIfError calls os.Exit(1) if err is not nil
func ExitIfError(err error) {
	if err == nil {
		return
	}
	log.Fatalln(err)
}

// GoBuild builds in provided dir and returns the work directory's true path
// The underlying go build always:
// - preserves the go work directory (-work)
// - forces recompilation of all dependencies (-a)
func GoBuild(dir string, args ...string) (string, error) {
	args = append([]string{"build", "-work", "-a"}, args...)
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
