package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type leaderHint struct {
	Leader string `json:"leader"`
}

// RemoteClient talks to the HTTP API and follows leader redirects.
type RemoteClient struct {
	HTTP *http.Client
	Base *url.URL
}

func (rc *RemoteClient) do(method, path string, q url.Values, body io.Reader) (*http.Response, error) {
	u := *rc.Base
	u.Path = path
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	return rc.HTTP.Do(req)
}

func (rc *RemoteClient) withLeader(h leaderHint) {
	if h.Leader == "" {
		return
	}
	leaderHost := h.Leader
	if h, _, ok := strings.Cut(leaderHost, ":"); ok {
		leaderHost = h
	}
	port := rc.Base.Port()
	if port == "" {
		port = "8081"
	}
	b := *rc.Base
	b.Host = leaderHost + ":" + port
	rc.Base = &b
}

func (rc *RemoteClient) Get(key string) (string, error) {
	for retries := 0; retries < 3; retries++ {
		q := url.Values{"key": {key}}
		resp, err := rc.do(http.MethodGet, "/kv", q, nil)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return string(b), nil
		}
		if resp.StatusCode == http.StatusConflict {
			var h leaderHint
			_ = json.NewDecoder(resp.Body).Decode(&h)
			rc.withLeader(h)
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		return "", errors.New(strings.TrimSpace(string(b)))
	}
	return "", fmt.Errorf("leader redirect loop")
}

func (rc *RemoteClient) Put(key, value string) error {
	for retries := 0; retries < 3; retries++ {
		q := url.Values{"key": {key}}
		resp, err := rc.do(http.MethodPut, "/kv", q, strings.NewReader(value))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		if resp.StatusCode == http.StatusConflict {
			var h leaderHint
			_ = json.NewDecoder(resp.Body).Decode(&h)
			rc.withLeader(h)
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		return errors.New(strings.TrimSpace(string(b)))
	}
	return fmt.Errorf("leader redirect loop")
}

func (rc *RemoteClient) Delete(key string) error {
	for retries := 0; retries < 3; retries++ {
		q := url.Values{"key": {key}}
		resp, err := rc.do(http.MethodDelete, "/kv", q, nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		if resp.StatusCode == http.StatusConflict {
			var h leaderHint
			_ = json.NewDecoder(resp.Body).Decode(&h)
			rc.withLeader(h)
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		return errors.New(strings.TrimSpace(string(b)))
	}
	return fmt.Errorf("leader redirect loop")
}

func runRemoteREPL(base string) {
	client := &RemoteClient{HTTP: &http.Client{}}
	u, err := url.Parse(base)
	if err != nil {
		fmt.Printf("Invalid --server URL: %v\n", err)
		os.Exit(1)
	}
	client.Base = u

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
		switch parts[0] {
		case "help":
			printHelp()
		case "get":
			if len(parts) != 2 {
				fmt.Println("Usage: get <key>")
				continue
			}
			val, err := client.Get(parts[1])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			fmt.Printf("%s\n", val)
		case "put":
			if len(parts) < 3 {
				fmt.Println("Usage: put <key> <value>")
				continue
			}
			if err := client.Put(parts[1], strings.Join(parts[2:], " ")); err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			fmt.Println("OK")
		case "delete":
			if len(parts) != 2 {
				fmt.Println("Usage: delete <key>")
				continue
			}
			if err := client.Delete(parts[1]); err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			fmt.Println("OK")
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Printf("Unknown command: %s\n", parts[0])
			printHelp()
		}
	}
}

func printHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  get <key>              - Get a value (leader, linearizable)")
	fmt.Println("  put <key> <value>      - Put a key-value pair (replicated)")
	fmt.Println("  delete <key>           - Delete a key (replicated)")
	fmt.Println("  help                   - Show this help message")
	fmt.Println("  exit, quit             - Exit the program")
}
