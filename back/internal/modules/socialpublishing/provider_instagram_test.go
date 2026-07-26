package socialpublishing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInstagramProviderDefaultsToGraphV24AndPreservesOverride(t *testing.T) {
	provider := NewInstagramGraphProvider("")
	if provider.baseURL != "https://graph.instagram.com/v24.0" {
		t.Fatalf("default baseURL = %q", provider.baseURL)
	}

	override := NewInstagramGraphProvider(" https://graph.example.test/custom/ ")
	if override.baseURL != "https://graph.example.test/custom" {
		t.Fatalf("override baseURL = %q", override.baseURL)
	}
}

func TestInstagramProviderUsesBearerAndPublishesImage(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if strings.Contains(r.URL.RawQuery, "secret-token") {
			t.Fatal("token leaked into URL query")
		}
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/me":
			_, _ = w.Write([]byte(`{"user_id":"ig-1","username":"omni","account_type":"BUSINESS","media_count":7}`))
		case "/ig-1/media":
			_ = r.ParseForm()
			if r.Form.Get("image_url") != "https://cdn.example.com/post.jpg" {
				t.Errorf("image_url = %q", r.Form.Get("image_url"))
			}
			_, _ = w.Write([]byte(`{"id":"creation-1"}`))
		case "/ig-1/media_publish":
			_ = r.ParseForm()
			if r.Form.Get("creation_id") != "creation-1" {
				t.Errorf("creation_id = %q", r.Form.Get("creation_id"))
			}
			_, _ = w.Write([]byte(`{"id":"media-1"}`))
		case "/media-1":
			_, _ = w.Write([]byte(`{"permalink":"https://www.instagram.com/p/example/"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	provider := NewInstagramGraphProvider(server.URL, server.Client())

	profile, err := provider.ValidateToken(context.Background(), "secret-token")
	if err != nil || profile.UserID != "ig-1" {
		t.Fatalf("ValidateToken() = %#v, %v", profile, err)
	}
	creationID, err := provider.CreateImageContainer(
		context.Background(),
		"secret-token",
		"ig-1",
		"https://cdn.example.com/post.jpg",
		"caption",
		"alt",
	)
	if err != nil {
		t.Fatal(err)
	}
	mediaID, err := provider.PublishContainer(context.Background(), "secret-token", "ig-1", creationID)
	if err != nil {
		t.Fatal(err)
	}
	permalink, err := provider.FetchPermalink(context.Background(), "secret-token", mediaID)
	if err != nil || permalink == "" {
		t.Fatalf("FetchPermalink() = %q, %v", permalink, err)
	}
	if len(paths) != 4 {
		t.Fatalf("paths = %v", paths)
	}
}

func TestInstagramInsightsSkipsUnsupportedMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metric := r.URL.Query().Get("metric")
		w.Header().Set("Content-Type", "application/json")
		if metric == "reach" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":100,"message":"unsupported"}}`))
			return
		}
		values := map[string]int64{
			"views": 20, "total_interactions": 9, "likes": 4,
			"comments": 2, "saved": 2, "shares": 1,
		}
		_, _ = fmt.Fprintf(w, `{"data":[{"name":%q,"values":[{"value":%d}]}]}`, metric, values[metric])
	}))
	defer server.Close()
	provider := NewInstagramGraphProvider(server.URL, server.Client())

	analytics, err := provider.FetchMediaInsights(context.Background(), "token", "media-1")
	if err != nil {
		t.Fatalf("FetchMediaInsights() error = %v", err)
	}
	if analytics.Reach != 0 || analytics.Views != 20 || analytics.Saved != 2 ||
		analytics.TotalInteractions != 9 {
		t.Fatalf("analytics = %#v", analytics)
	}
}
