package examples

import (
	"log"

	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/toolexec/utils"
)

func ExampleListGofiles() {
	args := []string{"/random/compile", "-trimpath", "randompath", "-p", "random", "-o", "/tmp/randomBuild/_pkg_.a", "-importcfg", "/tmp/random/b002/importcfg", "file1.go", "file2.go", "main.go"}
	cmd, err := proxy.ParseCommand(args)
	utils.ExitIfError(err)
	proxy.ProcessCommand(cmd, InjectCompile)
}

func InjectCompile(cmd *proxy.CompileCommand) {
	for _, f := range cmd.GoFiles() {
		log.Println(f)
	}
}
