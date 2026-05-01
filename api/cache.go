package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func getCacheDir() string {
	cacheBase, err := os.UserCacheDir()
	if err != nil {
		cacheBase = os.TempDir()
	}
	targetDir := filepath.Join(cacheBase, "olcheat")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return ""
	}
	return targetDir
}

func GetEnvPath() string {
	return filepath.Join(getCacheDir(), ".env")
}

// EnsureCacheWiped wipes the entire ./olcheat appcache but preserves .env
func EnsureCacheWiped() {
	targetDir := getCacheDir()
	if targetDir != "" {
		envPath := filepath.Join(targetDir, ".env")
		envData, err := os.ReadFile(envPath)

		os.RemoveAll(targetDir)
		os.MkdirAll(targetDir, 0755)

		if err == nil {
			os.WriteFile(envPath, envData, 0644)
		}

		log.Println("[CACHE] Successfully obliterated all cached dictionaries and definitions.")
	}
}

// WipeAuthCredentials completely deletes the .env configuration and unsets active session memory
func WipeAuthCredentials() {
	targetDir := getCacheDir()
	if targetDir != "" {
		envPath := filepath.Join(targetDir, ".env")
		os.Remove(envPath)
	}
	os.Remove(".env")
	os.Unsetenv("BEARER_TOKEN")
	os.Unsetenv("DEVICE_ID")
	os.Unsetenv("USER_AGENT")
	os.Unsetenv("APP_VERSION")
	os.Unsetenv("UNITY_VERSION")
}

// RemoveCacheFile wipes precisely one specific explicit cache file out of the OS target directory natively
func RemoveCacheFile(filename string) {
	targetDir := getCacheDir()
	if targetDir != "" {
		targetFile := filepath.Join(targetDir, filename)
		os.Remove(targetFile)
		log.Printf("[CACHE] Purged cache for variable %s", filename)
	}
}

// SafeForceUpdateCache explicitly overrides cache without deleting existing footprint using HTTP retries
func SafeForceUpdateCache(endpoint string, filename string, client *OverlewdClient, maxRetries int, progressIdx string, onProgress func(string, string)) bool {
	targetDir := getCacheDir()
	if targetDir == "" {
		return false
	}
	targetFile := filepath.Join(targetDir, filename)

	onProgress(progressIdx, fmt.Sprintf("Updating %s file", filename))

	for i := 1; i <= maxRetries; i++ {
		b, err := forceFetch(endpoint, client)
		if err == nil {
			errWrite := os.WriteFile(targetFile, b, 0644)
			if errWrite == nil {
				onProgress(progressIdx, fmt.Sprintf("[green]%s updated[white]", filename))
				return true
			}
		}

		if i < maxRetries {
			onProgress(progressIdx, fmt.Sprintf("[yellow]%s request failed retrying %d/%d[white]", filename, i, maxRetries))
			time.Sleep(2 * time.Second)
		}
	}

	onProgress(progressIdx, fmt.Sprintf("[red]%s failed to update... Retaining locally cached file.[white]", filename))
	return false
}

// LoadOrFetch checks if a payload exists in OS UserCache. If it does not, fetches from the internet
// and permanently caches the JSON byte payload.
func LoadOrFetch(endpoint string, filename string, client *OverlewdClient) ([]byte, error) {
	targetDir := getCacheDir()
	if targetDir == "" {
		return forceFetch(endpoint, client)
	}

	targetFile := filepath.Join(targetDir, filename)

	if _, err := os.Stat(targetFile); err == nil {
		// File exists! Load instantly.
		log.Printf("[CACHE] Loaded %s from cache.", filename)
		b, err := os.ReadFile(targetFile)
		if err == nil {
			return b, nil
		}
	}

	log.Printf("[CACHE] Miss for %s... Downloading from Server...", filename)
	b, err := forceFetch(endpoint, client)
	if err != nil {
		return nil, err
	}

	// Dump cleanly into cache for future boots
	err = os.WriteFile(targetFile, b, 0644)
	if err != nil {
		log.Printf("[CACHE] Warning: Failed to write %s to lock: %v", filename, err)
	}
	return b, nil
}

func forceFetch(endpoint string, client *OverlewdClient) ([]byte, error) {
	req, _ := http.NewRequest("GET", client.BaseURL+endpoint, nil)
	resp, err := client.DoRequestBypassCooldown(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d fetch failed for %s", resp.StatusCode, endpoint)
	}

	return io.ReadAll(resp.Body)
}
