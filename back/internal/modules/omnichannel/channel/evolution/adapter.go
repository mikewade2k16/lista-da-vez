package evolution

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

// providerID e a chave do adapter (= whatsapp_instances.provider = 'evolution', no CHECK da
// migration da F2).
const providerID = "evolution"

// requestTimeout e o timeout de TODAS as chamadas HTTP a Evolution (SPECS_PORT F3 item 1).
const requestTimeout = 30 * time.Second

// configKeyBaseURL / configKeyWebhookURL sao as chaves nao-secretas lidas de
// whatsapp_instances.provider_config (via Credentials.Config). baseURL sobrepoe o env;
// webhookURL, quando presente, habilita o auto-reparo do webhook no connect.
const (
	configKeyBaseURL    = "baseURL"
	configKeyWebhookURL = "webhookUrl"
)

// Assercoes de contrato: o adapter implementa AS DUAS interfaces do pacote channel. Falha de
// compilacao se a interface da Fundacao mudar (o SessionManager e resolvido por type-assert
// no Registry — sem esta linha, uma quebra so apareceria em runtime).
var (
	_ channel.Provider       = (*Provider)(nil)
	_ channel.SessionManager = (*Provider)(nil)
)

// Provider e o adapter Evolution. E um SINGLETON stateless no Registry: o estado (baseURL/
// apiKey por instancia) chega por Credentials em cada chamada; env e so fallback. Compartilha
// um unico *http.Client (timeout 30s) — seguro para uso concorrente.
type Provider struct {
	envBaseURL string
	envAPIKey  string
	http       *http.Client
	logger     *slog.Logger
}

// New cria o adapter Evolution. envBaseURL/envAPIKey sao o FALLBACK de ambiente
// (EVOLUTION_BASE_URL / EVOLUTION_API_KEY): a fonte real e a credencial por instancia
// (canonico §2 D-A). Ambos podem ser vazios — nesse caso so instancias com credencial
// propria funcionam. logger e opcional (variadico p/ nao quebrar os testes): ausente/nil =>
// slog.Default().
func New(envBaseURL, envAPIKey string, logger ...*slog.Logger) *Provider {
	lg := slog.Default()
	if len(logger) > 0 && logger[0] != nil {
		lg = logger[0]
	}
	return &Provider{
		envBaseURL: strings.TrimRight(strings.TrimSpace(envBaseURL), "/"),
		envAPIKey:  strings.TrimSpace(envAPIKey),
		http:       &http.Client{Timeout: requestTimeout},
		logger:     lg,
	}
}

// ID implementa channel.Provider.
func (p *Provider) ID() string { return providerID }

// Capabilities: a Evolution (WhatsApp/Baileys) NAO tem templates nem janela de 24h (isso e
// so Meta Cloud — F11). Suporta reacao/sticker/grupo. Teto de midia conservador (o WhatsApp
// aceita ~64-100MB por tipo; a UI degrada por numero — canonico §12 risco 2).
func (p *Provider) Capabilities() channel.Capabilities {
	return channel.Capabilities{
		SupportsTemplates: false,
		Requires24hWindow: false,
		SupportsReaction:  true,
		SupportsSticker:   true,
		SupportsGroups:    true,
		MaxMediaBytes:     60 << 20, // 60 MiB
	}
}

// ============================================================================
// Envio e midia (channel.Provider)
// ============================================================================

// SendMessage envia texto ou midia. TEXT usa /message/sendText; o resto usa /message/sendMedia
// com o mediatype derivado do MessageType canonico. O id atribuido pelo provider volta em
// SendResult; falha de transporte/HTTP nao vaza a apiKey (client.do mascara).
func (p *Provider) SendMessage(ctx context.Context, cred channel.Credentials, out channel.OutboundMessage) (channel.SendResult, error) {
	c, err := p.clientFor(cred)
	if err != nil {
		return channel.SendResult{}, err
	}
	number := strings.TrimSpace(out.ToPhone)
	if number == "" {
		return channel.SendResult{}, errors.New("evolution: numero de destino vazio")
	}

	msgType := strings.ToUpper(strings.TrimSpace(out.MessageType))
	if msgType == "" || msgType == "TEXT" {
		res, err := c.sendText(ctx, out.InstanceName, number, out.Content)
		if err != nil {
			return channel.SendResult{Status: "FAILED"}, err
		}
		return channel.SendResult{ExternalMessageID: res.externalID(), Status: "SENT"}, nil
	}

	res, err := c.sendMedia(ctx, out.InstanceName, mediaSend{
		Number:    number,
		MediaType: mediaTypeFor(msgType),
		Media:     out.MediaURL,
		MimeType:  out.MediaMimeType,
		FileName:  out.MediaFileName,
		Caption:   firstNonEmpty(out.MediaCaption, out.Content),
	})
	if err != nil {
		return channel.SendResult{Status: "FAILED"}, err
	}
	return channel.SendResult{ExternalMessageID: res.externalID(), Status: "SENT"}, nil
}

// DownloadMedia baixa a midia via getBase64FromMediaMessage e devolve um ReadCloser sobre os
// bytes decodificados. A base64 crua NUNCA vai a log.
func (p *Provider) DownloadMedia(ctx context.Context, cred channel.Credentials, ref channel.MediaRef) (io.ReadCloser, channel.MediaMeta, error) {
	c, err := p.clientFor(cred)
	if err != nil {
		return nil, channel.MediaMeta{}, err
	}
	id := strings.TrimSpace(ref.ExternalMessageID)
	if id == "" {
		return nil, channel.MediaMeta{}, errors.New("evolution: id da mensagem de midia vazio")
	}
	res, err := c.getBase64(ctx, ref.InstanceName, id)
	if err != nil {
		return nil, channel.MediaMeta{}, err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(res.Base64))
	if err != nil {
		return nil, channel.MediaMeta{}, errors.New("evolution: midia retornada nao e base64 valida")
	}
	meta := channel.MediaMeta{
		MimeType:  strings.TrimSpace(res.MimeType),
		FileName:  strings.TrimSpace(res.FileName),
		SizeBytes: int64(len(raw)),
	}
	return io.NopCloser(bytes.NewReader(raw)), meta, nil
}

// ============================================================================
// Acoes sincronas de mensagem (F7 — reaction / delete-for-all)
// ============================================================================

// SendReaction reage a uma mensagem: POST /message/sendReaction/{i} com a KEY da mensagem alvo
// {remoteJid, fromMe, id} + o emoji (vazio remove a reacao). Falha de transporte/HTTP nao vaza
// a apiKey (client.do mascara); o caller mapeia o erro para 502.
func (p *Provider) SendReaction(ctx context.Context, cred channel.Credentials, in channel.ReactionInput) error {
	c, err := p.clientFor(cred)
	if err != nil {
		return err
	}
	id := strings.TrimSpace(in.ExternalMessageID)
	if id == "" {
		return errors.New("evolution: id da mensagem para reagir vazio")
	}
	remoteJid := strings.TrimSpace(in.RemoteJID)
	if remoteJid == "" {
		return errors.New("evolution: remoteJid da reacao vazio")
	}
	return c.sendReaction(ctx, in.InstanceName, reactionSend{
		RemoteJid: remoteJid,
		FromMe:    in.FromMe,
		MessageID: id,
		Reaction:  in.Emoji,
	})
}

// DeleteForAll apaga uma mensagem para todos: DELETE /chat/deleteMessageForEveryone/{i} com a
// KEY {id, remoteJid, fromMe} (+ participant em grupo). Non-2xx/transporte nao vaza corpo/chave;
// o caller acumula a falha por-id em failedIds.
func (p *Provider) DeleteForAll(ctx context.Context, cred channel.Credentials, in channel.DeleteInput) error {
	c, err := p.clientFor(cred)
	if err != nil {
		return err
	}
	id := strings.TrimSpace(in.ExternalMessageID)
	if id == "" {
		return errors.New("evolution: id da mensagem para apagar vazio")
	}
	remoteJid := strings.TrimSpace(in.RemoteJID)
	if remoteJid == "" {
		return errors.New("evolution: remoteJid do apagar vazio")
	}
	participant := ""
	if strings.HasSuffix(remoteJid, "@g.us") {
		participant = strings.TrimSpace(in.ParticipantJID)
	}
	return c.deleteForEveryone(ctx, in.InstanceName, deleteKey{
		MessageID:   id,
		RemoteJid:   remoteJid,
		FromMe:      in.FromMe,
		Participant: participant,
	})
}

// ============================================================================
// SessionManager (ciclo QR/conexao)
// ============================================================================

// Connect garante a instancia (createInstance idempotente), configura o webhook quando a URL
// e conhecida (auto-reparo) e inicia a sessao. Devolve o QR (data URL) para o painel ler, ou
// Connected+PhoneNumber quando ja pareado.
func (p *Provider) Connect(ctx context.Context, cred channel.Credentials, instanceName string) (channel.SessionState, error) {
	c, err := p.clientFor(cred)
	if err != nil {
		return channel.SessionState{}, err
	}
	webhookURL := strings.TrimSpace(cred.Config[configKeyWebhookURL])

	// createInstance e idempotente por nome: instancia ja existente responde non-2xx e o
	// erro e ignorado (nao e falha real). So um erro de transporte aborta.
	if err := c.createInstance(ctx, instanceName, webhookURL); err != nil {
		var apiErr *apiError
		if !errors.As(err, &apiErr) {
			return channel.SessionState{}, err
		}
	}
	// Auto-reparo do webhook (best-effort: erro nao aborta o connect). Idempotente: cobre a
	// instancia preexistente (o create so leva o webhook quando ela e nova). Erro so LOGADO
	// em Warn (nao aborta) — a webhook_url NAO e secreta (pode logar); o token NUNCA (o err
	// do client so carrega status/generico, nunca a apiKey).
	if webhookURL != "" {
		if err := c.setWebhook(ctx, instanceName, webhookURL); err != nil {
			p.logger.Warn("evolution_set_webhook_failed",
				"instance", instanceName, "webhook_url", webhookURL, "err", err)
		}
	}

	resp, err := c.connect(ctx, instanceName)
	if err != nil {
		return channel.SessionState{}, err
	}
	// Ja pareado? O connect pode devolver a instancia conectada sem QR.
	if resp.Instance != nil && isConnectedState(resp.Instance.State, resp.Instance.Status) {
		return channel.SessionState{
			Connected:   true,
			PhoneNumber: numberFromJid(firstNonEmpty(resp.Instance.OwnerJid, resp.Instance.Owner, resp.Instance.Number)),
		}, nil
	}
	qr := normalizeQR(qrFrom(resp))
	return channel.SessionState{Connected: false, QRCode: qr}, nil
}

// Status consulta o estado sem forcar reconexao (fetchInstances) e concilia o numero pareado.
func (p *Provider) Status(ctx context.Context, cred channel.Credentials, instanceName string) (channel.SessionState, error) {
	c, err := p.clientFor(cred)
	if err != nil {
		return channel.SessionState{}, err
	}
	raw, err := c.fetchInstance(ctx, instanceName)
	if err != nil {
		return channel.SessionState{}, err
	}
	info := parseInstanceInfo(raw)
	connected := isConnectedState(info.State, info.Status)
	return channel.SessionState{
		Connected:   connected,
		PhoneNumber: numberFromJid(firstNonEmpty(info.OwnerJid, info.Owner, info.Number)),
	}, nil
}

// Logout desconecta a instancia no provider.
func (p *Provider) Logout(ctx context.Context, cred channel.Credentials, instanceName string) error {
	c, err := p.clientFor(cred)
	if err != nil {
		return err
	}
	return c.logout(ctx, instanceName)
}

// ============================================================================
// Resolucao de config (instancia -> env fallback)
// ============================================================================

// clientFor monta o client HTTP com baseURL/apiKey EFETIVOS: a credencial da instancia vence;
// o env e fallback. baseURL ausente em ambos => erro acionavel (principio 5), nunca chamada a
// um host vazio. A apiKey pode ser vazia (a Evolution pode nao exigir), mas o baseURL nao.
func (p *Provider) clientFor(cred channel.Credentials) (*client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cred.Config[configKeyBaseURL]), "/")
	if baseURL == "" {
		baseURL = p.envBaseURL
	}
	if baseURL == "" {
		return nil, errors.New("evolution: baseURL nao configurada (instancia provider_config.baseURL ou EVOLUTION_BASE_URL)")
	}
	apiKey := strings.TrimSpace(cred.Token)
	if apiKey == "" {
		apiKey = p.envAPIKey
	}
	return &client{baseURL: baseURL, apiKey: apiKey, http: p.http}, nil
}

// expectedToken resolve o segredo esperado do webhook: credencial da instancia (Token) com
// fallback no env. Usado pelo VerifyWebhook.
func (p *Provider) expectedToken(cred channel.Credentials) string {
	if t := strings.TrimSpace(cred.Token); t != "" {
		return t
	}
	return p.envAPIKey
}

// ============================================================================
// Helpers puros
// ============================================================================

// mediaTypeFor traduz o MessageType canonico para o `mediatype` da Evolution.
func mediaTypeFor(canonical string) string {
	switch strings.ToUpper(strings.TrimSpace(canonical)) {
	case "IMAGE":
		return "image"
	case "VIDEO":
		return "video"
	case "AUDIO":
		return "audio"
	default:
		return "document"
	}
}

// firstNonEmpty devolve o primeiro argumento nao-vazio (apos trim).
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// isConnectedState reconhece os rotulos de "conectado" da Evolution (varia entre versoes:
// state `open` / status `connected`|`online`).
func isConnectedState(state, status string) bool {
	s := strings.ToLower(strings.TrimSpace(state))
	st := strings.ToLower(strings.TrimSpace(status))
	return s == "open" || st == "open" || st == "connected" || st == "online"
}

// numberFromJid extrai o telefone (so digitos) de um JID do WhatsApp
// (ex.: `5511999999999@s.whatsapp.net` -> `5511999999999`).
func numberFromJid(jid string) string {
	jid = strings.TrimSpace(jid)
	if jid == "" {
		return ""
	}
	if i := strings.IndexByte(jid, '@'); i >= 0 {
		jid = jid[:i]
	}
	// Alguns JIDs trazem sufixo de device (`:12`); corta no `:`.
	if i := strings.IndexByte(jid, ':'); i >= 0 {
		jid = jid[:i]
	}
	var b strings.Builder
	for _, r := range jid {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
