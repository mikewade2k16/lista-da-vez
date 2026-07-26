package omnichannel

import (
	"context"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel/evolution"
)

// TestIngest_QRWebhookCachesQR prova o fix do BUG 3: um webhook QRCODE_UPDATED da Evolution
// (QR aninhado em data.qrcode.base64, o shape real da v2) e traduzido, e o QR normalizado cai
// no MESMO qrCache que o endpoint /qrcode le — escopado por (accountID, instanceName). Antes,
// o ingestOne so processava message_received e o QR do webhook se perdia. Sem banco: a rota de
// QR retorna antes de tocar o store.
func TestIngest_QRWebhookCachesQR(t *testing.T) {
	qr := newQRCache()
	reg := channel.NewRegistry(evolution.New("", ""))
	svc := NewInboundService(nil, reg, nil, nil, qr, nil, nil)

	body := []byte(`{"event":"qrcode.updated","instance":"omni-main",` +
		`"data":{"qrcode":{"base64":"data:image/png;base64,QRWEBHOOK"}}}`)

	status, err := svc.Ingest(context.Background(), "acc-1", "evolution", nil, body)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if status != inboundAccepted {
		t.Fatalf("status = %q, queria accepted", status)
	}
	if got := qr.get("acc-1", "omni-main"); got != "data:image/png;base64,QRWEBHOOK" {
		t.Errorf("qrCache[acc-1|omni-main] = %q (o QR do webhook deve alimentar o cache do /qrcode)", got)
	}
	// Isolamento: outra conta nao ve o QR.
	if got := qr.get("acc-2", "omni-main"); got != "" {
		t.Errorf("qrCache vazou para outra conta: %q", got)
	}
}

// TestIngest_ConnectionUpdateClearsQR: ao conectar (CONNECTION_UPDATE state=open) o QR pendente
// e limpo — o painel para de mostrar um QR que ja nao vale.
func TestIngest_ConnectionUpdateClearsQR(t *testing.T) {
	qr := newQRCache()
	qr.set("acc-1", "omni-main", "data:image/png;base64,STALE")
	reg := channel.NewRegistry(evolution.New("", ""))
	svc := NewInboundService(nil, reg, nil, nil, qr, nil, nil)

	body := []byte(`{"event":"connection.update","instance":"omni-main",` +
		`"data":{"state":"open","ownerJid":"5511999998888@s.whatsapp.net"}}`)

	if _, err := svc.Ingest(context.Background(), "acc-1", "evolution", nil, body); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if got := qr.get("acc-1", "omni-main"); got != "" {
		t.Errorf("QR deveria ter sido limpo ao conectar, veio %q", got)
	}
}
