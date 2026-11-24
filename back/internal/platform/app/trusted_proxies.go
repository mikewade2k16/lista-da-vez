package app

import (
	"fmt"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// configureTrustedProxies aplica TRUSTED_PROXIES: vazio = padrao (redes privadas/Docker);
// "none" = nenhum proxy confiavel; senao a lista de CIDRs/IPs informada.
func configureTrustedProxies(values []string) error {
	if len(values) == 0 {
		values = httpapi.DefaultTrustedProxies
	}
	if len(values) == 1 && strings.EqualFold(strings.TrimSpace(values[0]), "none") {
		httpapi.SetTrustedProxies(nil)
		return nil
	}
	nets, err := httpapi.ParseTrustedProxies(values)
	if err != nil {
		return fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}
	httpapi.SetTrustedProxies(nets)
	return nil
}
