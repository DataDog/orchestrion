package orchestrion

import (
	"fmt"
	"io"
	"testing"
)

func TestScanPackageDST(t *testing.T) {
	process := func(fullName string, out io.Reader) {
		fmt.Printf("%s:\n", fullName)
		// write the output
		txt, _ := io.ReadAll(out)
		fmt.Println(string(txt))
	}
	ScanPackage("./cmd/samples", process)
}
