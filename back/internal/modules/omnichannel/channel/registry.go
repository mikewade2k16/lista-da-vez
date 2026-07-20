package channel

import (
	"fmt"
	"sort"
	"sync"
)

// Registry resolve um Provider pela chave (= whatsapp_instances.provider). Os providers
// sao SINGLETONS stateless (a credencial e o estado vem por chamada, via Credentials), o
// que os torna seguros para uso concorrente e imunes a ordem de registro.
//
// Ponto unico de plug: registrar um provider novo (evolution, waha, meta_whatsapp_cloud)
// e uma linha em NewRegistry — o dominio e o webhook nao mudam.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry cria o registry com os providers passados ja registrados. Providers com ID
// duplicado sao um erro de programacao (panica no boot, nao silenciosamente).
func NewRegistry(providers ...Provider) *Registry {
	r := &Registry{providers: make(map[string]Provider, len(providers))}
	for _, p := range providers {
		r.mustRegister(p)
	}
	return r
}

func (r *Registry) mustRegister(p Provider) {
	id := p.ID()
	if id == "" {
		panic("channel: provider com ID vazio")
	}
	if _, exists := r.providers[id]; exists {
		panic(fmt.Sprintf("channel: provider %q registrado duas vezes", id))
	}
	r.providers[id] = p
}

// Get resolve o Provider pela chave. Chave desconhecida => erro nomeando a chave (feedback
// acionavel — principio 5): o operador cadastrou um provider sem adapter registrado.
func (r *Registry) Get(providerKey string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[providerKey]
	if !ok {
		return nil, fmt.Errorf("channel: provider %q sem adapter registrado (registrados: %v)",
			providerKey, r.idsLocked())
	}
	return p, nil
}

// Session resolve o Provider e faz o type-assert para SessionManager. Provider sem ciclo
// de sessao (ex.: um futuro provider so-de-envio) => erro claro, nunca panica.
func (r *Registry) Session(providerKey string) (SessionManager, error) {
	p, err := r.Get(providerKey)
	if err != nil {
		return nil, err
	}
	sm, ok := p.(SessionManager)
	if !ok {
		return nil, fmt.Errorf("channel: provider %q nao suporta ciclo de sessao/QR", providerKey)
	}
	return sm, nil
}

// Has diz se ha adapter para a chave (usado na validacao de cadastro de instancia).
func (r *Registry) Has(providerKey string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.providers[providerKey]
	return ok
}

// IDs devolve as chaves registradas (ordenadas), para diagnostico e para a UI listar os
// providers disponiveis.
func (r *Registry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.idsLocked()
}

func (r *Registry) idsLocked() []string {
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
