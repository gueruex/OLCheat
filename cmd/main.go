package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
	"jewrexxx-fearlessrevolution-overlewd-cheat/tui"
)

func main() {
	for {
		tui.NeedsRestart = false
		
		// Setup explicit unhandled trace dump catching completely dodging TTY clears natively!
		defer func() {
			if r := recover(); r != nil {
				errStr := fmt.Sprintf("FATAL CRASH TRACE:\n%v\n\nStack:\n%s", r, debug.Stack())
				os.WriteFile("crash_dump.log", []byte(errStr), 0644)
				log.Println("A fatal error occurred! Please check crash_dump.log")
			}
		}()

		// 1. Initialize our heavily threaded client
		client := api.NewClient("https://prod.api.overlewd.ru")

		// Phase 2: Launch UI Interface, trap OS thread!
		tui.StartApp(client)
		
		if !tui.NeedsRestart {
			// Clean user-directed shut down
			os.WriteFile("graceful_shutdown.log", []byte(fmt.Sprintf("Application structurally exited cleanly! NeedsRestart=false. Is the terminal silently closing the channel natively?")), 0644)
			break
		}
	}
}
