package metaads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestMetaClientCreateCampaignActionUsesBearerAndPausedForm(t *testing.T) {
	t.Parallel()

	const token = "create-campaign-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/act_123456/campaigns" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		assertMetaActionBearer(t, r, token)
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if r.Form.Get("name") != "Campanha segura" || r.Form.Get("status") != "PAUSED" {
			t.Errorf("form = %#v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "987654321"})
	}))
	defer server.Close()

	client := NewMetaClient(server.URL)
	result, err := client.CreateCampaignAction(context.Background(), token, "123456", url.Values{
		"name":   {"Campanha segura"},
		"status": {"PAUSED"},
	})
	if err != nil {
		t.Fatalf("CreateCampaignAction: %v", err)
	}
	if result.ID != "987654321" {
		t.Fatalf("ID = %q", result.ID)
	}
}

func TestMetaClientCopyCampaignActionUsesServerSidePathAndPausedForm(t *testing.T) {
	t.Parallel()

	const token = "copy-campaign-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/7654321/copies" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		assertMetaActionBearer(t, r, token)
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if r.Form.Get("deep_copy") != "true" || r.Form.Get("status_option") != "PAUSED" {
			t.Errorf("form = %#v", r.Form)
		}
		if strings.Contains(r.URL.RawQuery, "access_token") {
			t.Errorf("token leaked in URL: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"copied_campaign_id": "11223344"})
	}))
	defer server.Close()

	client := NewMetaClient(server.URL)
	result, err := client.CopyCampaignAction(context.Background(), token, "7654321", url.Values{
		"deep_copy":     {"true"},
		"status_option": {"PAUSED"},
	})
	if err != nil {
		t.Fatalf("CopyCampaignAction: %v", err)
	}
	if result.CopiedCampaignID != "11223344" {
		t.Fatalf("CopiedCampaignID = %q", result.CopiedCampaignID)
	}
}

func TestMetaClientCreatesInstagramAdTreeWithBearerForms(t *testing.T) {
	t.Parallel()

	const token = "instagram-tree-secret"
	paths := []string{"/act_123456/adsets", "/act_123456/adcreatives", "/act_123456/ads"}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests >= len(paths) || r.Method != http.MethodPost || r.URL.Path != paths[requests] {
			t.Fatalf("request %d = %s %s", requests, r.Method, r.URL.Path)
		}
		assertMetaActionBearer(t, r, token)
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if strings.Contains(r.URL.RawQuery, "access_token") {
			t.Fatalf("token leaked in URL: %s", r.URL.String())
		}
		switch requests {
		case 0:
			if r.Form.Get("status") != "PAUSED" ||
				!strings.Contains(r.Form.Get("targeting"), `"publisher_platforms":["instagram"]`) {
				t.Fatalf("ad set form = %#v", r.Form)
			}
		case 1:
			if r.Form.Get("object_id") != "55667788" ||
				r.Form.Get("instagram_user_id") != "66778899" ||
				r.Form.Get("source_instagram_media_id") != "77889900" ||
				r.Form.Has("page_id") {
				t.Fatalf("creative form = %#v", r.Form)
			}
		case 2:
			if r.Form.Get("status") != "PAUSED" ||
				!strings.Contains(r.Form.Get("creative"), `"creative_id":"98765432"`) {
				t.Fatalf("ad form = %#v", r.Form)
			}
		}
		requests++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "98765432"})
	}))
	defer server.Close()

	client := NewMetaClient(server.URL)
	if _, err := client.CreateAdSetAction(context.Background(), token, "123456", url.Values{
		"status": {"PAUSED"}, "daily_budget": {"2500"},
		"targeting": {`{"publisher_platforms":["instagram"]}`},
	}); err != nil {
		t.Fatalf("CreateAdSetAction: %v", err)
	}
	if _, err := client.CreateAdCreativeAction(context.Background(), token, "123456", url.Values{
		"object_id":                 {"55667788"},
		"instagram_user_id":         {"66778899"},
		"source_instagram_media_id": {"77889900"},
	}); err != nil {
		t.Fatalf("CreateAdCreativeAction: %v", err)
	}
	if _, err := client.CreateAdAction(context.Background(), token, "123456", url.Values{
		"status":   {"PAUSED"},
		"creative": {`{"creative_id":"98765432"}`},
	}); err != nil {
		t.Fatalf("CreateAdAction: %v", err)
	}
	if requests != len(paths) {
		t.Fatalf("requests = %d", requests)
	}
}

func assertMetaActionBearer(t *testing.T, r *http.Request, token string) {
	t.Helper()
	if strings.Contains(r.URL.String(), token) || r.URL.Query().Get("access_token") != "" {
		t.Errorf("token leaked in URL: %s", r.URL.String())
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+token {
		t.Errorf("Authorization = %q", got)
	}
	if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-www-form-urlencoded") {
		t.Errorf("Content-Type = %q", got)
	}
}
