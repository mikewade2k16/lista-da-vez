package finance

import (
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strings"
)

type pdfCurrencyWord struct {
	XMin float64 `xml:"xMin,attr"`
	XMax float64 `xml:"xMax,attr"`
	YMin float64 `xml:"yMin,attr"`
	YMax float64 `xml:"yMax,attr"`
	Text string  `xml:",chardata"`
}

var pdfIntegerRE = regexp.MustCompile(`^[0-9][0-9.]*$`)
var pdfCentsRE = regexp.MustCompile(`^[0-9]{2}$`)

// Some receipts print cents in a smaller raised font; plain text merges 12 + 00 into 1200.
// Restore the separator only when the currency and relative glyph geometry prove this layout.
func normalizePDFCurrency(text string, bbox []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(bbox))
	words := []pdfCurrencyWord{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return text
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "word" {
			var word pdfCurrencyWord
			if decoder.DecodeElement(&word, &start) != nil {
				return text
			}
			words = append(words, word)
		}
	}
	for i := 0; i+2 < len(words); i++ {
		currency, integer, cents := words[i], words[i+1], words[i+2]
		if strings.TrimSpace(currency.Text) != "R$" || !pdfIntegerRE.MatchString(integer.Text) || !pdfCentsRE.MatchString(cents.Text) {
			continue
		}
		height := integer.YMax - integer.YMin
		smallHeight := cents.YMax - cents.YMin
		gap := cents.XMin - integer.XMax
		if height <= 0 || smallHeight <= 0 || smallHeight > height*0.8 || cents.YMax > integer.YMax-height*0.15 || cents.YMin < integer.YMin-height*0.3 || gap < -height*0.15 || gap > height*0.4 {
			continue
		}
		pattern := regexp.MustCompile(`R\s*\$\s*` + regexp.QuoteMeta(integer.Text) + `\s*` + regexp.QuoteMeta(cents.Text) + `\b`)
		text = pattern.ReplaceAllStringFunc(text, func(string) string { return "R$ " + integer.Text + "," + cents.Text })
	}
	return text
}
