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
	"sync"

	"github.com/elazarl/goproxy"
)

func waitAndExit() {
	fmt.Println("\nPress Enter to exit...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	os.Exit(1)
}

func GenerateCertificateNatively() {
	if _, err := os.Stat("ca.cer"); os.IsNotExist(err) {
		cert := goproxy.GoproxyCa.Certificate[0]
		out, err := os.Create("ca.cer")
		if err != nil {
			log.Printf("Error creating ca.cer: %v", err)
			return
		}
		defer out.Close()
		if err := pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); err != nil {
			log.Printf("Error encoding ca.cer: %v", err)
			return
		}
		log.Println("[INFO] Successfully Generated 'ca.cer' in the execution folder.")
	} else {
		log.Println("[INFO] 'ca.cer' already exists natively.")
	}
}

var savedMetadata bool

func StartProxyRoutine(port string, onSuccess func(string)) {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: proxy,
	}

	// Dump Intercepted Requests and Harvest Metadata
	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			host := r.Host
			if strings.Contains(host, "cdn.overlewd.com") || strings.Contains(host, "prod.api.overlewd.ru") {
				if !savedMetadata {
					version := r.Header.Get("Version")
					unityVer := r.Header.Get("X-Unity-Version")
					ua := r.Header.Get("User-Agent")
					deviceId := r.Header.Get("X-Device-Id")
					if deviceId == "" {
						deviceId = r.Header.Get("Device-Id")
					}

					if version != "" && unityVer != "" && ua != "" {
						upsertEnv("APP_VERSION", version)
						upsertEnv("UNITY_VERSION", unityVer)
						upsertEnv("USER_AGENT", ua)
						if deviceId != "" {
							upsertEnv("DEVICE_ID", deviceId)
						}
						savedMetadata = true
					}
				}
			}
			return r, nil
		})

	var authSync sync.Once

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
						authSync.Do(func() {
							upsertEnv("BEARER_TOKEN", respData.AccessToken)

							if onSuccess != nil {
								go onSuccess(respData.AccessToken)
							}
							// Shutdown server gracefully
							go server.Close()
						})
					} else {
						log.Printf("[ERROR] Failed to unmarshal /auth/login accessToken: %v", err)
					}

				} else {
					log.Printf("[ERROR] Failed to read response body for /auth/login: %v", err)
				}
			}

			return r
		})

	log.Printf("Starting MITM proxy on :%s\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Failed to start proxy on port %s (is the port already in use?): %v", port, err)
		waitAndExit()
	}
}

func upsertEnv(key, value string) {
	envPath := GetEnvPath()
	b, _ := os.ReadFile(envPath)
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
	err := os.WriteFile(envPath, []byte(content), 0644)
	if err != nil {
		log.Printf("Error writing %s to .env: %v", key, err)
	} else {
		log.Printf("[SUCCESS] Saved %s to .env", key)
	}
}
