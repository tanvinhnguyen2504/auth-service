package auth

import (
	"context"
	"log"
	"time"
)

const cleanupInterval = 24 * time.Hour

// StartSessionCleanup deletes sessions past either TTL, at startup and daily.
// Housekeeping only: GetSession already enforces both TTLs in SQL. Returns
// immediately; the loop ends when ctx is cancelled.
func StartSessionCleanup(ctx context.Context, repo Repository, sessionTTL, idleTTL time.Duration) {
	sweep := func() {
		now := time.Now()
		if deleted, err := repo.DeleteExpiredSessions(ctx, now.Add(-sessionTTL), now.Add(-idleTTL)); err != nil {
			log.Printf("auth: session cleanup: %v", err)
		} else if deleted > 0 {
			log.Printf("auth: cleanup removed %d expired sessions", deleted)
		}
	}

	go func() {
		sweep()

		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()
}
