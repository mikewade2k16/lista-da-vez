package finance

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// DefaultReceiptOriginalDays é o padrão escolhido pelo usuário (3 meses) enquanto a
// conta não grava uma política própria. A tela mostra que é o padrão.
const DefaultReceiptOriginalDays = 90

type RetentionSettings struct {
	ReceiptOriginalDays int     `json:"receiptOriginalDays"`
	Version             int     `json:"version"` // 0 = padrão, ainda não gravado
	IsDefault           bool    `json:"isDefault"`
	UpdatedAt           *string `json:"updatedAt,omitempty"`
	UpdatedBy           *string `json:"updatedBy,omitempty"`
	Stored              int     `json:"stored"`
	StoredBytes         int64   `json:"storedBytes"`
	Purged              int     `json:"purged"`
	NotStored           int     `json:"notStored"`
	NextExpiry          *string `json:"nextExpiry,omitempty"`
	StorageAvailable    bool    `json:"storageAvailable"`
}

type RetentionSettingsInput struct {
	ReceiptOriginalDays int `json:"receiptOriginalDays"`
	ExpectedVersion     int `json:"expectedVersion"`
}

type RetentionEvent struct {
	Version             int    `json:"version"`
	PreviousDays        *int   `json:"previousDays,omitempty"`
	ReceiptOriginalDays int    `json:"receiptOriginalDays"`
	ActorID             string `json:"actorId"`
	CreatedAt           string `json:"createdAt"`
}

var ErrRetentionConflict = errors.New("retention settings changed")

// GetRetention resume a política da conta e o estado dos originais do usuário.
func (s *DocumentService) GetRetention(ctx context.Context, account, user string) (RetentionSettings, error) {
	out := RetentionSettings{ReceiptOriginalDays: DefaultReceiptOriginalDays, IsDefault: true, StorageAvailable: s.originals != nil}
	err := s.store.pool.QueryRow(ctx, `select receipt_original_days,version,to_char(updated_at at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),updated_by::text
 from finance.retention_settings where account_id=$1::uuid`, account).Scan(&out.ReceiptOriginalDays, &out.Version, &out.UpdatedAt, &out.UpdatedBy)
	switch {
	case err == nil:
		out.IsDefault = false
	case !errors.Is(err, pgx.ErrNoRows):
		return out, err
	}
	err = s.store.pool.QueryRow(ctx, `select count(*) filter (where status='stored'),coalesce(sum(size_bytes) filter (where status='stored'),0),
 count(*) filter (where status in ('purged','removed')),count(*) filter (where status='not_stored'),
 to_char((min(stored_at) filter (where status='stored')+make_interval(days=>$3)) at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
 from finance.document_originals where account_id=$1::uuid and user_id=$2::uuid`, account, user, out.ReceiptOriginalDays).
		Scan(&out.Stored, &out.StoredBytes, &out.Purged, &out.NotStored, &out.NextExpiry)
	return out, err
}

// UpdateRetention grava a política com CAS por versão e histórico append-only.
// O novo prazo vale também para os originais já guardados (expiração = guardado em + dias).
func (s *DocumentService) UpdateRetention(ctx context.Context, account, actor string, in RetentionSettingsInput) (RetentionSettings, error) {
	if in.ReceiptOriginalDays < 1 || in.ReceiptOriginalDays > 3650 || in.ExpectedVersion < 0 {
		return RetentionSettings{}, ErrFiscalInvalid
	}
	tx, err := s.store.pool.Begin(ctx)
	if err != nil {
		return RetentionSettings{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended('finance:retention:'||$1,0))`, account); err != nil {
		return RetentionSettings{}, err
	}
	version, previous := 0, (*int)(nil)
	var days int
	err = tx.QueryRow(ctx, `select version,receipt_original_days from finance.retention_settings where account_id=$1::uuid`, account).Scan(&version, &days)
	if err == nil {
		previous = &days
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return RetentionSettings{}, err
	}
	if version != in.ExpectedVersion {
		return RetentionSettings{}, ErrRetentionConflict
	}
	if previous != nil && *previous == in.ReceiptOriginalDays {
		return s.GetRetention(ctx, account, actor)
	}
	next := version + 1
	if _, err = tx.Exec(ctx, `insert into finance.retention_settings(account_id,receipt_original_days,version,updated_by) values($1::uuid,$2,$3,$4::uuid)
 on conflict(account_id) do update set receipt_original_days=excluded.receipt_original_days,version=excluded.version,updated_by=excluded.updated_by,updated_at=clock_timestamp()`,
		account, in.ReceiptOriginalDays, next, actor); err != nil {
		return RetentionSettings{}, err
	}
	if _, err = tx.Exec(ctx, `insert into finance.retention_setting_events(account_id,version,previous_days,receipt_original_days,actor_id) values($1::uuid,$2,$3,$4,$5::uuid)`,
		account, next, previous, in.ReceiptOriginalDays, actor); err != nil {
		return RetentionSettings{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return RetentionSettings{}, err
	}
	return s.GetRetention(ctx, account, actor)
}

func (s *DocumentService) RetentionHistory(ctx context.Context, account string) ([]RetentionEvent, error) {
	rows, err := s.store.pool.Query(ctx, `select version,previous_days,receipt_original_days,actor_id::text,to_char(created_at at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
 from finance.retention_setting_events where account_id=$1::uuid order by version desc limit 50`, account)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RetentionEvent{}
	for rows.Next() {
		var e RetentionEvent
		if err = rows.Scan(&e.Version, &e.PreviousDays, &e.ReceiptOriginalDays, &e.ActorID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type originalClaim struct{ documentID, account, ref string }

// PurgeExpiredOriginals remove do storage os originais vencidos (todas as contas).
// Cada linha é reservada com um lease antes da chamada externa, então dois workers
// não removem o mesmo arquivo; falha volta com backoff. Retorna quantos expurgou.
func (s *DocumentService) PurgeExpiredOriginals(ctx context.Context, now time.Time, limit int) (int, error) {
	if s.originals == nil {
		return 0, nil
	}
	rows, err := s.store.pool.Query(ctx, `update finance.document_originals o set next_purge_attempt_at=$1::timestamptz+interval '10 minutes',updated_at=clock_timestamp()
 where o.document_id in (select x.document_id from finance.document_originals x left join finance.retention_settings rs on rs.account_id=x.account_id
  where x.status='stored' and x.stored_at+make_interval(days=>coalesce(rs.receipt_original_days,90))<=$1::timestamptz
  and (x.next_purge_attempt_at is null or x.next_purge_attempt_at<=$1::timestamptz) order by x.stored_at limit $2 for update of x skip locked)
 returning o.document_id::text,o.account_id::text,o.storage_ref`, now, limit)
	if err != nil {
		return 0, err
	}
	claims, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (originalClaim, error) {
		var c originalClaim
		return c, r.Scan(&c.documentID, &c.account, &c.ref)
	})
	if err != nil {
		return 0, err
	}
	purged := 0
	for _, c := range claims {
		if err = s.removeOriginal(ctx, c, "purged"); err == nil {
			purged++
		}
	}
	return purged, nil
}

func (s *DocumentService) removeOriginal(ctx context.Context, c originalClaim, status string) error {
	if err := s.originals.Delete(ctx, c.account, originalSource, c.ref); err != nil {
		_, _ = s.store.pool.Exec(ctx, `update finance.document_originals set purge_attempts=purge_attempts+1,
 next_purge_attempt_at=clock_timestamp()+least(interval '24 hours',interval '1 hour'*power(2,purge_attempts)),updated_at=clock_timestamp()
 where document_id=$1::uuid and account_id=$2::uuid and status='stored'`, c.documentID, c.account)
		return err
	}
	_, err := s.store.pool.Exec(ctx, `update finance.document_originals set status=$3,storage_ref=null,purged_at=clock_timestamp(),next_purge_attempt_at=null,updated_at=clock_timestamp()
 where document_id=$1::uuid and account_id=$2::uuid and status='stored'`, c.documentID, c.account, status)
	return err
}

// RemoveOriginal apaga agora o original de um comprovante do próprio usuário.
// Os dados extraídos e os vínculos continuam no banco.
func (s *DocumentService) RemoveOriginal(ctx context.Context, account, user, id string) (DocumentOriginal, error) {
	if !isUUID(id) || s.originals == nil {
		return DocumentOriginal{}, ErrFiscalNotFound
	}
	c := originalClaim{documentID: id, account: account}
	err := s.store.pool.QueryRow(ctx, `select storage_ref from finance.document_originals where document_id=$1::uuid and account_id=$2::uuid and user_id=$3::uuid and status='stored'`, id, account, user).Scan(&c.ref)
	if errors.Is(err, pgx.ErrNoRows) {
		return DocumentOriginal{}, ErrFiscalNotFound
	}
	if err != nil {
		return DocumentOriginal{}, err
	}
	if err = s.removeOriginal(ctx, c, "removed"); err != nil {
		return DocumentOriginal{}, ErrOriginalUnavailable
	}
	return s.GetOriginal(ctx, account, user, id)
}

// runRetention executa o expurgo a cada hora até o contexto ser cancelado.
func (s *DocumentService) runRetention(ctx context.Context, logger *slog.Logger) {
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			for {
				n, err := s.PurgeExpiredOriginals(ctx, time.Now().UTC(), 50)
				if err != nil && logger != nil && ctx.Err() == nil {
					logger.Warn("finance retention purge failed", "error", err.Error())
				}
				if n > 0 && logger != nil {
					logger.Info("finance retention purged originals", "count", strconv.Itoa(n))
				}
				if err != nil || n < 50 {
					break
				}
			}
			timer.Reset(time.Hour)
		}
	}
}
