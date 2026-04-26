package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type EquipmentItem struct {
	ID          int    `json:"id"`
	CharacterId *int   `json:"characterId"`
	Rarity      string `json:"rarity"`
}

func FetchEquipment(client *OverlewdClient) ([]EquipmentItem, error) {
	log.Println("[INFO] Fetching Equipment...")
	req, _ := http.NewRequest("GET", client.BaseURL+"/battles/my/characters/equipment", nil)

	resp, err := client.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed fetching equipment: %d", resp.StatusCode)
	}

	b, _ := io.ReadAll(resp.Body)
	var roster []EquipmentItem
	if err := json.Unmarshal(b, &roster); err != nil {
		return nil, err
	}

	return roster, nil
}

func DissolveEquipmentRarities(client *OverlewdClient, equipment []EquipmentItem, targets map[string]bool) {
	if len(equipment) == 0 {
		return
	}

	var toDissolve []string
	for _, e := range equipment {
		if e.CharacterId == nil && targets[e.Rarity] {
			toDissolve = append(toDissolve, fmt.Sprintf("equipmentIds%%5b%%5d=%d", e.ID))
		}
	}

	if len(toDissolve) == 0 {
		log.Println("[DISSOLVE] Aborted. No unassigned equipment matched the targeted rarities.")
		return
	}

	batchSize := 300
	log.Printf("[DISSOLVE] Sequence Initiated for %d equipment items (Batches of %d)...", len(toDissolve), batchSize)

	for i := 0; i < len(toDissolve); i += batchSize {
		end := i + batchSize
		if end > len(toDissolve) {
			end = len(toDissolve)
		}
		
		chunk := toDissolve[i:end]
		payload := strings.Join(chunk, "&")

		req, _ := http.NewRequest("POST", client.BaseURL+"/battles/my/characters/equipment/dissolve", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.DoRequest(req)
		if err != nil {
			log.Printf("[DISSOLVE] HTTP Error on batch: %v", err)
			continue
		}

		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			log.Printf("[DISSOLVE] Batch %d-%d Successful! HTTP 200", i, end)
		} else {
			log.Printf("[DISSOLVE] Batch %d-%d Failed %d: %s", i, end, resp.StatusCode, string(b))
		}
	}
	log.Printf("[DISSOLVE] All batches perfectly completed!")
}
