package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) AccountExists(ctx context.Context, accountID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists (
			select 1 from core.accounts where id = $1::uuid and is_active = true
		)
	`, accountID).Scan(&exists)
	return exists, err
}

func (repository *PostgresRepository) IsAccountMember(ctx context.Context, accountID, userID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists (
			select 1
			from core.account_users
			where account_id = $1::uuid and user_id = $2::uuid and is_active = true
		)
	`, accountID, userID).Scan(&exists)
	return exists, err
}

func (repository *PostgresRepository) ListPermissionsForUser(ctx context.Context, accountID, userID string) ([]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select rp.permission_key
		from core.user_role_assignments ura
		join core.role_permissions rp on rp.role_id = ura.role_id
		join core.permissions p on p.key = rp.permission_key and p.deprecated_at is null
		where ura.account_id = $1::uuid and ura.user_id = $2::uuid

		union

		select permission_key
		from core.user_permission_overrides
		where account_id = $1::uuid and user_id = $2::uuid
		  and effect = 'allow' and is_active = true

		except

		select permission_key
		from core.user_permission_overrides
		where account_id = $1::uuid and user_id = $2::uuid
		  and effect = 'deny' and is_active = true

		order by 1 asc
	`, accountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		permissions = append(permissions, key)
	}
	return permissions, rows.Err()
}

func (repository *PostgresRepository) InsertNotification(ctx context.Context, input CreateNotificationInput) (Notification, error) {
	payloadJSON, err := json.Marshal(normalizeMap(input.Payload))
	if err != nil {
		return Notification{}, err
	}

	notification, err := scanNotification(repository.pool.QueryRow(ctx, `
		insert into notifications.user_notifications (
			account_id, user_id, source_module, source_event, title, body, link_path, payload
		) values (
			$1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::jsonb
		)
		returning id::text, account_id::text, user_id::text, source_module, source_event,
		          title, body, link_path, payload, read_at, archived_at, created_at
	`, input.AccountID, input.UserID, input.SourceModule, input.SourceEvent, input.Title, input.Body, input.LinkPath, payloadJSON).Scan)
	return notification, err
}

func (repository *PostgresRepository) InsertDeliveryLog(ctx context.Context, entry DeliveryLog) error {
	_, err := repository.pool.Exec(ctx, `
		insert into notifications.delivery_log (notification_id, channel, status, error)
		values ($1::uuid, $2, $3, $4)
	`, entry.NotificationID, entry.Channel, entry.Status, entry.Error)
	return err
}

func (repository *PostgresRepository) ListNotifications(ctx context.Context, accountID, userID string, cursor *listCursor, limit int) ([]Notification, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := repository.pool.Query(ctx, `
		select id::text, account_id::text, user_id::text, source_module, source_event,
		       title, body, link_path, payload, read_at, archived_at, created_at
		from notifications.user_notifications
		where account_id = $1::uuid
		  and user_id = $2::uuid
		  and archived_at is null
		  and (
			$3::timestamptz is null
			or created_at < $3::timestamptz
			or (created_at = $3::timestamptz and id::text < $4)
		  )
		order by created_at desc, id desc
		limit $5
	`, accountID, userID, nullableTime(cursor), nullableCursorID(cursor), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Notification, 0)
	for rows.Next() {
		notification, err := scanNotification(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, notification)
	}
	return items, rows.Err()
}

func (repository *PostgresRepository) MarkRead(ctx context.Context, accountID, userID, notificationID string) (Notification, error) {
	notification, err := scanNotification(repository.pool.QueryRow(ctx, `
		update notifications.user_notifications
		   set read_at = coalesce(read_at, now())
		 where account_id = $1::uuid and user_id = $2::uuid and id = $3::uuid
		returning id::text, account_id::text, user_id::text, source_module, source_event,
		          title, body, link_path, payload, read_at, archived_at, created_at
	`, accountID, userID, notificationID).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Notification{}, ErrNotificationNotFound
	}
	return notification, err
}

func (repository *PostgresRepository) MarkAllRead(ctx context.Context, accountID, userID string) (int64, error) {
	result, err := repository.pool.Exec(ctx, `
		update notifications.user_notifications
		   set read_at = now()
		 where account_id = $1::uuid and user_id = $2::uuid and read_at is null and archived_at is null
	`, accountID, userID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (repository *PostgresRepository) ListPreferences(ctx context.Context, accountID, userID string) ([]NotificationPreference, error) {
	rows, err := repository.pool.Query(ctx, `
		select account_id::text, user_id::text, channel, source_module, source_event, enabled, updated_at
		from notifications.notification_channels
		where account_id = $1::uuid and user_id = $2::uuid
		order by channel asc, source_module asc, source_event asc
	`, accountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]NotificationPreference, 0)
	for rows.Next() {
		item, err := scanPreference(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *PostgresRepository) SavePreferences(ctx context.Context, accountID, userID string, preferences []NotificationPreference) ([]NotificationPreference, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	for _, preference := range preferences {
		if _, err := tx.Exec(ctx, `
			insert into notifications.notification_channels (
				account_id, user_id, channel, source_module, source_event, enabled, updated_at
			) values (
				$1::uuid, $2::uuid, $3, $4, $5, $6, now()
			)
			on conflict (account_id, user_id, channel, source_module, source_event)
			do update set enabled = excluded.enabled, updated_at = now()
		`, accountID, userID, preference.Channel, preference.SourceModule, preference.SourceEvent, preference.Enabled); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return repository.ListPreferences(ctx, accountID, userID)
}

func (repository *PostgresRepository) IsChannelEnabled(ctx context.Context, accountID, userID, channel, sourceModule, sourceEvent string) (bool, error) {
	var enabled bool
	err := repository.pool.QueryRow(ctx, `
		select enabled
		from notifications.notification_channels
		where account_id = $1::uuid
		  and user_id = $2::uuid
		  and channel = $3
		  and (
			(source_module = $4 and source_event = $5)
			or (source_module = $4 and source_event = '')
			or (source_module = '' and source_event = '')
		  )
		order by
			case
				when source_module = $4 and source_event = $5 then 3
				when source_module = $4 and source_event = '' then 2
				when source_module = '' and source_event = '' then 1
				else 0
			end desc,
			updated_at desc
		limit 1
	`, accountID, userID, channel, sourceModule, sourceEvent).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return channel == ChannelInApp, nil
	}
	return enabled, err
}

func (repository *PostgresRepository) FindActiveMute(ctx context.Context, accountID, userID, resourceType, resourceID string, now time.Time) (*Mute, error) {
	mute, err := scanMute(repository.pool.QueryRow(ctx, `
		select account_id::text, user_id::text, resource_type, resource_id, until_at, created_at
		from notifications.mutes
		where account_id = $1::uuid and user_id = $2::uuid and resource_type = $3 and resource_id = $4
		  and until_at > $5
	`, accountID, userID, resourceType, resourceID, now).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &mute, nil
}

func (repository *PostgresRepository) UpsertMute(ctx context.Context, accountID, userID string, input MuteInput, until time.Time) (Mute, error) {
	mute, err := scanMute(repository.pool.QueryRow(ctx, `
		insert into notifications.mutes (
			account_id, user_id, resource_type, resource_id, until_at
		) values (
			$1::uuid, $2::uuid, $3, $4, $5
		)
		on conflict (account_id, user_id, resource_type, resource_id)
		do update set until_at = excluded.until_at
		returning account_id::text, user_id::text, resource_type, resource_id, until_at, created_at
	`, accountID, userID, input.ResourceType, input.ResourceID, until).Scan)
	return mute, err
}

func scanNotification(scan func(dest ...any) error) (Notification, error) {
	var notification Notification
	var payloadJSON []byte
	err := scan(
		&notification.ID,
		&notification.AccountID,
		&notification.UserID,
		&notification.SourceModule,
		&notification.SourceEvent,
		&notification.Title,
		&notification.Body,
		&notification.LinkPath,
		&payloadJSON,
		&notification.ReadAt,
		&notification.ArchivedAt,
		&notification.CreatedAt,
	)
	if err != nil {
		return Notification{}, err
	}
	if len(payloadJSON) > 0 {
		if err := json.Unmarshal(payloadJSON, &notification.Payload); err != nil {
			return Notification{}, err
		}
	}
	if notification.Payload == nil {
		notification.Payload = map[string]any{}
	}
	return notification, nil
}

func scanPreference(scan func(dest ...any) error) (NotificationPreference, error) {
	var preference NotificationPreference
	err := scan(
		&preference.AccountID,
		&preference.UserID,
		&preference.Channel,
		&preference.SourceModule,
		&preference.SourceEvent,
		&preference.Enabled,
		&preference.UpdatedAt,
	)
	return preference, err
}

func scanMute(scan func(dest ...any) error) (Mute, error) {
	var mute Mute
	err := scan(
		&mute.AccountID,
		&mute.UserID,
		&mute.ResourceType,
		&mute.ResourceID,
		&mute.Until,
		&mute.CreatedAt,
	)
	return mute, err
}

func normalizeMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	normalized := make(map[string]any, len(input))
	for key, value := range input {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		normalized[trimmed] = value
	}
	return normalized
}

func nullableTime(cursor *listCursor) any {
	if cursor == nil {
		return nil
	}
	return cursor.CreatedAt
}

func nullableCursorID(cursor *listCursor) any {
	if cursor == nil {
		return nil
	}
	return cursor.ID
}
