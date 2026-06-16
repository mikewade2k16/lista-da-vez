package bio

import "testing"

// TestResolveMediaTypeAvif garante que avif e aceito tanto pelo content-type
// (header image/avif setado pelo browser) quanto pela extensao .avif, mesmo
// quando o sniff (http.DetectContentType) devolve application/octet-stream.
func TestResolveMediaTypeAvif(t *testing.T) {
	// Magic bytes de um arquivo AVIF (ISO-BMFF: bytes 4..8 == "ftyp" + "avif").
	// http.DetectContentType nao reconhece avif e cai em octet-stream, entao o
	// fluxo precisa resolver pelo header ou pela extensao.
	avifContent := []byte("\x00\x00\x00\x20ftypavif\x00\x00\x00\x00avifmif1")

	t.Run("por content-type image/avif", func(t *testing.T) {
		mime, ext, isVideo := resolveMediaType(avifContent, "image/avif", "")
		if mime != "image/avif" || ext != ".avif" || isVideo {
			t.Fatalf("esperava (image/avif, .avif, false), obteve (%q, %q, %v)", mime, ext, isVideo)
		}
	})

	t.Run("por extensao .avif", func(t *testing.T) {
		mime, ext, isVideo := resolveMediaType(avifContent, "", "perola.avif")
		if mime != "image/avif" || ext != ".avif" || isVideo {
			t.Fatalf("esperava (image/avif, .avif, false), obteve (%q, %q, %v)", mime, ext, isVideo)
		}
	})

	t.Run("matchAllowedType image/avif", func(t *testing.T) {
		mime, ext, isVideo, ok := matchAllowedType("image/avif")
		if !ok || mime != "image/avif" || ext != ".avif" || isVideo {
			t.Fatalf("esperava (image/avif, .avif, false, true), obteve (%q, %q, %v, %v)", mime, ext, isVideo, ok)
		}
	})

	t.Run("typeFromExtension .avif", func(t *testing.T) {
		mime, ext, isVideo, ok := typeFromExtension("perola.AVIF")
		if !ok || mime != "image/avif" || ext != ".avif" || isVideo {
			t.Fatalf("esperava (image/avif, .avif, false, true), obteve (%q, %q, %v, %v)", mime, ext, isVideo, ok)
		}
	})
}
