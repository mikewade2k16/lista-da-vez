// Package mock e o adapter de canal SEM REDE (canonico D-A: o mock permite testar F5-F9
// sem numero real). Implementa channel.Provider + channel.SessionManager de forma
// DETERMINISTICA: QR falso, "conecta" na hora, ecoa o envio, e parseia um payload de
// webhook simples que o proprio harness de teste envia.
//
// NADA aqui fala com sistema externo (o modulo omnichannel e independente por construcao —
// canonico §4). O mock e stateless: o estado de sessao/conversa vive no banco do modulo.
package mock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

// providerID e a chave do adapter (= whatsapp_instances.provider = 'mock', no CHECK da
// migration 0200).
const providerID = "mock"

// fakeQR e um data URL determinístico para o painel exibir "um QR". Nao e um QR real —
// e so o suficiente para a tela renderizar a imagem no fluxo do mock.
const fakeQR = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M8AAAMBAQDJ/pLvAAAAAElFTkSuQmCC"

// Provider e o adapter mock.
type Provider struct{}

// New cria o adapter mock.
func New() *Provider { return &Provider{} }

// ID implementa channel.Provider.
func (p *Provider) ID() string { return providerID }

// VerifyWebhook: o mock nao autentica (nao ha segredo compartilhado com um provedor real).
// E aceitavel porque o mock so roda em ambiente de teste; a rota continua protegida por
// rate-limit, content-type e content-length no handler. Documentado em docs/LEGADO.md.
func (p *Provider) VerifyWebhook(_ http.Header, _ []byte, _ channel.Credentials) error {
	return nil
}

// mockWebhookPayload e o shape que o harness de teste posta para simular uma mensagem
// recebida. Todos os campos sao opcionais e parseados defensivamente.
type mockWebhookPayload struct {
	Event     string `json:"event"`
	Instance  string `json:"instance"`
	MessageID string `json:"messageId"`
	Phone     string `json:"phone"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	MediaURL  string `json:"mediaUrl"`
	Timestamp int64  `json:"timestamp"` // unix seconds do "provider"; 0 => now
}

// ParseWebhook traduz o payload do mock em eventos canonicos. Parseia defensivamente:
// JSON invalido nao vaza no erro (canonico §10) e vira lista vazia + erro generico.
func (p *Provider) ParseWebhook(_ context.Context, _ http.Header, body []byte) ([]channel.Event, error) {
	var in mockWebhookPayload
	if err := json.Unmarshal(body, &in); err != nil {
		// Erro SEM body: nunca ecoar o payload cru.
		return nil, errors.New("mock: payload de webhook nao e JSON valido")
	}

	instance := strings.TrimSpace(in.Instance)
	event := strings.ToLower(strings.TrimSpace(in.Event))
	if event != "message" && event != "message_received" && event != "" {
		// Qualquer outro evento e ignorado (202 ignored no handler).
		return []channel.Event{{Kind: channel.EventIgnored, InstanceName: instance}}, nil
	}

	messageID := strings.TrimSpace(in.MessageID)
	if messageID == "" || instance == "" {
		// Sem id ou sem instancia nao ha o que deduplicar/gravar: ignora.
		return []channel.Event{{Kind: channel.EventIgnored, InstanceName: instance}}, nil
	}

	occurredAt := time.Now().UTC()
	if in.Timestamp > 0 {
		occurredAt = time.Unix(in.Timestamp, 0).UTC()
	}

	msgType := strings.ToUpper(strings.TrimSpace(in.Type))
	if msgType == "" {
		msgType = "TEXT"
	}

	return []channel.Event{{
		Kind: channel.EventMessageReceived,
		// Compoe com o escopo da instancia para nao colidir entre contas (armadilha 2).
		ExternalEventID: instance + ":" + messageID,
		InstanceName:    instance,
		OccurredAt:      occurredAt,
		Message: &channel.InboundMessage{
			ExternalMessageID: messageID,
			Channel:           "WHATSAPP",
			ContactExternalID: strings.TrimSpace(in.Phone),
			ContactPhone:      strings.TrimSpace(in.Phone),
			ContactName:       strings.TrimSpace(in.Name),
			MessageType:       msgType,
			Content:           in.Content,
			MediaURL:          strings.TrimSpace(in.MediaURL),
		},
	}}, nil
}

// SendMessage ecoa o envio: gera um external id deterministico e responde SENT na hora.
func (p *Provider) SendMessage(_ context.Context, _ channel.Credentials, out channel.OutboundMessage) (channel.SendResult, error) {
	id := strings.TrimSpace(out.IdempotencyKey)
	if id == "" {
		id = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return channel.SendResult{ExternalMessageID: "mock-" + id, Status: "SENT"}, nil
}

// DownloadMedia nao e suportado pelo mock (nao ha midia real). Erro claro, sem body.
func (p *Provider) DownloadMedia(_ context.Context, _ channel.Credentials, _ channel.MediaRef) (io.ReadCloser, channel.MediaMeta, error) {
	return nil, channel.MediaMeta{}, errors.New("mock: download de midia nao suportado")
}

// SendReaction e no-op bem-sucedido: o mock "reage" sem rede (permite testar a acao da F7 sem
// numero real). O gate de Capabilities().SupportsReaction=false ja barra reaction antes daqui;
// o metodo existe so para satisfazer a interface channel.Provider.
func (p *Provider) SendReaction(_ context.Context, _ channel.Credentials, _ channel.ReactionInput) error {
	return nil
}

// DeleteForAll e no-op bem-sucedido: o mock "apaga para todos" sem rede (testa a acao da F7 sem
// numero real).
func (p *Provider) DeleteForAll(_ context.Context, _ channel.Credentials, _ channel.DeleteInput) error {
	return nil
}

// Capabilities: o mock declara o basico (nada de template/janela 24h).
func (p *Provider) Capabilities() channel.Capabilities {
	return channel.Capabilities{
		SupportsTemplates: false,
		Requires24hWindow: false,
		SupportsReaction:  false,
		SupportsSticker:   false,
		SupportsGroups:    false,
		MaxMediaBytes:     16 << 20, // 16 MiB, so para a UI ter um teto
	}
}

// ============================================================================
// SessionManager — conecta na hora, com QR falso
// ============================================================================

// Connect "conecta" a instancia imediatamente: devolve um QR falso (para a tela ter o que
// mostrar) E ja marca Connected com um numero deterministico derivado do nome da instancia
// (numeros distintos por instancia — nao colidem no indice unico da conta).
func (p *Provider) Connect(_ context.Context, _ channel.Credentials, instanceName string) (channel.SessionState, error) {
	return channel.SessionState{
		Connected:   true,
		PhoneNumber: fakeNumber(instanceName),
		QRCode:      fakeQR,
	}, nil
}

// Status reporta a instancia como conectada (o mock nunca "cai").
func (p *Provider) Status(_ context.Context, _ channel.Credentials, instanceName string) (channel.SessionState, error) {
	return channel.SessionState{Connected: true, PhoneNumber: fakeNumber(instanceName)}, nil
}

// Logout no mock e no-op bem-sucedido.
func (p *Provider) Logout(_ context.Context, _ channel.Credentials, _ string) error { return nil }

// fakeNumber deriva um telefone deterministico (so digitos) do nome da instancia, para o
// mock "parear" um numero sem colidir entre instancias distintas.
func fakeNumber(instanceName string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(instanceName))
	return fmt.Sprintf("5500%09d", h.Sum32()%1000000000)
}
