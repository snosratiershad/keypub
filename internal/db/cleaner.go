package db

import (
	"database/sql"
	"keypub/internal/db/.gen/table"
	"log"
	"time"

	. "github.com/go-jet/jet/v2/sqlite"

	_ "github.com/mattn/go-sqlite3"
)

type VerificationCleaner struct {
	db       *sql.DB
	duration time.Duration
	ticker   *time.Ticker
	done     chan struct{}
}

func NewVerificationCleaner(db *sql.DB, duration time.Duration) *VerificationCleaner {
	vc := &VerificationCleaner{
		db:       db,
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
	stmt := table.VerificationCodes.DELETE().
		WHERE(table.VerificationCodes.CreatedAt.LT(Int64(time.Now().Unix() - int64(vc.duration.Seconds()))))

	_, err := stmt.Exec(vc.db)
	if err != nil {
		// You might want to use your preferred logging solution here
		log.Printf("Error cleaning up verification codes: %v", err)
	}
}

func (vc *VerificationCleaner) Close() {
	vc.ticker.Stop()
	close(vc.done)
}
