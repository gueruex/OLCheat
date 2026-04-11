package tui

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
	"jewrexxx-fearlessrevolution-overlewd-cheat/models"
)

var (
	App      *tview.Application
	LogView  *tview.TextView
	TabPages *tview.Pages
)

// customLogger intercepts standard logs and pushes them linearly to the LogView pane
type customLogger struct{}

func (cl *customLogger) Write(p []byte) (n int, err error) {
	if LogView != nil && App != nil {
		// Log pane strictly appended to
		str := string(p)
		
		// Force the app to redraw the screen upon each newly emitted log
		// And ensure appending to LogView is thread-safe!
		go App.QueueUpdateDraw(func() {
			fmt.Fprintf(LogView, "%s", str)
			LogView.ScrollToEnd()
		})
	}
	return len(p), nil
}

func StartApp() {
	App = tview.NewApplication()

	// 1. Setup the shared Log output pane
	LogView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	LogView.SetBorder(true).SetTitle(" Operation Logs (Auto-Scroll) ")

	// Hook the native standard logger to write directly into our UI pane ONLY!
	log.SetOutput(&customLogger{})
	log.Println("[INFO] Overlewd Headless Client Booted. Initializing TUI...")

	// 2. Setup the Tabbed Pages
	TabPages = tview.NewPages()
	TabPages.AddPage("Grinder", BuildTabGrinder(), true, true)
	TabPages.AddPage("Gacha", BuildTabGacha(), true, false)
	TabPages.AddPage("Dissolve", BuildTabDissolve(), true, false)

	// 3. Navigation Header Instructions
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetTextAlign(tview.AlignCenter)

	// Decode JWT Payload for Expiration
	jwtStr := os.Getenv("BEARER_TOKEN")
	jwtExp := "Unknown"
	parts := strings.Split(jwtStr, ".")
	if len(parts) >= 2 {
		if b, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
			var payload struct {
				Exp int64 `json:"exp"`
			}
			if err := json.Unmarshal(b, &payload); err == nil && payload.Exp > 0 {
				jwtExp = time.Unix(payload.Exp, 0).Format(time.RFC822)
			}
		}
	}

	fmt.Fprintf(header, ` ["F1"][yellow][F1] The Grinder[""]  ["F2"][green][F2] Gacha Bomb[""]  ["F3"][red][F3] Batch Dissolve[""]  ["ESC"][white][Esc] Quit[""]  |  [red]JWT Expires: %s `, jwtExp)

	header.SetHighlightedFunc(func(added, removed, clear []string) {
		if len(added) > 0 {
			switch added[0] {
			case "F1":
				TabPages.SwitchToPage("Grinder")
			case "F2":
				TabPages.SwitchToPage("Gacha")
			case "F3":
				TabPages.SwitchToPage("Dissolve")
			case "ESC":
				App.Stop()
			}
		}
	})

	// Global Application Input capturing for Tab Navigation
	App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyF1 {
			TabPages.SwitchToPage("Grinder")
			return nil
		} else if event.Key() == tcell.KeyF2 {
			TabPages.SwitchToPage("Gacha")
			return nil
		} else if event.Key() == tcell.KeyF3 {
			TabPages.SwitchToPage("Dissolve")
			return nil
		} else if event.Key() == tcell.KeyEscape {
			App.Stop()
			return nil
		} else if event.Key() == tcell.KeyDelete {
			// Dev hook to silent wipe caches
			log.Println("[DEV] Cache purge triggered via Delete hotkey.")
			api.EnsureCacheWiped()
			localClient := api.NewClient("https://prod.api.overlewd.ru")
			go func() {
				models.FetchStages(localClient) // Not actually needed unless restarting entirely, but it hydrates our globals instantly
				models.EnrichStages(localClient)
				api.EnrichCurrencies(localClient)
				log.Println("[DEV] Caches fully replenished in memory!")
			}()
			return nil
		}
		return event
	})

	// 4. Master Layout construction (Fixed Header, Expanding Body, Fixed Footer)
	grid := tview.NewGrid().
		SetRows(2, 0, 10). // Header (2 units high), Pages (auto flex), Logs (10 units high)
		SetColumns(0).
		SetBorders(true)

	grid.AddItem(header, 0, 0, 1, 1, 0, 0, false)
	grid.AddItem(TabPages, 1, 0, 1, 1, 0, 0, true)
	grid.AddItem(LogView, 2, 0, 1, 1, 0, 0, false)

	if err := App.SetRoot(grid, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
