package main

import (
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		panic("no enough args")
	}
	switch strings.ToLower(os.Args[1]) {
	case "server":
		serverMain()
	case "client":
		clientMain()
	default:
		panic("unknown action")
	}
}
