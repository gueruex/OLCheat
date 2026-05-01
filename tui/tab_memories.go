package tui

import (
	"context"
	"fmt"

	"github.com/rivo/tview"

	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
)

var MemoryCache []api.Memory

func BuildTabMemories() *tview.Flex {
	flex := tview.NewFlex().SetDirection(tview.FlexColumn)
	flex.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	localClient := api.NewClient("https://prod.api.overlewd.ru")

	// --- LEFT PANE (MEMORIES NAVIGATOR) ---
	treeView := tview.NewTreeView().SetGraphics(true)
	treeView.SetBorder(true).SetTitle(" [white]Memory Gallery ").SetTitleAlign(tview.AlignLeft)

	root := tview.NewTreeNode("Memories").SetSelectable(false)
	treeView.SetRoot(root).SetCurrentNode(root)

	// --- RIGHT PANE (CONTROLS) ---
	rightPane := tview.NewFlex().SetDirection(tview.FlexRow)

	lblTarget := tview.NewTextView().SetDynamicColors(true).
		SetText("\n  Target: [gray]Nothing Selected\n")

	statusLog := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetScrollable(true).
		SetText("\n  [gray]Awaiting commands...\n")
	statusLog.SetBorder(true).SetTitle(" Operation Logs ")
	statusLog.SetChangedFunc(func() {
		statusLog.ScrollToEnd()
	})

	descView := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetScrollable(true).
		SetText("\n  [gray]Highlight a memory to see its shard requirements...")
	descView.SetBorder(true).SetTitle(" Scene Details ")

	var selectedMemory api.Memory

	treeView.SetChangedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref != nil {
			mem := ref.(api.Memory)
			
			statusColor := "green"
			if mem.Status == "hidden" {
				statusColor = "red"
			}
			
			reason := "None"
			if mem.BlockReason != nil && *mem.BlockReason != "" {
				reason = *mem.BlockReason
			}

			out := fmt.Sprintf("\n  [cyan]Title:[white] %s\n  [cyan]Status:[%s] %s[white]\n  [cyan]Type:[white] %s\n  [cyan]Block Reason:[white] %s\n\n  [yellow]Missing Puzzle Pieces:\n", 
				mem.Title, statusColor, mem.Status, mem.MemoryType, reason)
			
			missingCount := 0
			for _, piece := range mem.Pieces {
				if !piece.IsPurchased {
					missingCount++
					out += fmt.Sprintf("  [white]- %s (%s)\n", piece.ShardName, piece.Rarity)
				}
			}
			
			if len(mem.Pieces) == 0 {
				out += "  [yellow]This memory does not use shards. It unlocks natively via story/event progression.\n"
			} else if missingCount == 0 {
				out += "  [green]All pieces purchased! Scene fully unlocked!\n"
			}
			
			descView.SetText(out)
		} else {
			descView.SetText("\n  [gray]Highlight a memory to see its shard requirements...")
		}
	})

	treeView.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			node.SetExpanded(!node.IsExpanded())
		} else {
			selectedMemory = ref.(api.Memory)
			lblTarget.SetText(fmt.Sprintf("\n  Target: [green]%s [gray](ID: %d)\n", selectedMemory.Title, selectedMemory.ID))
		}
	})

	reloadTree := func() {
		root.ClearChildren()

		if len(MemoryCache) == 0 {
			root.AddChild(tview.NewTreeNode("No memory data...").SetSelectable(false))
		} else {
			// Group by MemoryType
			types := make(map[string][]*api.Memory)
			for i := range MemoryCache {
				m := &MemoryCache[i]
				types[m.MemoryType] = append(types[m.MemoryType], m)
			}

			for mType, mems := range types {
				typeNode := tview.NewTreeNode(mType).
					SetSelectable(true).
					SetExpanded(false).
					SetColor(tview.Styles.PrimaryTextColor)
				root.AddChild(typeNode)

				for _, m := range mems {
					statusMarker := "[green](Open)[white]"
					if m.Status == "hidden" {
						statusMarker = "[red](Locked)[white]"
					}
					nodeTitle := fmt.Sprintf("%s %s", m.Title, statusMarker)
					memNode := tview.NewTreeNode(nodeTitle).
						SetSelectable(true).
						SetReference(*m).
						SetColor(tview.Styles.ContrastBackgroundColor)
					typeNode.AddChild(memNode)
				}
			}
		}
	}
	reloadTree()

	form := tview.NewForm()

	form.AddButton("Fetch Memories", func() {
		lblTarget.SetText("\n  [yellow]Fetching memory configuration from server...")
		go func() {
			memories, err := api.FetchMemories(localClient)
			App.QueueUpdateDraw(func() {
				if err != nil {
					lblTarget.SetText("\n  [red]Error: " + err.Error())
					return
				}
				MemoryCache = memories
				reloadTree()
				lblTarget.SetText(fmt.Sprintf("\n  [green]Successfully loaded %d Memories!\n", len(memories)))
			})
		}()
	})

	var activeCtx context.Context
	var activeCancel context.CancelFunc

	form.AddButton("UNLOCK FULL SCENE", func() {
		if selectedMemory.ID == 0 {
			lblTarget.SetText("\n  [red]Error: You must select a specific memory from the tree!")
			return
		}

		if len(selectedMemory.Pieces) == 0 {
			lblTarget.SetText("\n  [red]Error: This memory cannot be unlocked via shards!")
			return
		}

		var shardsToBuy []string
		for _, piece := range selectedMemory.Pieces {
			if !piece.IsPurchased {
				shardsToBuy = append(shardsToBuy, piece.ShardName)
			}
		}

		if len(shardsToBuy) == 0 {
			lblTarget.SetText("\n  [green]This scene is already fully unlocked!")
			return
		}

		if activeCancel != nil {
			activeCancel()
		}
		activeCtx, activeCancel = context.WithCancel(context.Background())

		lblTarget.SetText(fmt.Sprintf("\n  [yellow]Unlocking %d puzzle pieces for '%s'...", len(shardsToBuy), selectedMemory.Title))

		go func() {
			api.BuyMemoryMany(activeCtx, localClient, selectedMemory.ID, shardsToBuy, func(msg string) {
				App.QueueUpdateDraw(func() {
					statusLog.SetText(statusLog.GetText(false) + "\n  " + msg)
				})
			})
			activeCancel = nil
			
			// Optional: Auto-refresh memory cache after completing to update the tree state
			App.QueueUpdateDraw(func() {
				statusLog.SetText(statusLog.GetText(false) + "\n  [gray]Consider hitting [Fetch Memories] to visually update the UI.")
			})
		}()
	})

	rightPane.AddItem(lblTarget, 3, 0, false)
	rightPane.AddItem(descView, 0, 1, false)
	rightPane.AddItem(form, 7, 0, true)
	rightPane.AddItem(statusLog, 0, 1, false)

	flex.AddItem(treeView, 0, 1, false)
	flex.AddItem(rightPane, 0, 1, true)
	return flex
}
