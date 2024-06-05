
package util

import (
	"log"
	"os"
	"time"
)

const defaultSyncInterval = 10 * time.Minute // Default sync interval

// GetSyncInterval retrieves the sync interval from the environment variable or returns the default value.
func GetSyncInterval() time.Duration {
	syncIntervalStr := os.Getenv("SYNC_INTERVAL")
	if syncIntervalStr == "" {
		log.Printf("SYNC_INTERVAL not set, using default value: %v", defaultSyncInterval)
		return defaultSyncInterval
	}

	syncInterval, err := time.ParseDuration(syncIntervalStr)
	if err != nil {
		log.Printf("Invalid SYNC_INTERVAL format, using default value: %v. Error: %v", defaultSyncInterval, err)
		return defaultSyncInterval
	}

	log.Printf("Using SYNC_INTERVAL from environment: %v", syncInterval)
	return syncInterval
}
