package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type retentionRow struct{ values []any }

func (r retentionRow) Scan(dest ...any) error {
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *string:
			*target = value.(string)
		case *int64:
			*target = value.(int64)
		}
	}
	return nil
}

type retentionTx struct {
	queries   []string
	committed bool
	role      string
}

func (t *retentionTx) Exec(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
	t.queries = append(t.queries, query)
	if query != "set local role staffy_voice_maintenance" {
		return pgconn.CommandTag{}, errors.New("unexpected exec")
	}
	t.role = "staffy_voice_maintenance"
	return pgconn.NewCommandTag("SET"), nil
}

func (t *retentionTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	t.queries = append(t.queries, query)
	if query == "select current_user" {
		return retentionRow{values: []any{t.role}}
	}
	return retentionRow{values: []any{int64(2), int64(3)}}
}

func (t *retentionTx) Commit(context.Context) error { t.committed = true; return nil }

func TestRunPurgesImmediatelyBeforeFirstInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	done := make(chan struct{})
	go func() {
		run(ctx, func(context.Context) error {
			calls.Add(1)
			cancel()
			return nil
		}, time.Hour)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("initial purge waited for interval")
	}
	if calls.Load() != 1 {
		t.Fatalf("purge calls = %d", calls.Load())
	}
}

func TestExecutePurgeActivatesRoleCallsFunctionAndCommits(t *testing.T) {
	tx := &retentionTx{}
	if err := executePurge(context.Background(), tx); err != nil {
		t.Fatal(err)
	}
	if !tx.committed {
		t.Fatal("purge transaction was not committed")
	}
	joined := strings.Join(tx.queries, "\n")
	for _, want := range []string{"set local role staffy_voice_maintenance", "select current_user", "select * from employee.purge_expired_voice_previews()"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing query %q in %q", want, joined)
		}
	}
}
