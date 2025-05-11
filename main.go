package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/conure-db/conure-db/db"
)

const (
	defaultDBPath = "conure.db"
)

func main() {
	fmt.Println("Conure DB - B-tree based key-value store with copy-on-write")
	fmt.Println("Type 'help' for available commands")

	// Open the database
	database, err := db.Open(defaultDBPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Start the REPL
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]
		switch cmd {
		case "help":
			printHelp()
		case "get":
			if len(parts) != 2 {
				fmt.Println("Usage: get <key>")
				continue
			}
			key := []byte(parts[1])
			value, err := database.Get(key)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			fmt.Printf("%s\n", value)
		case "put":
			if len(parts) < 3 {
				fmt.Println("Usage: put <key> <value>")
				continue
			}
			key := []byte(parts[1])
			value := []byte(strings.Join(parts[2:], " "))
			if err := database.Put(key, value); err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			fmt.Println("OK")
		case "delete":
			if len(parts) != 2 {
				fmt.Println("Usage: delete <key>")
				continue
			}
			key := []byte(parts[1])
			if err := database.Delete(key); err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			fmt.Println("OK")
		case "sync":
			if err := database.Sync(); err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			fmt.Println("Database synced to disk")
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Printf("Unknown command: %s\n", cmd)
			printHelp()
		}
	}
}

func printHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  get <key>              - Get a value")
	fmt.Println("  put <key> <value>      - Put a key-value pair")
	fmt.Println("  delete <key>           - Delete a key")
	fmt.Println("  sync                   - Sync the database to disk")
	fmt.Println("  help                   - Show this help message")
	fmt.Println("  exit, quit             - Exit the program")
}
