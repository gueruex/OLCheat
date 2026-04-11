package models

import (
	"encoding/json"
	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
	"log"
)

type FTUEResponse struct {
	Chapters []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Stages []int  `json:"stages"`
	} `json:"chapters"`
}

type Event struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Chapters []int  `json:"chapters"`
}

type EventChapter struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	EventID int    `json:"eventId"`
	Stages  []int  `json:"stages"`
}

type RewardItem struct {
	TradableID  int     `json:"tradableId"`
	Amount      int     `json:"amount"`
	Probability float64 `json:"probability"`
}

type RewardsPayload struct {
	Rewards []RewardItem `json:"rewards"`
}

type BattleDict struct {
	ID           int            `json:"id"`
	Title        string         `json:"title"`
	Rewards      RewardsPayload `json:"rewards"`
	FirstRewards RewardsPayload `json:"firstRewards"`
}

// EnrichStages downloads auxiliary dictionaries from the game and maps the unintuitive DB stage arrays over to human readable contexts!
func EnrichStages(client *api.OverlewdClient) {
	// 1. Fetch FTUE (Main Campaign Map)
	var ftue FTUEResponse
	if b, err := api.LoadOrFetch("/ftue", "ftue.json", client); err == nil {
		json.Unmarshal(b, &ftue)
	}

	// 2. Fetch Events overarching banners
	var events []Event
	if b, err := api.LoadOrFetch("/events", "events.json", client); err == nil {
		json.Unmarshal(b, &events)
	}

	// 3. Fetch specific Event Chapters
	var eventChaps []EventChapter
	if b, err := api.LoadOrFetch("/event-chapters", "event_chapters.json", client); err == nil {
		json.Unmarshal(b, &eventChaps)
	}

	// 4. Fetch the massive 9MB /battles global dictionary!
	var battlesDict []BattleDict
	if b, err := api.LoadOrFetch("/battles", "battles.json", client); err == nil {
		json.Unmarshal(b, &battlesDict)
	}

	// Build Memory Dict Linkages
	stageToFTUEChapter := make(map[int]string)
	for _, ch := range ftue.Chapters {
		for _, stID := range ch.Stages {
			stageToFTUEChapter[stID] = ch.Name
		}
	}

	globalBattleLoreTitles := make(map[int]string)
	globalBattleRewards := make(map[int][]RewardItem)
	globalBattleFirstRewards := make(map[int][]RewardItem)
	
	for _, b := range battlesDict {
		if b.Title != "" {
			globalBattleLoreTitles[b.ID] = b.Title
		}
		if len(b.Rewards.Rewards) > 0 {
			globalBattleRewards[b.ID] = b.Rewards.Rewards
		}
		if len(b.FirstRewards.Rewards) > 0 {
			globalBattleFirstRewards[b.ID] = b.FirstRewards.Rewards
		}
	}

	eventIDToName := make(map[int]string)
	for _, ev := range events {
		eventIDToName[ev.ID] = ev.Name
	}

	stageToEventName := make(map[int]string)
	stageToEventChapName := make(map[int]string)
	for _, ch := range eventChaps {
		evName := eventIDToName[ch.EventID]
		if evName == "" {
			evName = "Unknown Event"
		}
		for _, stID := range ch.Stages {
			stageToEventChapName[stID] = ch.Name
			stageToEventName[stID] = evName
		}
	}

	// Hot-inject the values in-place directly over our cached global arrays
	for i := range CachedBattles {
		b := &CachedBattles[i]
		if b.Endpoint == "ftue-stages" {
			b.ReadableCategory = "Main Campaign"
			if chName, ok := stageToFTUEChapter[b.ID]; ok {
				b.ReadableChapter = chName
			} else {
				b.ReadableChapter = "Uncategorized FTUE"
			}

			// Clean up "battle1" into "Battle 1"
			if len(b.Name) > 6 && b.Name[:6] == "battle" {
				b.Name = "Battle " + b.Name[6:]
			}
		} else {
			if evName, ok := stageToEventName[b.ID]; ok {
				b.ReadableCategory = "Event: " + evName
			} else {
				b.ReadableCategory = "Unknown Event Category"
			}

			if chName, ok := stageToEventChapName[b.ID]; ok {
				b.ReadableChapter = chName
			} else {
				b.ReadableChapter = "Uncategorized Event Chapter"
			}
		}

		// Inject the newly found Lore dictionary from /battles globally to ALL stages!
		if b.BattleID != nil {
			if loreTitle, ok := globalBattleLoreTitles[*b.BattleID]; ok {
				b.Name = loreTitle
			}
			if rw, ok := globalBattleRewards[*b.BattleID]; ok {
				b.Rewards = rw
			}
			if fRw, ok := globalBattleFirstRewards[*b.BattleID]; ok {
				b.FirstRewards = fRw
			}
		}
	}

	log.Println("[INFO] Successfully enriched Battle names with readable dictionaries!")
}
