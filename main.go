package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  mock-server mock <openapi.yaml> [--port 3000] [--data data.json]")
		os.Exit(1)
	}

	cmd := os.Args[1]
	if cmd != "mock" {
		log.Fatalf("unknown command: %s", cmd)
	}

	openapiFile := os.Args[2]

	fs := flag.NewFlagSet("mock", flag.ExitOnError)
	port := fs.Int("port", 3000, "server port")
	dataFile := fs.String("data", "data.json", "data storage file")

	_ = fs.Parse(os.Args[3:])

	startServer(openapiFile, *dataFile, *port)
}
