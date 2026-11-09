package finance

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// CT-15 pacote 6: a reviewed receipt becomes a Finance entry only after explicit
// confirmation. A receipt never becomes a note issued by the user by itself.

type ReceiptLinkInput struct {
	SheetID                   string `json:"sheetId"`
	ExistingLineID            string `json:"existingLineId"`
	Kind                      string `json:"kind"`
	Scope                     string `json:"scope"`
	Description               string `json:"description"`
	Confirmed                 bool   `json:"confirmed"`
	ExpectedDocumentUpdatedAt string `json:"expectedDocumentUpdatedAt"`
	ExpectedSheetUpdatedAt    string `json:"expectedSheetUpdatedAt"`
}

type ReceiptLink struct {
	DocumentID     string `json:"documentId"`
	SheetID        string `json:"sheetId"`
	LineID         string `json:"lineId"`
	Kind           string `json:"kind"`
	Scope          string `json:"scope"`
	Amount         string `json:"amount"`
	CreatedLine    bool   `json:"createdLine"`
	Status         string `json:"status"` // linked | line_removed | amount_changed
	SheetUpdatedAt string `json:"sheetUpdatedAt,omitempty"`
}

var ErrReceiptAlreadyLinked = errors.New("receipt already linked")

func receiptAmount(d DocumentDraft) (string, error) {
	if d.Fields.Amount == nil || *d.Fields.Amount <= 0 {
		return "", invalidFiscal("Revise o valor do comprovante antes de lançar.")
	}
	return strconv.FormatFloat(*roundedAmount(d.Fields.Amount), 'f', 2, 64), nil
}

func receiptDescription(d DocumentDraft, in string) string {
	for _, v := range []string{in, d.Fields.PayeeName, d.SourceName, "Comprovante"} {
		if v = strings.TrimSpace(v); v != "" {
			return assistantText(v, 120)
		}
	}
	return "Comprovante"
}

func (s *DocumentService) LinkReceipt(ctx context.Context, account, user, id string, in ReceiptLinkInput) (ReceiptLink, error) {
	if !isUUID(id) || !isUUID(in.SheetID) || (in.ExistingLineID != "" && !isUUID(in.ExistingLineID)) {
		return ReceiptLink{}, ErrFiscalNotFound
	}
	if in.Kind != "entrada" && in.Kind != "saida" {
		return ReceiptLink{}, invalidFiscal("Informe se o lançamento é entrada ou saída.")
	}
	if in.Scope != "pessoal" && in.Scope != "empresarial" {
		return ReceiptLink{}, invalidFiscal("Informe se a despesa é pessoal ou empresarial.")
	}
	if !in.Confirmed {
		return ReceiptLink{}, invalidFiscal("Confirme o lançamento antes de gravar.")
	}
	d, err := s.store.Get(ctx, account, user, id)
	if err != nil {
		return ReceiptLink{}, err
	}
	if d.UpdatedAt != in.ExpectedDocumentUpdatedAt {
		return ReceiptLink{}, ErrFiscalConflict
	}
	amount, err := receiptAmount(d)
	if err != nil {
		return ReceiptLink{}, err
	}
	tx, err := s.store.pool.Begin(ctx)
	if err != nil {
		return ReceiptLink{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	var updated time.Time
	err = tx.QueryRow(ctx, `select status,updated_at from finance.sheets where account_id=$1::uuid and id=$2::uuid for update`, account, in.SheetID).Scan(&status, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReceiptLink{}, ErrSheetNotFound
	}
	if err != nil {
		return ReceiptLink{}, err
	}
	if status != "aberta" {
		return ReceiptLink{}, ErrSheetClosed
	}
	out := ReceiptLink{DocumentID: id, SheetID: in.SheetID, Kind: in.Kind, Scope: in.Scope, Amount: amount, Status: "linked"}
	var linkedSheet, linkedLine string
	err = tx.QueryRow(ctx, `select sheet_id::text,line_id::text,created_line from finance.receipt_links where account_id=$1::uuid and user_id=$2::uuid and document_id=$3::uuid`, account, user, id).Scan(&linkedSheet, &linkedLine, &out.CreatedLine)
	switch {
	case err == nil && linkedSheet == in.SheetID && (in.ExistingLineID == "" || in.ExistingLineID == linkedLine):
		// Replay of the same confirmation: no second entry.
		out.LineID = linkedLine
		out.SheetUpdatedAt = updated.UTC().Format(time.RFC3339Nano)
		return out, tx.Commit(ctx)
	case err == nil:
		return ReceiptLink{}, ErrReceiptAlreadyLinked
	case !errors.Is(err, pgx.ErrNoRows):
		return ReceiptLink{}, err
	}
	expected, e := time.Parse(time.RFC3339Nano, in.ExpectedSheetUpdatedAt)
	if e != nil || !expected.Equal(updated) {
		return ReceiptLink{}, ErrSheetConflict
	}
	if in.ExistingLineID != "" {
		var valid bool
		if err = tx.QueryRow(ctx, `select exists(select 1 from finance.lines l where l.sheet_id=$1::uuid and l.id=$2::uuid and l.kind=$3 and l.amount=$4::numeric)
 and not exists(select 1 from finance.receipt_links r where r.account_id=$5::uuid and r.sheet_id=$1::uuid and r.line_id=$2::uuid)`, in.SheetID, in.ExistingLineID, in.Kind, amount, account).Scan(&valid); err != nil {
			return ReceiptLink{}, err
		}
		if !valid {
			return ReceiptLink{}, ErrSheetConflict
		}
		out.LineID = in.ExistingLineID
	} else {
		out.CreatedLine = true
		if err = tx.QueryRow(ctx, `insert into finance.lines(sheet_id,kind,description,effective,effective_date,amount,position)
 select $1::uuid,$2,$3,$4,$5::date,$6::numeric,coalesce(max(position)+1,0) from finance.lines where sheet_id=$1::uuid and kind=$2 returning id::text`,
			in.SheetID, in.Kind, receiptDescription(d, in.Description), d.Fields.PaymentDate != "", dateArg(d.Fields.PaymentDate), amount).Scan(&out.LineID); err != nil {
			return ReceiptLink{}, err
		}
	}
	if _, err = tx.Exec(ctx, `insert into finance.receipt_links(account_id,user_id,document_id,sheet_id,line_id,kind,scope,amount,created_line)
 values($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,$6,$7,$8::numeric,$9)`, account, user, id, in.SheetID, out.LineID, in.Kind, in.Scope, amount, out.CreatedLine); err != nil {
		return ReceiptLink{}, err
	}
	if err = tx.QueryRow(ctx, `update finance.sheets set updated_at=clock_timestamp() where account_id=$1::uuid and id=$2::uuid returning updated_at`, account, in.SheetID).Scan(&updated); err != nil {
		return ReceiptLink{}, err
	}
	out.SheetUpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	return out, tx.Commit(ctx)
}

// GetReceiptLink revalidates the entry: an edited or removed line is reported, never hidden.
func (s *DocumentService) GetReceiptLink(ctx context.Context, account, user, id string) (ReceiptLink, error) {
	if !isUUID(id) {
		return ReceiptLink{}, ErrFiscalNotFound
	}
	out := ReceiptLink{DocumentID: id}
	var lineAmount *string
	err := s.store.pool.QueryRow(ctx, `select r.sheet_id::text,r.line_id::text,r.kind,r.scope,r.amount::text,r.created_line,l.amount::text
 from finance.receipt_links r join finance.sheets s on s.id=r.sheet_id and s.account_id=r.account_id
 left join finance.lines l on l.id=r.line_id and l.sheet_id=r.sheet_id and l.kind=r.kind
 where r.account_id=$1::uuid and r.user_id=$2::uuid and r.document_id=$3::uuid`, account, user, id).Scan(&out.SheetID, &out.LineID, &out.Kind, &out.Scope, &out.Amount, &out.CreatedLine, &lineAmount)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrFiscalNotFound
	}
	if err != nil {
		return out, err
	}
	switch {
	case lineAmount == nil:
		out.Status = "line_removed"
	case *lineAmount != out.Amount:
		out.Status = "amount_changed"
	default:
		out.Status = "linked"
	}
	return out, nil
}

// UnlinkReceipt removes only the link; the Finance entry stays for the user to edit.
func (s *DocumentService) UnlinkReceipt(ctx context.Context, account, user, id string) error {
	if !isUUID(id) {
		return ErrFiscalNotFound
	}
	tag, err := s.store.pool.Exec(ctx, `delete from finance.receipt_links where account_id=$1::uuid and user_id=$2::uuid and document_id=$3::uuid`, account, user, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrFiscalNotFound
	}
	return err
}
