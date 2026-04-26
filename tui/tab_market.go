package tui

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rivo/tview"

	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
)

var MarketCache []api.Market

func BuildTabMarket() *tview.Flex {
	flex := tview.NewFlex().SetDirection(tview.FlexColumn)
	flex.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	localClient := api.NewClient("https://prod.api.overlewd.ru")

	// --- LEFT PANE (MARKET NAVIGATOR) ---
	treeView := tview.NewTreeView().SetGraphics(true)
	treeView.SetBorder(true).SetTitle(" [white]Market Stores ").SetTitleAlign(tview.AlignLeft)

	root := tview.NewTreeNode("Markets").SetSelectable(false)
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
		SetText("\n  [gray]Highlight a bundle to see its extended description and price...")
	descView.SetBorder(true).SetTitle(" Bundle Details ")

	type MarketAsset struct {
		MarketID int
		GoodID   int
	}

	var selectedAsset MarketAsset

	treeView.SetChangedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref != nil {
			asset := ref.(MarketAsset)
			details := api.GetTradableDetails(asset.GoodID)
			
			// Build extended description
			out := fmt.Sprintf("\n  [cyan]Price:[white] %s\n\n  [cyan]Description:[white] %s\n", details.PriceString, details.Description)
			
			if len(details.Includes) > 0 {
				out += "\n  [yellow]Bundle Contents:\n"
				for _, item := range details.Includes {
					itemName := api.GetTradableName(item.TradableID)
					// Sometimes bundle items are nested bundles themselves, but we just print their name globally mapped.
					out += fmt.Sprintf("  [white]- %dx %s\n", item.Count, itemName)
				}
			}
			
			descView.SetText(out)
		} else {
			descView.SetText("\n  [gray]Highlight a bundle to see its extended description and price...")
		}
	})

	treeView.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			node.SetExpanded(!node.IsExpanded())
		} else {
			selectedAsset = ref.(MarketAsset)
			goodName := api.GetTradableName(selectedAsset.GoodID)
			lblTarget.SetText(fmt.Sprintf("\n  Target: [green]%s [gray](Market: %d | ID: %d)\n", goodName, selectedAsset.MarketID, selectedAsset.GoodID))
		}
	})

	reloadTree := func() {
		root.ClearChildren()

		if len(MarketCache) == 0 {
			root.AddChild(tview.NewTreeNode("No market data...").SetSelectable(false))
		} else {
			for i := range MarketCache {
				m := &MarketCache[i]

				marketNode := tview.NewTreeNode(m.Name).
					SetSelectable(true).
					SetExpanded(false).
					SetColor(tview.Styles.PrimaryTextColor)
				root.AddChild(marketNode)

				for j := range m.Tabs {
					tab := &m.Tabs[j]

					var validGoods []int
					for _, goodID := range tab.Goods {
						if !api.GetTradableDetails(goodID).HideFromMarket {
							validGoods = append(validGoods, goodID)
						}
					}

					if len(validGoods) == 0 {
						continue
					}

					var attachNode *tview.TreeNode

					// Collapse identical Market Name -> Tab Title structures
					if tab.Title == m.Name || tab.Title == "" {
						attachNode = marketNode
					} else if len(validGoods) == 1 && api.GetTradableName(validGoods[0]) == tab.Title || len(validGoods) == 1 && tab.Title == "Resource exchange" {
						// If tab only has exactly 1 item and shares the title, strip the tab folder to avoid UI bloat.
						attachNode = marketNode
					} else {
						tabNode := tview.NewTreeNode(tab.Title).
							SetSelectable(true).
							SetExpanded(false).
							SetColor(tview.Styles.SecondaryTextColor)
						marketNode.AddChild(tabNode)
						attachNode = tabNode
					}

					for _, goodID := range validGoods {
						goodName := api.GetTradableName(goodID)
						goodNode := tview.NewTreeNode(goodName).
							SetSelectable(true).
							SetReference(MarketAsset{MarketID: m.ID, GoodID: goodID}).
							SetColor(tview.Styles.ContrastBackgroundColor)
						attachNode.AddChild(goodNode)
					}
				}
			}
		}
	}
	reloadTree()

	form := tview.NewForm()

	form.AddButton("Fetch Markets", func() {
		lblTarget.SetText("\n  [yellow]Fetching markets...")
		go func() {
			markets, err := api.FetchMarkets(localClient)
			App.QueueUpdateDraw(func() {
				if err != nil {
					lblTarget.SetText("\n  [red]Error: " + err.Error())
					return
				}
				MarketCache = markets
				reloadTree()
				lblTarget.SetText("\n  [green]Markets Successfully Hydrated!\n")
			})
		}()
	})

	form.AddButton("BUY SELECTED", func() {
		if selectedAsset.GoodID == 0 {
			lblTarget.SetText("\n  [red]Error: You must select a specific good from the TreeView to purchase!")
			return
		}
		
		goodName := api.GetTradableName(selectedAsset.GoodID)
		lblTarget.SetText(fmt.Sprintf("\n  [yellow]Executing purchase for %s...", goodName))
		
		go func() {
			out, err := api.BuyMarketItem(localClient, selectedAsset.MarketID, selectedAsset.GoodID)
			App.QueueUpdateDraw(func() {
				if err != nil {
					statusLog.SetText(statusLog.GetText(false) + fmt.Sprintf("\n  [red]Transaction Failed: [white]%s %s", err.Error(), out))
					return
				}
				statusLog.SetText(statusLog.GetText(false) + fmt.Sprintf("\n  [green]Transaction Successful! [white]%s", out))
			})
		}()
	})

	inputAmount := tview.NewInputField().
		SetLabel("Amount to Buy: ").
		SetFieldWidth(10).
		SetText("1").
		SetAcceptanceFunc(tview.InputFieldInteger)

	form.AddFormItem(inputAmount)

	var activeCtx context.Context
	var activeCancel context.CancelFunc

	form.AddButton("BUY MULTIPLE", func() {
		if selectedAsset.GoodID == 0 {
			lblTarget.SetText("\n  [red]Error: You must select a specific good from the TreeView to purchase!")
			return
		}

		rawAmt := inputAmount.GetText()
		amt, err := strconv.Atoi(rawAmt)
		if err != nil || amt <= 0 {
			lblTarget.SetText("\n  [red]Error: You must specify a valid integer amount greater than 0!")
			return
		}

		if activeCancel != nil {
			activeCancel()
		}
		activeCtx, activeCancel = context.WithCancel(context.Background())

		goodName := api.GetTradableName(selectedAsset.GoodID)
		lblTarget.SetText(fmt.Sprintf("\n  [yellow]Executing %d purchases for %s across 6 routines...", amt, goodName))

		go func() {
			api.BuyMarketMany(activeCtx, localClient, selectedAsset.MarketID, selectedAsset.GoodID, amt, func(msg string) {
				App.QueueUpdateDraw(func() {
					statusLog.SetText(statusLog.GetText(false) + "\n  " + msg)
				})
			})
			activeCancel = nil
		}()
	})

	rightPane.AddItem(lblTarget, 3, 0, false)
	rightPane.AddItem(descView, 0, 1, false)
	rightPane.AddItem(form, 9, 0, true)
	rightPane.AddItem(statusLog, 0, 1, false)

	flex.AddItem(treeView, 0, 1, false)
	flex.AddItem(rightPane, 0, 1, true)
	return flex
}
