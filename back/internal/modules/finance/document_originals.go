package finance

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// OriginalFiles é a porta do storage privado para o arquivo original do comprovante.
// A composition root liga R2 (ou disco privado local fora de /uploads). Sem porta,
// o comprovante continua sendo lido e extraído, mas o original fica "not_stored".
type OriginalFiles interface {
	Put(ctx context.Context, account, actor, source, key, name, mime string, content []byte) (string, error)
	Open(ctx context.Context, account, source, ref string) (io.ReadCloser, error)
	Delete(ctx context.Context, account, source, ref string) error
}

const originalSource = "finance"

var (
	ErrOriginalPurged      = errors.New("receipt original purged by retention policy")
	ErrOriginalUnavailable = errors.New("receipt original unavailable")
)

// DocumentOriginal descreve o arquivo original de um comprovante. Os dados extraídos
// continuam no banco mesmo depois do expurgo; só o binário sai do storage.
type DocumentOriginal struct {
	DocumentID      string  `json:"documentId"`
	Status          string  `json:"status"` // stored | not_stored | purged | removed
	NotStoredReason string  `json:"notStoredReason,omitempty"`
	FileName        string  `json:"fileName"`
	ContentType     string  `json:"contentType"`
	SizeBytes       int64   `json:"sizeBytes"`
	StoredAt        *string `json:"storedAt,omitempty"`
	PurgedAt        *string `json:"purgedAt,omitempty"`
	ExpiresAt       *string `json:"expiresAt,omitempty"`
}

func originalContentType(raw []byte) string {
	if len(raw) >= 5 && string(raw[:5]) == "%PDF-" {
		return "application/pdf"
	}
	sniff := raw
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	switch t := http.DetectContentType(sniff); t {
	case "image/jpeg", "image/png", "image/webp":
		return t
	}
	return "text/plain"
}

// storeOriginal guarda o arquivo recebido, byte a byte, uma vez por comprovante.
// Falha do storage não impede a extração: registra not_stored com o motivo, e um
// novo envio do mesmo arquivo tenta guardar de novo.
func (s *DocumentService) storeOriginal(ctx context.Context, account, user string, d DocumentDraft, name, hash string, raw []byte) {
	var status string
	err := s.store.pool.QueryRow(ctx, `select status from finance.document_originals where document_id=$1::uuid and account_id=$2::uuid and user_id=$3::uuid`, d.ID, account, user).Scan(&status)
	if err == nil && status != "not_stored" {
		return
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return
	}
	mime := originalContentType(raw)
	ref, reason := "", ""
	if s.originals == nil {
		reason = "storage_unavailable"
	} else if ref, err = s.originals.Put(ctx, account, user, originalSource, "receipt:"+d.ID+":"+hash, name, mime, raw); err != nil {
		ref, reason = "", "storage_unavailable"
	}
	newStatus, storedAt := "stored", any(time.Now().UTC())
	if ref == "" {
		newStatus, storedAt = "not_stored", nil
	}
	_, _ = s.store.pool.Exec(ctx, `insert into finance.document_originals(document_id,account_id,user_id,storage_ref,file_name,content_type,size_bytes,sha256,status,not_stored_reason,stored_at)
 values($1::uuid,$2::uuid,$3::uuid,nullif($4,''),$5,$6,$7,$8,$9,nullif($10,''),$11)
 on conflict(document_id) do update set storage_ref=excluded.storage_ref,status=excluded.status,not_stored_reason=excluded.not_stored_reason,stored_at=excluded.stored_at,updated_at=clock_timestamp()
 where finance.document_originals.status='not_stored'`, d.ID, account, user, ref, assistantText(name, 200), mime, len(raw), hash, newStatus, reason, storedAt)
}

const originalColumns = `o.document_id::text,o.status,coalesce(o.not_stored_reason,''),o.file_name,o.content_type,o.size_bytes,
 to_char(o.stored_at at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(o.purged_at at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
 case when o.status='stored' then to_char((o.stored_at+make_interval(days=>coalesce(rs.receipt_original_days,90))) at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') end`

func scanOriginal(row pgx.Row) (DocumentOriginal, error) {
	var o DocumentOriginal
	err := row.Scan(&o.DocumentID, &o.Status, &o.NotStoredReason, &o.FileName, &o.ContentType, &o.SizeBytes, &o.StoredAt, &o.PurgedAt, &o.ExpiresAt)
	return o, err
}

func (s *DocumentService) GetOriginal(ctx context.Context, account, user, id string) (DocumentOriginal, error) {
	if !isUUID(id) {
		return DocumentOriginal{}, ErrFiscalNotFound
	}
	o, err := scanOriginal(s.store.pool.QueryRow(ctx, `select `+originalColumns+` from finance.document_originals o
 left join finance.retention_settings rs on rs.account_id=o.account_id
 where o.account_id=$1::uuid and o.user_id=$2::uuid and o.document_id=$3::uuid`, account, user, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrFiscalNotFound
	}
	return o, err
}

// OpenOriginal devolve o binário somente ao dono do comprovante e enquanto retido.
func (s *DocumentService) OpenOriginal(ctx context.Context, account, user, id string) (DocumentOriginal, io.ReadCloser, error) {
	o, err := s.GetOriginal(ctx, account, user, id)
	if err != nil {
		return o, nil, err
	}
	switch o.Status {
	case "purged", "removed":
		return o, nil, ErrOriginalPurged
	case "not_stored":
		return o, nil, ErrOriginalUnavailable
	}
	var ref string
	if err = s.store.pool.QueryRow(ctx, `select storage_ref from finance.document_originals where document_id=$1::uuid and account_id=$2::uuid and user_id=$3::uuid and status='stored'`, id, account, user).Scan(&ref); err != nil {
		return o, nil, ErrOriginalUnavailable
	}
	if s.originals == nil {
		return o, nil, ErrOriginalUnavailable
	}
	body, err := s.originals.Open(ctx, account, originalSource, ref)
	if err != nil {
		return o, nil, ErrOriginalUnavailable
	}
	return o, body, nil
}
