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
	rootFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	rootFlex.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	topFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
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

	var globalPotions []api.GlobalPotion
	var myPotions []api.MyPotion
	
	buyDropdown := tview.NewDropDown().
		SetLabel("Potion to Buy: ").
		SetOptions([]string{"Hit 'Fetch Markets' to load..."}, nil)
	
	useDropdown := tview.NewDropDown().
		SetLabel("Potion to Use: ").
		SetOptions([]string{"Hit 'Fetch Markets' to load..."}, nil)

	form := tview.NewForm()

	form.AddButton("Fetch Markets & Potions", func() {
		lblTarget.SetText("\n  [yellow]Fetching markets and syncing inventory...")
		go func() {
			markets, err := api.FetchMarkets(localClient)
			gPots, _ := api.FetchGlobalPotions(localClient)
			mPots, _ := api.FetchMyPotions(localClient)
			
			App.QueueUpdateDraw(func() {
				if err != nil {
					lblTarget.SetText("\n  [red]Error: " + err.Error())
					return
				}
				MarketCache = markets
				globalPotions = gPots
				myPotions = mPots
				
				var buyOpts []string
				for _, p := range globalPotions {
					buyOpts = append(buyOpts, fmt.Sprintf("[%d] %s", p.ID, p.Name))
				}
				if len(buyOpts) > 0 {
					buyDropdown.SetOptions(buyOpts, nil)
					buyDropdown.SetCurrentOption(0)
				}
				
				var useOpts []string
				for _, p := range myPotions {
					name := "Unknown Potion"
					for _, gp := range globalPotions {
						if gp.ID == p.ID {
							name = gp.Name
							break
						}
					}
					useOpts = append(useOpts, fmt.Sprintf("[%d] %s (Inventory: %d)", p.ID, name, p.Count))
				}
				if len(useOpts) > 0 {
					useDropdown.SetOptions(useOpts, nil)
					useDropdown.SetCurrentOption(0)
				}

				reloadTree()
				lblTarget.SetText("\n  [green]Markets and Potions Successfully Hydrated!\n")
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

	topFlex.AddItem(treeView, 0, 1, false)
	topFlex.AddItem(rightPane, 0, 1, true)

	// --- BOTTOM PANE (POTIONS BULK BUY/USE) ---
	bottomFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	
	// POTIONS BUY FORM
	buyForm := tview.NewForm()
	buyForm.SetBorder(true).SetTitle(" Bulk Buy Potions ")
	
	buyForm.AddFormItem(buyDropdown)

	inputBuyAmt := tview.NewInputField().
		SetLabel("Amount to Buy: ").
		SetFieldWidth(10).
		SetText("10").
		SetAcceptanceFunc(tview.InputFieldInteger)
	buyForm.AddFormItem(inputBuyAmt)

	buyForm.AddButton("BUY POTIONS", func() {
		idx, _ := buyDropdown.GetCurrentOption()
		if len(globalPotions) == 0 || idx < 0 || idx >= len(globalPotions) {
			statusLog.SetText(statusLog.GetText(false) + "\n  [red]Error: You must fetch markets first or select a valid potion!")
			return
		}
		pID := globalPotions[idx].ID

		amt, _ := strconv.Atoi(inputBuyAmt.GetText())
		if pID <= 0 || amt <= 0 {
			statusLog.SetText(statusLog.GetText(false) + "\n  [red]Error: Invalid Amount!")
			return
		}
		statusLog.SetText(statusLog.GetText(false) + fmt.Sprintf("\n  [yellow]Executing Bulk Buy: %dx Potion %d...", amt, pID))
		go func() {
			out, err := api.BuyPotions(localClient, pID, amt)
			App.QueueUpdateDraw(func() {
				if err != nil {
					statusLog.SetText(statusLog.GetText(false) + fmt.Sprintf("\n  [red]Buy Failed: [white]%s %s", err.Error(), out))
					return
				}
				statusLog.SetText(statusLog.GetText(false) + fmt.Sprintf("\n  [green]Buy Successful! [white]%s", out))
			})
		}()
	})

	// POTIONS USE FORM
	useForm := tview.NewForm()
	useForm.SetBorder(true).SetTitle(" Bulk Use Potions ")

	useForm.AddFormItem(useDropdown)

	inputUseAmt := tview.NewInputField().
		SetLabel("Amount to Use: ").
		SetFieldWidth(10).
		SetText("5").
		SetAcceptanceFunc(tview.InputFieldInteger)
	useForm.AddFormItem(inputUseAmt)

	useForm.AddButton("USE POTIONS", func() {
		idx, _ := useDropdown.GetCurrentOption()
		if len(myPotions) == 0 || idx < 0 || idx >= len(myPotions) {
			statusLog.SetText(statusLog.GetText(false) + "\n  [red]Error: You must fetch markets first or select a valid potion!")
			return
		}
		pID := myPotions[idx].ID

		amt, _ := strconv.Atoi(inputUseAmt.GetText())
		if pID <= 0 || amt <= 0 {
			statusLog.SetText(statusLog.GetText(false) + "\n  [red]Error: Invalid Amount!")
			return
		}
		statusLog.SetText(statusLog.GetText(false) + fmt.Sprintf("\n  [yellow]Executing Bulk Use: %dx Potion %d...", amt, pID))
		go func() {
			out, err := api.UsePotions(localClient, pID, amt)
			App.QueueUpdateDraw(func() {
				if err != nil {
					statusLog.SetText(statusLog.GetText(false) + fmt.Sprintf("\n  [red]Use Failed: [white]%s %s", err.Error(), out))
					return
				}
				statusLog.SetText(statusLog.GetText(false) + fmt.Sprintf("\n  [green]Use Successful! [white]%s", out))
			})
		}()
	})

	bottomFlex.AddItem(buyForm, 0, 1, false)
	bottomFlex.AddItem(useForm, 0, 1, false)

	rootFlex.AddItem(topFlex, 0, 2, true)
	rootFlex.AddItem(bottomFlex, 0, 1, false)

	return rootFlex
}
