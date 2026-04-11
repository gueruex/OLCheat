package api

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
)

type TradablePriceNode struct {
	CurrencyID int `json:"currencyId"`
	Amount     int `json:"amount"`
}

type TradablePriceBlock struct {
	Price []TradablePriceNode `json:"price"`
}

type TradableItemPackNode struct {
	TradableID int `json:"tradableId"`
	Count      int `json:"count"`
}

type CurrencyNode struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	Rarity      string                 `json:"rarity"`
	Key         string                 `json:"key"`
	Description string                 `json:"description"`
	ItemPack    []TradableItemPackNode `json:"itemPack"`
	Price       TradablePriceBlock     `json:"price"`
}

type TradableDetails struct {
	Description string
	PriceString string
	Includes    []TradableItemPackNode
}

var (
	cacheMutex     sync.RWMutex
	CurrencyCache  = make(map[int]string)
	RarityCache    = make(map[int]string)
	CharacterCache = make(map[int]string)
	TradableCache  = make(map[int]TradableDetails)
)

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

		cacheMutex.Lock()
		for _, n := range nodes {
			if endpoint == "/battles/characters" {
				CharacterCache[n.ID] = n.Name
			} else {
				CurrencyCache[n.ID] = n.Name
				if n.Rarity != "" {
					RarityCache[n.ID] = n.Rarity
				}
				if endpoint == "/tradable" {
					priceStr := "Free"
					if len(n.Price.Price) > 0 {
						p := n.Price.Price[0]
						cName := "Unknown Currency"
						if val, ok := CurrencyCache[p.CurrencyID]; ok {
							cName = val
						} else if p.CurrencyID == 65 {
							cName = "Gems"
						}
						
						if cName == "US Dollar Cent" {
							priceStr = fmt.Sprintf("%.2f USD", float64(p.Amount)/100.0)
						} else {
							priceStr = fmt.Sprintf("%d %s", p.Amount, cName)
						}
					}
					desc := n.Description
					if desc == "" {
						desc = "No description provided."
					}
					TradableCache[n.ID] = TradableDetails{
						Description: desc,
						PriceString: priceStr,
						Includes:    n.ItemPack,
					}
				}
			}
		}
		cacheMutex.Unlock()
		log.Printf("[INFO] Pushed %d items from [%s] into Dictionary.", len(nodes), endpoint)
	}

	fetchAndMerge("/currencies", "currencies.json")
	fetchAndMerge("/tradable", "tradable.json")
	fetchAndMerge("/battles/characters", "characters.json")
}

func GetCurrencyName(id int) string {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
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
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	if rarity, exists := RarityCache[id]; exists {
		return rarity
	}
	return ""
}

func GetCharacterName(id int) string {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	if name, exists := CharacterCache[id]; exists {
		return name
	}
	return fmt.Sprintf("Unknown Character [%d]", id)
}

func GetTradableDetails(id int) TradableDetails {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	if details, exists := TradableCache[id]; exists {
		return details
	}
	return TradableDetails{
		Description: "No description provided.",
		PriceString: "Unknown Price",
	}
}
