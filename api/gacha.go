package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"strconv"
)

func ExtractGachaLootRecursively(data interface{}) map[string]int {
	loot := make(map[string]int) 
	var walk func(v interface{})
	walk = func(v interface{}) {
		switch val := v.(type) {
		case map[string]interface{}:
			var itemType string
			var id int
			var ok bool
			
			if tid, found := val["tradableId"].(float64); found {
				itemType = "tradable"
				id = int(tid)
				ok = true
			} else if cid, found := val["characterId"].(float64); found {
				itemType = "character"
				id = int(cid)
				ok = true
			} else if curId, found := val["currencyId"].(float64); found {
				itemType = "currency"
				id = int(curId)
				ok = true
			}

			if ok {
				amt := 1
				if a, hasAmt := val["amount"].(float64); hasAmt {
					amt = int(a)
				}
				key := fmt.Sprintf("%s:%d", itemType, id)
				loot[key] += amt
			}
			
			for _, v := range val {
				walk(v)
			}
		case []interface{}:
			for _, v := range val {
				walk(v)
			}
		}
	}
	walk(data)
	return loot
}

type PriceBlock struct {
	CurrencyId int `json:"currencyId"`
	Amount     int `json:"amount"`
}

type PriceStruct struct {
	Price []PriceBlock `json:"price"`
}

type GachaBanner struct {
	ID           int         `json:"id"`
	TabType      string      `json:"tabType"`
	TabTitle     string      `json:"tabTitle"`
	Description  string      `json:"description"`
	Available    bool        `json:"available"`
	PriceForOne  PriceStruct `json:"priceForOne"`
	PriceForMany PriceStruct `json:"priceForMany"`
}

func (g *GachaBanner) FormattedTitle() string {
	cost := 0
	currencyName := "N/A"
	if len(g.PriceForMany.Price) > 0 {
		cost = g.PriceForMany.Price[0].Amount
		currencyName = GetCurrencyName(g.PriceForMany.Price[0].CurrencyId)
	} else if len(g.PriceForOne.Price) > 0 {
		cost = g.PriceForOne.Price[0].Amount
		currencyName = GetCurrencyName(g.PriceForOne.Price[0].CurrencyId)
	}
	tType := strings.Title(strings.ReplaceAll(g.TabType, "_", " "))
	return fmt.Sprintf("[%s] %s (Cost: %dx %s)", tType, g.TabTitle, cost, currencyName)
}

var CachedBanners []GachaBanner

func FetchBanners(client *OverlewdClient) error {
	log.Println("[INFO] Fetching Gacha Banners...")
	
	b, err := LoadOrFetch("/gacha", "gacha.json", client)
	if err != nil {
		return fmt.Errorf("failed to load banners: %w", err)
	}
	var banners []GachaBanner
	if err := json.Unmarshal(b, &banners); err != nil {
		return err
	}

	sort.Slice(banners, func(i, j int) bool {
		if banners[i].TabType == banners[j].TabType {
			return banners[i].ID < banners[j].ID
		}
		return banners[i].TabType < banners[j].TabType
	})

	CachedBanners = banners
	log.Printf("[INFO] Cached %d active banners.", len(banners))
	return nil
}

func GachaSpamLoop(ctx context.Context, client *OverlewdClient, bannerID int, amount int, onProgress func(string), onResult func(string)) {
	log.Printf("[GACHA] Initiating %d spins on Banner ID: %d", amount, bannerID)

	var wg sync.WaitGroup
	concurrencyLimit := make(chan struct{}, 10)

	var mu sync.Mutex
	globalLootMap := make(map[string]int)
	var failedSpins int
	
	reportProgress := func(msg string) {
		if onProgress != nil {
			onProgress(msg)
		}
	}

Loop:
	for i := 1; i <= amount; i++ {
		select {
		case <-ctx.Done():
			log.Println("[GACHA] Execution aborted by User.")
			mu.Lock()
			failedSpins += (amount - i + 1)
			mu.Unlock()
			break Loop
		default:
		}

		concurrencyLimit <- struct{}{}
		wg.Add(1)

		go func(spinIndex int) {
			defer wg.Done()
			defer func() { <-concurrencyLimit }()

			endpoint := fmt.Sprintf("/gacha/%d/buy-many", bannerID)
			req, _ := http.NewRequest("POST", client.BaseURL+endpoint, nil)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Content-Length", "0")

			req = req.WithContext(ctx)
			resp, err := client.DoRequest(req)

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				mu.Lock()
				failedSpins++
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
				var rawData interface{}
				json.Unmarshal(b, &rawData)
				lootMap := ExtractGachaLootRecursively(rawData)

				if len(lootMap) > 0 {
					mu.Lock()
					for key, amt := range lootMap {
						globalLootMap[key] += amt
					}
					mu.Unlock()
					reportProgress(fmt.Sprintf("[Spin %d] Complete", spinIndex))
				} else {
					log.Printf("[Spin %d] Success! Raw: %s", spinIndex, string(b))
					reportProgress(fmt.Sprintf("[Spin %d] Complete (Unmapped Drop)", spinIndex))
				}
			} else {
				mu.Lock()
				failedSpins++
				mu.Unlock()
				reportProgress(fmt.Sprintf("[Spin %d] Failed (State %d)", spinIndex, resp.StatusCode))
				log.Printf("[Spin %d] Failed %d: %s", spinIndex, resp.StatusCode, string(b))
			}
		}(i)
	}

	wg.Wait()
	log.Println("[GACHA] Completed all requested spins!")
	reportProgress(fmt.Sprintf("[GACHA] Finished dispatching %d sweeps!", amount))

	if len(globalLootMap) > 0 || failedSpins > 0 {
		type SortableLoot struct {
			Key      string
			Name     string
			Rarity   string
			Amount   int
			Rank     int
			Color    string
		}

		var finalLoot []SortableLoot
		for key, amt := range globalLootMap {
			parts := strings.Split(key, ":")
			if len(parts) != 2 {
				continue
			}

			itemType := parts[0]
			id, _ := strconv.Atoi(parts[1])

			var name, rarity string
			if itemType == "character" {
				name = GetCharacterName(id)
				rarity = GetItemRarity(id)
			} else {
				name = GetCurrencyName(id)
				rarity = GetItemRarity(id)
			}

			rank := 0
			color := "white"
			r := strings.ToLower(rarity)
			switch r {
			case "legendary":
				rank = 4
				color = "yellow"
			case "epic":
				rank = 3
				color = "magenta"
			case "heroic":
				rank = 2
				color = "#00BFFF"
			case "rare":
				rank = 1
				color = "blue"
			}

			finalLoot = append(finalLoot, SortableLoot{
				Key:    key,
				Name:   name,
				Rarity: rarity,
				Amount: amt,
				Rank:   rank,
				Color:  color,
			})
		}

		sort.Slice(finalLoot, func(i, j int) bool {
			if finalLoot[i].Rank != finalLoot[j].Rank {
				return finalLoot[i].Rank > finalLoot[j].Rank
			}
			return finalLoot[i].Name < finalLoot[j].Name
		})

		var earned []string
		for _, v := range finalLoot {
			amtStr := ""
			if v.Amount > 1 {
				amtStr = fmt.Sprintf("%dx ", v.Amount)
			}

			rDisplay := ""
			if v.Rarity != "" {
				rTitle := strings.Title(v.Rarity)
				// Prevent Duplicate Rarity string rendering
				if !strings.HasPrefix(strings.ToLower(v.Name), strings.ToLower(rTitle)) {
					rDisplay = rTitle + " "
				}
			}

			earned = append(earned, fmt.Sprintf("%s[%s]%s%s[white]", amtStr, v.Color, rDisplay, v.Name))
		}

		if failedSpins > 0 {
			earned = append(earned, fmt.Sprintf("\n  [red]%dx Spins Failed or Aborted[white]", failedSpins))
		}

		if onResult != nil {
			onResult(strings.Join(earned, "\n"))
		}
	} else {
		if onResult != nil {
			onResult("[red]No Results Returned!")
		}
	}
}
