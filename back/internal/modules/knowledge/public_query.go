package knowledge

import (
	"sort"
	"strings"
)

// Consulta lexical do acervo público, sem IA (CT-15 fase C, P15-34). Cada termo da
// pergunta vira um "conceito": siglas e palavras curtas casam exatamente (DAS não vira
// "da"), palavras longas casam pelo radical (declarar → declar:*), e grupos de
// sinônimos de vocabulário ampliam o conceito. Os grupos são só vocabulário de busca,
// nunca valores ou regras fiscais. O ranking prioriza quantos conceitos o trecho cobre.

var publicSynonymGroups = [][]string{
	{"das", "guia", "boleto"},
	{"nota", "nfse", "nfs", "nf"},
	{"declaracao", "dasn", "declarar"},
	{"receita", "faturamento", "recebimento"},
	{"limite", "teto"},
	{"mei", "microempreendedor", "simei"},
	{"prazo", "vencimento", "vence", "data"},
	{"atividade", "atividades", "ocupacao", "cnae"},
	{"empregado", "funcionario", "contratar"},
	{"emitir", "emissao", "gerar"},
	{"cancelar", "cancelamento"},
	{"pagar", "pagamento", "recolher", "recolhimento"},
	{"multa", "penalidade", "atraso"},
	{"desenquadramento", "desenquadrar", "excluir", "exclusao"},
	{"imposto", "tributo", "das"},
	{"abrir", "abertura", "formalizar", "formalizacao"},
	{"baixa", "encerrar", "encerramento", "fechar"},
	{"municipio", "prefeitura", "municipal"},
}

var publicSuffixes = []string{"amentos", "amento", "imentos", "imento", "acoes", "icoes", "acao", "icao", "coes", "cao", "mente", "idades", "idade", "ados", "adas", "idos", "idas", "ado", "ada", "ido", "ida", "ores", "ora", "or", "ar", "er", "ir", "ais", "eis", "es", "s"}

// publicStem reduz palavras longas a um radical conservador (mínimo de 4 letras).
func publicStem(term string) string {
	if len(term) < 6 {
		return term
	}
	for _, suffix := range publicSuffixes {
		if strings.HasSuffix(term, suffix) && len(term)-len(suffix) >= 4 {
			return term[:len(term)-len(suffix)]
		}
	}
	return term
}

func publicLexeme(term string) string {
	if len(term) <= 4 {
		return term
	}
	return publicStem(term) + ":*"
}

// publicConcepts devolve uma tsquery por conceito (alternativas em OR), sem repetir
// conceitos equivalentes. Termos já chegam validados ([a-z0-9]{2,50}).
func publicConcepts(terms []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, term := range terms {
		alternatives := map[string]bool{publicLexeme(term): true}
		for _, group := range publicSynonymGroups {
			for _, word := range group {
				if word == term || publicStem(word) == publicStem(term) {
					for _, alt := range group {
						alternatives[publicLexeme(alt)] = true
					}
					break
				}
			}
		}
		list := make([]string, 0, len(alternatives))
		for alt := range alternatives {
			list = append(list, alt)
		}
		sort.Strings(list)
		key := strings.Join(list, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, "("+strings.Join(list, " | ")+")")
	}
	return out
}

// publicMinCoverage exige ao menos dois conceitos quando a pergunta tem dois ou mais,
// para um termo genérico ("mei") sozinho não trazer trecho sem relação.
func publicMinCoverage(concepts int) int {
	if concepts >= 2 {
		return 2
	}
	return concepts
}
