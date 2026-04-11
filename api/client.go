package api

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

type OverlewdClient struct {
	BaseURL      string
	HTTPClient   *http.Client
	BearerToken  string
	AppVersion   string
	UnityVersion string
	UserAgent    string
}

var (
	globalLock    sync.RWMutex
	lastErrorTime time.Time
)

// NewClient initializes the OverlewdClient using credentials stored in .env
func NewClient(baseURL string) *OverlewdClient {
	_ = godotenv.Load(GetEnvPath()) // Loads .env into process directly from cache

	return &OverlewdClient{
		BaseURL:      baseURL,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		BearerToken:  os.Getenv("BEARER_TOKEN"),
		AppVersion:   os.Getenv("APP_VERSION"),
		UnityVersion: os.Getenv("UNITY_VERSION"),
		UserAgent:    os.Getenv("USER_AGENT"),
	}
}

func (c *OverlewdClient) prepareRequest(req *http.Request) {
	if c.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.BearerToken)
	}
	if c.AppVersion != "" {
		req.Header.Set("Version", c.AppVersion)
	}
	if c.UnityVersion != "" {
		req.Header.Set("X-Unity-Version", c.UnityVersion)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	req.Header.Set("Accept", "*/*")
}

// DoRequest wraps http.Client.Do but dynamically appends tokens, spoofed headers, and acts on 502/504 errors globally.
func (c *OverlewdClient) DoRequest(req *http.Request) (*http.Response, error) {
	globalLock.RLock() // Crucial: This blocks if a 10s cooldown is currently locking operations.
	c.prepareRequest(req)
	resp, err := c.HTTPClient.Do(req)
	globalLock.RUnlock() // Release the reader lock immediately once our packet lands

	// If HTTP failure is 502 or 504, we immediately pause the world
	if err == nil && (resp.StatusCode == 502 || resp.StatusCode == 504) {
		triggerGlobalCooldown()
	}

	return resp, err
}

// DoRequestBypassCooldown wraps DoRequest precisely but purposefully drops the global 10s cooldown block, perfect for Caches picking up slack natively.
func (c *OverlewdClient) DoRequestBypassCooldown(req *http.Request) (*http.Response, error) {
	globalLock.RLock()
	c.prepareRequest(req)
	resp, err := c.HTTPClient.Do(req)
	globalLock.RUnlock()

	return resp, err
}

func triggerGlobalCooldown() {
	globalLock.Lock() // Seize absolute control, causing all new DoRequest calls to hang at RLock()
	defer globalLock.Unlock()

	// If a cooldown triggered recently (< 10s ago) from another thread bumping into a 502, just skip.
	if time.Since(lastErrorTime) < 10*time.Second {
		return
	}

	log.Println("[WARNING] 502/504 Gateway Error detected from server! Suspending all active outbound requests for 10 seconds...")
	time.Sleep(10 * time.Second)
	lastErrorTime = time.Now()
	log.Println("[INFO] Cooldown resolved! Automatically resuming suspended concurrent requests.")
}
