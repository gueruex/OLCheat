package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type MarketTab struct {
	TabID    int    `json:"tabId"`
	MarketID int    `json:"marketId"`
	Title    string `json:"title"`
	Goods    []int  `json:"goods"`
}

type Market struct {
	ID   int         `json:"id"`
	Name string      `json:"name"`
	Tabs []MarketTab `json:"tabs"`
}

func FetchMarkets(client *OverlewdClient) ([]Market, error) {
	log.Println("[INFO] Fetching Global Markets...")
	req, _ := http.NewRequest("GET", client.BaseURL+"/markets", nil)

	resp, err := client.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed fetching markets: %d", resp.StatusCode)
	}

	b, _ := io.ReadAll(resp.Body)
	var roster []Market
	if err := json.Unmarshal(b, &roster); err != nil {
		return nil, err
	}

	return roster, nil
}
