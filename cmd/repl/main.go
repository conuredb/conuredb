package main

import (
	"flag"
	"fmt"
)

func main() {
	var serverFlag = flag.String("server", "http://127.0.0.1:8081", "HTTP base URL for the server (replicated mode)")
	flag.Parse()

	fmt.Println("Conure DB - B-tree based key-value store with copy-on-write")
	fmt.Println("Type 'help' for available commands")
	fmt.Printf("Using remote server: %s\n", *serverFlag)
	runRemoteREPL(*serverFlag)
}
