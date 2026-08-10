package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const cloudflareAPIBaseURL = "https://api.cloudflare.com/client/v4"

type UsageClient interface {
	Usage(ctx context.Context, start, end time.Time) (CloudUsage, error)
}

type CloudflareUsageClient struct {
	accountID string
	token     string
	client    *http.Client
}

func NewCloudflareUsageClient(cfg Config) *CloudflareUsageClient {
	return &CloudflareUsageClient{
		accountID: strings.TrimSpace(cfg.AccountID),
		token:     strings.TrimSpace(cfg.AnalyticsToken),
		client:    &http.Client{Timeout: cfg.RequestTimeout},
	}
}

func (client *CloudflareUsageClient) Usage(ctx context.Context, start, end time.Time) (CloudUsage, error) {
	if client == nil || client.token == "" || client.accountID == "" {
		return CloudUsage{}, ErrAnalyticsUnavailable
	}
	type storageResult struct {
		bytes, metadataBytes, objects int64
		err                           error
	}
	type operationResult struct {
		classA, classB int64
		err            error
	}
	storageResults := make(chan storageResult, 1)
	operationResults := make(chan operationResult, 1)
	go func() {
		bytes, metadataBytes, objects, err := client.storageMetrics(ctx)
		storageResults <- storageResult{bytes: bytes, metadataBytes: metadataBytes, objects: objects, err: err}
	}()
	go func() {
		classA, classB, err := client.operationMetrics(ctx, start, end)
		operationResults <- operationResult{classA: classA, classB: classB, err: err}
	}()
	storage := <-storageResults
	operations := <-operationResults
	if storage.err != nil {
		return CloudUsage{}, storage.err
	}
	if operations.err != nil {
		return CloudUsage{}, operations.err
	}
	return CloudUsage{
		Available: true, Configured: true, Source: "cloudflare_account",
		WindowStart: start.UTC(), WindowEnd: end.UTC(), FetchedAt: time.Now().UTC(),
		StoredBytes: storage.bytes, MetadataBytes: storage.metadataBytes, ObjectCount: storage.objects,
		ClassARequests: operations.classA, ClassBRequests: operations.classB,
	}, nil
}

func (client *CloudflareUsageClient) storageMetrics(ctx context.Context) (int64, int64, int64, error) {
	if !cloudflareAccountPattern.MatchString(client.accountID) {
		return 0, 0, 0, ErrInvalidConfig
	}
	// #nosec G704 -- scheme/host are a compile-time Cloudflare endpoint and accountID is restricted to 32 hex characters.
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/accounts/%s/r2/metrics", cloudflareAPIBaseURL, client.accountID), nil)
	if err != nil {
		return 0, 0, 0, err
	}
	client.authorize(request)
	// #nosec G704 -- request host is the compile-time Cloudflare API endpoint validated above.
	response, err := client.client.Do(request)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("cloudflare account metrics: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	var envelope struct {
		Success bool `json:"success"`
		Result  map[string]map[string]struct {
			PayloadSize  int64 `json:"payloadSize"`
			MetadataSize int64 `json:"metadataSize"`
			Objects      int64 `json:"objects"`
		} `json:"result"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&envelope) != nil || !envelope.Success {
		return 0, 0, 0, ErrAnalyticsUnavailable
	}
	var payloadBytes, metadataBytes, totalObjects int64
	for _, class := range envelope.Result {
		for _, state := range class {
			payloadBytes += state.PayloadSize
			metadataBytes += state.MetadataSize
			totalObjects += state.Objects
		}
	}
	return payloadBytes, metadataBytes, totalObjects, nil
}

func (client *CloudflareUsageClient) operationMetrics(ctx context.Context, start, end time.Time) (int64, int64, error) {
	const query = `query R2AccountOperations($accountTag: string!, $startDate: Time!, $endDate: Time!) {
  viewer { accounts(filter: { accountTag: $accountTag }) {
    r2OperationsAdaptiveGroups(limit: 10000, filter: { datetime_geq: $startDate, datetime_leq: $endDate }) {
      sum { requests } dimensions { actionType }
    }
  } }
}`
	body, err := json.Marshal(map[string]any{
		"query": query,
		"variables": map[string]any{
			"accountTag": client.accountID,
			"startDate":  start.UTC().Format(time.RFC3339),
			"endDate":    end.UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		return 0, 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, cloudflareAPIBaseURL+"/graphql", bytes.NewReader(body))
	if err != nil {
		return 0, 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	client.authorize(request)
	response, err := client.client.Do(request)
	if err != nil {
		return 0, 0, fmt.Errorf("cloudflare GraphQL metrics: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	var envelope struct {
		Errors []json.RawMessage `json:"errors"`
		Data   struct {
			Viewer struct {
				Accounts []struct {
					Groups []struct {
						Sum struct {
							Requests int64 `json:"requests"`
						} `json:"sum"`
						Dimensions struct {
							ActionType string `json:"actionType"`
						} `json:"dimensions"`
					} `json:"r2OperationsAdaptiveGroups"`
				} `json:"accounts"`
			} `json:"viewer"`
		} `json:"data"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&envelope) != nil || len(envelope.Errors) > 0 {
		return 0, 0, ErrAnalyticsUnavailable
	}
	var classA, classB int64
	for _, account := range envelope.Data.Viewer.Accounts {
		for _, group := range account.Groups {
			switch r2OperationClass(group.Dimensions.ActionType) {
			case "A":
				classA += group.Sum.Requests
			case "B":
				classB += group.Sum.Requests
			}
		}
	}
	return classA, classB, nil
}

func (client *CloudflareUsageClient) authorize(request *http.Request) {
	request.Header.Set("Authorization", "Bearer "+client.token)
}

func r2OperationClass(action string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	if _, ok := classAActions[action]; ok {
		return "A"
	}
	if _, ok := classBActions[action]; ok {
		return "B"
	}
	return ""
}

var classAActions = map[string]struct{}{
	"listbuckets": {}, "putbucket": {}, "listobjects": {}, "listobjectsv2": {},
	"putobject": {}, "copyobject": {}, "completemultipartupload": {},
	"createmultipartupload": {}, "lifecyclestoragetiertransition": {},
	"listmultipartuploads": {}, "uploadpart": {}, "uploadpartcopy": {}, "listparts": {},
	"putbucketencryption": {}, "putbucketcors": {}, "putbucketlifecycleconfiguration": {},
}

var classBActions = map[string]struct{}{
	"headbucket": {}, "headobject": {}, "getobject": {}, "usagesummary": {},
	"getbucketencryption": {}, "getbucketlocation": {}, "getbucketcors": {},
	"getbucketlifecycleconfiguration": {},
}
