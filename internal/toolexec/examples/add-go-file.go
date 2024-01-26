package examples

import (
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/toolexec/utils"
)

func ExampleAddGoFiles() {
	args := []string{"/random/compile", "-trimpath", "randompath", "-p", "random", "-o", "/tmp/randomBuild/_pkg_.a", "-importcfg", "/tmp/random/b002/importcfg", "file1.go", "file2.go", "main.go"}
	cmd, err := proxy.ParseCommand(args)
	utils.ExitIfError(err)
	filesAdder := goFilesAdder{files: []string{"added1.go", "added2.go"}}
	cmd.Inject(&filesAdder)
}

type goFilesAdder struct {
	files []string
}

func (i goFilesAdder) InjectCompile(cmd *proxy.CompileCommand) {
	cmd.AddGoFiles(i.files...)
}

func (i goFilesAdder) InjectLink(*proxy.LinkCommand) { /* No-op on link step */ }
