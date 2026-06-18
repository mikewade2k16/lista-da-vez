package commission

// PayoutMode define como o valor de uma faixa e interpretado.
//
//	ModePercent: o value e um percentual aplicado sobre a base (venda).
//	ModeAmount:  o value e um valor fixo em reais (nao depende da base).
type PayoutMode string

const (
	ModePercent PayoutMode = "percent"
	ModeAmount  PayoutMode = "amount"
)

// ConsultantBase define a base de calculo do consultor.
//
//	BaseSelf:  percentual incide sobre a PROPRIA venda do consultor.
//	BaseStore: percentual incide sobre o total da loja.
type ConsultantBase string

const (
	BaseSelf  ConsultantBase = "self"
	BaseStore ConsultantBase = "store"
)

// RoleGroup e o grupo de papel usado para escolher as faixas e a regra.
type RoleGroup string

const (
	GroupConsultant RoleGroup = "consultant"
	GroupManager    RoleGroup = "manager"
	GroupSupport    RoleGroup = "support"
)

// Rule e uma faixa de recebimento. Threshold e o % de atingimento (da loja)
// a partir do qual a faixa vale; Value/Mode definem o quanto.
type Rule struct {
	Threshold float64    `json:"threshold"`
	Value     float64    `json:"value"`
	Mode      PayoutMode `json:"mode"`
}

// ConsultantRules sao as regras especificas do grupo consultor.
//
// O consultor recebe % sobre a PROPRIA venda, escolhendo a faixa pela PROPRIA
// meta (Policy.Consultant, threshold = % da meta do consultor). A LOJA atua como
// gate sobre esse valor:
//   - loja < StoreFloorPercent            -> recebe 0;
//   - loja em [StoreFloorPercent, StoreFullPercent) -> faixa REDUZIDA: so recebe
//     se a propria meta >= ReducedRequiresOwnPercent, e entao recebe ReducedRate;
//   - loja >= StoreFullPercent            -> faixa propria normal (Policy.Consultant).
type ConsultantRules struct {
	Base                      ConsultantBase `json:"base"`
	QualityPenaltyPercent     float64        `json:"qualityPenaltyPercent"`
	StoreFloorPercent         float64        `json:"storeFloorPercent"`         // loja < isso: 0
	StoreFullPercent          float64        `json:"storeFullPercent"`          // loja >= isso: faixas proprias
	ReducedRate               float64        `json:"reducedRate"`               // % na faixa reduzida da loja
	ReducedRequiresOwnPercent float64        `json:"reducedRequiresOwnPercent"` // meta propria minima na faixa reduzida
}

// Policy e a politica de comissao v2 (armazenada como JSONB tenant-wide).
//
// Retrocompat: linhas antigas tem apenas "consultant"/"manager"/"support".
// NormalizePolicy semeia managerShopping/managerBairro a partir de Manager
// (legado) quando os campos novos faltam, e aplica os defaults v2.
type Policy struct {
	Consultant      []Rule          `json:"consultant"`
	ManagerShopping []Rule          `json:"managerShopping"`
	ManagerBairro   []Rule          `json:"managerBairro"`
	Support         []Rule          `json:"support"`
	ConsultantRules ConsultantRules `json:"consultantRules"`

	// Manager e o campo LEGADO (v1, sem distincao de loja). So lido pelo
	// normalize para semear managerShopping/managerBairro quando ausentes.
	Manager []Rule `json:"manager,omitempty"`
}

// Input sao os insumos para um calculo unitario de comissao.
type Input struct {
	Role               string
	StoreType          string // "shopping" | "bairro" (default bairro)
	StoreSold          float64
	StoreProgress      float64 // % de atingimento da loja (storeSold/storeGoal*100)
	ConsultantSold     float64
	ConsultantProgress float64 // % de atingimento do consultor (consultantSold/monthlyGoal*100)
	HitPa              bool
	HitTicket          bool
	Policy             Policy
}

// Result e o payout calculado. Amount em reais; RatePercent ja com penalidade.
type Result struct {
	Amount         float64   `json:"amount"`
	RatePercent    float64   `json:"ratePercent"`
	Base           float64   `json:"base"`
	Group          RoleGroup `json:"group"`
	RuleLabel      string    `json:"ruleLabel"`
	PenaltyApplied float64   `json:"penaltyApplied"`
	RuleMatched    bool      `json:"ruleMatched"`
}
