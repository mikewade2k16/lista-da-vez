package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type promptPublishRepositoryFake struct {
	PromptRepository
	prompt      PromptVersion
	evaluations []PromptEvaluation
	published   bool
}

func (f *promptPublishRepositoryFake) GetPromptVersion(
	context.Context,
	string,
	string,
) (PromptVersion, error) {
	return f.prompt, nil
}

func (f *promptPublishRepositoryFake) ListPromptEvaluations(
	context.Context,
	string,
	string,
	int,
) ([]PromptEvaluation, error) {
	return append([]PromptEvaluation(nil), f.evaluations...), nil
}

func (f *promptPublishRepositoryFake) AgentVersionClientScope(
	context.Context,
	string,
	string,
) (string, error) {
	return f.prompt.ClientAccountID, nil
}

func (f *promptPublishRepositoryFake) PublishPrompt(
	_ context.Context,
	accountID, _ string,
	_ string,
	input PublishPromptInput,
) (PromptBinding, error) {
	f.published = true
	return PromptBinding{
		ID:              headlessTestCapability,
		AccountID:       accountID,
		ClientAccountID: input.ClientAccountID,
	}, nil
}

func promptPublishService(
	repository PromptRepository,
) *Service {
	return NewServiceWithRepositories(
		nil,
		repository,
		nil,
		nil,
		nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
	)
}

func validPromptPublishInput() PublishPromptInput {
	return PublishPromptInput{
		AgentVersionID:  "77777777-7777-4777-8777-777777777777",
		SourcePolicy:    json.RawMessage(`[]`),
		ToolPolicy:      json.RawMessage(`[]`),
		KnowledgePolicy: json.RawMessage(`[]`),
		RuntimePolicy:   json.RawMessage(`{}`),
	}
}

func TestPublishPromptRequiresPassedEvaluationAfterValidation(t *testing.T) {
	t.Parallel()
	validatedAt := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name        string
		evaluations []PromptEvaluation
	}{
		{name: "missing"},
		{
			name: "failed",
			evaluations: []PromptEvaluation{{
				Status: "failed", CreatedAt: validatedAt.Add(time.Second),
			}},
		},
		{
			name: "stale",
			evaluations: []PromptEvaluation{{
				Status: "passed", CreatedAt: validatedAt.Add(-time.Second),
			}},
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &promptPublishRepositoryFake{
				prompt: PromptVersion{
					ID: headlessTestCapability, AccountID: headlessTestAccount,
					ClientAccountID: headlessTestClient, Status: "validated",
					ValidatedAt: &validatedAt,
				},
				evaluations: test.evaluations,
			}
			_, err := promptPublishService(repository).PublishPromptVersion(
				context.Background(),
				headlessTestAccount,
				headlessTestActor,
				headlessTestCapability,
				validPromptPublishInput(),
			)
			if !errors.Is(err, ErrPromptEvaluationRequired) || repository.published {
				t.Fatalf("publish inseguro: err=%v published=%v", err, repository.published)
			}
		})
	}
}

func TestPublishPromptAcceptsCurrentPassedEvaluation(t *testing.T) {
	t.Parallel()
	validatedAt := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	repository := &promptPublishRepositoryFake{
		prompt: PromptVersion{
			ID: headlessTestCapability, AccountID: headlessTestAccount,
			ClientAccountID: headlessTestClient, Status: "validated",
			ValidatedAt: &validatedAt,
		},
		evaluations: []PromptEvaluation{{
			Status: "passed", CreatedAt: validatedAt.Add(time.Second),
		}},
	}
	_, err := promptPublishService(repository).PublishPromptVersion(
		context.Background(),
		headlessTestAccount,
		headlessTestActor,
		headlessTestCapability,
		validPromptPublishInput(),
	)
	if err != nil || !repository.published {
		t.Fatalf("avaliacao atual nao publicou: err=%v", err)
	}
}
