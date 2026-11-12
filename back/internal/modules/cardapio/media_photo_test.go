package cardapio

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type fakeEncoder struct {
	fail  bool
	calls []photoVariant
}

func (f *fakeEncoder) EncodeAVIF(_ context.Context, _, dst string, variant photoVariant) error {
	f.calls = append(f.calls, variant)
	if f.fail {
		return errors.New("encoder down")
	}
	return os.WriteFile(dst, []byte("avif"), 0o600)
}

func sampleJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSaveProductPhotoConvertsAndKeepsOriginal(t *testing.T) {
	root := t.TempDir()
	encoder := &fakeEncoder{}
	storage := &DiskMediaStorage{rootDir: root, encoder: encoder}

	photo, err := storage.SaveProductPhoto(context.Background(), "acc-1", "prato.jpg", "image/jpeg", sampleJPEG(t))
	if err != nil {
		t.Fatalf("SaveProductPhoto: %v", err)
	}
	if !photo.Converted || !strings.HasSuffix(photo.URL, ".avif") || !strings.HasSuffix(photo.ThumbURL, ".thumb.avif") {
		t.Fatalf("esperava versoes AVIF, veio %+v", photo)
	}
	if !strings.Contains(photo.OriginalURL, ".orig.jpg") {
		t.Fatalf("original nao preservado: %s", photo.OriginalURL)
	}
	for _, url := range []string{photo.URL, photo.ThumbURL, photo.OriginalURL} {
		if _, err := os.Stat(filepath.Join(root, strings.TrimPrefix(url, "/uploads/"))); err != nil {
			t.Fatalf("arquivo ausente %s: %v", url, err)
		}
	}
	if len(encoder.calls) != 2 || encoder.calls[0] != photoVariantThumb {
		t.Fatalf("miniatura deve ser gerada antes do site: %v", encoder.calls)
	}
}

func TestSaveProductPhotoFallsBackToOriginalWhenConversionFails(t *testing.T) {
	storage := &DiskMediaStorage{rootDir: t.TempDir(), encoder: &fakeEncoder{fail: true}}
	photo, err := storage.SaveProductPhoto(context.Background(), "acc-1", "prato.jpg", "image/jpeg", sampleJPEG(t))
	if err != nil {
		t.Fatalf("falha de conversao nao pode derrubar o upload: %v", err)
	}
	if photo.Converted || photo.ThumbURL != "" || photo.URL != photo.OriginalURL {
		t.Fatalf("esperava fallback para o original, veio %+v", photo)
	}
}

func TestSaveProductPhotoRejectsNonImagesAndOversize(t *testing.T) {
	storage := &DiskMediaStorage{rootDir: t.TempDir(), encoder: &fakeEncoder{}}
	ctx := context.Background()
	cases := map[string][]byte{
		"texto com header de imagem": []byte("<html>nao e imagem</html>"),
		"video":                      append([]byte{0, 0, 0, 0x18}, []byte("ftypmp42")...),
		"acima de 20MB":              append(sampleJPEG(t), make([]byte, maxProductPhotoBytes)...),
	}
	for name, content := range cases {
		if _, err := storage.SaveProductPhoto(ctx, "acc-1", "x.jpg", "image/jpeg", content); !errors.Is(err, ErrInvalidMedia) {
			t.Errorf("%s: esperava ErrInvalidMedia, veio %v", name, err)
		}
	}
}

func TestBackfillHelpers(t *testing.T) {
	site, thumb := backfillTargets("/uploads/cardapio/acc/abc.webp")
	if site != "/uploads/cardapio/acc/abc.avif" || thumb != "/uploads/cardapio/acc/abc.thumb.avif" {
		t.Fatalf("targets inesperados: %s %s", site, thumb)
	}
	if site, _ := backfillTargets("/uploads/cardapio/acc/abc.orig.png"); site != "/uploads/cardapio/acc/abc.avif" {
		t.Fatalf("original de upload deve mapear para a versao do site: %s", site)
	}
	for url, want := range map[string]bool{
		"/uploads/cardapio/acc/a.jpg":     true,
		"/uploads/cardapio/acc/a.webp":    true,
		"/uploads/cardapio/acc/a.avif":    false,
		"/uploads/cardapio/acc/a.gif":     false,
		"/uploads/bio/acc/a.jpg":          false,
		"https://cdn.exemplo/a.jpg":       false,
		"/uploads/cardapio/../../etc.jpg": false,
	} {
		if got := convertibleUploadURL(url); got != want {
			t.Errorf("convertibleUploadURL(%q) = %v, want %v", url, got, want)
		}
	}
	if _, ok := uploadPath("/data/uploads", "/uploads/cardapio/acc/a.jpg"); !ok {
		t.Fatal("caminho valido rejeitado")
	}
	if _, ok := uploadPath("/data/uploads", "/uploads/outro/a.jpg"); ok {
		t.Fatal("caminho fora de uploads/cardapio aceito")
	}
}

type fakeMembers struct{ member bool }

func (f fakeMembers) IsMember(context.Context, string, string) (bool, error) { return f.member, nil }

func TestMemberAccountForOnlyCoversUsersWithoutAccount(t *testing.T) {
	ctx := context.Background()
	member := &Service{members: fakeMembers{member: true}}
	stranger := &Service{members: fakeMembers{member: false}}
	noAccount := auth.Principal{UserID: "u1"}

	if got := member.memberAccountFor(ctx, noAccount, "acc-bari"); got != "acc-bari" {
		t.Fatalf("membro sem conta padrao deveria usar a account do header, veio %q", got)
	}
	if got := stranger.memberAccountFor(ctx, noAccount, "acc-bari"); got != "" {
		t.Fatalf("nao-membro nunca recebe escopo, veio %q", got)
	}
	if got := member.memberAccountFor(ctx, noAccount, ""); got != "" {
		t.Fatalf("sem header nao ha escopo, veio %q", got)
	}
	withTenant := auth.Principal{UserID: "u1", TenantID: "acc-propria"}
	if got := member.memberAccountFor(ctx, withTenant, "acc-bari"); got != "" {
		t.Fatalf("usuario com conta propria segue o fluxo antigo, veio %q", got)
	}
	admin := auth.Principal{UserID: "u1", Role: auth.RolePlatformAdmin}
	if got := member.memberAccountFor(ctx, admin, "acc-bari"); got != "" {
		t.Fatalf("platform_admin segue o fluxo antigo, veio %q", got)
	}
	if got := (&Service{}).memberAccountFor(ctx, noAccount, "acc-bari"); got != "" {
		t.Fatalf("sem checker o recurso fica desligado, veio %q", got)
	}
}
