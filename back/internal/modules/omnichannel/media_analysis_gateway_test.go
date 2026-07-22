package omnichannel

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

func TestIssueMediaStreamTokenIsOpaqueAndBound(t *testing.T) {
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	account := "00000000-0000-0000-0000-000000000001"
	message := "00000000-0000-0000-0000-000000000002"
	analysis := "00000000-0000-0000-0000-000000000003"
	token, err := IssueMediaStreamToken(box, account, message, analysis, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || token == account || token == message || token == analysis {
		t.Fatalf("token leaked identity: %q", token)
	}
	plaintext, err := box.Decrypt(token)
	if err != nil {
		t.Fatal(err)
	}
	var claims mediaStreamClaims
	if err := json.Unmarshal([]byte(plaintext), &claims); err != nil {
		t.Fatal(err)
	}
	if claims.AccountID != account || claims.MessageID != message || claims.AnalysisID != analysis || claims.Version != "media-stream.v1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestIssueMediaStreamTokenRejectsInvalidScope(t *testing.T) {
	box, _ := secretbox.New(make([]byte, 32))
	if _, err := IssueMediaStreamToken(box, "bad", "00000000-0000-0000-0000-000000000002", "00000000-0000-0000-0000-000000000003", time.Minute); err == nil {
		t.Fatal("invalid scope accepted")
	}
}
