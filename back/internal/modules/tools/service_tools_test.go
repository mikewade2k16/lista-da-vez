package tools

import (
	"context"
	"errors"
	"testing"
)

// fakeShortLinks guarda um link e registra o patch exatamente como chegaria ao
// banco. updates conta as gravacoes: erro de validacao nao pode gravar nada.
type fakeShortLinks struct {
	item      ShortLinkItem
	lastPatch ShortLinkPatch
	updates   int
	resolved  []bool
}

func (f *fakeShortLinks) List(context.Context, string, ListFilter) ([]ShortLinkItem, int, error) {
	return nil, 0, nil
}
func (f *fakeShortLinks) Create(_ context.Context, _, slug, target string) (ShortLinkItem, error) {
	return ShortLinkItem{Slug: slug, TargetURL: target, IsActive: true}, nil
}
func (f *fakeShortLinks) Get(_ context.Context, id, _ string) (ShortLinkItem, error) {
	if id != f.item.ID {
		return ShortLinkItem{}, ErrShortLinkNotFound
	}
	return f.item, nil
}
func (f *fakeShortLinks) Update(_ context.Context, id, _ string, p ShortLinkPatch) (ShortLinkItem, error) {
	if id != f.item.ID {
		return ShortLinkItem{}, ErrShortLinkNotFound
	}
	f.updates++
	f.lastPatch = p
	if p.Slug != nil {
		f.item.Slug = *p.Slug
	}
	if p.TargetURL != nil {
		f.item.TargetURL = *p.TargetURL
	}
	if p.IsActive != nil {
		f.item.IsActive = *p.IsActive
	}
	return f.item, nil
}
func (f *fakeShortLinks) Delete(context.Context, string, string) error { return nil }
func (f *fakeShortLinks) Resolve(_ context.Context, _ string, count bool) (string, error) {
	f.resolved = append(f.resolved, count)
	return f.item.TargetURL, nil
}

type fakeQrCodes struct {
	item      QrCodeItem
	lastPatch QrCodePatch
	updates   int
	created   []QrCodeRecord
}

func (f *fakeQrCodes) List(context.Context, string, ListFilter) ([]QrCodeItem, int, error) {
	return nil, 0, nil
}
func (f *fakeQrCodes) Create(_ context.Context, _ string, rec QrCodeRecord) (QrCodeItem, error) {
	f.created = append(f.created, rec)
	return QrCodeItem{Slug: rec.Slug, FillColor: rec.FillColor, BackColor: rec.BackColor}, nil
}
func (f *fakeQrCodes) Get(_ context.Context, id, _ string) (QrCodeItem, error) {
	if id != f.item.ID {
		return QrCodeItem{}, ErrQrCodeNotFound
	}
	return f.item, nil
}
func (f *fakeQrCodes) Update(_ context.Context, id, _ string, p QrCodePatch) (QrCodeItem, error) {
	if id != f.item.ID {
		return QrCodeItem{}, ErrQrCodeNotFound
	}
	f.updates++
	f.lastPatch = p
	return f.item, nil
}
func (f *fakeQrCodes) Delete(context.Context, string, string) error { return nil }
func (f *fakeQrCodes) Resolve(context.Context, string, bool) (string, error) {
	return f.item.TargetURL, nil
}

const testID = "11111111-2222-3333-4444-555555555555"

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }

func newShortFixture() (*Service, *fakeShortLinks) {
	sl := &fakeShortLinks{item: ShortLinkItem{ID: testID, Slug: "promo", TargetURL: "https://a.com", IsActive: true}}
	return NewService(sl, &fakeQrCodes{}, "https://crowvisuals.com.br"), sl
}

func newQrFixture() (*Service, *fakeQrCodes) {
	qr := &fakeQrCodes{item: QrCodeItem{ID: testID, Slug: "cardapio", TargetURL: "https://a.com",
		FillColor: "#000000", BackColor: "#ffffff", IsActive: true}}
	return NewService(&fakeShortLinks{}, qr, "https://crowvisuals.com.br"), qr
}

func TestShortLinkSlugChangeRequiresConfirmation(t *testing.T) {
	ctx := context.Background()

	t.Run("sem confirmacao recusa e nao grava", func(t *testing.T) {
		svc, sl := newShortFixture()
		_, err := svc.UpdateShortLink(ctx, testID, "acc", ShortLinkInput{Slug: strp("nova")})
		if !errors.Is(err, ErrSlugChangeNotConfirmed) {
			t.Fatalf("esperado ErrSlugChangeNotConfirmed, veio %v", err)
		}
		if sl.updates != 0 || sl.item.Slug != "promo" {
			t.Fatalf("nao podia gravar: updates=%d slug=%s", sl.updates, sl.item.Slug)
		}
	})
	t.Run("confirmacao false recusa", func(t *testing.T) {
		svc, sl := newShortFixture()
		_, err := svc.UpdateShortLink(ctx, testID, "acc", ShortLinkInput{Slug: strp("nova"), ConfirmSlugChange: boolp(false)})
		if !errors.Is(err, ErrSlugChangeNotConfirmed) || sl.updates != 0 {
			t.Fatalf("esperado recusa sem gravar, veio err=%v updates=%d", err, sl.updates)
		}
	})
	t.Run("com confirmacao grava o slug normalizado", func(t *testing.T) {
		svc, sl := newShortFixture()
		it, err := svc.UpdateShortLink(ctx, testID, "acc", ShortLinkInput{Slug: strp("Nova Promo"), ConfirmSlugChange: boolp(true)})
		if err != nil || it.Slug != "nova-promo" || sl.lastPatch.Slug == nil || *sl.lastPatch.Slug != "nova-promo" {
			t.Fatalf("esperado slug nova-promo, veio err=%v item=%+v", err, it)
		}
		if it.ShortURL != "https://crowvisuals.com.br/s/nova-promo" {
			t.Fatalf("shortUrl errado: %s", it.ShortURL)
		}
	})
	t.Run("mesmo slug e no-op e nao exige confirmacao", func(t *testing.T) {
		svc, sl := newShortFixture()
		_, err := svc.UpdateShortLink(ctx, testID, "acc", ShortLinkInput{Slug: strp("PROMO"), TargetURL: strp("b.com")})
		if err != nil || sl.lastPatch.Slug != nil || *sl.lastPatch.TargetURL != "https://b.com" {
			t.Fatalf("esperado so troca de destino, veio err=%v patch=%+v", err, sl.lastPatch)
		}
	})
	t.Run("slug vazio recusa (nao vira codigo aleatorio)", func(t *testing.T) {
		svc, sl := newShortFixture()
		_, err := svc.UpdateShortLink(ctx, testID, "acc", ShortLinkInput{Slug: strp("  "), ConfirmSlugChange: boolp(true)})
		if !errors.Is(err, ErrInvalidSlug) || sl.updates != 0 {
			t.Fatalf("esperado ErrInvalidSlug sem gravar, veio err=%v updates=%d", err, sl.updates)
		}
	})
	t.Run("link de outra conta = 404 antes de qualquer gravacao", func(t *testing.T) {
		svc, sl := newShortFixture()
		_, err := svc.UpdateShortLink(ctx, "99999999-2222-3333-4444-555555555555", "acc",
			ShortLinkInput{Slug: strp("x"), ConfirmSlugChange: boolp(true)})
		if !errors.Is(err, ErrShortLinkNotFound) || sl.updates != 0 {
			t.Fatalf("esperado 404 sem gravar, veio err=%v updates=%d", err, sl.updates)
		}
	})
}

func TestShortLinkActivationToggle(t *testing.T) {
	ctx := context.Background()
	svc, sl := newShortFixture()
	it, err := svc.UpdateShortLink(ctx, testID, "acc", ShortLinkInput{IsActive: boolp(false)})
	if err != nil || it.IsActive || sl.lastPatch.IsActive == nil || *sl.lastPatch.IsActive {
		t.Fatalf("desativar: err=%v item=%+v", err, it)
	}
	if sl.lastPatch.Slug != nil || sl.lastPatch.TargetURL != nil {
		t.Fatalf("desativar nao pode mexer em slug/destino: %+v", sl.lastPatch)
	}
	it, err = svc.UpdateShortLink(ctx, testID, "acc", ShortLinkInput{IsActive: boolp(true)})
	if err != nil || !it.IsActive {
		t.Fatalf("reativar: err=%v item=%+v", err, it)
	}
}

func TestResolvePassesCountDecision(t *testing.T) {
	svc, sl := newShortFixture()
	if _, err := svc.ResolveShortLink(context.Background(), "promo", true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ResolveShortLink(context.Background(), "promo", false); err != nil {
		t.Fatal(err)
	}
	if len(sl.resolved) != 2 || !sl.resolved[0] || sl.resolved[1] {
		t.Fatalf("decisao de contagem nao chegou ao store: %v", sl.resolved)
	}
}

func TestQrSlugIsImmutable(t *testing.T) {
	ctx := context.Background()
	for _, slug := range []string{"outro", "", "   ", "cardapio-2"} {
		t.Run("recusa "+slug, func(t *testing.T) {
			svc, qr := newQrFixture()
			_, err := svc.UpdateQrCode(ctx, testID, "acc", QrCodeInput{Slug: strp(slug), IsActive: boolp(false)})
			if !errors.Is(err, ErrQrSlugImmutable) {
				t.Fatalf("esperado ErrQrSlugImmutable, veio %v", err)
			}
			if qr.updates != 0 {
				t.Fatalf("nada pode ser gravado quando o slug e recusado (updates=%d)", qr.updates)
			}
		})
	}
	t.Run("mesmo slug (form antigo) e aceito", func(t *testing.T) {
		svc, qr := newQrFixture()
		_, err := svc.UpdateQrCode(ctx, testID, "acc", QrCodeInput{Slug: strp("Cardapio"), TargetURL: strp("b.com")})
		if err != nil || qr.updates != 1 || *qr.lastPatch.TargetURL != "https://b.com" {
			t.Fatalf("esperado gravar so o destino, veio err=%v updates=%d", err, qr.updates)
		}
	})
}

func TestQrColorsValidatedAsEffectivePair(t *testing.T) {
	ctx := context.Background()
	t.Run("so a cor do QR, conferida contra o fundo atual", func(t *testing.T) {
		svc, qr := newQrFixture()
		qr.item.BackColor = "#eeeeee"
		_, err := svc.UpdateQrCode(ctx, testID, "acc", QrCodeInput{FillColor: strp("#bbbbbb")})
		if !errors.Is(err, ErrLowContrast) || qr.updates != 0 {
			t.Fatalf("esperado ErrLowContrast sem gravar, veio err=%v updates=%d", err, qr.updates)
		}
	})
	t.Run("par valido grava com trava nas cores atuais", func(t *testing.T) {
		svc, qr := newQrFixture()
		_, err := svc.UpdateQrCode(ctx, testID, "acc", QrCodeInput{FillColor: strp("#1A2B3C")})
		if err != nil {
			t.Fatal(err)
		}
		p := qr.lastPatch
		if *p.FillColor != "#1a2b3c" || *p.BackColor != "#ffffff" || p.ExpectFill != "#000000" || p.ExpectBack != "#ffffff" {
			t.Fatalf("patch errado: %+v fill=%s back=%s", p, *p.FillColor, *p.BackColor)
		}
	})
	t.Run("cor invalida e erro, nao vira preto", func(t *testing.T) {
		svc, qr := newQrFixture()
		_, err := svc.UpdateQrCode(ctx, testID, "acc", QrCodeInput{BackColor: strp("branco")})
		if !errors.Is(err, ErrInvalidColor) || qr.updates != 0 {
			t.Fatalf("esperado ErrInvalidColor sem gravar, veio err=%v", err)
		}
	})
	t.Run("sem mexer em cor nao le nem trava", func(t *testing.T) {
		svc, qr := newQrFixture()
		_, err := svc.UpdateQrCode(ctx, testID, "acc", QrCodeInput{IsActive: boolp(false)})
		if err != nil || qr.lastPatch.ExpectFill != "" || qr.lastPatch.FillColor != nil {
			t.Fatalf("toggle nao pode tocar em cores: err=%v patch=%+v", err, qr.lastPatch)
		}
	})
	t.Run("create recusa contraste baixo e cor invalida", func(t *testing.T) {
		svc, qr := newQrFixture()
		for _, in := range []QrCodeInput{
			{TargetURL: strp("a.com"), FillColor: strp("#cccccc")},
			{TargetURL: strp("a.com"), FillColor: strp("#ffffff"), BackColor: strp("#000000")},
			{TargetURL: strp("a.com"), FillColor: strp("preto")},
		} {
			if _, err := svc.CreateQrCode(ctx, "acc", in); err == nil {
				t.Fatalf("create devia recusar %+v", in)
			}
		}
		if len(qr.created) != 0 {
			t.Fatalf("nenhum QR podia ser criado, criou %d", len(qr.created))
		}
		if _, err := svc.CreateQrCode(ctx, "acc", QrCodeInput{TargetURL: strp("a.com")}); err != nil {
			t.Fatalf("create com cores padrao falhou: %v", err)
		}
		if got := qr.created[0]; got.FillColor != "#000000" || got.BackColor != "#ffffff" {
			t.Fatalf("cores padrao erradas: %+v", got)
		}
	})
}
