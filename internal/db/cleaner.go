package db

import (
	"context"
	"database/sql"
	db "keypub/internal/db/.gen"
	"log"
	"time"
)

type VerificationCleaner struct {
	sqlDb    *sql.DB
	duration time.Duration
	ticker   *time.Ticker
	done     chan struct{}
}

func NewVerificationCleaner(sqlDb *sql.DB, duration time.Duration) *VerificationCleaner {
	vc := &VerificationCleaner{
		sqlDb:    sqlDb,
		duration: duration,
		done:     make(chan struct{}),
	}

	// Initial cleanup
	vc.cleanup()

	// Start periodic cleanup
	vc.ticker = time.NewTicker(duration)
	go vc.run()

	return vc
}

func (vc *VerificationCleaner) run() {
	for {
		select {
		case <-vc.ticker.C:
			vc.cleanup()
		case <-vc.done:
			return
		}
	}
}

func (vc *VerificationCleaner) cleanup() {
	// Delete verification codes older than the specified duration
	err := db.New(vc.sqlDb).DeleteVerificationCodesOlderThanDuration(
		context.TODO(),
		int64(time.Now().Unix()-int64(vc.duration.Seconds())),
	)

	if err != nil {
		// You might want to use your preferred logging solution here
		log.Printf("Error cleaning up verification codes: %v", err)
	}
}

func (vc *VerificationCleaner) Close() {
	vc.ticker.Stop()
	close(vc.done)
}
