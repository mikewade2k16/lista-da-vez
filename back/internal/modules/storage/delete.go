package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// DeleteObjectClient e opcional: somente adapters que removem objetos o implementam.
// No R2, DeleteObject e operacao gratuita (nao entra em Classe A/B), por isso nao
// reserva cota no ledger.
type DeleteObjectClient interface {
	DeleteObject(ctx context.Context, key string) error
}

// DeletableRepository marca o objeto como deleted depois da remocao no provider.
type DeletableRepository interface {
	MarkDeleted(ctx context.Context, accountID, objectID, sourceModule string) error
}

var ErrDeleteUnsupported = errors.New("object deletion is not supported by this provider")

func (client *R2Client) DeleteObject(ctx context.Context, key string) error {
	_, err := client.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(client.bucket), Key: aws.String(key)})
	return err
}

func (repository *PostgresRepository) MarkDeleted(ctx context.Context, accountID, objectID, sourceModule string) error {
	_, err := repository.pool.Exec(ctx, `
		update storage.objects
		set status = 'deleted', deleted_at = now()
		where id = $1 and account_id = $2::uuid and source_module = $3 and status = 'available'
	`, objectID, accountID, sourceModule)
	return err
}

// DeleteForSource remove definitivamente um objeto disponivel do modulo de origem.
// Escopo (conta + modulo) e validado antes de chamar o provider; objeto pendente
// (entrega em andamento) nao e removido. Ausente/ja removido devolve ErrObjectNotFound.
// So o binario sai do provider: a linha em storage.objects fica como deleted.
func (service *Service) DeleteForSource(ctx context.Context, accountID, objectID, sourceModule string) error {
	if !service.config.Enabled || service.client == nil {
		return ErrDisabled
	}
	deleter, ok := service.client.(DeleteObjectClient)
	repository, okRepository := service.repository.(DeletableRepository)
	if !ok || !okRepository || strings.TrimSpace(sourceModule) == "" {
		return ErrDeleteUnsupported
	}
	state, err := service.repository.ProviderState(ctx)
	if err != nil {
		return err
	}
	if err := service.ensureMatchingProvider(state); err != nil {
		return err
	}
	object, err := service.ObjectMetadata(ctx, accountID, objectID, sourceModule)
	if err != nil {
		return err
	}
	if object.Status != "available" {
		return ErrObjectNotFound
	}
	requestCtx, cancel := context.WithTimeout(ctx, service.config.RequestTimeout)
	defer cancel()
	if err := deleter.DeleteObject(requestCtx, object.ObjectKey); err != nil {
		return fmt.Errorf("R2 DeleteObject: %w", err)
	}
	return repository.MarkDeleted(ctx, object.AccountID, object.ID, object.SourceModule)
}
