package tui

import (
	"strconv"

	"github.com/rivo/tview"

	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
)

var RosterCache []api.CharacterItem
var EquipmentCache []api.EquipmentItem

func BuildTabDissolve() *tview.Flex {
	flex := tview.NewFlex().SetDirection(tview.FlexColumn)
	flex.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	localClient := api.NewClient("https://prod.api.overlewd.ru")

	// ---------------- LEFT PANEL (STATS) ---------------- //
	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow)
	
	charStatsView := tview.NewTextView().
		SetDynamicColors(true).
		SetText("\n[yellow]No Data. Refresh Roster to load inventory counts.")
	charStatsView.SetBorder(true).SetTitle(" Character Inventory Counts ")

	equipStatsView := tview.NewTextView().
		SetDynamicColors(true).
		SetText("\n[yellow]No Data. Refresh Gear to load equipment counts.")
	equipStatsView.SetBorder(true).SetTitle(" Equipment Inventory Counts ")

	leftPanel.AddItem(charStatsView, 0, 1, false)
	leftPanel.AddItem(equipStatsView, 0, 1, false)

	// ---------------- RIGHT PANEL (CONTROLS) ---------------- //
	rightPanel := tview.NewFlex().SetDirection(tview.FlexRow)

	// --- 1. CHARACTER FORM ---
	charForm := tview.NewForm()

	charForm.AddButton("Fetch Roster", func() {
		charStatsView.SetText("\n[yellow]Fetching... please wait.")
		go func() {
			roster, err := api.FetchCharacters(localClient)
			App.QueueUpdateDraw(func() {
				if err != nil {
					charStatsView.SetText("\n[red]Error: " + err.Error())
					return
				}
				RosterCache = roster
				counts := make(map[string]int)
				for _, c := range roster {
					counts[c.Rarity]++
				}
				charStatsView.SetText("\n[green]Roster Successfully Fetched!\n\n" +
					"[white]Basic: " + strconv.Itoa(counts["basic"]) + "\n" +
					"Advanced: " + strconv.Itoa(counts["advanced"]) + "\n" +
					"[purple]Epic: " + strconv.Itoa(counts["epic"]) + "\n" +
					"[red]Heroic: " + strconv.Itoa(counts["heroic"]) + "\n" +
					"[yellow]Legendary: " + strconv.Itoa(counts["legendary"]) + "\n\n" +
					"[blue]Total Characters: " + strconv.Itoa(len(roster)))
			})
		}()
	})

	charForm.AddCheckbox("Basic", true, nil)
	charForm.AddCheckbox("Advanced", false, nil)
	charForm.AddCheckbox("Epic", false, nil)
	charForm.AddCheckbox("Heroic", false, nil)

	executeDissolveChars := func(targets map[string]bool) {
		go func() {
			api.DissolveRarities(localClient, RosterCache, targets)
			roster, err := api.FetchCharacters(localClient)
			App.QueueUpdateDraw(func() {
				if err == nil {
					RosterCache = roster
					counts := make(map[string]int)
					for _, c := range roster {
						counts[c.Rarity]++
					}
					charStatsView.SetText("\n[green]Wipe Complete!\n\n" +
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

	charForm.AddButton("DISSOLVE CHARACTERS", func() {
		targets := map[string]bool{
			"basic":    charForm.GetFormItem(0).(*tview.Checkbox).IsChecked(),
			"advanced": charForm.GetFormItem(1).(*tview.Checkbox).IsChecked(),
			"epic":     charForm.GetFormItem(2).(*tview.Checkbox).IsChecked(),
			"heroic":   charForm.GetFormItem(3).(*tview.Checkbox).IsChecked(),
		}

		if len(RosterCache) == 0 {
			charStatsView.SetText("\n[red]Fetch Roster First before dissolving!")
			return
		}

		if targets["heroic"] {
			modal := tview.NewModal().
				SetText("WARNING: You selected 'Heroic' rarity for dissolving. This will delete highly valuable characters. Are you fundamentally sure you want to proceed?").
				AddButtons([]string{"Yes, Wipe Them", "Cancel"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					if buttonLabel == "Yes, Wipe Them" {
						executeDissolveChars(targets)
					}
					TabPages.RemovePage("modal_dissolve")
				})
			TabPages.AddPage("modal_dissolve", modal, true, true)
		} else {
			executeDissolveChars(targets)
		}
	})

	charForm.SetBorder(true).SetTitle(" Character Dissolve Criteria ")

	// --- 2. EQUIPMENT FORM ---
	equipForm := tview.NewForm()

	equipForm.AddButton("Fetch Gear", func() {
		equipStatsView.SetText("\n[yellow]Fetching Gear... please wait.")
		go func() {
			equipment, err := api.FetchEquipment(localClient)
			App.QueueUpdateDraw(func() {
				if err != nil {
					equipStatsView.SetText("\n[red]Error: " + err.Error())
					return
				}
				EquipmentCache = equipment
				
				countsUnassigned := make(map[string]int)
				countsTotal := make(map[string]int)
				for _, e := range equipment {
					countsTotal[e.Rarity]++
					if e.CharacterId == nil {
						countsUnassigned[e.Rarity]++
					}
				}
				equipStatsView.SetText("\n[green]Gear Successfully Fetched!\n\n" +
					"[gray]Count formatting: Unassigned / Total\n" +
					"[white]Basic: " + strconv.Itoa(countsUnassigned["basic"]) + " / " + strconv.Itoa(countsTotal["basic"]) + "\n" +
					"Advanced: " + strconv.Itoa(countsUnassigned["advanced"]) + " / " + strconv.Itoa(countsTotal["advanced"]) + "\n" +
					"[purple]Epic: " + strconv.Itoa(countsUnassigned["epic"]) + " / " + strconv.Itoa(countsTotal["epic"]) + "\n" +
					"[red]Heroic: " + strconv.Itoa(countsUnassigned["heroic"]) + " / " + strconv.Itoa(countsTotal["heroic"]) + "\n" +
					"[yellow]Legendary: " + strconv.Itoa(countsUnassigned["legendary"]) + " / " + strconv.Itoa(countsTotal["legendary"]) + "\n\n" +
					"[blue]Total Unassigned Gear: " + strconv.Itoa(len(equipment)))
			})
		}()
	})

	equipForm.AddCheckbox("Basic", true, nil)
	equipForm.AddCheckbox("Advanced", false, nil)
	equipForm.AddCheckbox("Epic", false, nil)
	equipForm.AddCheckbox("Heroic", false, nil)

	executeDissolveEquip := func(targets map[string]bool) {
		go func() {
			api.DissolveEquipmentRarities(localClient, EquipmentCache, targets)
			equipment, err := api.FetchEquipment(localClient)
			App.QueueUpdateDraw(func() {
				if err == nil {
					EquipmentCache = equipment
					countsUnassigned := make(map[string]int)
					countsTotal := make(map[string]int)
					for _, e := range equipment {
						countsTotal[e.Rarity]++
						if e.CharacterId == nil {
							countsUnassigned[e.Rarity]++
						}
					}
					equipStatsView.SetText("\n[green]Gear Wipe Complete!\n\n" +
						"[gray]Count formatting: Unassigned / Total\n" +
						"[white]Basic: " + strconv.Itoa(countsUnassigned["basic"]) + " / " + strconv.Itoa(countsTotal["basic"]) + "\n" +
						"Advanced: " + strconv.Itoa(countsUnassigned["advanced"]) + " / " + strconv.Itoa(countsTotal["advanced"]) + "\n" +
						"[purple]Epic: " + strconv.Itoa(countsUnassigned["epic"]) + " / " + strconv.Itoa(countsTotal["epic"]) + "\n" +
						"[red]Heroic: " + strconv.Itoa(countsUnassigned["heroic"]) + " / " + strconv.Itoa(countsTotal["heroic"]) + "\n" +
						"[yellow]Legendary: " + strconv.Itoa(countsUnassigned["legendary"]) + " / " + strconv.Itoa(countsTotal["legendary"]) + "\n\n" +
						"[blue]Total Gear Left: " + strconv.Itoa(len(equipment)))
				}
			})
		}()
	}

	equipForm.AddButton("DISSOLVE GEAR", func() {
		targets := map[string]bool{
			"basic":    equipForm.GetFormItem(0).(*tview.Checkbox).IsChecked(),
			"advanced": equipForm.GetFormItem(1).(*tview.Checkbox).IsChecked(),
			"epic":     equipForm.GetFormItem(2).(*tview.Checkbox).IsChecked(),
			"heroic":   equipForm.GetFormItem(3).(*tview.Checkbox).IsChecked(),
		}

		if len(EquipmentCache) == 0 {
			equipStatsView.SetText("\n[red]Fetch Gear First before dissolving!")
			return
		}

		if targets["heroic"] {
			modal := tview.NewModal().
				SetText("WARNING: You selected 'Heroic' rarity for dissolving. This will delete highly valuable gear. Are you sure you want to proceed?").
				AddButtons([]string{"Yes, Wipe Unassigned Gear", "Cancel"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					if buttonLabel == "Yes, Wipe Unassigned Gear" {
						executeDissolveEquip(targets)
					}
					TabPages.RemovePage("modal_dissolve")
				})
			TabPages.AddPage("modal_dissolve", modal, true, true)
		} else {
			executeDissolveEquip(targets)
		}
	})

	equipForm.SetBorder(true).SetTitle(" Equipment Dissolve Criteria ")

	rightPanel.AddItem(charForm, 0, 1, true)
	rightPanel.AddItem(equipForm, 0, 1, false)

	flex.AddItem(leftPanel, 0, 1, false)
	flex.AddItem(rightPanel, 0, 1, true)
	return flex
}
