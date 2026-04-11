package main

import (
	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
	"jewrexxx-fearlessrevolution-overlewd-cheat/tui"
)

func main() {
	for {
		tui.NeedsRestart = false
		// 1. Initialize our heavily threaded client
		client := api.NewClient("https://prod.api.overlewd.ru")

		// Phase 2: Launch UI Interface, trap OS thread!
		tui.StartApp(client)
		
		if !tui.NeedsRestart {
			// Clean user-directed shut down
			break
		}
	}
}
