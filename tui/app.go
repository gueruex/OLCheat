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
	App          *tview.Application
	LogView      *tview.TextView
	TabPages     *tview.Pages
	NeedsRestart bool
)

// customLogger intercepts standard logs and pushes them linearly to the LogView pane
type customLogger struct{}

func (cl *customLogger) Write(p []byte) (n int, err error) {
	if LogView != nil && App != nil {
		str := string(p)
		go App.QueueUpdateDraw(func() {
			fmt.Fprintf(LogView, "%s", str)
			LogView.ScrollToEnd()
		})
	}
	return len(p), nil
}

func StartApp(client *api.OverlewdClient) {
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
	TabPages.AddPage("Battler", BuildTabGrinder(), true, true)
	TabPages.AddPage("Gacha", BuildTabGacha(), true, false)
	TabPages.AddPage("Dissolve", BuildTabDissolve(), true, false)
	TabPages.AddPage("Market", BuildTabMarket(), true, false)
	TabPages.AddPage("Campaigner", BuildTabCampaigner(), true, false)
	TabPages.AddPage("Memories", BuildTabMemories(), true, false)

	api.OnSessionInvalidated = func() {
		if App != nil {
			App.QueueUpdateDraw(func() {
				api.WipeAuthCredentials()
				modal := tview.NewModal().
					SetText("Session Invalidated! (Multiple devices detected or JWT expired).\nYour .env has been completely wiped.\nPlease restart the application to pull a fresh token.").
					AddButtons([]string{"Exit"}).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						App.Stop()
					})
				TabPages.AddPage("AuthError", modal, true, true)
			})
		}
	}

	// 3. Navigation Header Instructions
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetTextAlign(tview.AlignCenter)

	updateHeader := func(expStr string) {
		fmt.Fprintf(header, ` ["F1"][yellow][F1] Battler[""]  ["F2"][green][F2] Gacha[""]  ["F3"][red][F3] Batch Dissolve[""]  ["F4"][blue][F4] Market[""]  ["F5"][magenta][F5] Campaigner[""]  ["F6"][cyan][F6] Scenes[""]  ["ESC"][white][Esc] Quit[""]  |  [red]JWT Expires: %s `, expStr)
	}

	// Decode JWT Payload for Expiration
	jwtStr := os.Getenv("BEARER_TOKEN")
	jwtExp := "Unknown"
	isValid := false

	parts := strings.Split(jwtStr, ".")
	if len(parts) >= 2 {
		if b, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
			var payload struct {
				Exp int64 `json:"exp"`
			}
			if err := json.Unmarshal(b, &payload); err == nil && payload.Exp > 0 {
				jwtExp = time.Unix(payload.Exp, 0).Format(time.RFC822)
				if payload.Exp > time.Now().Unix() {
					isValid = true
				}
			}
		}
	}

	updateHeader(jwtExp)

	continueBootSequence := func() {
		go func() {
			log.Println("[INFO] Fetching FTUE and Event Stages...")
			if err := models.FetchStages(client); err != nil {
				log.Printf("[ERROR] Critical Failure ingesting JSON stage metadata arrays: %v\n", err)
			}
			log.Println("[INFO] Fetching Dictionaries...")
			models.EnrichStages(client)
			api.EnrichCurrencies(client)

			if ForceReloadGacha != nil {
				ForceReloadGacha()
			}
		}()
	}

	if !isValid {
		form := tview.NewForm().
			AddButton("1. Generate Certificate", func() {
				log.Println("[INFO] Generating Certificate...")
				api.GenerateCertificateNatively()
			}).
			AddButton("2. Start Proxy", func() {
				TabPages.HidePage("AuthModal")
				App.SetFocus(TabPages)
				log.Println("[INFO] Booting Background Proxy on :8080...")
				go api.StartProxyRoutine("8080", func(newToken string) {
					// Callback when proxy catches the login!
					os.Setenv("BEARER_TOKEN", newToken)
					client.BearerToken = newToken

					App.QueueUpdateDraw(func() {
						header.Clear()
						fmt.Fprintf(header, " [green]Authentication Successful! Restarting TUI...")

						go func() {
							time.Sleep(1 * time.Second)
							NeedsRestart = true
							App.Stop()
						}()
					})
				})
			})

		form.SetBorder(true).
			SetTitle(" Authentication Missing or Expired! ").
			SetTitleAlign(tview.AlignCenter)

		helpText := tview.NewTextView().
			SetDynamicColors(true).
			SetText("[red]Authentication Missing or Expired![white]\n\n" +
				"If you have not installed the Trusted Root Cert, click [yellow]Generate Certificate[white].\n" +
				"Double click the ca.cer file in your folder to install it.\n\n" +
				"Once installed, click [yellow]Start Proxy[white] and set your Windows proxy to 127.0.0.1:8080.\n" +
				"Log into the game to automatically intercept your token!")

		flex := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(helpText, 10, 1, false).
			AddItem(form, 5, 1, true)

		modalLayout := tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(flex, 15, 1, true).
				AddItem(nil, 0, 1, false), 80, 1, true).
			AddItem(nil, 0, 1, false)

		TabPages.AddPage("AuthModal", modalLayout, true, true)
	} else {
		continueBootSequence()
	}

	header.SetHighlightedFunc(func(added, removed, clear []string) {
		if len(added) > 0 {
			switch added[0] {
			case "F1":
				TabPages.SwitchToPage("Battler")
			case "F2":
				TabPages.SwitchToPage("Gacha")
			case "F3":
				TabPages.SwitchToPage("Dissolve")
			case "F4":
				TabPages.SwitchToPage("Market")
			case "F5":
				TabPages.SwitchToPage("Campaigner")
			case "F6":
				TabPages.SwitchToPage("Memories")
			case "ESC":
				App.Stop()
			}
		}
	})

	// Global Application Input capturing for Tab Navigation
	App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyF1 {
			TabPages.SwitchToPage("Battler")
			return nil
		} else if event.Key() == tcell.KeyF2 {
			TabPages.SwitchToPage("Gacha")
			return nil
		} else if event.Key() == tcell.KeyF3 {
			TabPages.SwitchToPage("Dissolve")
			return nil
		} else if event.Key() == tcell.KeyF4 {
			TabPages.SwitchToPage("Market")
			return nil
		} else if event.Key() == tcell.KeyF5 {
			TabPages.SwitchToPage("Campaigner")
			return nil
		} else if event.Key() == tcell.KeyF6 {
			TabPages.SwitchToPage("Memories")
			return nil
		} else if event.Key() == tcell.KeyHome {
			api.WipeAuthCredentials()
			modal := tview.NewModal().
				SetText(".env has been deleted!\nPlease restart the app to pull a new token.").
				AddButtons([]string{"Exit"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					App.Stop()
				})
			TabPages.AddPage("LogOutModal", modal, true, true)
			TabPages.ShowPage("LogOutModal")
			return nil
		} else if event.Key() == tcell.KeyDelete {
			// Dev hook to silent wipe caches
			log.Println("[INFO] Cache purge triggered via Delete hotkey.")
			api.EnsureCacheWiped()
			localClient := api.NewClient("https://prod.api.overlewd.ru")
			go func() {
				models.FetchStages(localClient) // Not actually needed unless restarting entirely, but it hydrates our globals instantly
				models.EnrichStages(localClient)
				api.EnrichCurrencies(localClient)
				log.Println("[INFO] Caches fully replenished in memory!")
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
