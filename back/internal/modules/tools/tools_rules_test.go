package tools

import (
	"errors"
	"math"
	"net/http/httptest"
	"testing"
)

func TestQrContrastMatchesWCAG(t *testing.T) {
	// Valores de referencia WCAG: preto/branco = 21; #777777/branco ~ 4.48.
	cases := []struct {
		fill, back string
		want       float64
	}{
		{"#000000", "#ffffff", 21},
		{"#ffffff", "#ffffff", 1},
		{"#777777", "#ffffff", 4.48},
		{"#767676", "#ffffff", 4.54},
	}
	for _, c := range cases {
		got, _, ok := qrContrast(c.fill, c.back)
		if !ok || math.Abs(got-c.want) > 0.01 {
			t.Fatalf("contraste %s/%s = %.3f ok=%v, esperado %.2f", c.fill, c.back, got, ok, c.want)
		}
	}
}

func TestValidateQrColors(t *testing.T) {
	cases := []struct {
		name, fill, back string
		want             error
	}{
		{"preto no branco", "#000000", "#ffffff", nil},
		{"azul escuro no branco", "#112233", "#ffffff", nil},
		{"limite aprovado #767676", "#767676", "#ffffff", nil},
		{"limite reprovado #777777", "#777777", "#ffffff", ErrLowContrast},
		{"cinza claro (caso real que nao leu)", "#cccccc", "#ffffff", ErrLowContrast},
		{"mesma cor", "#336699", "#336699", ErrLowContrast},
		{"invertido branco no preto", "#ffffff", "#000000", ErrLowContrast},
		{"formato curto", "#000", "#ffffff", ErrInvalidColor},
		{"sem cerquilha", "000000", "#ffffff", ErrInvalidColor},
		{"nome de cor", "black", "#ffffff", ErrInvalidColor},
		{"vazio", "", "#ffffff", ErrInvalidColor},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := validateQrColors(c.fill, c.back); !errors.Is(err, c.want) {
				t.Fatalf("validateQrColors(%s,%s) = %v, esperado %v", c.fill, c.back, err, c.want)
			}
		})
	}
}

func TestShouldCountVisit(t *testing.T) {
	const iphone = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"
	const android = "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36"
	const cubot = "Mozilla/5.0 (Linux; Android 12; CUBOT KINGKONG 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Mobile Safari/537.36"
	const instagramApp = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 330.0.0.0"
	const facebookApp = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/FBIOS;FBAV/470.0]"
	cases := []struct {
		name, method, ua string
		want             bool
	}{
		{"iPhone Safari (camera)", "GET", iphone, true},
		{"Android Chrome", "GET", android, true},
		{"celular Cubot nao e robo", "GET", cubot, true},
		{"navegador interno do Instagram", "GET", instagramApp, true},
		{"navegador interno do Facebook", "GET", facebookApp, true},
		{"HEAD de verificacao", "HEAD", iphone, false},
		{"sem User-Agent", "GET", "", false},
		{"previa do WhatsApp", "GET", "WhatsApp/2.23.20.0 A", false},
		{"previa do Facebook", "GET", "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)", false},
		{"previa do Telegram", "GET", "TelegramBot (like TwitterBot)", false},
		{"previa do Slack", "GET", "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)", false},
		{"previa do Discord", "GET", "Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)", false},
		{"previa do LinkedIn", "GET", "LinkedInBot/1.0 (compatible; Mozilla/5.0; Apache-HttpClient +http://www.linkedin.com)", false},
		{"previa do iMessage", "GET", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_1) AppleWebKit/601.2.4 (KHTML, like Gecko) Version/9.0.1 Safari/601.2.4 facebookexternalhit/1.1 Facebot Twitterbot/1.0", false},
		{"Googlebot", "GET", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(c.method, "/s/x", nil)
			r.Header.Set("User-Agent", c.ua)
			if c.ua == "" {
				r.Header.Del("User-Agent")
			}
			if got := shouldCountVisit(r); got != c.want {
				t.Fatalf("shouldCountVisit(%s %q) = %v, esperado %v", c.method, c.ua, got, c.want)
			}
		})
	}
}

func TestHasToolsPermission(t *testing.T) {
	cases := []struct {
		name     string
		granted  []string
		required string
		want     bool
	}{
		{"view concede view", []string{"tools.qr.view"}, "tools.qr.view", true},
		{"manage concede view", []string{"tools.qr.manage"}, "tools.qr.view", true},
		{"view NAO concede manage", []string{"tools.qr.view"}, "tools.qr.manage", false},
		{"permissao de outra ferramenta nao vale", []string{"tools.shortlinks.manage"}, "tools.qr.view", false},
		{"roteiros nao concede QR", []string{"tools.scripts.manage"}, "tools.qr.manage", false},
		{"menu nao concede API", []string{"workspace.tools.view"}, "tools.shortlinks.view", false},
		{"sem permissoes", nil, "tools.shortlinks.view", false},
		{"prefixo parecido nao vale", []string{"tools.qr.viewer"}, "tools.qr.view", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasToolsPermission(c.granted, c.required); got != c.want {
				t.Fatalf("hasToolsPermission(%v, %s) = %v, esperado %v", c.granted, c.required, got, c.want)
			}
		})
	}
}
