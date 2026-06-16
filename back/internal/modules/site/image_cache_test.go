package site

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// TestImageCacheDownloadsAndRewrites cobre o caminho feliz: baixa a imagem
// externa, grava em /uploads/site/products/{account}/ e reescreve item.Image
// para o path local. Idempotente: segunda passada NAO baixa de novo.
func TestImageCacheDownloadsAndRewrites(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("RIFF....WEBPfake-bytes"))
	}))
	defer srv.Close()

	root := t.TempDir()
	cache := NewImageCache(root)

	items := []ProductUpsertItem{
		{ExternalID: "1", Source: "perola", Image: srv.URL + "/assets/images/products/ouro/0011795.webp"},
		{ExternalID: "2", Source: "perola", Image: ""},                 // sem imagem: ignorado
		{ExternalID: "3", Source: "perola", Image: "/uploads/ja.webp"}, // ja local: ignorado
	}

	cached := cache.CacheItems(context.Background(), "acc-1", items)
	if cached != 1 {
		t.Fatalf("cached = %d, want 1", cached)
	}
	if !strings.HasPrefix(items[0].Image, "/uploads/site/products/acc-1/") {
		t.Fatalf("item[0].Image nao reescrito: %q", items[0].Image)
	}
	if !strings.HasSuffix(items[0].Image, ".webp") {
		t.Fatalf("item[0].Image deveria manter a extensao .webp: %q", items[0].Image)
	}
	if items[1].Image != "" || items[2].Image != "/uploads/ja.webp" {
		t.Fatalf("itens sem/com imagem local nao deveriam mudar: %q %q", items[1].Image, items[2].Image)
	}
	// arquivo gravado em disco
	rel := strings.TrimPrefix(items[0].Image, "/uploads/")
	if _, err := os.Stat(root + "/" + rel); err != nil {
		t.Fatalf("arquivo nao gravado: %v", err)
	}

	// Idempotencia: re-rodar com a MESMA URL externa nao baixa de novo (o item
	// volta ao estado externo para simular o proximo sync, que sempre traz a URL
	// da origem).
	items2 := []ProductUpsertItem{{ExternalID: "1", Image: srv.URL + "/assets/images/products/ouro/0011795.webp"}}
	if c := cache.CacheItems(context.Background(), "acc-1", items2); c != 1 {
		t.Fatalf("segunda passada cached = %d, want 1 (resolve do cache)", c)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("downloads = %d, want 1 (segunda passada usa o cache em disco)", hits)
	}
}

// TestImageCacheFailureKeepsExternal: origem fora (404) MANTEM a URL externa
// (fallback) e nao conta como cacheada — nunca quebra o sync.
func TestImageCacheFailureKeepsExternal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cache := NewImageCache(t.TempDir())
	external := srv.URL + "/assets/images/products/0011795.webp"
	items := []ProductUpsertItem{{ExternalID: "1", Image: external}}

	cached := cache.CacheItems(context.Background(), "acc-1", items)
	if cached != 0 {
		t.Fatalf("cached = %d, want 0 (404)", cached)
	}
	if items[0].Image != external {
		t.Fatalf("URL externa deveria ser mantida como fallback, got %q", items[0].Image)
	}
}

// TestImageCacheTriesCandidates: com ImageCandidates, o cache pula a 1a URL que
// da 404 e fica com a 1a que responde 200 (a thumb _sm.avif). Reproduz o caso
// real da Perola, onde o nome cru .webp nao existe mas a variante _sm.avif sim.
func TestImageCacheTriesCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "_sm.avif") {
			w.Header().Set("Content-Type", "image/avif")
			_, _ = w.Write([]byte("avif-bytes"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cache := NewImageCache(t.TempDir())
	items := []ProductUpsertItem{{
		ExternalID: "1", Source: "perola",
		Image: srv.URL + "/assets/images/products/default/0278091.webp",
		ImageCandidates: []string{
			srv.URL + "/assets/images/products/default/0278091.webp",    // 404
			srv.URL + "/assets/images/products/default/0278091_sm.avif", // 200
		},
	}}

	cached := cache.CacheItems(context.Background(), "acc-1", items)
	if cached != 1 {
		t.Fatalf("cached = %d, want 1 (cai na 2a candidata)", cached)
	}
	if !strings.HasPrefix(items[0].Image, "/uploads/site/products/acc-1/") || !strings.HasSuffix(items[0].Image, ".avif") {
		t.Fatalf("Image deveria virar local .avif: %q", items[0].Image)
	}
}

func TestAllPerolaHost(t *testing.T) {
	if allPerolaHost(nil) {
		t.Fatal("lista vazia deveria ser false")
	}
	if !allPerolaHost([]string{perolaBaseHost + "/a.avif", perolaBaseHost + "/b.jpg"}) {
		t.Fatal("todas no host da Perola deveria ser true")
	}
	if allPerolaHost([]string{perolaBaseHost + "/a.avif", "https://cdn.x/b.jpg"}) {
		t.Fatal("mistura de hosts deveria ser false")
	}
}

// TestImageCacheNoRootDirIsNoop: sem UPLOADS_DIR, nao faz nada e nao quebra.
func TestImageCacheNoRootDirIsNoop(t *testing.T) {
	cache := NewImageCache("")
	items := []ProductUpsertItem{{ExternalID: "1", Image: "https://x/y.webp"}}
	if c := cache.CacheItems(context.Background(), "acc-1", items); c != 0 {
		t.Fatalf("sem rootDir deveria ser no-op, cached=%d", c)
	}
	if items[0].Image != "https://x/y.webp" {
		t.Fatalf("no-op nao deveria mudar a imagem")
	}
}

// TestSaveUploadValidatesImage: grava upload valido (.webp via magic bytes) e
// rejeita corpo nao-imagem (HTML); extensao vem do filename ou do content-type.
func TestSaveUploadValidatesImage(t *testing.T) {
	cache := NewImageCache(t.TempDir())
	webp := append([]byte("RIFF\x00\x00\x00\x00WEBP"), make([]byte, 8)...)

	rel, err := cache.SaveUpload("acc-1", "foto.webp", "image/webp", webp)
	if err != nil {
		t.Fatalf("upload valido falhou: %v", err)
	}
	if !strings.HasPrefix(rel, "/uploads/site/products/acc-1/up-") || !strings.HasSuffix(rel, ".webp") {
		t.Fatalf("path inesperado: %q", rel)
	}

	// HTML (pagina de erro) deve ser rejeitado, mesmo com content-type mentindo.
	if _, err := cache.SaveUpload("acc-1", "x.webp", "image/webp", []byte("<br /><b>Fatal error</b>")); err == nil {
		t.Fatal("HTML deveria ser rejeitado")
	}
	// Sem extensao no nome: cai no content-type.
	if r2, err := cache.SaveUpload("acc-1", "blob", "image/png", []byte("\x89PNG\r\n\x1a\n____")); err != nil || !strings.HasSuffix(r2, ".png") {
		t.Fatalf("ext via content-type falhou: %q %v", r2, err)
	}
}

func TestImageExtFromURL(t *testing.T) {
	cases := map[string]string{
		"https://x/a.avif":       ".avif",
		"https://x/a.webp":       ".webp",
		"https://x/a.JPG":        ".jpg",
		"https://x/a.jpeg?v=2":   ".jpeg",
		"https://x/a.png":        ".png",
		"https://x/sem-extensao": ".img",
		"https://x/a.bmp":        ".img",
	}
	for in, want := range cases {
		if got := imageExtFromURL(in); got != want {
			t.Fatalf("imageExtFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}
