package main

import (
	"log"

	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
)

func main() {
	log.Println("Initializing MITM Proxy (Phase 0)...")
	api.StartProxy("8080")
}
