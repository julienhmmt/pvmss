package utils

import (
	"os"
	"sync"
	"time"
)

var (
	appLocation     *time.Location
	appLocationOnce sync.Once
)

// GetAppLocation returns the application-wide timezone location.
// It uses the TZ environment variable when set and valid, otherwise
// falls back to the server local time.
func GetAppLocation() *time.Location {
	appLocationOnce.Do(func() {
		loc := time.Local
		if tz := os.Getenv("TZ"); tz != "" {
			if l, err := time.LoadLocation(tz); err == nil {
				loc = l
			}
		}
		appLocation = loc
	})
	return appLocation
}
