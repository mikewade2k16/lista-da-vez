package socialpublishing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

func TestConnectionWithoutRecordReturnsDisconnected(t *testing.T) {
	repository := &stubRepository{connectionErr: ErrNotConnected}
	service := NewService(repository, &stubProvider{}, nil, nil)

	connection, err := service.Connection(context.Background(), "account-1")
	if err != nil {
		t.Fatalf("Connection() error = %v", err)
	}
	if connection.Status != "disconnected" || connection.Provider != "instagram" {
		t.Fatalf("Connection() = %#v, want safe disconnected state", connection)
	}
	if connection.Secret.Set {
		t.Fatal("disconnected connection must not expose a configured secret")
	}
}

func TestConnectValidatesAndEncryptsToken(t *testing.T) {
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repository := &stubRepository{}
	provider := &stubProvider{profile: InstagramProfile{
		UserID: "ig-1", Username: "omni", AccountType: "BUSINESS",
	}}
	service := NewService(repository, provider, box, nil)

	connection, err := service.Connect(
		context.Background(),
		"account-1",
		"user-1",
		"token-super-secret-1234",
	)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if provider.validatedWith != "token-super-secret-1234" {
		t.Fatal("provider did not receive the transient token")
	}
	if repository.savedCiphertext == "" ||
		repository.savedCiphertext == "token-super-secret-1234" {
		t.Fatal("repository must receive ciphertext, never plaintext")
	}
	if repository.savedLast4 != "1234" {
		t.Fatalf("last4 = %q, want 1234", repository.savedLast4)
	}
	if !connection.Secret.Set || connection.Secret.Last4 != "1234" {
		t.Fatalf("secret status = %#v, want masked last4", connection.Secret)
	}
}

func TestCreateScheduledPostNormalizesUTCAndConnection(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	when := time.Date(2026, 7, 23, 12, 30, 0, 0, time.FixedZone("BRT", -3*60*60))
	repository := &stubRepository{connection: ConnectionRecord{
		Connection:            Connection{ID: "connection-1", Status: "connected"},
		AccessTokenCiphertext: "v1:ciphertext",
	}}
	service := NewService(
		repository,
		&stubProvider{},
		nil,
		nil,
		WithServiceClock(func() time.Time { return now }),
	)

	result, err := service.CreatePost(context.Background(), "account-1", "user-1", CreatePostInput{
		Caption:      "  campanha  ",
		MediaURL:     "https://cdn.example.com/post.jpg",
		MediaType:    "image",
		Status:       PostStatusScheduled,
		ScheduledFor: &when,
		Timezone:     "America/Sao_Paulo",
	})
	if err != nil {
		t.Fatalf("CreatePost() error = %v", err)
	}
	if !result.Created || repository.createCommand.ConnectionID != "connection-1" {
		t.Fatalf("CreatePost() command = %#v", repository.createCommand)
	}
	if got := repository.createCommand.Input.ScheduledFor; got == nil ||
		!got.Equal(when.UTC()) || got.Location() != time.UTC {
		t.Fatalf("scheduledFor = %v, want UTC %v", got, when.UTC())
	}
	if repository.createCommand.Input.Caption != "campanha" {
		t.Fatalf("caption = %q, want trimmed", repository.createCommand.Input.Caption)
	}
}

func TestPatchScheduledPostIsAllowedAndReturnsDraft(t *testing.T) {
	scheduled := time.Now().UTC().Add(time.Hour)
	repository := &stubRepository{post: Post{
		ID:           "post-1",
		Status:       PostStatusScheduled,
		Version:      3,
		Caption:      "antes",
		MediaURL:     "https://cdn.example.com/post.jpg",
		ScheduledFor: &scheduled,
		Timezone:     "UTC",
	}}
	service := NewService(repository, &stubProvider{}, nil, nil)
	caption := "depois"

	post, err := service.PatchPost(
		context.Background(),
		"account-1",
		"user-1",
		"post-1",
		PatchPostInput{Caption: &caption, Version: 3},
	)
	if err != nil {
		t.Fatalf("PatchPost() error = %v", err)
	}
	if post.Status != PostStatusDraft || post.ScheduledFor != nil {
		t.Fatalf("PatchPost() = %#v, want draft without schedule", post)
	}
}

func TestCreateRejectsNonHTTPSMedia(t *testing.T) {
	service := NewService(&stubRepository{}, &stubProvider{}, nil, nil)
	_, err := service.CreatePost(context.Background(), "account-1", "user-1", CreatePostInput{
		MediaURL:  "http://cdn.example.com/post.jpg",
		MediaType: "image",
		Status:    PostStatusDraft,
	})
	if !errors.Is(err, ErrInvalidMediaURL) {
		t.Fatalf("CreatePost() error = %v, want ErrInvalidMediaURL", err)
	}
}
