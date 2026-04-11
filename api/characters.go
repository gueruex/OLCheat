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
	ID     int    `json:"id"`
	Rarity string `json:"rarity"`
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
	var toDissolve []string
	for _, c := range chars {
		if targets[c.Rarity] {
			toDissolve = append(toDissolve, fmt.Sprintf("characterIds%%5b%%5d=%d", c.ID))
		}
	}

	if len(toDissolve) == 0 {
		log.Println("[DISSOLVE] Aborted. No characters matched the targeted rarities.")
		return
	}

	log.Printf("[DISSOLVE] Dissolve Sequence Initiated for %d characters...", len(toDissolve))
	payload := strings.Join(toDissolve, "&")

	req, _ := http.NewRequest("POST", client.BaseURL+"/battles/my/characters/dissolve", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.DoRequest(req)
	if err != nil {
		log.Printf("[DISSOLVE] HTTP Error: %v", err)
		return
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		log.Printf("[DISSOLVE] Dissolve Successful! You gained massive resources. HTTP 200: %s", string(b))
	} else {
		log.Printf("[DISSOLVE] Failed %d: %s", resp.StatusCode, string(b))
	}
}
