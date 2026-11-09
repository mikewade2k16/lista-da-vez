package finance

import (
	"context"
	"encoding/json"
	"time"
)

// ReceiptItem é a linha da tela de comprovantes: dados extraídos, vínculo com a
// planilha (revalidado) e situação do arquivo original.
type ReceiptItem struct {
	Document   DocumentDraft     `json:"document"`
	Link       *ReceiptLink      `json:"link,omitempty"`
	SheetTitle string            `json:"sheetTitle,omitempty"`
	Original   *DocumentOriginal `json:"original,omitempty"`
}

type ReceiptPage struct {
	Items []ReceiptItem `json:"items"`
	Page  int           `json:"page"`
	More  bool          `json:"more"`
}

// ListReceipts pagina os documentos do próprio usuário (conta + usuário da sessão).
// Uma única consulta traz documento, vínculo, linha atual e original, sem N+1.
func (s *DocumentService) ListReceipts(ctx context.Context, account, user string, page, limit int) (ReceiptPage, error) {
	if page < 1 || page > 10000 || limit < 1 || limit > 100 {
		return ReceiptPage{}, ErrFiscalInvalid
	}
	rows, err := s.store.pool.Query(ctx, `select d.id::text,d.source_name,d.data,d.updated_at,
 r.sheet_id::text,r.line_id::text,r.kind,r.scope,r.amount::text,r.created_line,l.amount::text,s.title,
 o.document_id::text,o.status,coalesce(o.not_stored_reason,''),o.file_name,o.content_type,o.size_bytes,
 to_char(o.stored_at at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(o.purged_at at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
 case when o.status='stored' then to_char((o.stored_at+make_interval(days=>coalesce(rs.receipt_original_days,90))) at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') end
 from finance.document_drafts d
 left join finance.receipt_links r on r.account_id=d.account_id and r.user_id=d.user_id and r.document_id=d.id
 left join finance.sheets s on s.id=r.sheet_id and s.account_id=d.account_id
 left join finance.lines l on l.id=r.line_id and l.sheet_id=r.sheet_id and l.kind=r.kind
 left join finance.document_originals o on o.document_id=d.id and o.account_id=d.account_id and o.user_id=d.user_id
 left join finance.retention_settings rs on rs.account_id=d.account_id
 where d.account_id=$1::uuid and d.user_id=$2::uuid
 order by d.updated_at desc,d.id desc limit $3 offset $4`, account, user, limit+1, (page-1)*limit)
	if err != nil {
		return ReceiptPage{}, err
	}
	defer rows.Close()
	out := ReceiptPage{Items: []ReceiptItem{}, Page: page}
	for rows.Next() {
		var (
			d                                             DocumentDraft
			raw                                           []byte
			updated                                       time.Time
			sheetID, lineID, kind, scope, amount, lineAmt *string
			createdLine                                   *bool
			sheetTitle                                    *string
			o                                             DocumentOriginal
			oID, oStatus, oReason, oName, oType           *string
			oSize                                         *int64
		)
		targets := []any{&d.ID, &d.SourceName, &raw, &updated, &sheetID, &lineID, &kind, &scope, &amount, &createdLine, &lineAmt, &sheetTitle,
			&oID, &oStatus, &oReason, &oName, &oType, &oSize, &o.StoredAt, &o.PurgedAt, &o.ExpiresAt}
		if err = rows.Scan(targets...); err != nil {
			return ReceiptPage{}, err
		}
		if err = json.Unmarshal(raw, &d.Fields); err != nil {
			return ReceiptPage{}, err
		}
		d.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
		d = documentView(d)
		item := ReceiptItem{Document: d}
		if sheetID != nil && sheetTitle != nil {
			link := ReceiptLink{DocumentID: d.ID, SheetID: *sheetID, LineID: *lineID, Kind: *kind, Scope: *scope, Amount: *amount, CreatedLine: *createdLine, Status: "linked"}
			switch {
			case lineAmt == nil:
				link.Status = "line_removed"
			case *lineAmt != link.Amount:
				link.Status = "amount_changed"
			}
			item.Link, item.SheetTitle = &link, *sheetTitle
		}
		if oID != nil {
			o.DocumentID, o.Status, o.NotStoredReason, o.FileName, o.ContentType, o.SizeBytes = *oID, *oStatus, *oReason, *oName, *oType, *oSize
			item.Original = &o
		}
		out.Items = append(out.Items, item)
	}
	if len(out.Items) > limit {
		out.Items, out.More = out.Items[:limit], true
	}
	return out, rows.Err()
}
