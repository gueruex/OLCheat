package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// BuyPotions natively buys bulk potions. potionID typically maps to 1:Health, 2:Mana, 3:Energy, 67:Meadow, 100:Tona, etc.
func BuyPotions(client *OverlewdClient, potionID int, amount int) (string, error) {
	endpoint := fmt.Sprintf("/potions/%d/buy", potionID)
	payloadStr := fmt.Sprintf("count=%d", amount)

	req, _ := http.NewRequest("POST", client.BaseURL+endpoint, strings.NewReader(payloadStr))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

// UsePotions natively consumes bulk potions to instantly replenish global energy/stamina tracking stats.
func UsePotions(client *OverlewdClient, potionID int, amount int) (string, error) {
	endpoint := fmt.Sprintf("/potions/%d/use", potionID)
	payloadStr := fmt.Sprintf("count=%d", amount)

	req, _ := http.NewRequest("POST", client.BaseURL+endpoint, strings.NewReader(payloadStr))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

type GlobalPotion struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

type MyPotion struct {
	ID    int `json:"id"`
	Count int `json:"count"`
}

func FetchGlobalPotions(client *OverlewdClient) ([]GlobalPotion, error) {
	req, _ := http.NewRequest("GET", client.BaseURL+"/potions", nil)
	resp, err := client.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	b, _ := io.ReadAll(resp.Body)
	var arr []GlobalPotion
	if err := json.Unmarshal(b, &arr); err != nil {
		return nil, err
	}
	return arr, nil
}

func FetchMyPotions(client *OverlewdClient) ([]MyPotion, error) {
	req, _ := http.NewRequest("GET", client.BaseURL+"/me", nil)
	resp, err := client.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	b, _ := io.ReadAll(resp.Body)
	var obj map[string]interface{}
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, err
	}

	var pots []MyPotion
	if rawPots, ok := obj["potions"]; ok {
		b2, _ := json.Marshal(rawPots)
		json.Unmarshal(b2, &pots)
	}
	return pots, nil
}
