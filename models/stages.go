package models

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"jewrexxx-fearlessrevolution-overlewd-cheat/api"
)

type Stage struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Key      string `json:"key"`
	Type     string `json:"type"`
	BattleID *int   `json:"battleId"`
	Status   string `json:"status"`

	// Custom fields we inject for internal TUI routing
	Endpoint string
	Name     string

	// Enriched dict relationships
	ReadableCategory string
	ReadableChapter  string
	Rewards          []RewardItem
	FirstRewards     []RewardItem
}

// Global cached slice for our dropdown menus
var CachedBattles []Stage

// FetchStages requests both stage pools using our customized client.
func FetchStages(client *api.OverlewdClient) error {
	var allStages []Stage

	// Fetch FTUE Stages
	if b, err := api.LoadOrFetch("/ftue-stages", "ftue_stages.json", client); err == nil {
		var stages []Stage
		if err := json.Unmarshal(b, &stages); err == nil {
			for i := range stages {
				stages[i].Endpoint = "ftue-stages"
				stages[i].Name = stages[i].Key // ftue-stages use "key"
			}
			allStages = append(allStages, stages...)
		} else {
			log.Printf("[ERROR] JSON Unmarshal FTUE failed: %v", err)
		}
	} else {
		log.Printf("[ERROR] Fetch failure to /ftue-stages: %v", err)
	}

	// Fetch Event Stages
	if b, err := api.LoadOrFetch("/event-stages", "event_stages.json", client); err == nil {
		var stages []Stage
		if err := json.Unmarshal(b, &stages); err == nil {
					for i := range stages {
						stages[i].Endpoint = "event-stages"
						stages[i].Name = stages[i].Title
					}
			allStages = append(allStages, stages...)
		} else {
			log.Printf("[ERROR] JSON Unmarshal Event failed: %v", err)
		}
	} else {
		log.Printf("[ERROR] Fetch failure to /event-stages: %v", err)
	}

	// Filter purely for stages that have battles to grind AND are accessible (open or complete)
	var validBattles []Stage
	for _, st := range allStages {
		if st.BattleID != nil && *st.BattleID > 0 {
			if st.Status == "open" || st.Status == "complete" || st.Status == "completed" {
				if st.Name == "" {
					st.Name = fmt.Sprintf("Unknown Battle %d", *st.BattleID)
				}
				validBattles = append(validBattles, st)
			}
		}
	}

	// Sort alphabetically by name for the UI
	sort.Slice(validBattles, func(i, j int) bool {
		return validBattles[i].Name < validBattles[j].Name
	})

	CachedBattles = validBattles
	log.Printf("[INFO] Successfully cached %d valid battle stages into memory.", len(CachedBattles))
	return nil
}
