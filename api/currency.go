package api

import (
	"encoding/json"
	"fmt"
	"log"
)

type CurrencyNode struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Rarity string `json:"rarity"`
	Key    string `json:"key"`
}

var CurrencyCache = make(map[int]string)
var RarityCache = make(map[int]string)
var CharacterCache = make(map[int]string)

func EnrichCurrencies(client *OverlewdClient) {
	log.Println("[INFO] Hydrating Global Mixed Dictionary (Currencies + Tradables)...")
	
	fetchAndMerge := func(endpoint string, filename string) {
		b, err := LoadOrFetch(endpoint, filename, client)
		if err != nil {
			log.Printf("[WARNING] Failed to load %s: %v", endpoint, err)
			return
		}
		
		var nodes []CurrencyNode
		if err := json.Unmarshal(b, &nodes); err != nil {
			log.Printf("[WARNING] %s payload failed to unmarshal: %v", endpoint, err)
			return
		}

		for _, n := range nodes {
			if endpoint == "/battles/characters" {
				CharacterCache[n.ID] = n.Name
			} else {
				CurrencyCache[n.ID] = n.Name
				if n.Rarity != "" {
					RarityCache[n.ID] = n.Rarity
				}
			}
		}
		log.Printf("[INFO] Pushed %d items from [%s] into Dictionary.", len(nodes), endpoint)
	}

	fetchAndMerge("/currencies", "currencies.json")
	fetchAndMerge("/tradable", "tradable.json")
	fetchAndMerge("/battles/characters", "characters.json")
}

func GetCurrencyName(id int) string {
	if name, exists := CurrencyCache[id]; exists {
		return name
	}
	// Fallback to generic tag if completely missing
	if id == 65 {
		return "Gems"
	}
	return fmt.Sprintf("Unknown Item [%d]", id)
}

func GetItemRarity(id int) string {
	if rarity, exists := RarityCache[id]; exists {
		return rarity
	}
	return ""
}

func GetCharacterName(id int) string {
	if name, exists := CharacterCache[id]; exists {
		return name
	}
	return fmt.Sprintf("Unknown Character [%d]", id)
}
