package api

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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
	DeviceID     string
}

var (
	globalLock    sync.RWMutex
	lastErrorTime time.Time
	OnSessionInvalidated func()
)

// NewClient initializes the OverlewdClient using credentials stored in .env
func NewClient(baseURL string) *OverlewdClient {
	_ = godotenv.Load(GetEnvPath()) // Loads .env into process directly from cache

	return &OverlewdClient{
		BaseURL:      baseURL,
		HTTPClient:   &http.Client{Timeout: 180 * time.Second},
		BearerToken:  os.Getenv("BEARER_TOKEN"),
		AppVersion:   os.Getenv("APP_VERSION"),
		UnityVersion: os.Getenv("UNITY_VERSION"),
		UserAgent:    os.Getenv("USER_AGENT"),
		DeviceID:     os.Getenv("DEVICE_ID"),
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
	if c.DeviceID != "" {
		req.Header.Set("X-Device-Id", c.DeviceID)
		req.Header.Set("Device-Id", c.DeviceID)
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

	checkSessionInvalidation(resp, err)

	return resp, err
}

// DoRequestBypassCooldown wraps DoRequest precisely but purposefully drops the global 10s cooldown block, perfect for Caches picking up slack natively.
func (c *OverlewdClient) DoRequestBypassCooldown(req *http.Request) (*http.Response, error) {
	globalLock.RLock()
	c.prepareRequest(req)
	resp, err := c.HTTPClient.Do(req)
	globalLock.RUnlock()

	checkSessionInvalidation(resp, err)

	return resp, err
}

// checkSessionInvalidation inspects non-2xx responses for session death signals.
// Only reads the body on error status codes to avoid unnecessary allocations on healthy requests.
func checkSessionInvalidation(resp *http.Response, err error) {
	if err != nil || resp == nil || resp.Body == nil {
		return
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return
	}

	importBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	bodyLower := strings.ToLower(string(importBytes))
	if resp.StatusCode == 401 || strings.Contains(bodyLower, "multiple devices") || strings.Contains(bodyLower, "jwt expired") {
		if OnSessionInvalidated != nil {
			OnSessionInvalidated()
		}
	}

	// Restore body so callers can still read it
	resp.Body = io.NopCloser(strings.NewReader(string(importBytes)))
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
