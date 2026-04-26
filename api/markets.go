package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
)

type FlexIntArray []int

func (fia *FlexIntArray) UnmarshalJSON(b []byte) error {
	var raw []interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	var res []int
	for _, v := range raw {
		switch val := v.(type) {
		case float64:
			res = append(res, int(val))
		case string:
			if num, err := strconv.Atoi(val); err == nil {
				res = append(res, num)
			}
		}
	}
	*fia = res
	return nil
}

type MarketTab struct {
	TabID    int          `json:"tabId"`
	MarketID int          `json:"marketId"`
	Title    string       `json:"title"`
	Goods    FlexIntArray `json:"goods"`
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

func BuyMarketItem(client *OverlewdClient, marketID int, tradableID int) (string, error) {
	log.Printf("[INFO] Attempting purchase: Market %d -> Tradable %d", marketID, tradableID)
	endpoint := fmt.Sprintf("/markets/%d/tradable/%d/buy", marketID, tradableID)
	req, _ := http.NewRequest("POST", client.BaseURL+endpoint, nil)
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", "0")

	resp, err := client.DoRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return string(b), fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return string(b), nil
}

func BuyMarketMany(ctx context.Context, client *OverlewdClient, marketID int, tradableID int, amount int, onProgress func(string)) {
	log.Printf("[INFO] Initiating %d spins on Market %d -> Tradable %d", amount, marketID, tradableID)

	var wg sync.WaitGroup
	concurrencyLimit := make(chan struct{}, 6)
	var mu sync.Mutex
	var successful int
	var failed int

	reportProgress := func(msg string) {
		if onProgress != nil {
			onProgress(msg)
		}
	}

	Loop:
	for i := 1; i <= amount; i++ {
		select {
		case <-ctx.Done():
			reportProgress("[CANCELLED] Halting remaining batches due to context stop.")
			break Loop
		default:
		}

		concurrencyLimit <- struct{}{}
		wg.Add(1)

		go func(spinIndex int) {
			defer wg.Done()
			defer func() { <-concurrencyLimit }()

			endpoint := fmt.Sprintf("/markets/%d/tradable/%d/buy", marketID, tradableID)
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
				failed++
				mu.Unlock()
				reportProgress(fmt.Sprintf("[Spin %d] Failed (Network Error)", spinIndex))
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
				mu.Lock()
				successful++
				mu.Unlock()
				reportProgress(fmt.Sprintf("[Spin %d] Complete", spinIndex))
			} else {
				mu.Lock()
				failed++
				mu.Unlock()
				reportProgress(fmt.Sprintf("[Spin %d] Failed (State %d)", spinIndex, resp.StatusCode))
			}
		}(i)
	}

	wg.Wait()
	log.Println("[INFO] Completed all requested Market batch spins!")
	reportProgress(fmt.Sprintf("[BATCH FINISHED] %d Successful | %d Failed", successful, failed))
}
