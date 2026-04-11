package tui

import (
	"strconv"

	"github.com/rivo/tview"

	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
)

var RosterCache []api.CharacterItem

func BuildTabDissolve() *tview.Flex {
	flex := tview.NewFlex().SetDirection(tview.FlexColumn)
	flex.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	localClient := api.NewClient("https://prod.api.overlewd.ru")

	// Left: Stats
	statsView := tview.NewTextView().
		SetDynamicColors(true).
		SetText("\n[yellow]No Data. Refresh Roster to load inventory counts.")
	statsView.SetBorder(true).SetTitle(" Inventory Counts ")

	// Right: Controls
	rightPanel := tview.NewFlex().SetDirection(tview.FlexRow)
	
	form := tview.NewForm()
	
	form.AddButton("Fetch Roster", func() {
		statsView.SetText("\n[yellow]Fetching... please wait.")
		go func() {
			roster, err := api.FetchCharacters(localClient)
			App.QueueUpdateDraw(func() {
				if err != nil {
					statsView.SetText("\n[red]Error: " + err.Error())
					return
				}
				RosterCache = roster
				counts := make(map[string]int)
				for _, c := range roster {
					counts[c.Rarity]++
				}
				statsView.SetText("\n[green]Roster Successfully Fetched!\n\n" +
					"[white]Basic: " + strconv.Itoa(counts["basic"]) + "\n" +
					"Advanced: " + strconv.Itoa(counts["advanced"]) + "\n" +
					"[purple]Epic: " + strconv.Itoa(counts["epic"]) + "\n" +
					"[red]Heroic: " + strconv.Itoa(counts["heroic"]) + "\n" +
					"[yellow]Legendary: " + strconv.Itoa(counts["legendary"]) + "\n\n" +
					"[blue]Total Characters: " + strconv.Itoa(len(roster)))
			})
		}()
	})

	form.AddCheckbox("Basic", true, nil)
	form.AddCheckbox("Advanced", false, nil)
	form.AddCheckbox("Epic", false, nil)
	form.AddCheckbox("Heroic", false, nil)

	executeDissolve := func(targets map[string]bool) {
		go func() {
			api.DissolveRarities(localClient, RosterCache, targets)
			// Trigger a re-fetch visually
			roster, err := api.FetchCharacters(localClient)
			App.QueueUpdateDraw(func() {
				if err == nil {
					RosterCache = roster
					counts := make(map[string]int)
					for _, c := range roster {
						counts[c.Rarity]++
					}
					statsView.SetText("\n[green]Wipe Complete!\n\n" +
						"[white]Basic: " + strconv.Itoa(counts["basic"]) + "\n" +
						"Advanced: " + strconv.Itoa(counts["advanced"]) + "\n" +
						"[purple]Epic: " + strconv.Itoa(counts["epic"]) + "\n" +
						"[red]Heroic: " + strconv.Itoa(counts["heroic"]) + "\n" +
						"[yellow]Legendary: " + strconv.Itoa(counts["legendary"]) + "\n\n" +
						"[blue]Total Characters Left: " + strconv.Itoa(len(roster)))
				}
			})
		}()
	}

	form.AddButton("DISSOLVE SELECTED", func() {
		targets := map[string]bool{
			"basic":    form.GetFormItem(0).(*tview.Checkbox).IsChecked(),
			"advanced": form.GetFormItem(1).(*tview.Checkbox).IsChecked(),
			"epic":     form.GetFormItem(2).(*tview.Checkbox).IsChecked(),
			"heroic":   form.GetFormItem(3).(*tview.Checkbox).IsChecked(),
		}

		if len(RosterCache) == 0 {
			statsView.SetText("\n[red]Fetch Roster First before dissolving!")
			return
		}

		if targets["heroic"] {
			modal := tview.NewModal().
				SetText("WARNING: You selected 'Heroic' rarity for dissolving. This will delete highly valuable characters. Are you fundamentally sure you want to proceed?").
				AddButtons([]string{"Yes, Wipe Them", "Cancel"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					if buttonLabel == "Yes, Wipe Them" {
						executeDissolve(targets)
					}
					TabPages.RemovePage("modal_dissolve")
				})
			TabPages.AddPage("modal_dissolve", modal, true, true)
		} else {
			executeDissolve(targets)
		}
	})

	form.SetBorder(true).SetTitle(" Rarity Criteria ")
	rightPanel.AddItem(form, 0, 1, true)

	flex.AddItem(statsView, 0, 1, false)
	flex.AddItem(rightPanel, 0, 1, true)
	return flex
}
