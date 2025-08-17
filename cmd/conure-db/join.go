package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type joinRequest struct {
	ID       string `json:"ID"`
	RaftAddr string `json:"RaftAddr"`
}

type leaderHintResp struct {
	Leader string `json:"leader"`
}

func parseSeeds() []string {
	if v := os.Getenv("CONURE_SEEDS"); v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []string{"http://conure-0.conure-hs:8081"}
}

// joinCluster attempts to join the cluster by posting to seeds and following leader redirects.
func joinCluster(nodeID, raftAddr string, backoff time.Duration, maxRetries int) {
	logger := log.New(os.Stdout, fmt.Sprintf("[JOIN %s] ", nodeID), log.LstdFlags)
	
	seeds := parseSeeds()
	client := &http.Client{Timeout: 10 * time.Second} // Increased timeout for k8s
	if backoff <= 0 {
		backoff = 2 * time.Second
	}
	if maxRetries <= 0 {
		maxRetries = 0 // 0 = infinite
	}

	logger.Printf("Starting cluster join process with seeds: %v", seeds)
	
	// Check if already part of cluster before attempting to join
	if isAlreadyInCluster(client, seeds, nodeID, logger) {
		logger.Printf("Node %s is already part of the cluster, skipping join", nodeID)
		return
	}

	attempt := 0
	currentBackoff := backoff
	
	for {
		joinSuccessful := false
		
		for _, seed := range seeds {
			attempt++
			logger.Printf("Join attempt %d to seed %s", attempt, seed)
			
			// First check if seed is healthy
			if !isSeedHealthy(client, seed, logger) {
				logger.Printf("Seed %s is not healthy, trying next", seed)
				continue
			}
			
			// Validate URL
			u, err := url.Parse(seed)
			if err != nil {
				logger.Printf("Invalid seed URL %s: %v", seed, err)
				continue
			}
			u.Path = "/join"
			
			jr := joinRequest{ID: nodeID, RaftAddr: raftAddr}
			bodyBytes, err := json.Marshal(jr)
			if err != nil {
				logger.Printf("Failed to marshal join request: %v", err)
				continue
			}
			
			req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
			if err != nil {
				logger.Printf("Failed to create request: %v", err)
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			
			resp, err := client.Do(req)
			if err != nil {
				logger.Printf("Failed to contact seed %s: %v", seed, err)
				continue
			}
			
			switch resp.StatusCode {
			case http.StatusOK:
				logger.Printf("Successfully joined cluster via %s", seed)
				resp.Body.Close()
				return
				
			case http.StatusConflict:
				// Follow leader hint
				var h leaderHintResp
				if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
					logger.Printf("Failed to decode leader hint: %v", err)
					resp.Body.Close()
					continue
				}
				resp.Body.Close()
				
				if h.Leader != "" {
					logger.Printf("Redirecting to leader: %s", h.Leader)
					if tryJoinLeader(client, h.Leader, jr, logger) {
						logger.Printf("Successfully joined cluster via leader %s", h.Leader)
						return
					}
				}
				
			case http.StatusServiceUnavailable, http.StatusInternalServerError:
				logger.Printf("Seed %s is temporarily unavailable (status %d)", seed, resp.StatusCode)
				resp.Body.Close()
				
			default:
				logger.Printf("Unexpected response from %s: status %d", seed, resp.StatusCode)
				resp.Body.Close()
			}
		}
		
		if !joinSuccessful {
			if maxRetries > 0 && attempt >= maxRetries {
				logger.Printf("Exhausted all join attempts (%d), giving up", attempt)
				return
			}
			
			logger.Printf("Join round failed, sleeping for %v before retrying", currentBackoff)
			time.Sleep(currentBackoff)
			
			// Exponential backoff with jitter, max 30 seconds
			currentBackoff = time.Duration(float64(currentBackoff) * 1.5)
			if currentBackoff > 30*time.Second {
				currentBackoff = 30 * time.Second
			}
		}
	}
}

// isAlreadyInCluster checks if this node is already part of the cluster
func isAlreadyInCluster(client *http.Client, seeds []string, nodeID string, logger *log.Logger) bool {
	for _, seed := range seeds {
		u, err := url.Parse(seed)
		if err != nil {
			continue
		}
		u.Path = "/raft/config"
		
		resp, err := client.Get(u.String())
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			var config struct {
				Servers []struct {
					ID string `json:"id"`
				} `json:"servers"`
			}
			
			if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
				continue
			}
			
			for _, server := range config.Servers {
				if server.ID == nodeID {
					return true
				}
			}
		}
	}
	return false
}

// isSeedHealthy checks if a seed is responding to health checks
func isSeedHealthy(client *http.Client, seed string, logger *log.Logger) bool {
	u, err := url.Parse(seed)
	if err != nil {
		return false
	}
	u.Path = "/status"
	
	resp, err := client.Get(u.String())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == http.StatusOK
}

// tryJoinLeader attempts to join via the leader directly
func tryJoinLeader(client *http.Client, leader string, jr joinRequest, logger *log.Logger) bool {
	leaderURL := fmt.Sprintf("http://%s/join", leader)
	bodyBytes, err := json.Marshal(jr)
	if err != nil {
		logger.Printf("Failed to marshal join request for leader: %v", err)
		return false
	}
	
	req, err := http.NewRequest(http.MethodPost, leaderURL, bytes.NewReader(bodyBytes))
	if err != nil {
		logger.Printf("Failed to create leader request: %v", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("Failed to contact leader %s: %v", leader, err)
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == http.StatusOK
}
