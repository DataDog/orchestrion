package main

import "os"

func main() {
	source := os.Getenv("TAINT_PATH")
	value := source[0]
	_, _ = os.Open(string(value))
}
