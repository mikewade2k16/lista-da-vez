package tools

import (
	"math"
	"strconv"
	"strings"
)

// MinQrContrast e o contraste minimo (formula WCAG) entre a cor do QR e a do
// fundo. Abaixo disso camera de celular erra a leitura (cinza claro em branco,
// por exemplo). O front usa o mesmo valor em web/app/utils/qr-contrast.ts.
const MinQrContrast = 4.5

// parseHexColor le #rrggbb. ok=false para qualquer outro formato.
func parseHexColor(s string) (r, g, b float64, ok bool) {
	if !colorPattern.MatchString(s) {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(strings.TrimPrefix(s, "#"), 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255, true
}

// relativeLuminance segue a definicao WCAG 2.x (sRGB linearizado).
func relativeLuminance(r, g, b float64) float64 {
	lin := func(c float64) float64 {
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// qrContrast devolve a razao de contraste e se o QR e "escuro sobre claro".
func qrContrast(fill, back string) (ratio float64, darkOnLight bool, ok bool) {
	fr, fg, fb, ok1 := parseHexColor(fill)
	br, bg, bb, ok2 := parseHexColor(back)
	if !ok1 || !ok2 {
		return 0, false, false
	}
	lf := relativeLuminance(fr, fg, fb)
	lb := relativeLuminance(br, bg, bb)
	hi, lo := math.Max(lf, lb), math.Min(lf, lb)
	return (hi + 0.05) / (lo + 0.05), lf < lb, true
}

// validateQrColors exige #rrggbb, QR mais escuro que o fundo (QR invertido
// falha em varios leitores) e contraste >= MinQrContrast.
func validateQrColors(fill, back string) error {
	ratio, darkOnLight, ok := qrContrast(fill, back)
	if !ok {
		return ErrInvalidColor
	}
	if !darkOnLight || ratio < MinQrContrast {
		return ErrLowContrast
	}
	return nil
}
