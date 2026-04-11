package main

import (
	"log"

	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
	"jewrexxx-fearlessrevolution-overlewd-cheat/models"
	"jewrexxx-fearlessrevolution-overlewd-cheat/tui"
)

func main() {
	// 1. Initialize our heavily threaded client
	client := api.NewClient("https://prod.api.overlewd.ru")

	// 2. Phase 2: Ingest all Battle Stages into cache locally BEFORE bringing up the UI
	// This ensures dropdown menus will have data to query instantaneously on boot.
	if err := models.FetchStages(client); err != nil {
		log.Fatalf("Critical Failure ingesting JSON stage metadata arrays: %v", err)
	}

	// Phase 2.5: Enrich Stages with human-readable dictionary names!
	models.EnrichStages(client)
	api.EnrichCurrencies(client)

	// 3. Phase 3: Launch UI Interface and trap the OS process
	tui.StartApp()
}
