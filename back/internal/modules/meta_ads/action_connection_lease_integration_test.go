package metaads

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoreActionConnectionLeaseSerializesRotationAndDeletion(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL nao definido; pulando integracao do advisory lease")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var guardsAvailable bool
	if err := pool.QueryRow(ctx, `select exists (
		select 1 from information_schema.columns
		where table_schema = 'meta_ads' and table_name = 'connections'
		  and column_name = 'revision'
	)`).Scan(&guardsAvailable); err != nil {
		t.Fatal(err)
	}
	if !guardsAvailable {
		t.Skip("banco de teste ainda nao recebeu as migrations Meta 0290/0291")
	}

	suffix := time.Now().UTC().UnixNano()
	var accountID string
	if err := pool.QueryRow(ctx, `insert into core.accounts (slug, name, is_active)
		values ($1, $2, true) returning id::text`,
		fmt.Sprintf("meta-action-lease-%d", suffix), "Meta action lease test",
	).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from core.accounts where id = $1::uuid`, accountID)
	})

	store := NewStore(pool, "meta-action-lease-test-key")
	expiresAt := time.Now().UTC().Add(time.Hour)
	initial, err := store.SaveConnectionSnapshot(
		ctx, accountID, "business-1", "Connection", "token-1", &expiresAt, nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	rotated := assertActionConnectionMutationWaits(t, store, accountID, initial, func() error {
		_, saveErr := store.SaveConnectionSnapshot(
			ctx, accountID, "business-1", "Connection", "token-2", &expiresAt, nil,
		)
		return saveErr
	})
	if !rotated {
		t.Fatal("rotation did not wait for the active token lease")
	}
	callbackCalled := false
	err = store.WithDecryptedTokenAtRevision(
		ctx, accountID, initial.ID, initial.Revision,
		func(string) error {
			callbackCalled = true
			return nil
		},
	)
	if !errors.Is(err, ErrConnectionChanged) || callbackCalled {
		t.Fatalf("old revision err = %v, callbackCalled = %v", err, callbackCalled)
	}

	expiredAt := time.Now().UTC().Add(-time.Minute)
	expired, err := store.SaveConnectionSnapshot(
		ctx, accountID, "business-1", "Connection", "expired-token", &expiredAt, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetConnection(ctx, accountID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetConnection(expired) error = %v", err)
	}
	callbackCalled = false
	err = store.WithDecryptedTokenAtRevision(
		ctx, accountID, expired.ID, expired.Revision,
		func(string) error {
			callbackCalled = true
			return nil
		},
	)
	if !errors.Is(err, ErrConnectionChanged) || callbackCalled {
		t.Fatalf("expired token err = %v, callbackCalled = %v", err, callbackCalled)
	}

	current, err := store.SaveConnectionSnapshot(
		ctx, accountID, "business-1", "Connection", "token-3", &expiresAt, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	deleted := assertActionConnectionMutationWaits(t, store, accountID, current, func() error {
		return store.DeleteConnection(ctx, accountID)
	})
	if !deleted {
		t.Fatal("delete did not wait for the active token lease")
	}
}

func assertActionConnectionMutationWaits(
	t *testing.T,
	store *Store,
	accountID string,
	connection Connection,
	mutate func() error,
) bool {
	t.Helper()
	entered := make(chan struct{})
	release := make(chan struct{})
	leaseDone := make(chan error, 1)
	go func() {
		leaseDone <- store.WithDecryptedTokenAtRevision(
			context.Background(), accountID, connection.ID, connection.Revision,
			func(token string) error {
				if token == "" {
					return errors.New("empty decrypted token")
				}
				close(entered)
				<-release
				return nil
			},
		)
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("token lease did not start")
	}

	mutationDone := make(chan error, 1)
	go func() { mutationDone <- mutate() }()
	select {
	case err := <-mutationDone:
		close(release)
		if err != nil {
			t.Fatalf("connection mutation failed early: %v", err)
		}
		return false
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-leaseDone:
		if err != nil {
			t.Fatalf("lease failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("lease did not finish")
	}
	select {
	case err := <-mutationDone:
		if err != nil {
			t.Fatalf("connection mutation failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("connection mutation did not resume")
	}
	return true
}
