package omnichannel

import (
	"sync"
	"time"
)

// qrCache guarda o QR (data URL) por (accountID, instanceName) com TTL curto. Fica em
// MEMORIA (o go.mod nao tem Redis — spec C4): consequencia registrada em docs/LEGADO.md —
// nao sobrevive a restart nem a deploy multi-instancia. Hoje a api e um container so.
type qrCache struct {
	mu      sync.Mutex
	entries map[string]qrEntry
	ttl     time.Duration
	now     func() time.Time
}

type qrEntry struct {
	dataURL   string
	expiresAt time.Time
}

// qrCacheTTL espelha o TTL do legado (Redis 120s).
const qrCacheTTL = 120 * time.Second

func newQRCache() *qrCache {
	return &qrCache{
		entries: make(map[string]qrEntry),
		ttl:     qrCacheTTL,
		now:     time.Now,
	}
}

func qrKey(accountID, instanceName string) string {
	return accountID + "|" + instanceName
}

// set guarda o QR. dataURL vazio limpa a entrada (ex.: instancia ja conectou).
func (c *qrCache) set(accountID, instanceName, dataURL string) {
	key := qrKey(accountID, instanceName)
	c.mu.Lock()
	defer c.mu.Unlock()
	if dataURL == "" {
		delete(c.entries, key)
		return
	}
	c.entries[key] = qrEntry{dataURL: dataURL, expiresAt: c.now().Add(c.ttl)}
}

// get devolve o QR vigente (ou "" se ausente/expirado). Limpa a entrada expirada.
func (c *qrCache) get(accountID, instanceName string) string {
	key := qrKey(accountID, instanceName)
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return ""
	}
	if !c.now().Before(e.expiresAt) {
		delete(c.entries, key)
		return ""
	}
	return e.dataURL
}
