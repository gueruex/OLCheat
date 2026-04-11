package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
	"jewrexxx-fearlessrevolution-overlewd-cheat/models"

	"github.com/rivo/tview"
)

var (
	GlobalTargetStage *models.Stage
	slotCancels       = make(map[int]context.CancelFunc)
	slotTargets       = make(map[int]*models.Stage)
)

func BuildTabGrinder() tview.Primitive {
	localClient := api.NewClient("https://prod.api.overlewd.ru")
	masterLayout := tview.NewFlex().SetDirection(tview.FlexColumn)
	masterLayout.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	// --- LEFT PANE (TREE NAVIGATOR) ---
	treeView := tview.NewTreeView().SetGraphics(true)
	treeView.SetBorder(true).SetTitle(" [white]Stage Navigator ").SetTitleAlign(tview.AlignLeft)

	root := tview.NewTreeNode("Categories").SetSelectable(false)
	treeView.SetRoot(root).SetCurrentNode(root)

	reloadTree := func() {
		root.ClearChildren()
		catMap := make(map[string]map[string][]*models.Stage)

		if len(models.CachedBattles) == 0 {
			root.AddChild(tview.NewTreeNode("No battles loaded...").SetSelectable(false))
		} else {
			for i := range models.CachedBattles {
				b := &models.CachedBattles[i]
				if _, ok := catMap[b.ReadableCategory]; !ok {
					catMap[b.ReadableCategory] = make(map[string][]*models.Stage)
				}
				catMap[b.ReadableCategory][b.ReadableChapter] = append(catMap[b.ReadableCategory][b.ReadableChapter], b)
			}

			for catName, chaps := range catMap {
				catColor := tview.Styles.PrimaryTextColor
				if catName == "Main Campaign" {
					catColor = tview.Styles.TertiaryTextColor
				}

				catNode := tview.NewTreeNode(catName).
					SetSelectable(true).
					SetExpanded(false).
					SetColor(catColor)
				root.AddChild(catNode)

				for chapName, stages := range chaps {
					chapNode := tview.NewTreeNode(chapName).
						SetSelectable(true).
						SetExpanded(false).
						SetColor(tview.Styles.SecondaryTextColor)
					catNode.AddChild(chapNode)

					for _, stg := range stages {
						stgText := fmt.Sprintf("[%d] %s", stg.ID, stg.Name)
						stgNode := tview.NewTreeNode(stgText).
							SetSelectable(true).
							SetReference(stg).
							SetColor(tview.Styles.ContrastBackgroundColor) 
						chapNode.AddChild(stgNode)
					}
				}
			}
		}
	}
	// Initial Tree Hydration
	reloadTree()

	rewardsView := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true)
	rewardsView.SetBorder(true).SetTitle(" [magenta]Stage Rewards ").SetTitleAlign(tview.AlignCenter)
	rewardsView.SetText("\n[gray]  Select a stage to view probabilities...")

	// --- RIGHT PANE (THE SPOTS) ---
	rightPane := tview.NewFlex().SetDirection(tview.FlexRow)
	rightPane.SetBorder(true).SetTitle(" [yellow]The Spawner Slots ")

	lblTarget := tview.NewTextView().SetDynamicColors(true).
		SetText("\n  Target: [red]None Selected (Click a Stage in the left menu!)\n")
	rightPane.AddItem(lblTarget, 3, 0, false)

	// Tree selection dynamic event
	treeView.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			// This is a folder node (Category or Chapter), toggle its expansion
			node.SetExpanded(!node.IsExpanded())
		} else {
			// This is a leaf node (Stage)
			GlobalTargetStage = ref.(*models.Stage)
			lblTarget.SetText(fmt.Sprintf("\n  Target: [green]%s -> %s -> %s\n", 
				GlobalTargetStage.ReadableCategory, 
				GlobalTargetStage.ReadableChapter, 
				GlobalTargetStage.Name))
		}
	})

	slotsFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	for i := 1; i <= 8; i++ {
		slotContainer := tview.NewFlex().SetDirection(tview.FlexColumn)
		slotContainer.SetBorder(true).SetTitle(fmt.Sprintf(" Slot #%d ", i))

		curTargetLabel := tview.NewTextView().SetDynamicColors(true).SetText("\n[red]Assigned: None")

		// Create a localized snapshot scope for loop variables
		slotForm := tview.NewForm()
		currentSlotID := i
		
		assignClosure := func(lbl *tview.TextView) func() {
			return func() {
				if GlobalTargetStage != nil {
					slotTargets[currentSlotID] = GlobalTargetStage
					lbl.SetText(fmt.Sprintf("\n[yellow]Assigned:[white] %s -> %s", GlobalTargetStage.ReadableChapter, GlobalTargetStage.Name))
				}
			}
		}(curTargetLabel)

		startClosure := func(slot int, form *tview.Form, lbl *tview.TextView) func() {
			return func() {
				if cancel, exists := slotCancels[slot]; exists {
					// Stop Logic
					cancel()
					delete(slotCancels, slot)
					form.GetButton(1).SetLabel("Start Battle")
					lbl.SetText("\n[red]Assigned: Loop Stopped")
				} else {
					// Start Logic
					target := slotTargets[slot]
					if target == nil {
						lbl.SetText("\n[red]No valid target assigned!")
						return
					}
					
					ctx, cancelFunc := context.WithCancel(context.Background())
					slotCancels[slot] = cancelFunc
					
					form.GetButton(1).SetLabel("Stop (Running)")
					lbl.SetText(fmt.Sprintf("\n[green]Grinding:[white] %s -> %s", target.ReadableChapter, target.Name))
					
					go api.WorkerLoop(ctx, localClient, target.Endpoint, target.ID, slot)
				}
			}
		}(currentSlotID, slotForm, curTargetLabel)

		slotForm.
			AddButton("Assign Target", assignClosure).
			AddButton("Start Battle", startClosure)

		slotContainer.AddItem(slotForm, 40, 0, false)
		slotContainer.AddItem(curTargetLabel, 0, 1, false)

		slotsFlex.AddItem(slotContainer, 6, 1, false)
	}

	rightPane.AddItem(slotsFlex, 0, 2, false)

	// Live highlight dynamic event
	treeView.SetChangedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref != nil {
			stg := ref.(*models.Stage)
			rText := ""
			if len(stg.FirstRewards) > 0 {
				if stg.Status == "complete" {
					rText += "\n[yellow]  *** First Clear Rewards [white](Claimed)[yellow] ***\n"
				} else {
					rText += "\n[yellow]  *** First Clear Rewards ***\n"
				}
				for _, rw := range stg.FirstRewards {
					rText += fmt.Sprintf("  - [white]%dx[green] %s [yellow](%.1f%%)\n", rw.Amount, api.GetCurrencyName(rw.TradableID), rw.Probability*100)
				}
			}
			if len(stg.Rewards) > 0 {
				rText += "\n[white]  *** Farming Drops ***\n"
				for _, rw := range stg.Rewards {
					rText += fmt.Sprintf("  - [white]%dx[green] %s [gray](%.1f%%)\n", rw.Amount, api.GetCurrencyName(rw.TradableID), rw.Probability*100)
				}
			}
			if rText == "" {
				rText = "\n[gray]  No visible rewards on this stage."
			}
			rewardsView.SetText(rText)
		} else {
			rewardsView.SetText("\n[gray]  Select a stage to view probabilities...")
		}
	})

	controlForm := tview.NewForm()
	controlForm.AddButton("Update Stages", func() {
		root.ClearChildren()
		root.AddChild(tview.NewTreeNode("[yellow]Fetching Stages from Network... Please Wait...").SetSelectable(false))
		
		go func() {
			endpoints := []struct {
				Ep string
				Fn string
			}{
				{"/ftue-stages", "ftue_stages.json"},
				{"/event-stages", "event_stages.json"},
			}

			successCount := 0
			failCount := 0

			pushLog := func(idxStr, msg string) {
				log.Printf("%s %s", idxStr, msg)
			}

			for i, e := range endpoints {
				idxStr := fmt.Sprintf("[%d/%d]", i+1, len(endpoints))
				ok := api.SafeForceUpdateCache(e.Ep, e.Fn, localClient, 3, idxStr, pushLog)
				if ok {
					successCount++
				} else {
					failCount++
				}
			}

			if failCount > 0 {
				log.Printf("[green]Successfully updated %d files. [red]Failed to update %d files.[white]", successCount, failCount)
			} else {
				log.Printf("[green]Successfully updated %d files on the network block.[white]", successCount)
			}
			
			// Force reload into ram
			models.FetchStages(localClient)
			models.EnrichStages(localClient)
			
			App.QueueUpdateDraw(func() {
				reloadTree()
			})
		}()
	})

	controlForm.AddButton("View Resources", func() {
		log.Println("[INFO] Fetching User Wallet from API...")
		go func() {
			req, _ := http.NewRequest("GET", localClient.BaseURL+"/me", nil)
			resp, err := localClient.DoRequestBypassCooldown(req)
			if err != nil {
				log.Printf("[ERROR] Network failure hitting /me: %v", err)
				return
			}
			defer resp.Body.Close()
			
			var data struct {
				Wallet []struct {
					CurrencyID int `json:"currencyId"`
					Amount     int `json:"amount"`
				} `json:"wallet"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
				log.Printf("[ERROR] JSON parse failure on /me: %v", err)
				return
			}

			targetIDs := map[int]string{
				22: "Wood",
				66: "Stone",
				25: "Silver",
				65: "Gems",
				3:  "Crystals",
				24: "Magic Spheres",
				84: "Free Summon Runes",
				83: "Sacred Runes",
			}
			
			order := []int{22, 66, 25, 65, 3, 24, 84, 83}
			
			output := "[white]===============\n[magenta]Current Resources[white]\n===============\n\n"
			for _, id := range order {
				amt := 0
				for _, w := range data.Wallet {
					if w.CurrencyID == id {
						amt = w.Amount
						break
					}
				}
				output += fmt.Sprintf(" [yellow]%-18s[white] %d\n", targetIDs[id]+":", amt)
			}
			output += "\n"
			
			App.QueueUpdateDraw(func() {
				modal := tview.NewModal().
					SetText(output).
					AddButtons([]string{"Close"}).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						TabPages.RemovePage("ResourcesModal")
					})
				TabPages.AddPage("ResourcesModal", modal, true, true)
			})
		}()
	})

	leftPane := tview.NewFlex().SetDirection(tview.FlexRow)
	leftPane.AddItem(treeView, 0, 3, true)
	leftPane.AddItem(controlForm, 3, 0, false)
	leftPane.AddItem(rewardsView, 0, 1, false)

	masterLayout.AddItem(leftPane, 0, 3, true)
	masterLayout.AddItem(rightPane, 0, 4, false)

	return masterLayout
}
