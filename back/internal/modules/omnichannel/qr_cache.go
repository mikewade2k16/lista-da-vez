package omnichannel

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// qrCache guarda o QR (data URL) por (accountID, instanceName) com TTL curto. O modo local fica em
// MEMORIA (o go.mod nao tem Redis — spec C4): consequencia registrada em docs/LEGADO.md —
// O modo local nao sobrevive a restart; newSharedQRCache usa PostgreSQL entre replicas.
type qrCache struct {
	mu      sync.Mutex
	entries map[string]qrEntry
	ttl     time.Duration
	now     func() time.Time
	pool    *pgxpool.Pool
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

func newSharedQRCache(pool *pgxpool.Pool) *qrCache {
	cache := newQRCache()
	cache.pool = pool
	return cache
}

func qrKey(accountID, instanceName string) string {
	return accountID + "|" + instanceName
}

// set guarda o QR. dataURL vazio limpa a entrada (ex.: instancia ja conectou).
func (c *qrCache) set(accountID, instanceName, dataURL string) {
	if c.pool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var err error
		if dataURL == "" {
			_, err = c.pool.Exec(ctx, `delete from messaging.runtime_qr_cache
				where account_id=$1::uuid and instance_name=$2`, accountID, instanceName)
		} else {
			_, err = c.pool.Exec(ctx, `insert into messaging.runtime_qr_cache
				(account_id,instance_name,data_url,expires_at)
				values ($1::uuid,$2,$3,$4)
				on conflict(account_id,instance_name) do update set
				 data_url=excluded.data_url,expires_at=excluded.expires_at,updated_at=now()`,
				accountID, instanceName, dataURL, c.now().Add(c.ttl))
		}
		if err == nil {
			c.deleteLocal(accountID, instanceName)
			return
		}
	}
	c.setLocal(accountID, instanceName, dataURL)
}

func (c *qrCache) setLocal(accountID, instanceName, dataURL string) {
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
	if c.pool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var dataURL string
		err := c.pool.QueryRow(ctx, `select data_url from messaging.runtime_qr_cache
			where account_id=$1::uuid and instance_name=$2 and expires_at>$3`,
			accountID, instanceName, c.now()).Scan(&dataURL)
		if err == nil {
			return dataURL
		}
		if errors.Is(err, pgx.ErrNoRows) {
			_, _ = c.pool.Exec(ctx, `delete from messaging.runtime_qr_cache
				where account_id=$1::uuid and instance_name=$2 and expires_at<=$3`,
				accountID, instanceName, c.now())
			return ""
		}
	}
	return c.getLocal(accountID, instanceName)
}

func (c *qrCache) getLocal(accountID, instanceName string) string {
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

func (c *qrCache) deleteLocal(accountID, instanceName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, qrKey(accountID, instanceName))
}
