package tools

import (
	"net/http"
	"strings"
)

// previewAgents identifica robos que abrem o link sem uma pessoa clicar:
// geradores de previa de apps de mensagem/redes e buscadores. Eles recebem o
// redirect normalmente, mas nao entram na contagem de cliques/scans.
// Lista explicita (minusculas): um "bot" generico pegaria aparelhos reais
// (ex.: celulares Cubot). Nao incluir nomes que tambem aparecem no navegador
// interno de apps (Teams, Snapchat, Viber, Instagram): ali e uma pessoa clicando.
var previewAgents = []string{
	"whatsapp",
	"facebookexternalhit",
	"facebookcatalog",
	"facebot",
	"meta-externalagent",
	"telegrambot",
	"twitterbot",
	"slackbot",
	"slack-imgproxy",
	"discordbot",
	"linkedinbot",
	"skypeuripreview",
	"microsoftpreview",
	"pinterestbot",
	"redditbot",
	"applebot",
	"googlebot",
	"google-inspectiontool",
	"googleother",
	"adsbot-google",
	"bingbot",
	"bingpreview",
	"yandexbot",
	"duckduckbot",
	"baiduspider",
	"embedly",
	"iframely",
	"vkshare",
	"line-poker",
	"tiktokbot",
	"bytespider",
	"petalbot",
	"semrushbot",
	"ahrefsbot",
	"mj12bot",
	"ia_archiver",
	"headlesschrome",
}

// shouldCountVisit decide se o acesso conta como clique/scan: so GET de um
// navegador identificado. HEAD (verificacao de link), User-Agent vazio
// (scripts) e robos de previa recebem o redirect, mas nao contam.
func shouldCountVisit(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	ua := strings.ToLower(strings.TrimSpace(r.UserAgent()))
	if ua == "" {
		return false
	}
	for _, agent := range previewAgents {
		if strings.Contains(ua, agent) {
			return false
		}
	}
	return true
}
