package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CampaignTask defines the strict structural payload stripped of cyclic model scopes natively
type CampaignTask struct {
	Endpoint string
	ID       int
	Status   string
	Name     string
}

// CampaignWorker executes a sequential Queue of stages using explicit concurrent concurrency up to maximum Workers!
func CampaignWorker(
	ctx context.Context,
	cancelFunc context.CancelFunc,
	client *OverlewdClient,
	queue []CampaignTask,
) {
	// A channel to dish out stage bounds natively tracking our 3 concurrent matrices!
	workChan := make(chan CampaignTask, len(queue))
	for _, stg := range queue {
		workChan <- stg
	}
	close(workChan)

	var wg sync.WaitGroup
	workers := 3

	log.Printf("[yellow]Campaign Exec: Initiating 3-Thread Sequence Engine...")

	for i := 1; i <= workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for stg := range workChan {
				select {
				case <-ctx.Done():
					log.Printf("[Slot %d] [gray]Context aborted, halting...", workerID)
					return
				default:
				}

				endpoint := stg.Endpoint
				if endpoint == "" {
					endpoint = "ftue-stages" // Fallback safety
				}

				// If stage is ALREADY marked exclusively completed mathematically earlier, we skip /start entirely!
				isCompleted := stg.Status == "complete" || stg.Status == "completed"

				if !isCompleted {
					// POST /start
					startURL := fmt.Sprintf("%s/%s/%d/start", client.BaseURL, endpoint, stg.ID)
					reqStart, err := http.NewRequestWithContext(ctx, "POST", startURL, strings.NewReader("teamId=1")) // Most gachas fall back safely to 1 natively
					if err != nil {
						continue
					}
					reqStart.Header.Set("Content-Type", "application/x-www-form-urlencoded")

					log.Printf("[Slot %d] [cyan]Starting [white]%s", workerID, stg.Name)
					respStart, err := client.DoRequest(reqStart)
					if err != nil {
						if ctx.Err() == nil {
							log.Printf("[Slot %d] [red]Network Error dropping queue: %v", workerID, err)
							cancelFunc()
						}
						return
					}

					// Verify Energy/Validation checks mapping successfully on bounds
					if respStart.StatusCode < 200 || respStart.StatusCode > 299 {
						b, _ := io.ReadAll(respStart.Body)
						respStart.Body.Close()
						log.Printf("[Slot %d] [red]Execution Failed! (Energy Cap ?) -> Status %d: %s", workerID, respStart.StatusCode, string(b))
						cancelFunc() // Globally kills EVERYTHING mathematically instantly tracking limits!
						return
					}
					respStart.Body.Close()

					// Fake a small Jitter delay to let server process states cleanly
					time.Sleep(1000 * time.Millisecond)
				}

				// POST /end
				endURL := fmt.Sprintf("%s/%s/%d/end", client.BaseURL, endpoint, stg.ID)
				reqEnd, _ := http.NewRequestWithContext(ctx, "POST", endURL, strings.NewReader("result=win&mana=0&hp=0"))
				reqEnd.Header.Set("Content-Type", "application/x-www-form-urlencoded")

				if isCompleted {
					log.Printf("[Slot %d] [cyan]Re-Farming Completed Node [white]%s", workerID, stg.Name)
				} else {
					log.Printf("[Slot %d] [yellow]Finishing [white]%s", workerID, stg.Name)
				}
				
				respEnd, err := client.DoRequest(reqEnd)
				if err != nil {
					if ctx.Err() == nil {
						log.Printf("[Slot %d] [red]End Network Error: %v", workerID, err)
						cancelFunc()
					}
					return
				}

				b, _ := io.ReadAll(respEnd.Body)
				respEnd.Body.Close()

				if respEnd.StatusCode >= 200 && respEnd.StatusCode <= 299 {
					log.Printf("[Slot %d] [green]Cleared! [white]%s [gray](Payload: %s...)", workerID, stg.Name, getShortenedResp(string(b)))
				} else {
					log.Printf("[Slot %d] [red]Failed to End [white]%s - Status %d: %s", workerID, stg.Name, respEnd.StatusCode, string(b))
					cancelFunc()
					return // Abandon gracefully
				}
				
				// Jitter offset explicitly spreading requests organically out
				time.Sleep(750 * time.Millisecond)
			}
		}(i)
	}

	// Wait for completion gracefully
	go func() {
		wg.Wait()
		log.Printf("[green]Campaign Execution Engine fully resolved all bounds successfully!")
	}()
}

func getShortenedResp(s string) string {
	if len(s) > 50 {
		return s[:50]
	}
	return s
}
