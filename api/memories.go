package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type MemoryPiece struct {
	IsPurchased bool   `json:"isPurchased"`
	Rarity      string `json:"rarity"`
	ShardID     int    `json:"shardId"`
	ShardName   string `json:"shardName"`
}

type Memory struct {
	ID              int           `json:"id"`
	MatriarchID     int           `json:"matriarchId"`
	Title           string        `json:"title"`
	MemoryType      string        `json:"memoryType"`
	Status          string        `json:"status"`
	BlockReason     *string       `json:"blockReason"`
	Condition       *string       `json:"condition"`
	EventID         *int          `json:"eventId"`
	MemoryBackArt   *string       `json:"memoryBackArt"`
	SexSceneID      int           `json:"sexSceneId"`
	SexScenePreview string        `json:"sexScenePreview"`
	UserID          int           `json:"userId"`
	Pieces          []MemoryPiece `json:"pieces"`
}

func FetchMemories(client *OverlewdClient) ([]Memory, error) {
	log.Println("[INFO] Fetching Memories (Scenes)...")
	req, _ := http.NewRequest("GET", client.BaseURL+"/matriarchs/memories", nil)

	resp, err := client.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed fetching memories: %d", resp.StatusCode)
	}

	b, _ := io.ReadAll(resp.Body)
	var memories []Memory
	if err := json.Unmarshal(b, &memories); err != nil {
		return nil, err
	}

	return memories, nil
}

func BuyMemoryMany(ctx context.Context, client *OverlewdClient, memoryID int, shardsToBuy []string, onProgress func(string)) {
	log.Printf("[INFO] Initiating %d shard purchases for Memory %d sequentially", len(shardsToBuy), memoryID)

	var successful int
	var failed int

	reportProgress := func(msg string) {
		if onProgress != nil {
			onProgress(msg)
		}
	}

	Loop:
	for _, shardName := range shardsToBuy {
		select {
		case <-ctx.Done():
			reportProgress("[CANCELLED] Halting remaining batches due to context stop.")
			break Loop
		default:
		}

		endpoint := fmt.Sprintf("/matriarchs/memories/%d/piece-of-glass/%s/buy", memoryID, shardName)
		req, _ := http.NewRequest("POST", client.BaseURL+endpoint, nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Content-Length", "0")
		req = req.WithContext(ctx)

		resp, err := client.DoRequest(req)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			failed++
			reportProgress(fmt.Sprintf("[Shard %s] Failed (Network Error)", shardName))
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			successful++
			reportProgress(fmt.Sprintf("[Shard %s] Successfully unlocked!", shardName))
		} else {
			failed++
			reportProgress(fmt.Sprintf("[Shard %s] Failed (State %d)", shardName, resp.StatusCode))
		}
		resp.Body.Close()
	}

	log.Println("[INFO] Completed all requested Memory shard unlocks sequentially!")
	reportProgress(fmt.Sprintf("[BATCH FINISHED] %d Successful | %d Failed", successful, failed))
}
