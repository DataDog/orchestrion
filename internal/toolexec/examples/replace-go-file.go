package examples

import (
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/toolexec/utils"
)

func ExampleReplaceGoFile() {
	args := []string{"/random/compile", "-trimpath", "randompath", "-p", "random", "-o", "/tmp/randomBuild/_pkg_.a", "-importcfg", "/tmp/random/b002/importcfg", "file1.go", "file2.go", "main.go"}
	cmd, err := proxy.ParseCommand(args)
	utils.ExitIfError(err)
	filesReplacer := goFilesReplacer{files: map[string]string{"main.go": "custom-main.go"}}
	cmd.Inject(&filesReplacer)
}

type goFilesReplacer struct {
	files map[string]string
}

func (i goFilesReplacer) InjectCompile(cmd *proxy.CompileCommand) {
	for old, new := range i.files {
		cmd.ReplaceParam(old, new)
	}
}

func (i goFilesReplacer) InjectLink(*proxy.LinkCommand) { /* No-op on link step */ }
