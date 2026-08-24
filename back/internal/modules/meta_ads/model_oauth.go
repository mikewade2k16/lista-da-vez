package metaads

import "time"

// OAuthConfig contem somente configuracao server-side do Facebook Login.
type OAuthConfig struct {
	AppID       string
	AppSecret   string
	RedirectURI string
}

// OAuthStartResult e o unico payload do inicio que chega ao browser. Nao
// contem app secret, token ou account id.
type OAuthStartResult struct {
	AuthorizationURL string `json:"authorizationUrl"`
	ExpiresAt        string `json:"expiresAt"`
}

// OAuthPendingState e o minimo necessario para concluir o callback publico.
// O state bruto e o code da Meta nunca entram nesta estrutura nem no banco.
type OAuthPendingState struct {
	AccountID   string
	RedirectURI string
	ExpiresAt   time.Time
}

// OAuthAccessToken permanece apenas em memoria durante o callback. Nunca deve
// ser serializado, logado ou incorporado a erros.
type OAuthAccessToken struct {
	Value     string
	ExpiresAt time.Time
}

// OAuthPermission representa o estado efetivamente concedido pelo usuario no
// Facebook Login. Ela existe apenas durante o callback e nao e persistida.
type OAuthPermission struct {
	Name   string
	Status string
}
