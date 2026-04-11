package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
	"jewrexxx-fearlessrevolution-overlewd-cheat/models"
)

var (
	SelectedStartStage *models.Stage
	SelectedEndStage   *models.Stage
	StartIndex         int = -1
	EndIndex           int = -1
	CurrentCampaign    string
)

func BuildTabCampaigner() *tview.Flex {
	localClient := api.NewClient("https://prod.api.overlewd.ru")
	flex := tview.NewFlex().SetDirection(tview.FlexColumn)
	flex.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	campaignMap := make(map[string][]*models.Stage)

	// 1. LEFT PANE (Campaign List)
	catList := tview.NewList().ShowSecondaryText(false)
	catList.SetBorder(true).SetTitle(" Campaign Map ")

	// 2. MIDDLE PANE (Stage Sequence)
	stgList := tview.NewList().ShowSecondaryText(false)
	stgList.SetBorder(true).SetTitle(" Stage Timelines ")
	
	// FIX: Make text readable when highlighted!
	stgList.SetSelectedBackgroundColor(tcell.ColorDarkBlue)
	stgList.SetSelectedTextColor(tcell.ColorWhite)

	// 3. RIGHT PANE (Controls)
	rightPane := tview.NewFlex().SetDirection(tview.FlexRow)
	rightPane.SetBorder(true).SetTitle(" Router Dashboard ")

	statusLog := tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetText("\n  [gray]Awaiting trajectory bounds...\n")
	
	btnFlex := tview.NewFlex().SetDirection(tview.FlexRow)

	reloadCampaigns := func() {
		catList.Clear()
		stgList.Clear()
		campaignMap = make(map[string][]*models.Stage)
		
		stageMapById := make(map[int]*models.Stage)
		for _, s := range models.AllRawStages {
			stageMapById[s.ID] = s
		}

		// Topological Sort Alg for any fractured Array!
		topologicalSort := func(stgIds []int) []*models.Stage {
			inDegree := make(map[int]int)
			validSet := make(map[int]bool)
			for _, id := range stgIds {
				validSet[id] = true
				inDegree[id] = 0 // Initialize
			}

			// Map edges
			for _, id := range stgIds {
				stg, ok := stageMapById[id]
				if ok {
					for _, nextId := range stg.NextStages {
						if validSet[nextId] {
							inDegree[nextId]++
						}
					}
				}
			}

			// Queue roots
			var queue []int
			for _, id := range stgIds {
				if inDegree[id] == 0 {
					queue = append(queue, id)
				}
			}

			var sorted []*models.Stage
			for len(queue) > 0 {
				currId := queue[0]
				queue = queue[1:]

				stg, ok := stageMapById[currId]
				if ok {
					sorted = append(sorted, stg)
					for _, nextId := range stg.NextStages {
						if validSet[nextId] {
							inDegree[nextId]--
							if inDegree[nextId] == 0 {
								queue = append(queue, nextId)
							}
						}
					}
				}
			}
			return sorted
		}

		// 1. Map Main Campaign explicitly via its chronological structural array
		for _, ch := range models.GlobalFTUE.Chapters {
			sorted := topologicalSort(ch.Stages)
			campaignMap["Main Campaign"] = append(campaignMap["Main Campaign"], sorted...)
		}

		// 2. Map Events explicitly via their chronological structural arrays
		for _, ev := range models.GlobalEvents {
			evName := "Event: " + ev.Name
			
			// Drop overloads structurally
			evLower := strings.ToLower(evName)
			if strings.Contains(evLower, "overlord") || strings.Contains(evLower, "arena") {
				continue
			}

			// Collect the Chapters for this Event and SORT them by their native Order property!
			var sortedChaps []models.EventChapter
			for _, chId := range ev.Chapters {
				ch, ok := models.GlobalEventChapters[chId]
				if ok {
					sortedChaps = append(sortedChaps, ch)
				}
			}

			sort.Slice(sortedChaps, func(i, j int) bool {
				return sortedChaps[i].Order < sortedChaps[j].Order
			})

			// Trace explicit linear arrays
			for _, ch := range sortedChaps {
				sorted := topologicalSort(ch.Stages)
				campaignMap[evName] = append(campaignMap[evName], sorted...)
			}
		}

		for catName := range campaignMap {
			catList.AddItem(catName, "", 0, nil)
		}
		
		statusLog.SetText(statusLog.GetText(false) + "\n  [green]Successfully built explicitly structured Maps from Memory!")
	}

	var activeStages []*models.Stage

	// Helpers
	refreshStatus := func() {
		out := "\n  [white]Bounds Tracking:\n"
		if SelectedStartStage != nil {
			out += fmt.Sprintf("  [green]START:[white] %s\n", SelectedStartStage.Name)
		} else {
			out += "  [gray]START: Unassigned\n"
		}
		
		if SelectedEndStage != nil {
			out += fmt.Sprintf("  [red]END:[white] %s\n", SelectedEndStage.Name)
		} else {
			out += "  [gray]END: Unassigned\n"
		}
		
		if SelectedStartStage != nil && SelectedEndStage != nil {
			if StartIndex > EndIndex {
				out += "\n  [red]ERROR: End stage is before Start stage chronologically!"
			} else {
				nodeCount := (EndIndex - StartIndex) + 1
				cost := 0
				for i := StartIndex; i <= EndIndex; i++ {
					stg := activeStages[i]
					if stg.Status == "complete" || stg.Status == "completed" {
						continue // Skips execution logic!
					}
					if stg.BattleID != nil {
						cost += stg.BattleEnergyCost
					}
				}
				out += fmt.Sprintf("\n  [cyan]Trajectory Valid: %d nodes queued. (Est. Load: %d Resources)", nodeCount, cost)
			}
		}
		statusLog.SetText(out)
	}

	catList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		stgList.Clear()
		CurrentCampaign = mainText
		
		SelectedStartStage = nil
		SelectedEndStage = nil
		StartIndex = -1
		EndIndex = -1
		refreshStatus()

		if stages, ok := campaignMap[mainText]; ok {
			activeStages = stages
			for i, stg := range stages {
				col := ""
				if stg.Status == "complete" || stg.Status == "completed" {
					col = "[green]"
				} else if stg.Status == "open" || stg.Status == "available" {
					col = "[yellow]"
				} else {
					col = "[red]"
				}
				
				// Inject the Readble Chapter explicitly mapping nested trees dynamically
				chapterPre := ""
				if stg.ReadableChapter != "" {
					chapterPre = fmt.Sprintf("[%s] ", stg.ReadableChapter)
				}
				
				stgName := tview.Escape(fmt.Sprintf("%d. %s%s", i+1, chapterPre, stg.Name))
				stgList.AddItem(col+stgName, "", 0, nil)
			}
		}
	})

	btnStartSet := tview.NewButton("Set START Node").SetSelectedFunc(func() {
		currIdx := stgList.GetCurrentItem()
		if len(activeStages) > 0 && currIdx >= 0 && currIdx < len(activeStages) {
			SelectedStartStage = activeStages[currIdx]
			StartIndex = currIdx
			
			if SelectedEndStage != nil && EndIndex < StartIndex {
				SelectedEndStage = nil
				EndIndex = -1
			}
			refreshStatus()
		}
	})

	btnEndSet := tview.NewButton("Set END Node").SetSelectedFunc(func() {
		currIdx := stgList.GetCurrentItem()
		if len(activeStages) > 0 && currIdx >= 0 && currIdx < len(activeStages) {
			if SelectedStartStage == nil {
				return
			}
			if currIdx < StartIndex {
				return
			}
			
			SelectedEndStage = activeStages[currIdx]
			EndIndex = currIdx
			refreshStatus()
		}
	})

	btnClear := tview.NewButton("Clear Validation Bounds").SetSelectedFunc(func() {
		SelectedStartStage = nil
		SelectedEndStage = nil
		StartIndex = -1
		EndIndex = -1
		refreshStatus()
	})

	btnReload := tview.NewButton("Reload Local Maps").SetSelectedFunc(func() {
		reloadCampaigns()
	})

	var execCancelFunc context.CancelFunc

	btnExec := tview.NewButton("Start Sequence").SetSelectedFunc(func() {
		if SelectedStartStage == nil || SelectedEndStage == nil {
			statusLog.SetText(statusLog.GetText(false) + "\n\n  [red]Cannot Execute: You must explicitly clamp Start and End paths!")
			return
		}
		if StartIndex > EndIndex {
			statusLog.SetText(statusLog.GetText(false) + "\n\n  [red]Cannot Execute: End stage is physically before Start stage!")
			return
		}
		if execCancelFunc != nil {
			statusLog.SetText(statusLog.GetText(false) + "\n\n  [red]Cannot Execute: An execution boundary is currently processing active payloads!")
			return
		}
		
		nodeCount := (EndIndex - StartIndex) + 1
		statusLog.SetText(statusLog.GetText(false) + fmt.Sprintf("\n\n  [yellow]Executing concurrent sequence physically mapping over %d nodes!\n  Please refer to Operation Logs (Auto-Scroll) for execution routines!", nodeCount))

		queue := make([]api.CampaignTask, 0, nodeCount)
		for i := StartIndex; i <= EndIndex; i++ {
			stg := activeStages[i]
			queue = append(queue, api.CampaignTask{
				Endpoint: stg.Endpoint,
				ID:       stg.ID,
				Status:   stg.Status,
				Name:     stg.Name,
			})
		}

		ctx, cancel := context.WithCancel(context.Background())
		execCancelFunc = cancel

		go func() {
			api.CampaignWorker(ctx, cancel, localClient, queue)
			time.Sleep(1 * time.Second) // Protect global boundary context teardown
			execCancelFunc = nil
		}()
	})

	// Wrap Buttons in explicit horizontal row splits adding fixed-width boundaries avoiding stretching
	row1 := tview.NewFlex().SetDirection(tview.FlexColumn)
	row1.AddItem(tview.NewBox(), 0, 1, false) // Padding
	row1.AddItem(btnStartSet, 20, 1, false)
	row1.AddItem(tview.NewBox(), 2, 0, false)
	row1.AddItem(btnEndSet, 20, 1, false)
	row1.AddItem(tview.NewBox(), 2, 0, false)
	row1.AddItem(btnClear, 25, 1, false)
	row1.AddItem(tview.NewBox(), 0, 1, false) // Padding

	row2 := tview.NewFlex().SetDirection(tview.FlexColumn)
	row2.AddItem(tview.NewBox(), 0, 1, false)
	row2.AddItem(btnReload, 20, 1, false)
	row2.AddItem(tview.NewBox(), 2, 0, false)
	row2.AddItem(btnExec, 20, 1, false)
	row2.AddItem(tview.NewBox(), 0, 1, false)

	btnFlex.AddItem(tview.NewBox(), 1, 0, false) // V-Padding
	btnFlex.AddItem(row1, 1, 0, false)
	btnFlex.AddItem(tview.NewBox(), 1, 0, false) // V-Padding
	btnFlex.AddItem(row2, 1, 0, false)
	btnFlex.AddItem(tview.NewBox(), 1, 0, false) // V-Padding

	rightPane.AddItem(statusLog, 0, 1, false)
	rightPane.AddItem(btnFlex, 5, 0, true)

	flex.AddItem(catList, 0, 1, true)
	flex.AddItem(stgList, 0, 2, false) // Expanded list ratio
	flex.AddItem(rightPane, 0, 3, false) // Expanded dash ratio

	return flex
}
