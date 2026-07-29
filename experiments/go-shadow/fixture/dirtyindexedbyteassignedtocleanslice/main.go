package main

import "os"

func main() {
	source := os.Getenv("TAINT_PATH")
	clean := make([]byte, 1)
	clean[0] = source[0]
	_, _ = os.Open(string(clean))
}
