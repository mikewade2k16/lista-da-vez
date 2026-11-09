package finance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryOriginals struct {
	mu      sync.Mutex
	files   map[string][]byte
	keys    map[string]string
	seq     int
	deleted []string
	failPut bool
	failDel bool
}

func (m *memoryOriginals) Put(_ context.Context, account, _, source, key, _, _ string, content []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failPut {
		return "", errors.New("quota")
	}
	if ref, ok := m.keys[account+source+key]; ok {
		return ref, nil
	}
	m.seq++
	ref := fmt.Sprintf("disk:00000000-0000-4000-8000-%012d", m.seq)
	m.keys[account+source+key] = ref
	m.files[ref] = append([]byte(nil), content...)
	return ref, nil
}

func (m *memoryOriginals) Open(_ context.Context, _, _, ref string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[ref]
	if !ok {
		return nil, errors.New("missing")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *memoryOriginals) Delete(_ context.Context, _, _, ref string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failDel {
		return errors.New("provider down")
	}
	delete(m.files, ref)
	m.deleted = append(m.deleted, ref)
	return nil
}

// CT-15 (P15-05/P15-33): original guardado, retenção configurável com histórico,
// expurgo só do binário (dados extraídos ficam), falha com nova tentativa e isolamento.
func TestReceiptOriginalRetention(t *testing.T) {
	f := newReceiptFixture(t)
	ctx := f.ctx
	pool := f.documents.store.pool
	files := &memoryOriginals{files: map[string][]byte{}, keys: map[string]string{}}
	raw := []byte("Recibo Uber\nValor R$ 32,50\nData 05/11/2026\n")

	// Sem storage: extrai normalmente, original fica not_stored.
	d, err := f.documents.Extract(ctx, f.account, f.user, "uber.txt", raw)
	if err != nil {
		t.Fatal(err)
	}
	o, err := f.documents.GetOriginal(ctx, f.account, f.user, d.ID)
	if err != nil || o.Status != "not_stored" || o.NotStoredReason != "storage_unavailable" {
		t.Fatal("without storage", o, err)
	}
	// Com storage: reenviar o mesmo arquivo guarda o original uma única vez.
	f.documents.originals = files
	for range 2 {
		if _, err = f.documents.Extract(ctx, f.account, f.user, "uber.txt", raw); err != nil {
			t.Fatal(err)
		}
	}
	o, _ = f.documents.GetOriginal(ctx, f.account, f.user, d.ID)
	if o.Status != "stored" || o.ExpiresAt == nil || len(files.files) != 1 || o.SizeBytes != int64(len(raw)) {
		t.Fatal("stored original", o, len(files.files))
	}
	_, body, err := f.documents.OpenOriginal(ctx, f.account, f.user, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(body)
	if !bytes.Equal(got, raw) {
		t.Fatal("original bytes changed")
	}
	if _, _, err = f.documents.OpenOriginal(ctx, f.account, f.other, d.ID); !errors.Is(err, ErrFiscalNotFound) {
		t.Fatal("other user opened original", err)
	}

	// Política: padrão 90 dias, CAS por versão e histórico append-only.
	settings, err := f.documents.GetRetention(ctx, f.account, f.user)
	if err != nil || !settings.IsDefault || settings.ReceiptOriginalDays != DefaultReceiptOriginalDays || settings.Stored != 1 {
		t.Fatal("default", settings, err)
	}
	if _, err = f.documents.UpdateRetention(ctx, f.account, f.user, RetentionSettingsInput{ReceiptOriginalDays: 0}); !errors.Is(err, ErrFiscalInvalid) {
		t.Fatal("invalid days accepted", err)
	}
	if settings, err = f.documents.UpdateRetention(ctx, f.account, f.user, RetentionSettingsInput{ReceiptOriginalDays: 30, ExpectedVersion: 0}); err != nil || settings.Version != 1 || settings.IsDefault {
		t.Fatal("update", settings, err)
	}
	if _, err = f.documents.UpdateRetention(ctx, f.account, f.user, RetentionSettingsInput{ReceiptOriginalDays: 45, ExpectedVersion: 0}); !errors.Is(err, ErrRetentionConflict) {
		t.Fatal("stale version accepted", err)
	}
	if _, err = pool.Exec(ctx, `update finance.retention_setting_events set receipt_original_days=1 where account_id=$1::uuid`, f.account); err == nil {
		t.Fatal("retention history is mutable")
	}

	// Ainda dentro do prazo: nada é expurgado.
	if n, _ := f.documents.PurgeExpiredOriginals(ctx, time.Now().UTC(), 50); n != 0 {
		t.Fatal("purged before expiry", n)
	}
	if _, err = pool.Exec(ctx, `update finance.document_originals set stored_at=now()-interval '31 days' where document_id=$1::uuid`, d.ID); err != nil {
		t.Fatal(err)
	}
	// Falha do provider: continua guardado, tentativa registrada, sem repetir antes do backoff.
	files.failDel = true
	if n, _ := f.documents.PurgeExpiredOriginals(ctx, time.Now().UTC(), 50); n != 0 {
		t.Fatal("purge reported despite provider failure")
	}
	var attempts int
	_ = pool.QueryRow(ctx, `select purge_attempts from finance.document_originals where document_id=$1::uuid`, d.ID).Scan(&attempts)
	if o, _ = f.documents.GetOriginal(ctx, f.account, f.user, d.ID); o.Status != "stored" || attempts != 1 {
		t.Fatal("failed purge state", o.Status, attempts)
	}
	// Aumentar o prazo vale para os já guardados.
	if _, err = f.documents.UpdateRetention(ctx, f.account, f.user, RetentionSettingsInput{ReceiptOriginalDays: 60, ExpectedVersion: 1}); err != nil {
		t.Fatal(err)
	}
	files.failDel = false
	if n, _ := f.documents.PurgeExpiredOriginals(ctx, time.Now().Add(3*time.Hour).UTC(), 50); n != 0 {
		t.Fatal("longer retention ignored", n)
	}
	if n, _ := f.documents.PurgeExpiredOriginals(ctx, time.Now().Add(30*24*time.Hour).UTC(), 50); n != 1 || len(files.deleted) != 1 {
		t.Fatal("expired original not purged", n, files.deleted)
	}
	if _, _, err = f.documents.OpenOriginal(ctx, f.account, f.user, d.ID); !errors.Is(err, ErrOriginalPurged) {
		t.Fatal("purged original still served", err)
	}
	// Os dados extraídos continuam no banco.
	kept, err := f.documents.Get(ctx, f.account, f.user, d.ID)
	if err != nil || kept.Fields.Amount == nil || *kept.Fields.Amount != 32.5 {
		t.Fatal("extracted data lost", kept, err)
	}
	history, _ := f.documents.RetentionHistory(ctx, f.account)
	if len(history) != 2 || history[0].ReceiptOriginalDays != 60 || *history[0].PreviousDays != 30 {
		t.Fatal("history", history)
	}

	// Lista de comprovantes traz vínculo e original numa consulta, isolada por usuário.
	second, _ := f.documents.Extract(ctx, f.account, f.user, "outro.txt", []byte("Recibo mercado\nValor R$ 10,00\nData 06/11/2026\n"))
	removed, err := f.documents.RemoveOriginal(ctx, f.account, f.user, second.ID)
	if err != nil || removed.Status != "removed" {
		t.Fatal("manual removal", removed, err)
	}
	page, err := f.documents.ListReceipts(ctx, f.account, f.user, 1, 30)
	if err != nil || len(page.Items) != 2 || page.Items[0].Original == nil {
		t.Fatal("list", page, err)
	}
	if other, _ := f.documents.ListReceipts(ctx, f.account, f.other, 1, 30); len(other.Items) != 0 {
		t.Fatal("other user sees receipts", other.Items)
	}
}

// Manual edit of a saved customer: CAS, immutable version, document unchanged, isolation.
func TestFiscalCustomerManualEdit(t *testing.T) {
	f := newReceiptFixture(t)
	store := f.documents.customers
	amount := 500.0
	saved, _, err := store.Remember(f.ctx, f.account, f.user, DocumentFields{CustomerName: "João Teste", CustomerDocument: "52998224725", Amount: &amount}, "")
	if err != nil {
		t.Fatal(err)
	}
	newAmount := 650.0
	in := FiscalCustomerInput{Name: "João Teste ME", ServiceDescription: "Consultoria", Amount: &newAmount, ExpectedVersion: saved.Version}
	if _, err = store.Update(f.ctx, f.account, f.other, saved.ID, in); err == nil {
		t.Fatal("other user edited customer")
	}
	edited, err := store.Update(f.ctx, f.account, f.user, saved.ID, in)
	if err != nil || edited.Version != 2 || edited.Document != "52998224725" || *edited.Amount != 650 {
		t.Fatal("edit", edited, err)
	}
	if _, err = store.Update(f.ctx, f.account, f.user, saved.ID, in); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("stale version accepted", err)
	}
	in.Name = " "
	in.ExpectedVersion = 2
	if _, err = store.Update(f.ctx, f.account, f.user, saved.ID, in); !errors.Is(err, ErrFiscalInvalid) {
		t.Fatal("empty name accepted", err)
	}
	versions, err := store.Versions(f.ctx, f.account, f.user, saved.ID)
	if err != nil || len(versions) != 2 || !strings.Contains(string(versions[0].Data), "editedManually") {
		t.Fatal("versions", versions, err)
	}
}
