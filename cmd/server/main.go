package main

import (
	"os"

	"cloudfunction233-server/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
