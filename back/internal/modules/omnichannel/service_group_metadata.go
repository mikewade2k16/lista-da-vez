package omnichannel

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type groupMetadataResolver struct {
	store     *Store
	registry  *channel.Registry
	secretBox *secretbox.Box
	logger    *slog.Logger

	mu    sync.Mutex
	cache map[string]groupMetadataCacheEntry
}

type groupMetadataCacheEntry struct {
	name      string
	expiresAt time.Time
}

func newGroupMetadataResolver(store *Store, registry *channel.Registry, box *secretbox.Box, logger *slog.Logger) *groupMetadataResolver {
	if logger == nil {
		logger = slog.Default()
	}
	return &groupMetadataResolver{
		store: store, registry: registry, secretBox: box, logger: logger,
		cache: make(map[string]groupMetadataCacheEntry),
	}
}

func (r *groupMetadataResolver) resolve(ctx context.Context, accountID, instanceName, groupJID string) (string, error) {
	key := strings.Join([]string{accountID, instanceName, groupJID}, "|")
	now := time.Now()
	r.mu.Lock()
	if entry, ok := r.cache[key]; ok && now.Before(entry.expiresAt) {
		r.mu.Unlock()
		return entry.name, nil
	}
	r.mu.Unlock()

	providerName, found, err := r.store.FindInstanceProviderByName(ctx, accountID, instanceName)
	if err != nil || !found {
		return "", err
	}
	provider, err := r.registry.Get(providerName)
	if err != nil {
		return "", err
	}
	groupProvider, ok := provider.(channel.GroupMetadataProvider)
	if !ok {
		return "", errors.New("provider sem consulta de metadata de grupo")
	}
	cred, err := r.credentials(ctx, accountID, providerName, instanceName)
	if err != nil {
		return "", err
	}
	metadataCtx, cancel := context.WithTimeout(ctx, 900*time.Millisecond)
	defer cancel()
	metadata, err := groupProvider.FetchGroupMetadata(metadataCtx, cred, instanceName, groupJID)
	if err != nil || strings.TrimSpace(metadata.Name) == "" {
		r.mu.Lock()
		r.cache[key] = groupMetadataCacheEntry{expiresAt: time.Now().Add(30 * time.Second)}
		r.mu.Unlock()
		return "", err
	}

	name := strings.TrimSpace(metadata.Name)
	r.mu.Lock()
	r.cache[key] = groupMetadataCacheEntry{name: name, expiresAt: time.Now().Add(10 * time.Minute)}
	r.mu.Unlock()
	return name, nil
}

func (r *groupMetadataResolver) credentials(ctx context.Context, accountID, providerName, instanceName string) (channel.Credentials, error) {
	ciphertext, config, found, err := r.store.FindProviderCredentialForKey(ctx, accountID, providerName, instanceName)
	if err != nil {
		return channel.Credentials{}, err
	}
	cred := channel.Credentials{Config: config}
	if !found || strings.TrimSpace(ciphertext) == "" || r.secretBox == nil {
		return cred, nil
	}
	token, err := r.secretBox.Decrypt(ciphertext)
	if err != nil {
		r.logger.Error("omnichannel_group_metadata_credential_decrypt_failed", "account_id", accountID, "provider", providerName)
		return channel.Credentials{}, err
	}
	cred.Token = token
	return cred, nil
}
