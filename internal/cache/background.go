package cache

import (
	"time"

	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/jira"
	"github.com/qawatake/tkt/internal/ticket"
	"github.com/qawatake/tkt/internal/verbose"
)

// StartBackgroundUpdate starts a background goroutine to update the cache
// This is the same logic as fetch command but runs in background without UI feedback
func StartBackgroundUpdate() {
	go func() {
		err := performBackgroundUpdate()
		if err != nil {
			verbose.Printf("Background cache update failed: %v\n", err)
		} else {
			verbose.Printf("Background cache update completed successfully\n")
		}
	}()
}

// performBackgroundUpdate performs the cache update logic from fetch command
func performBackgroundUpdate() error {
	// 1. Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		verbose.Printf("Background cache update: Failed to load config: %v\n", err)
		return err
	}

	verbose.Printf("Background cache update: Starting...\n")

	// 2. Create JIRA client
	jiraClient, err := jira.NewClient(cfg)
	if err != nil {
		verbose.Printf("Background cache update: Failed to create JIRA client: %v\n", err)
		return err
	}

	// 3. Determine if this should be incremental or full fetch
	var tickets []*ticket.Ticket
	startTime := time.Now()

	lastFetch, fetchErr := config.GetLastFetchTime()
	if fetchErr != nil {
		verbose.Printf("Background cache update: Failed to get last fetch time: %v\n", fetchErr)
		verbose.Printf("Background cache update: Performing full fetch\n")
		tickets, err = jiraClient.FetchIssues()
	} else if lastFetch.IsZero() {
		verbose.Printf("Background cache update: First fetch, performing full fetch\n")
		tickets, err = jiraClient.FetchIssues()
	} else {
		verbose.Printf("Background cache update: Last fetch time: %s\n", lastFetch.Format(time.RFC3339))
		verbose.Printf("Background cache update: Performing incremental fetch\n")
		tickets, err = jiraClient.FetchIssuesIncremental(lastFetch)
	}

	if err != nil {
		verbose.Printf("Background cache update: Failed to fetch tickets: %v\n", err)
		return err
	}

	verbose.Printf("Background cache update: Fetched %d tickets\n", len(tickets))

	// 4. Ensure cache directory exists
	cacheDir, err := config.EnsureCacheDir()
	if err != nil {
		verbose.Printf("Background cache update: Failed to ensure cache directory: %v\n", err)
		return err
	}

	// 5. Save tickets to cache
	savedCount := 0
	for _, ticket := range tickets {
		savedCachePath, err := ticket.SaveToFile(cacheDir)
		if err != nil {
			verbose.Printf("Background cache update: Failed to save ticket %s: %v\n", ticket.Key, err)
		} else {
			verbose.Printf("Background cache update: Saved %s -> %s\n", ticket.Key, savedCachePath)
			savedCount++
		}
	}

	// 6. Save last fetch time
	if saveErr := config.SaveLastFetchTime(startTime); saveErr != nil {
		verbose.Printf("Background cache update: Failed to save last fetch time: %v\n", saveErr)
	} else {
		verbose.Printf("Background cache update: Saved last fetch time: %s\n", startTime.Format(time.RFC3339))
	}

	verbose.Printf("Background cache update: Completed successfully, saved %d tickets\n", savedCount)
	return nil
}
