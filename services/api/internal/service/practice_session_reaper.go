package service

import (
	"context"
	"log"
	"time"
)

const defaultPracticeSessionExpiryInterval = time.Minute

func StartPracticeSessionExpiryLoop(ctx context.Context, practiceService PracticeService, interval time.Duration, logger *log.Logger) {
	if practiceService == nil {
		return
	}
	if interval <= 0 {
		interval = defaultPracticeSessionExpiryInterval
	}

	runSweep := func() {
		if _, err := practiceService.ExpireStalePracticeSessions(ctx); err != nil && logger != nil {
			logger.Printf("practice session expiry sweep failed: %v", err)
		}
	}

	runSweep()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runSweep()
		}
	}
}
