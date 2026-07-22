package omnichannel

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// moduleID e o id do modulo em core.account_modules (para o LimitReader).
const moduleID = "omnichannel"

// defaultBootstrapProvider resolve o provider quando o FRONT nao o envia — e ele NUNCA
// envia: o inbox verbatim e Evolution-only e nao conhece o conceito de provider (D-A e do
// backend). Sem isto, todo bootstrap caia em ErrProviderUnsupported.
//
// Ordem: env OMNI_DEFAULT_WHATSAPP_PROVIDER (para trocar para 'evolution' quando o container
// estiver no ar) -> 'mock' (permite testar o fluxo inteiro sem numero real).
//
// LEGADO: isto e um default de pilOTO. A escolha REAL de provider por conta/numero (D-A) mora
// na coluna whatsapp_instances.provider (gestao de instancia) e/ou em account_modules.config;
// a tela que a expoe e a F10. Ate la, o env decide o default de contas sem instancia.
func defaultBootstrapProvider() string {
	if p := strings.TrimSpace(os.Getenv("OMNI_DEFAULT_WHATSAPP_PROVIDER")); p != "" {
		return p
	}
	return "mock"
}

// limitKeyChannels e a chave do teto de numeros por conta (canonico §5.3, aplicada aqui).
const limitKeyChannels = "max_whatsapp_numbers"

// SessionService cuida do ciclo de sessao/QR das instancias (bootstrap/connect/status/
// qrcode/logout) e da gravacao da credencial cifrada. Provider-agnostico: fala com o canal
// so pela interface channel.SessionManager, resolvida no Registry. Toda operacao e
// escopada por account (nunca do body) e restrita a admin da conta.
type SessionService struct {
	store     *Store
	registry  *channel.Registry
	secretBox *secretbox.Box
	qr        *qrCache
	limits    *modules.LimitReader
	logger    *slog.Logger
}

// NewSessionService monta o service de sessao. O qrCache e INJETADO (compartilhado com o
// InboundService no module.go) para o QR que a Evolution empurra por webhook chegar ao mesmo
// cache que o /qrcode le. nil => cria um proprio (nunca em prod; conveniencia de teste).
func NewSessionService(store *Store, registry *channel.Registry, box *secretbox.Box, limits *modules.LimitReader, qr *qrCache, logger *slog.Logger) *SessionService {
	if logger == nil {
		logger = slog.Default()
	}
	if qr == nil {
		qr = newQRCache()
	}
	return &SessionService{
		store:     store,
		registry:  registry,
		secretBox: box,
		qr:        qr,
		limits:    limits,
		logger:    logger,
	}
}

// SessionView e o estado servido ao painel apos cada operacao de sessao.
//
// Configured + ConnectionState existem para o front VERBATIM (WhatsAppStatusResponse): o inbox
// le connectionState.instance.state ("open"=conectado) — o shape nativo do Evolution que o legado
// devolvia. Sem eles o banner mostra "desconectado (estado: unknown)" mesmo conectado (D-B: o Go
// se adapta ao front). instanceName/provider/connected servem a camada de config (OmniSession).
type SessionView struct {
	InstanceName    string         `json:"instanceName"`
	Provider        string         `json:"provider"`
	IsDefault       bool           `json:"isDefault"`
	IsActive        bool           `json:"isActive"`
	Connected       bool           `json:"connected"`
	PhoneNumber     *string        `json:"phoneNumber"`
	QRCode          *string        `json:"qrCode"`
	CredentialsSet  bool           `json:"credentialsSet"`
	Configured      bool           `json:"configured"`
	ConnectionState map[string]any `json:"connectionState,omitempty"`
}

// SessionBootstrapInput e o body de bootstrap. provider deve ter adapter registrado.
type SessionBootstrapInput struct {
	InstanceName string `json:"instanceName"`
	DisplayName  string `json:"displayName"`
	Provider     string `json:"provider"`
}

// SessionInstanceInput e o body das operacoes que agem sobre uma instancia (connect/logout). O
// front verbatim manda `instanceId` (o id), nao `instanceName`; aceitamos os dois (id tem
// prioridade), senao cai na default da conta.
type SessionInstanceInput struct {
	InstanceName string `json:"instanceName"`
	InstanceID   string `json:"instanceId"`
}

// CredentialInput e o body de gravacao da credencial (item 7). apiKey e cifrado; nunca
// volta ao front (so {set,last4}).
type CredentialInput struct {
	APIKey string `json:"apiKey"`
}

// Bootstrap garante a instancia da conta: cria quando nao existe (validando o limite de
// canais -> 409), promove a default quando e a primeira. Idempotente pelo nome.
func (s *SessionService) Bootstrap(ctx context.Context, accountID string, caller Caller, in SessionBootstrapInput) (SessionView, error) {
	if !caller.IsAdmin {
		return SessionView{}, ErrForbidden
	}
	provider := strings.TrimSpace(in.Provider)
	if provider == "" {
		// O front verbatim nao manda provider (D-A e do backend). Default do pilOTO.
		provider = defaultBootstrapProvider()
	}
	if !s.registry.Has(provider) {
		return SessionView{}, ErrProviderUnsupported
	}
	name := strings.TrimSpace(in.InstanceName)
	if name == "" {
		name = "omni-main"
	}

	// Ja existe? Idempotente: nao recria nem reconta o limite.
	existing, err := s.store.GetSessionInstance(ctx, accountID, name)
	switch {
	case err == nil:
		return s.viewFor(ctx, accountID, existing, nil), nil
	case !errors.Is(err, pgx.ErrNoRows):
		return SessionView{}, err
	}

	// Nova instancia: valida o teto de numeros da conta (platform/modules.LimitReader).
	current, err := s.store.CountActiveInstances(ctx, accountID)
	if err != nil {
		return SessionView{}, err
	}
	if err := s.limits.Check(ctx, accountID, moduleID, limitKeyChannels, int64(current)); err != nil {
		if modules.IsLimitExceeded(err) {
			return SessionView{}, ErrChannelLimit
		}
		return SessionView{}, err
	}

	id, err := s.store.CreateInstance(ctx, accountID, name, in.DisplayName, provider, caller.UserID)
	if err != nil {
		if isUniqueViolation(err) {
			// Corrida com outro cadastro do mesmo nome: recupera o existente.
			if again, gErr := s.store.GetSessionInstance(ctx, accountID, name); gErr == nil {
				return s.viewFor(ctx, accountID, again, nil), nil
			}
		}
		return SessionView{}, err
	}
	// Primeira instancia ativa da conta -> promove a default.
	if current == 0 {
		if err := s.store.PromoteDefault(ctx, accountID, id); err != nil {
			return SessionView{}, err
		}
	}
	inst, err := s.store.GetSessionInstance(ctx, accountID, name)
	if err != nil {
		return SessionView{}, err
	}
	return s.viewFor(ctx, accountID, inst, nil), nil
}

// Connect inicia a sessao no provider. QR -> cache + devolve para o painel. Conectado com
// numero -> valida "um numero, uma instancia" (409) e grava o telefone.
func (s *SessionService) Connect(ctx context.Context, accountID string, caller Caller, in SessionInstanceInput) (SessionView, error) {
	inst, sm, cred, err := s.resolveForSession(ctx, accountID, caller, in.InstanceID, in.InstanceName)
	if err != nil {
		return SessionView{}, err
	}
	state, err := sm.Connect(ctx, cred, inst.InstanceName)
	if err != nil {
		s.logger.Warn("omnichannel_session_connect_failed", "account_id", accountID, "instance", inst.InstanceName)
		return SessionView{}, err
	}
	if err := s.applyState(ctx, accountID, inst, state); err != nil {
		return SessionView{}, err
	}
	return s.viewFromState(ctx, accountID, inst.InstanceName, state)
}

// Status consulta o estado da sessao no provider (sem forcar reconexao) e concilia o
// numero resolvido.
func (s *SessionService) Status(ctx context.Context, accountID string, caller Caller, instanceID, instanceName string) (SessionView, error) {
	inst, sm, cred, err := s.resolveForSession(ctx, accountID, caller, instanceID, instanceName)
	if err != nil {
		return SessionView{}, err
	}
	state, err := sm.Status(ctx, cred, inst.InstanceName)
	if err != nil {
		return SessionView{}, err
	}
	if err := s.applyState(ctx, accountID, inst, state); err != nil {
		return SessionView{}, err
	}
	return s.viewFromState(ctx, accountID, inst.InstanceName, state)
}

// QRCode devolve o QR vigente do cache (memoria, TTL 120s). Vazio => sem QR ativo.
func (s *SessionService) QRCode(ctx context.Context, accountID string, caller Caller, instanceID, instanceName string) (SessionView, error) {
	inst, sm, cred, err := s.resolveForSession(ctx, accountID, caller, instanceID, instanceName)
	if err != nil {
		return SessionView{}, err
	}
	state, err := sm.Status(ctx, cred, inst.InstanceName)
	if err != nil {
		return SessionView{}, err
	}
	if err := s.applyState(ctx, accountID, inst, state); err != nil {
		return SessionView{}, err
	}
	view, err := s.viewFromState(ctx, accountID, inst.InstanceName, state)
	if err != nil {
		return SessionView{}, err
	}
	if qr := s.qr.get(accountID, inst.InstanceName); qr != "" {
		view.QRCode = &qr
		view.ConnectionState = connectionStateFor(view.Connected, view.QRCode)
	}
	return view, nil
}

// Logout desconecta a instancia no provider, limpa o QR e zera o numero (a instancia
// continua cadastrada).
func (s *SessionService) Logout(ctx context.Context, accountID string, caller Caller, in SessionInstanceInput) (SessionView, error) {
	inst, sm, cred, err := s.resolveForSession(ctx, accountID, caller, in.InstanceID, in.InstanceName)
	if err != nil {
		return SessionView{}, err
	}
	if err := sm.Logout(ctx, cred, inst.InstanceName); err != nil {
		return SessionView{}, err
	}
	s.qr.set(accountID, inst.InstanceName, "")
	if err := s.store.ClearInstancePhone(ctx, accountID, inst.ID); err != nil {
		return SessionView{}, err
	}
	return s.viewFromState(ctx, accountID, inst.InstanceName, channel.SessionState{Connected: false})
}

// SetCredentials cifra e grava a credencial da instancia (item 7). A chave crua NUNCA volta
// ao front nem vai a log — a resposta e o status mascarado {set,last4}.
func (s *SessionService) SetCredentials(ctx context.Context, accountID string, caller Caller, instanceName string, in CredentialInput) (secretbox.Status, error) {
	inst, err := s.assertInstance(ctx, accountID, caller, "", instanceName)
	if err != nil {
		return secretbox.Status{}, err
	}
	apiKey := strings.TrimSpace(in.APIKey)
	if apiKey == "" {
		return secretbox.Status{}, ErrInvalidBody
	}
	if s.secretBox == nil {
		return secretbox.Status{}, errors.New("omnichannel: secretbox nao inicializado")
	}
	ciphertext, err := s.secretBox.Encrypt(apiKey)
	if err != nil {
		return secretbox.Status{}, err
	}
	if err := s.store.SetInstanceCredentials(ctx, accountID, inst.ID, ciphertext); err != nil {
		return secretbox.Status{}, err
	}
	return secretbox.Mask(apiKey), nil
}

// InstanceCapabilities resolve o provider da instancia (por id) e devolve o que ele suporta
// (templates, janela 24h, reaction, sticker, grupos, teto de midia). A UI degrada POR NUMERO
// (canonico §12 risco 2): sem esta rota, todo numero mostra "capacidades indisponiveis". E
// metadado do provider, nao dado sensivel — basta ser membro da conta (o middleware ja garante),
// sem exigir admin. Instancia fora de escopo / conta sem instancia -> 404.
func (s *SessionService) InstanceCapabilities(ctx context.Context, accountID, instanceID string) (channel.Capabilities, error) {
	inst, err := s.store.ResolveInstanceForOps(ctx, accountID, instanceID, "")
	if errors.Is(err, pgx.ErrNoRows) {
		return channel.Capabilities{}, ErrSessionUnavailable
	}
	if err != nil {
		return channel.Capabilities{}, err
	}
	prov, err := s.registry.Get(inst.Provider)
	if err != nil {
		return channel.Capabilities{}, ErrProviderUnsupported
	}
	return prov.Capabilities(), nil
}

// DeleteInstance remove uma instancia da conta (F10). BLOQUEIA com 409 se houver conversas
// atreladas — o front usa "Desativar" (PATCH isActive:false) no caso comum; o delete duro so
// quando nao ha historico (senao apagaria conversas em cascata sem intencao). So admin. Fora de
// escopo / inexistente -> 404.
func (s *SessionService) DeleteInstance(ctx context.Context, accountID string, caller Caller, instanceID string) error {
	if !caller.IsAdmin {
		return ErrForbidden
	}
	inst, err := s.store.ResolveInstanceForOps(ctx, accountID, instanceID, "")
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSessionUnavailable
	}
	if err != nil {
		return err
	}
	count, err := s.store.CountInstanceConversations(ctx, accountID, inst.ID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrInstanceHasConversations
	}
	return s.store.DeleteInstance(ctx, accountID, inst.ID)
}

// UpdateChannelLimit altera o teto de numeros ativos da conta. E configuracao
// comercial de plataforma, portanto owner/director da conta nao podem modifica-la.
func (s *SessionService) UpdateChannelLimit(ctx context.Context, accountID string, caller Caller, in ChannelLimitInput) (ChannelLimitView, error) {
	if !caller.IsPlatformAdmin {
		return ChannelLimitView{}, ErrForbidden
	}
	if in.MaxChannels < 1 || in.MaxChannels > 100 {
		return ChannelLimitView{}, ErrInvalidBody
	}
	current, err := s.store.CountActiveInstances(ctx, accountID)
	if err != nil {
		return ChannelLimitView{}, err
	}
	if in.MaxChannels < current {
		return ChannelLimitView{}, ErrInvalidBody
	}
	if err := s.store.SetModuleMaxChannels(ctx, accountID, in.MaxChannels); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChannelLimitView{}, ErrSessionUnavailable
		}
		return ChannelLimitView{}, err
	}
	return ChannelLimitView{MaxChannels: in.MaxChannels, CurrentChannels: current}, nil
}

// ============================================================================
// Helpers internos
// ============================================================================

// resolveForSession valida admin + escopo e resolve a instancia, o SessionManager do
// provider e as Credentials decifradas. instanceID tem prioridade sobre instanceName; ambos
// vazios => a default/1a ativa da conta.
func (s *SessionService) resolveForSession(ctx context.Context, accountID string, caller Caller, instanceID, instanceName string) (sessionInstance, channel.SessionManager, channel.Credentials, error) {
	inst, err := s.assertInstance(ctx, accountID, caller, instanceID, instanceName)
	if err != nil {
		return sessionInstance{}, nil, channel.Credentials{}, err
	}
	sm, err := s.registry.Session(inst.Provider)
	if err != nil {
		return sessionInstance{}, nil, channel.Credentials{}, ErrProviderUnsupported
	}
	cred, err := s.credentialsFor(ctx, accountID, inst)
	if err != nil {
		return sessionInstance{}, nil, channel.Credentials{}, err
	}
	return inst, sm, cred, nil
}

// assertInstance garante admin da conta e resolve a instancia por id (prioridade) ou nome;
// ambos vazios => a default/1a ativa (o /status e /qrcode do inbox nao mandam nome). Fora de
// escopo / conta sem instancia -> 404.
func (s *SessionService) assertInstance(ctx context.Context, accountID string, caller Caller, instanceID, instanceName string) (sessionInstance, error) {
	if !caller.IsAdmin {
		return sessionInstance{}, ErrForbidden
	}
	inst, err := s.store.GetSessionInstanceByRef(ctx, accountID, instanceID, instanceName)
	if errors.Is(err, pgx.ErrNoRows) {
		return sessionInstance{}, ErrSessionUnavailable
	}
	if err != nil {
		return sessionInstance{}, err
	}
	return inst, nil
}

// applyState concilia o SessionState do provider com o banco/cache: QR -> cache; numero
// resolvido -> guard (409) + grava.
func (s *SessionService) applyState(ctx context.Context, accountID string, inst sessionInstance, state channel.SessionState) error {
	// So mexe no cache do QR quando ha um QR NOVO (connect/webhook) ou quando conectou (limpa
	// o QR pendente). Status() nao carrega QR (vem vazio); sobrescrever com vazio APAGARIA o QR
	// que o connect acabou de cachear — o generateQrCode roda status+qrcode em paralelo, e a
	// corrida fazia o QR aparecer e sumir. So limpa em vazio SE conectou.
	if qr := strings.TrimSpace(state.QRCode); qr != "" {
		s.qr.set(accountID, inst.InstanceName, state.QRCode)
	} else if state.Connected {
		s.qr.set(accountID, inst.InstanceName, "")
	}
	phone := strings.TrimSpace(state.PhoneNumber)
	if !state.Connected || phone == "" {
		return nil
	}
	// Se o numero ja e o gravado, nada a fazer.
	if inst.PhoneNumber != nil && *inst.PhoneNumber == phone {
		return nil
	}
	if err := ensureNumberFree(ctx, s.store, accountID, phone, inst.ID); err != nil {
		return err
	}
	if err := s.store.SetInstancePhone(ctx, accountID, inst.ID, phone); err != nil {
		if isUniqueViolation(err) {
			// A constraint fechou a corrida: reporta como colisao acionavel.
			return &NumberInUseError{InstanceName: "outra instancia"}
		}
		return err
	}
	// Conectou: nao ha mais QR pendente.
	s.qr.set(accountID, inst.InstanceName, "")
	return nil
}

// credentialsFor decifra a credencial da instancia (por provider). Sem ciphertext (mock)
// => Credentials vazio. A chave crua nunca vai a log.
func (s *SessionService) credentialsFor(ctx context.Context, accountID string, inst sessionInstance) (channel.Credentials, error) {
	cipher, config, found, err := s.store.FindProviderCredential(ctx, accountID, inst.Provider)
	if err != nil {
		return channel.Credentials{}, err
	}
	cred := channel.Credentials{Config: config}
	if cred.Config == nil {
		cred.Config = map[string]string{}
	}
	// webhookUrl: para onde o provider (Evolution) devolve QR + mensagens inbound. SEM isto
	// a Evolution gera o QR mas nao tem destino, e o pareamento nunca aparece (LEGADO item j).
	// Base = WEBHOOK_RECEIVER_BASE_URL (env, http://api:8080 em dev; dominio publico em prod).
	// A rota resolve a conta pelo SLUG, entao montamos com o slug (nao o UUID).
	if base := strings.TrimSpace(os.Getenv("WEBHOOK_RECEIVER_BASE_URL")); base != "" {
		if slug, sErr := s.store.AccountSlug(ctx, accountID); sErr == nil && slug != "" {
			// "webhookUrl" e a chave de contrato que o adapter evolution le (adapter.go
			// configKeyWebhookURL, private ao pacote dele) — literal de proposito.
			cred.Config["webhookUrl"] = strings.TrimRight(base, "/") +
				"/v1/webhooks/omnichannel/" + inst.Provider + "/" + slug
		}
	}
	if !found || strings.TrimSpace(cipher) == "" || s.secretBox == nil {
		return cred, nil
	}
	token, err := s.secretBox.Decrypt(cipher)
	if err != nil {
		s.logger.Error("omnichannel_credential_decrypt_failed", "account_id", accountID, "provider", inst.Provider)
		return channel.Credentials{}, err
	}
	cred.Token = token
	return cred, nil
}

// viewFromState re-le a instancia e monta a view com o estado AUTORITATIVO devolvido pelo
// provider. phone_number e dado cadastral/identificador e nunca prova que existe uma sessao.
func (s *SessionService) viewFromState(ctx context.Context, accountID, instanceName string, state channel.SessionState) (SessionView, error) {
	inst, err := s.store.GetSessionInstance(ctx, accountID, instanceName)
	if err != nil {
		return SessionView{}, err
	}
	return s.viewFor(ctx, accountID, inst, &state), nil
}

// viewFor monta a SessionView. connected so pode vir de SessionState consultado no provider;
// telefone preenchido manualmente continua disponivel para exibicao, mas nao abre a sessao.
// state=nil (ex.: bootstrap) e deliberadamente "nao conectado" ate o proximo /status.
func (s *SessionService) viewFor(_ context.Context, accountID string, inst sessionInstance, state *channel.SessionState) SessionView {
	connected := state != nil && state.Connected
	v := SessionView{
		InstanceName:   inst.InstanceName,
		Provider:       inst.Provider,
		IsDefault:      inst.IsDefault,
		IsActive:       inst.IsActive,
		Connected:      connected,
		PhoneNumber:    inst.PhoneNumber,
		CredentialsSet: inst.CredentialsSet,
		Configured:     true,
	}
	if qr := s.qr.get(accountID, inst.InstanceName); qr != "" {
		v.QRCode = &qr
	}
	v.ConnectionState = connectionStateFor(v.Connected, v.QRCode)
	return v
}

// connectionStateFor monta o connectionState.instance.state que o front verbatim le
// (useOmnichannelAdminConnectionState.ts): "open"=conectado, "connecting"=ha QR pendente,
// "close"=sem sessao. E o shape nativo do Evolution — o legado devolvia a resposta crua do
// provider; aqui derivamos do estado ja conciliado no banco/cache.
func connectionStateFor(connected bool, qr *string) map[string]any {
	state := "close"
	switch {
	case connected:
		state = "open"
	case qr != nil && strings.TrimSpace(*qr) != "":
		state = "connecting"
	}
	return map[string]any{"instance": map[string]any{"state": state}}
}
