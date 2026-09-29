// staffy-voice-retention is a separate process. Provision a dedicated LOGIN role
// with membership in staffy_voice_maintenance; never use the application URL.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const interval = time.Minute

func main() {
	url := os.Getenv("STAFFY_VOICE_MAINTENANCE_DATABASE_URL")
	if url == "" || url == os.Getenv("STAFFY_DATABASE_URL") {
		log.Fatal("a separate STAFFY_VOICE_MAINTENANCE_DATABASE_URL is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	run(ctx, func(ctx context.Context) error { return purge(ctx, pool) }, interval)
}

func run(ctx context.Context, purgeOnce func(context.Context) error, every time.Duration) {
	for {
		if err := purgeOnce(ctx); err != nil {
			log.Printf("voice retention failed: %v", err)
		}
		timer := time.NewTimer(every)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func purge(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	return executePurge(ctx, tx)
}

type purgeTx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Commit(context.Context) error
}

func executePurge(ctx context.Context, tx purgeTx) error {
	if _, err := tx.Exec(ctx, "set local role staffy_voice_maintenance"); err != nil {
		return err
	}
	var role string
	if err := tx.QueryRow(ctx, "select current_user").Scan(&role); err != nil {
		return err
	}
	if role != "staffy_voice_maintenance" {
		return errors.New("maintenance role not active")
	}
	var assessments, previews int64
	if err := tx.QueryRow(ctx, "select * from employee.purge_expired_voice_previews()").Scan(&assessments, &previews); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	log.Printf("voice retention removed %d assessments and %d previews", assessments, previews)
	return nil
}
