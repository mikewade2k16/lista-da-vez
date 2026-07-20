package omnichannel

import (
	"context"
	"fmt"
	"strings"
)

// "Um numero, uma instancia" — validacao INTERNA (canonico §2, C6): o mesmo numero nao
// entra em duas instancias DA MESMA CONTA. Escopo da checagem: SO
// messaging.whatsapp_instances da propria conta — nenhuma leitura de outro modulo, nenhum
// sistema externo (o modulo e independente por construcao). A garantia final e do banco
// (indice unico parcial (account_id, phone_number) da 0201); esta checagem e UX: devolve
// erro acionavel nomeando a instancia que ja usa o numero.

// NumberInUseError carrega qual instancia ja usa o numero, para o handler montar o 409
// acionavel (principio 5). Unwrap para ErrNumberInUse (errors.Is continua funcionando).
type NumberInUseError struct {
	InstanceName string
}

func (e *NumberInUseError) Error() string {
	return fmt.Sprintf("numero ja usado pela instancia %q nesta conta", e.InstanceName)
}

func (e *NumberInUseError) Unwrap() error { return ErrNumberInUse }

// ensureNumberFree falha com *NumberInUseError quando outra instancia da conta ja usa o
// numero. excludeInstanceID e a propria instancia (ignorada para nao acusar colisao
// consigo mesma). Numero vazio nao valida nada (so resolve depois de conectar).
func ensureNumberFree(ctx context.Context, store *Store, accountID, phone, excludeInstanceID string) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil
	}
	name, found, err := store.FindInstanceUsingPhone(ctx, accountID, phone, excludeInstanceID)
	if err != nil {
		return err
	}
	if found {
		return &NumberInUseError{InstanceName: name}
	}
	return nil
}
