package omnichannel

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBuiltinAIToolRegistryKeepsKnowledgeSearchExplicit(t *testing.T) {
	registry := NewAIToolRegistry()
	registerBuiltinAITools(registry, nil)
	if _, ok := registry.resolve("knowledge.search"); ok {
		t.Fatal("store nil nao pode registrar uma tool executavel")
	}
}

func TestKnowledgeSearchToolRejectsUnknownArguments(t *testing.T) {
	// O handler e exercitado sem banco apenas para garantir que o parser fecha o
	// contrato antes de qualquer consulta; um Store nil nao deve ser chamado para
	// argumentos invalidos.
	registry := NewAIToolRegistry()
	store := &Store{}
	registerBuiltinAITools(registry, store)
	handler, ok := registry.resolve("knowledge.search")
	if !ok {
		t.Fatal("knowledge.search nao registrada")
	}
	_, err := handler(context.Background(), AIToolInvocation{Arguments: json.RawMessage(`{"query":"faq","unexpected":true}`)})
	if err != ErrAIToolArguments {
		t.Fatalf("erro inesperado: %v", err)
	}
}
