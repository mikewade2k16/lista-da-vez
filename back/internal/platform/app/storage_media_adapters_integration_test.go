package app

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
	objectstorage "github.com/mikewade2k16/lista-da-vez/back/internal/modules/storage"
)

// TestR2MediaAdaptersLive e opt-in porque consome operacoes e bytes reais do
// budget. Ele prova a costura Calendar/Tasks -> storage -> R2, a integridade do
// original e o Range usado pelo player de video.
func TestR2PendingUploadReconciliationLive(t *testing.T) {
	if os.Getenv("STORAGE_RECONCILE_LIVE") != "1" {
		t.Skip("set STORAGE_RECONCILE_LIVE=1 to reconcile pending uploads against R2")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()
	cfg := objectstorage.Config{
		Enabled: true, AccountID: os.Getenv("R2_ACCOUNT_ID"), Bucket: os.Getenv("R2_BUCKET"),
		AccessKeyID: os.Getenv("R2_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		RequestTimeout: 30 * time.Second, UploadTimeout: 15 * time.Minute,
	}
	client, err := objectstorage.NewR2Client(cfg)
	if err != nil {
		t.Fatalf("create R2 client: %v", err)
	}
	service := objectstorage.NewService(cfg, objectstorage.NewPostgresRepository(pool), client)
	if _, err := service.CheckConnection(ctx); err != nil {
		t.Fatalf("check connection and reconcile pending uploads: %v", err)
	}
}

func TestR2MediaAdaptersLive(t *testing.T) {
	if os.Getenv("STORAGE_LIVE_SMOKE") != "1" {
		t.Skip("set STORAGE_LIVE_SMOKE=1 to use the configured R2 provider")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()
	var accountID, userID string
	if err := pool.QueryRow(ctx, `
		select a.id::text, u.id::text
		from core.accounts a
		cross join core.users u
		where a.is_active = true and u.is_active = true
		order by a.is_agency desc, a.created_at, u.created_at
		limit 1
	`).Scan(&accountID, &userID); err != nil {
		t.Fatalf("resolve live scope: %v", err)
	}

	cfg := objectstorage.Config{
		Enabled: true, AccountID: os.Getenv("R2_ACCOUNT_ID"), Bucket: os.Getenv("R2_BUCKET"),
		AccessKeyID: os.Getenv("R2_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		RequestTimeout: 30 * time.Second,
		UploadTimeout:  5 * time.Minute,
		AnalyticsToken: os.Getenv("R2_ANALYTICS_API_TOKEN"),
	}
	client, err := objectstorage.NewR2Client(cfg)
	if err != nil {
		t.Fatalf("create R2 client: %v", err)
	}
	var previousUploadsEnabled bool
	if err := pool.QueryRow(ctx, `select uploads_enabled from storage.settings where id = 1`).Scan(&previousUploadsEnabled); err != nil {
		t.Fatalf("read upload mode: %v", err)
	}
	if _, err := pool.Exec(ctx, `update storage.settings set uploads_enabled = true where id = 1`); err != nil {
		t.Fatalf("enable R2 uploads for live smoke: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `update storage.settings set uploads_enabled = $1 where id = 1`, previousUploadsEnabled)
	}()
	service := objectstorage.NewService(
		cfg,
		objectstorage.NewPostgresRepository(pool),
		client,
		objectstorage.NewCloudflareUsageClient(cfg),
	)
	provider := func() *objectstorage.Service { return service }

	calendarAdapter := newHybridCalendarMediaStorage(provider, nil)
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d}
	item, err := calendarAdapter.Save(ctx, accountID, userID, "live-calendar-"+time.Now().Format("20060102150405.000000000"),
		"original.png", "image/png", png, calendar.MediaLimits{})
	if err != nil {
		t.Fatalf("calendar upload: %v", err)
	}
	if !strings.HasPrefix(item.URL, "/uploads/calendar/"+accountID+"/") {
		t.Fatalf("unexpected calendar URL: %s", item.URL)
	}
	calendarContent, err := calendarAdapter.Open(ctx, accountID, item.ID, "")
	if err != nil {
		t.Fatalf("calendar download: %v", err)
	}
	downloadedPNG, err := io.ReadAll(calendarContent.Body)
	_ = calendarContent.Body.Close()
	if err != nil || string(downloadedPNG) != string(png) {
		t.Fatalf("calendar original changed: read=%v", err)
	}

	taskAdapter := newHybridTaskVideoStorage(provider, nil)
	mp4 := []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 2, 0, 'i', 's', 'o', 'm'}
	video, err := taskAdapter.Save(ctx, accountID, userID, "smoke-task",
		"live-task-"+time.Now().Format("20060102150405.000000000"), "original.mp4", "video/mp4", mp4)
	if err != nil {
		t.Fatalf("task upload: %v", err)
	}
	if !strings.HasPrefix(video.Path, "/uploads/tasks/"+accountID+"/") {
		t.Fatalf("unexpected task URL: %s", video.Path)
	}
	partial, err := taskAdapter.Open(ctx, accountID, video.ID, "bytes=0-3")
	if err != nil {
		t.Fatalf("task range download: %v", err)
	}
	downloadedRange, err := io.ReadAll(partial.Body)
	_ = partial.Body.Close()
	if err != nil || string(downloadedRange) != string(mp4[:4]) || partial.ContentRange == "" {
		t.Fatalf("task range/original mismatch: range=%q read=%v", partial.ContentRange, err)
	}
}
