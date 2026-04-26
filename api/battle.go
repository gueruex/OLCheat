package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// WorkerLoop loops the `/end` battle command until cancelled
func WorkerLoop(ctx context.Context, client *OverlewdClient, endpoint string, stageID int, slotID int) {
	url := fmt.Sprintf("%s/%s/%d/end", client.BaseURL, endpoint, stageID)
	payloadStr := "result=win&mana=0&hp=0"

	log.Printf("[Slot %d] Started grinding loop for %s Stage %d", slotID, endpoint, stageID)

	// Offset initial loop to spread out immediate API slamming
	select {
	case <-time.After(time.Duration(rand.Intn(1000)) * time.Millisecond):
	case <-ctx.Done():
		return
	}

	for {
		// Emulate Battle Time Duration
		select {
		case <-time.After(time.Duration(2500+rand.Intn(1500)) * time.Millisecond):
		case <-ctx.Done():
			log.Printf("[Slot %d] Terminated loop gracefully.", slotID)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(payloadStr))
		if err != nil {
			log.Printf("[Slot %d] Request build error: %v", slotID, err)
			continue
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.DoRequest(req)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled right during network req
			}
			log.Printf("[Slot %d] Server/Network error: %v", slotID, err)
			continue
		}

		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			var respMap map[string]interface{}
			if err := json.Unmarshal(b, &respMap); err == nil {
				var earned []string
				lootMap := ExtractGachaLootRecursively(respMap)
				for key, amt := range lootMap {
					parts := strings.Split(key, ":")
					if len(parts) == 2 {
						id, _ := strconv.Atoi(parts[1])
						var name string
						switch parts[0] {
						case "character":
							name = GetCharacterName(id)
						case "tradable":
							name = GetTradableName(id)
						case "equipment":
							name = fmt.Sprintf("Equipment Item %d", id)
						default:
							name = GetCurrencyName(id)
						}
						earned = append(earned, fmt.Sprintf("[white]%dx [green]%s[white]", amt, name))
					}
				}
				if len(earned) > 0 {
					log.Printf("[Slot %d] Victory! Loot: %s", slotID, strings.Join(earned, "[white], "))
				} else {
					log.Printf("[Slot %d] Victory! Loot: nothing", slotID)
				}
			} else {
				log.Printf("[Slot %d] Victory! Loot: %s", slotID, string(b))
			}
		} else {
			log.Printf("[Slot %d] [red]Invalid Status %d:[white] %s", slotID, resp.StatusCode, string(b))
		}
	}
}
