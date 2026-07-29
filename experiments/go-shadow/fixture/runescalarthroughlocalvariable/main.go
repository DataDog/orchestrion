package main

import "os"

func main() {
	source := os.Getenv("TAINT_PATH")
	value := rune(source[0])
	_, _ = os.Open(string(value))
}
