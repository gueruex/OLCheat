package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type CharacterItem struct {
	ID              int           `json:"id"`
	BaseCharacterID int           `json:"baseCharacterId"`
	Rarity          string        `json:"rarity"`
	Equipment       []interface{} `json:"equipment"`
}

func FetchCharacters(client *OverlewdClient) ([]CharacterItem, error) {
	log.Println("[INFO] Fetching Roster...")
	req, _ := http.NewRequest("GET", client.BaseURL+"/battles/my/characters", nil)

	resp, err := client.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed fetching characters: %d", resp.StatusCode)
	}

	b, _ := io.ReadAll(resp.Body)
	var roster []CharacterItem
	if err := json.Unmarshal(b, &roster); err != nil {
		return nil, err
	}

	return roster, nil
}

func DissolveRarities(client *OverlewdClient, chars []CharacterItem, targets map[string]bool) {
	if len(chars) == 0 {
		return
	}

	var toDissolve []string
	for _, c := range chars {
		if c.BaseCharacterID == 1 {
			continue // SAfEGUARD OVERLORD
		}
		if len(c.Equipment) == 0 && targets[c.Rarity] {
			toDissolve = append(toDissolve, fmt.Sprintf("characterIds%%5b%%5d=%d", c.ID))
		}
	}

	if len(toDissolve) == 0 {
		log.Println("[DISSOLVE] Aborted. No characters matched the targeted rarities.")
		return
	}

	batchSize := 300
	log.Printf("[DISSOLVE] Sequence Initiated for %d characters (Batches of %d)...", len(toDissolve), batchSize)

	for i := 0; i < len(toDissolve); i += batchSize {
		end := i + batchSize
		if end > len(toDissolve) {
			end = len(toDissolve)
		}
		
		chunk := toDissolve[i:end]
		payload := strings.Join(chunk, "&")

		req, _ := http.NewRequest("POST", client.BaseURL+"/battles/my/characters/dissolve", strings.NewReader(payload))
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
