package processors

import (
	"log"

	"github.com/datadog/orchestrion/internal/toolexec/proxy"
)

type GoFileSwapper struct {
	// Key: file to replace
	// Value: file to replace with
	swapMap map[string]string
}

func NewGoFileSwapper(swapMap map[string]string) GoFileSwapper {
	return GoFileSwapper{
		swapMap: swapMap,
	}
}

func (s *GoFileSwapper) ProcessCompile(cmd *proxy.CompileCommand) {
	if cmd.Stage() != "b001" {
		return
	}
	log.Printf("[%s] Replacing Go files", cmd.Stage())

	for old, new := range s.swapMap {
		if err := cmd.ReplaceFile(old, new); err != nil {
			log.Printf("couldn't replace param: %v", err)
		} else {
			log.Printf("====> Replacing %s by %s", old, new)
		}
	}
}
