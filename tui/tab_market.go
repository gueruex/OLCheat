package tui

import (
	"fmt"

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
	treeView.SetBorder(true).SetTitle(" [white]Market Stores [red](WiP) ").SetTitleAlign(tview.AlignLeft)

	root := tview.NewTreeNode("Markets").SetSelectable(false)
	treeView.SetRoot(root).SetCurrentNode(root)

	// --- RIGHT PANE (CONTROLS) ---
	rightPane := tview.NewFlex().SetDirection(tview.FlexRow)

	lblTarget := tview.NewTextView().SetDynamicColors(true).
		SetText("\n  Target: [gray]Nothing Selected\n")

	descView := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetText("\n  [red]This module is under construction.\n  [gray]Highlight a bundle to see its extended description and price...")
	descView.SetBorder(true).SetTitle(" Bundle Details [yellow](Not Implemented) ")

	var selectedGoodID int = 0

	treeView.SetChangedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref != nil {
			goodID := ref.(int)
			details := api.GetTradableDetails(goodID)
			
			// Build extended description
			out := fmt.Sprintf("\n  [cyan]Price:[white] %s\n\n  [cyan]Description:[white] %s\n", details.PriceString, details.Description)
			
			if len(details.Includes) > 0 {
				out += "\n  [yellow]Bundle Contents:\n"
				for _, item := range details.Includes {
					itemName := api.GetCurrencyName(item.TradableID)
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
			selectedGoodID = ref.(int)
			goodName := api.GetCurrencyName(selectedGoodID)
			lblTarget.SetText(fmt.Sprintf("\n  Target: [green]%s [gray](ID: %d)\n", goodName, selectedGoodID))
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

					tabNode := tview.NewTreeNode(tab.Title).
						SetSelectable(true).
						SetExpanded(false).
						SetColor(tview.Styles.SecondaryTextColor)
					marketNode.AddChild(tabNode)

					for _, goodID := range tab.Goods {
						goodName := api.GetCurrencyName(goodID)
						goodNode := tview.NewTreeNode(goodName).
							SetSelectable(true).
							SetReference(goodID).
							SetColor(tview.Styles.ContrastBackgroundColor)
						tabNode.AddChild(goodNode)
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
		if selectedGoodID == 0 {
			lblTarget.SetText("\n  [red]Error: You must select a specific good from the TreeView to purchase!")
			return
		}
		lblTarget.SetText(fmt.Sprintf("\n  [yellow]Prototype Mode Active. Refused transaction for ID: %d", selectedGoodID))
	})

	rightPane.AddItem(lblTarget, 3, 0, false)
	rightPane.AddItem(descView, 15, 0, false)
	rightPane.AddItem(form, 0, 1, true)

	flex.AddItem(treeView, 0, 1, false)
	flex.AddItem(rightPane, 0, 1, true)
	return flex
}
