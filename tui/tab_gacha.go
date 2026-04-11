package tui

import (
	"context"
	"strconv"

	"github.com/rivo/tview"

	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
)

func BuildTabGacha() *tview.Flex {
	flex := tview.NewFlex().SetDirection(tview.FlexColumn)
	flex.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	localClient := api.NewClient("https://prod.api.overlewd.ru")

	// Left: Banners List
	bannerList := tview.NewList().ShowSecondaryText(false)
	bannerList.SetBorder(true).SetTitle(" Active Banners ")

	selectedBannerID := -1

	// Right: Controls
	rightPanel := tview.NewFlex().SetDirection(tview.FlexRow)

	disclaimer := tview.NewTextView().
		SetDynamicColors(true).
		SetText("\n[red]WARNING: Make sure you have enough to complete at least 1 spin before starting!").
		SetTextAlign(tview.AlignCenter)

	infoDisplay := tview.NewTextView().
		SetDynamicColors(true).
		SetText("\n[yellow]Currently Selected: None\n")

	statusDisplay := tview.NewTextView().
		SetDynamicColors(true).
		SetMaxLines(500).
		SetWrap(true).
		SetText("\n Idle...")
	statusDisplay.SetBorder(true).SetTitle(" Status Log ")

	resultsView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true).
		SetText("\n[gray]Results will appear here...\n\nScroll with Up/Down arrows")
	resultsView.SetBorder(true).SetTitle(" Session Summary ")

	descView := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetText("\n[gray]Highlight a banner to read its lore/description...")
	descView.SetBorder(true).SetTitle(" Item Description ")

	form := tview.NewForm()
	form.AddInputField("Spin Amount (Total)", "10", 10, nil, nil)

	// Banner Loader
	reloadBanners := func() {
		statusDisplay.SetText("\n[yellow]Fetching Banners...")
		go func() {
			err := api.FetchBanners(localClient)
			App.QueueUpdateDraw(func() {
				if err != nil {
					statusDisplay.SetText("\n[red]Error fetching banners: " + err.Error())
					return
				}
				statusDisplay.SetText("\n[green]Loaded " + strconv.Itoa(len(api.CachedBanners)) + " Banners!")

				bannerList.Clear()
				for _, b := range api.CachedBanners {
					capturedBanner := b
					bannerList.AddItem(b.FormattedTitle(), "", 0, func() {
						selectedBannerID = capturedBanner.ID
						infoDisplay.SetText("\n[green]Selected: " + capturedBanner.TabTitle + " (ID: " + strconv.Itoa(capturedBanner.ID) + ")")
					})
				}

				bannerList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
					if index >= 0 && index < len(api.CachedBanners) {
						desc := api.CachedBanners[index].Description
						if desc == "" {
							desc = "No description available for this banner."
						}
						descView.SetText(desc)
					}
				})
			})
		}()
	}

	form.AddButton("Force Update UI", func() {
		api.RemoveCacheFile("gacha.json")
		reloadBanners()
	})

	form.AddButton("Spin Selected", func() {
		if selectedBannerID == -1 {
			statusDisplay.SetText("\n[red]Error: You must select a banner first!")
			return
		}
		amountStr := form.GetFormItem(0).(*tview.InputField).GetText()
		amount, err := strconv.Atoi(amountStr)
		if err != nil || amount <= 0 {
			statusDisplay.SetText("\n[red]Error: Amount must be a positive integer!")
			return
		}

		statusDisplay.SetText("\n[blue]Spawning " + strconv.Itoa(amount) + " Spins in Background...\n")
		resultsView.SetText("[yellow]Processing background roll loop... Please wait...")

		go func() {
			api.GachaSpamLoop(context.Background(), localClient, selectedBannerID, amount,
				func(progressMsg string) {
					App.QueueUpdateDraw(func() {
						current := statusDisplay.GetText(true)
						statusDisplay.SetText(progressMsg + "\n" + current)
						statusDisplay.ScrollToBeginning()
					})
				},
				func(resultMsg string) {
					App.QueueUpdateDraw(func() {
						resultsView.SetText(resultMsg)
						resultsView.ScrollToBeginning()
					})
				})
		}()
	})

	form.SetBorder(true).SetTitle(" Gacha Settings ")

	rightPanel.AddItem(form, 0, 1, false)
	rightPanel.AddItem(descView, 10, 0, false)
	rightPanel.AddItem(disclaimer, 5, 0, false)
	rightPanel.AddItem(infoDisplay, 4, 0, false)
	rightPanel.AddItem(statusDisplay, 0, 2, false)

	flex.AddItem(bannerList, 0, 3, true)
	flex.AddItem(resultsView, 0, 3, false)
	flex.AddItem(rightPanel, 0, 3, false)

	// Automatically boot from cache on tab initialization
	reloadBanners()

	return flex
}
