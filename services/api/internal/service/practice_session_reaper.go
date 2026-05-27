package service

import (
	"context"
	"log"
	"time"
)

const defaultPracticeSessionExpiryInterval = time.Minute
const defaultWorkspaceCleanupSweepLimit = 20

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

func StartWorkspaceCleanupLoop(ctx context.Context, practiceService PracticeService, interval time.Duration, logger *log.Logger) {
	if practiceService == nil {
		return
	}
	if interval <= 0 {
		interval = defaultPracticeSessionExpiryInterval
	}

	runSweep := func() {
		if err := practiceService.RunWorkspaceCleanupDueJobs(ctx, defaultWorkspaceCleanupSweepLimit); err != nil && logger != nil {
			logger.Printf("workspace cleanup sweep failed: %v", err)
		}
		summary, err := practiceService.ReconcileWorkspaceCleanupJobs(ctx, defaultWorkspaceCleanupSweepLimit)
		if err != nil {
			if logger != nil {
				logger.Printf("workspace cleanup reconciliation failed: %v", err)
			}
			return
		}
		if logger != nil && summary.BackfilledJobs > 0 {
			logger.Printf("workspace cleanup reconciliation backfilled %d jobs", summary.BackfilledJobs)
		}
		if logger != nil && summary.ExhaustedFailedJobs > 0 {
			logger.Printf("workspace cleanup reconciliation found %d exhausted failed jobs", summary.ExhaustedFailedJobs)
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
