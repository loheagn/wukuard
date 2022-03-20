package main

import (
	"os"
	"strings"
)

func main() {
	args := os.Args
	if len(args) < 3 {
		panic("no enough args")
	}

	switch strings.ToLower(args[1]) {
	case "server":
		confPath := args[2]
		serverMain(confPath)
	case "client":
		serverAddr := args[2]
		inputInterfaceName := ""
		if len(args) > 3 {
			inputInterfaceName = args[3]
		}
		clientMain(serverAddr, inputInterfaceName)
	default:
		panic("unknown action")
	}
}
