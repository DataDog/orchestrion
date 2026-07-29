package main

import "os"

func main() {
	source := os.Getenv("TAINT_PATH")
	_, _ = os.Open(string(source[0]))
}
