# Overlewd Headless Client

## Initial Token Setup (Phase 0)
Because this application bypasses standard game client execution to interact directly with the REST API, it requires an active `BEARER_TOKEN` (which expires every 30 days). 

Whenever you need a new token, follow these steps to capture one using the built-in MITM sniffer:
1. Run `go run ./cmd/main.go` from the project root.
2. The proxy will generate a `ca.pem` certificate in the root folder. You must install and trust this on your Android Emulator or Windows PC as a Trusted Root CA.
3. Keep the proxy running in the background. Note: It listens on port `8080`.
4. Open your OS or Emulator's Wi-Fi / Connection settings. Set your Proxy to `Manual`, IP to your Host's Local IP address (`127.0.0.1` or `192.168.1.X`), and Port to `8080`.
5. Open the game. The proxy will watch the traffic and, when the `/auth/login` request concludes, securely extract the fresh token and write it exactly into the `.env` file!
6. Remember to turn off your proxy settings once the console logs `[SUCCESS] Saved BEARER_TOKEN to .env`.
