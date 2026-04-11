package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/elazarl/goproxy"
)

func waitAndExit() {
	fmt.Println("\nPress Enter to exit...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	os.Exit(1)
}

func checkCaCert() {
	if _, err := os.Stat("ca.pem"); os.IsNotExist(err) {
		cert := goproxy.GoproxyCa.Certificate[0]
		out, err := os.Create("ca.pem")
		if err != nil {
			log.Printf("Error creating ca.pem: %v", err)
			waitAndExit()
		}
		defer out.Close()
		if err := pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); err != nil {
			log.Printf("Error encoding ca.pem: %v", err)
			waitAndExit()
		}
		log.Println("----------------- ACTION REQUIRED -----------------")
		log.Println("I just generated 'ca.pem' in the current directory.")
		log.Println("You MUST install and trust this as a root CA on your")
		log.Println("device or emulator. Without it, the game's HTTPS")
		log.Println("traffic will fail and we won't capture anything.")
		log.Println("After installing it, start this proxy again.")
		waitAndExit()
	}
}

var savedMetadata bool

func StartProxy(port string) {
	checkCaCert()

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	// Dump Intercepted Requests and Harvest Metadata
	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			host := r.Host
			if strings.Contains(host, "cdn.overlewd.com") || strings.Contains(host, "prod.api.overlewd.ru") {
				path := r.URL.Path
				if path == "" {
					path = "/"
				}

				if !savedMetadata {
					version := r.Header.Get("Version")
					unityVer := r.Header.Get("X-Unity-Version")
					ua := r.Header.Get("User-Agent")

					if version != "" && unityVer != "" && ua != "" {
						upsertEnv("APP_VERSION", version)
						upsertEnv("UNITY_VERSION", unityVer)
						upsertEnv("USER_AGENT", ua)
						savedMetadata = true
					}
				}
			}
			return r, nil
		})

	proxy.OnResponse().DoFunc(
		func(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			if r == nil || r.Request == nil {
				return r
			}

			// Intercept `/auth/login` responses
			if strings.Contains(r.Request.URL.Path, "/auth/login") {
				bodyBytes, err := io.ReadAll(r.Body)
				if err == nil {
					_ = r.Body.Close()
					r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

					var respData struct {
						AccessToken string `json:"accessToken"`
					}
					if err := json.Unmarshal(bodyBytes, &respData); err == nil && respData.AccessToken != "" {
						log.Printf("[DEBUG] Successfully parsed accessToken.")
						upsertEnv("BEARER_TOKEN", respData.AccessToken)
					} else {
						log.Printf("[ERROR] Failed to unmarshal /auth/login accessToken: %v", err)
					}
					log.Printf("===================================================")
				} else {
					log.Printf("[ERROR] Failed to read response body for /auth/login: %v", err)
				}
			}

			return r
		})

	log.Printf("Starting MITM proxy on :%s\n", port)
	if err := http.ListenAndServe(":"+port, proxy); err != nil {
		log.Printf("Failed to start proxy on port %s (is the port already in use?): %v", port, err)
		waitAndExit()
	}
}

func upsertEnv(key, value string) {
	b, _ := os.ReadFile(".env")
	lines := strings.Split(string(b), "\n")

	var out []string
	found := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+"=") {
			out = append(out, key+"="+value)
			found = true
		} else if line != "" {
			out = append(out, line)
		}
	}
	if !found {
		out = append(out, key+"="+value)
	}

	content := strings.Join(out, "\n") + "\n"
	err := os.WriteFile(".env", []byte(content), 0644)
	if err != nil {
		log.Printf("Error writing %s to .env: %v", key, err)
	} else {
		log.Printf("[SUCCESS] Saved %s to .env", key)
	}
}
