package main

import (
	"log"

	"github.com/bowmanmike/playlistgen/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		log.Fatal(err)
	}
}
