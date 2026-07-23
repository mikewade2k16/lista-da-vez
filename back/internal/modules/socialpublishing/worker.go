package socialpublishing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type moduleEnabledChecker interface {
	IsEnabled(ctx context.Context, accountID, moduleID string) (bool, error)
}

type PublishHandler struct {
	repo     workerRepository
	provider InstagramProvider
	secrets  *secretbox.Box
	modules  moduleEnabledChecker
	logger   *slog.Logger
	now      func() time.Time
}

func NewPublishHandler(
	repo workerRepository,
	provider InstagramProvider,
	secrets *secretbox.Box,
	modules moduleEnabledChecker,
	logger *slog.Logger,
) *PublishHandler {
	return &PublishHandler{
		repo: repo, provider: provider, secrets: secrets,
		modules: modules, logger: logger, now: time.Now,
	}
}

func (h *PublishHandler) Handle(ctx context.Context, job jobs.Job) error {
	var payload publishJobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil ||
		strings.TrimSpace(payload.PostID) == "" || payload.Revision <= 0 {
		return &jobs.StatusError{Unrecoverable: true, Err: jobs.ErrInvalidJob}
	}
	enabled, err := h.moduleEnabled(ctx, job.AccountID)
	if err != nil {
		return err
	}
	if !enabled {
		if err := h.repo.MarkPublishFailed(
			ctx,
			job.AccountID,
			payload.PostID,
			payload.Revision,
			"module_disabled",
			"Modulo desabilitado antes da publicacao.",
		); err != nil {
			return err
		}
		return nil
	}
	target, ready, err := h.repo.PreparePublish(
		ctx,
		job.AccountID,
		payload.PostID,
		payload.Revision,
	)
	if err != nil {
		if errors.Is(err, ErrNotConnected) {
			if err := h.repo.MarkPublishFailed(
				ctx,
				job.AccountID,
				payload.PostID,
				payload.Revision,
				"not_connected",
				"Conexao do Instagram indisponivel.",
			); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	if !ready {
		return nil
	}
	if target.PublishAttemptedAt != nil && target.ExternalMediaID == "" {
		if err := h.repo.MarkPublishFailed(
			ctx,
			target.AccountID,
			target.PostID,
			target.Revision,
			"publish_outcome_unknown",
			"Execucao anterior interrompida; confira o Instagram antes de tentar novamente.",
		); err != nil {
			return err
		}
		return nil
	}
	if h.secrets == nil {
		return h.fail(
			ctx,
			target,
			"secrets_unavailable",
			"Cofre de segredos indisponivel.",
			&jobs.StatusError{Unrecoverable: true, Err: ErrSecretsUnavailable},
		)
	}
	token, err := h.secrets.Decrypt(target.TokenCiphertext)
	if err != nil {
		return h.fail(
			ctx,
			target,
			"credential_invalid",
			"Credencial do Instagram invalida.",
			&jobs.StatusError{Unrecoverable: true, Err: ErrSecretsUnavailable},
		)
	}

	creationID := target.ExternalCreationID
	if creationID == "" {
		creationID, err = h.provider.CreateImageContainer(
			ctx,
			token,
			target.IGUserID,
			target.MediaURL,
			target.Caption,
			target.AltText,
		)
		if err != nil {
			return h.failProvider(ctx, target, err)
		}
		if err := h.repo.SaveCreationID(
			ctx,
			target.AccountID,
			target.PostID,
			target.Revision,
			creationID,
		); err != nil {
			return err
		}
	}
	enabled, err = h.moduleEnabled(ctx, job.AccountID)
	if err != nil {
		return err
	}
	if !enabled {
		return h.fail(
			ctx,
			target,
			"module_disabled",
			"Modulo desabilitado antes da publicacao.",
			nil,
		)
	}
	attemptMarked, err := h.repo.MarkPublishAttempted(
		ctx,
		target.AccountID,
		target.PostID,
		target.Revision,
	)
	if err != nil {
		return err
	}
	if !attemptMarked {
		return h.fail(
			ctx,
			target,
			"publish_outcome_unknown",
			"Confira o Instagram antes de tentar novamente.",
			nil,
		)
	}
	mediaID, err := h.provider.PublishContainer(ctx, token, target.IGUserID, creationID)
	if err != nil {
		var providerErr *ProviderError
		if errors.As(err, &providerErr) && providerErr.StatusCode >= 400 &&
			providerErr.StatusCode < 500 && providerErr.StatusCode != http.StatusTooManyRequests {
			return h.fail(
				ctx,
				target,
				fmt.Sprintf("instagram_http_%d", providerErr.StatusCode),
				"Instagram recusou a publicacao.",
				nil,
			)
		}
		// Timeout, transporte, 429 ou 5xx deixam o efeito externo indeterminado.
		return h.fail(ctx, target, "publish_outcome_unknown",
			"Confira o Instagram antes de tentar novamente.", nil)
	}

	publishedAt := h.now().UTC()
	if err := h.repo.MarkPublished(
		ctx,
		target.AccountID,
		target.PostID,
		target.Revision,
		mediaID,
		publishedAt,
	); err != nil {
		return err
	}

	// O external media ID e persistido antes do enriquecimento. Falha ao buscar o
	// permalink nunca repete media_publish e, portanto, nunca duplica o post.
	if permalink, fetchErr := h.provider.FetchPermalink(ctx, token, mediaID); fetchErr == nil && permalink != "" {
		if saveErr := h.repo.SavePermalink(
			ctx,
			target.AccountID,
			target.PostID,
			mediaID,
			permalink,
		); saveErr != nil {
			h.warn("social_publishing_permalink_save_failed", target.AccountID, target.PostID, saveErr)
		}
	} else if fetchErr != nil {
		h.warn("social_publishing_permalink_fetch_failed", target.AccountID, target.PostID, fetchErr)
	}
	return nil
}

func (h *PublishHandler) moduleEnabled(ctx context.Context, accountID string) (bool, error) {
	if h.modules == nil {
		return false, ErrProviderUnavailable
	}
	return h.modules.IsEnabled(ctx, accountID, ModuleID)
}

func (h *PublishHandler) failProvider(
	ctx context.Context,
	target publishTarget,
	err error,
) error {
	code := "provider_unavailable"
	message := "Instagram temporariamente indisponivel."
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		code = fmt.Sprintf("instagram_http_%d", providerErr.StatusCode)
		if providerErr.StatusCode >= 400 && providerErr.StatusCode < 500 {
			message = "Instagram recusou a publicacao."
		}
	}
	return h.fail(ctx, target, code, message, classifyProviderError(err))
}

func (h *PublishHandler) fail(
	ctx context.Context,
	target publishTarget,
	code, message string,
	result error,
) error {
	if err := h.repo.MarkPublishFailed(
		ctx,
		target.AccountID,
		target.PostID,
		target.Revision,
		code,
		message,
	); err != nil {
		return err
	}
	return result
}

func (h *PublishHandler) warn(message, accountID, postID string, err error) {
	if h.logger != nil {
		h.logger.Warn(
			message,
			"module", ModuleID,
			"account_id", accountID,
			"post_id", postID,
			"error", err.Error(),
		)
	}
}

type AnalyticsHandler struct {
	repo interface {
		analyticsRepository
	}
	provider InstagramProvider
	secrets  *secretbox.Box
	modules  moduleEnabledChecker
	now      func() time.Time
}

func NewAnalyticsHandler(
	repo analyticsRepository,
	provider InstagramProvider,
	secrets *secretbox.Box,
	modules moduleEnabledChecker,
) *AnalyticsHandler {
	return &AnalyticsHandler{
		repo: repo, provider: provider, secrets: secrets, modules: modules, now: time.Now,
	}
}

func (h *AnalyticsHandler) Handle(ctx context.Context, job jobs.Job) error {
	var payload analyticsJobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil ||
		strings.TrimSpace(payload.PostID) == "" {
		return &jobs.StatusError{Unrecoverable: true, Err: jobs.ErrInvalidJob}
	}
	if h.modules == nil {
		return ErrProviderUnavailable
	}
	enabled, err := h.modules.IsEnabled(ctx, job.AccountID, ModuleID)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	target, err := h.repo.AnalyticsTarget(ctx, job.AccountID, payload.PostID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if h.secrets == nil || target.TokenCiphertext == "" {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrSecretsUnavailable}
	}
	token, err := h.secrets.Decrypt(target.TokenCiphertext)
	if err != nil {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrSecretsUnavailable}
	}
	analytics, err := h.provider.FetchMediaInsights(ctx, token, target.ExternalMediaID)
	if err != nil {
		return classifyProviderError(err)
	}
	analytics.PostID = target.PostID
	analytics.CapturedAt = h.now().UTC()
	return h.repo.SaveAnalytics(ctx, target.AccountID, target.PostID, analytics)
}

func classifyProviderError(err error) error {
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		return &jobs.StatusError{StatusCode: 0, Err: ErrProviderUnavailable}
	}
	unrecoverable := providerErr.StatusCode == http.StatusBadRequest ||
		providerErr.StatusCode == http.StatusUnauthorized ||
		providerErr.StatusCode == http.StatusForbidden ||
		providerErr.StatusCode == http.StatusNotFound
	return &jobs.StatusError{
		StatusCode:    providerErr.StatusCode,
		Unrecoverable: unrecoverable,
		Err:           providerErr,
	}
}
