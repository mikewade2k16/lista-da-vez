package observability

import (
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Normalização de SQL: troca literais por "?" para que a mesma consulta com
// valores diferentes tenha o mesmo fingerprint e nenhum valor seja gravado.

var (
	sqlLineComment  = regexp.MustCompile(`--[^\n]*`)
	sqlBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	sqlString       = regexp.MustCompile(`'(?:[^']|'')*'`)
	sqlDollarString = regexp.MustCompile(`(?s)\$\$.*?\$\$`)
	sqlNumber       = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	sqlInList       = regexp.MustCompile(`(?i)\bin\s*\(\s*(?:\?|\$n)(?:\s*,\s*(?:\?|\$n))*\s*\)`)
	sqlValuesList   = regexp.MustCompile(`(?i)\bvalues\s*(\([^()]*\))(?:\s*,\s*\([^()]*\))+`)
	sqlSpaces       = regexp.MustCompile(`\s+`)
)

type normalizedSQL struct {
	Fingerprint string
	Statement   string
	Verb        string
}

type sqlNormalizer struct {
	maxLength int
	cache     sync.Map
	size      int64
	mu        sync.Mutex
}

const sqlCacheLimit = 5000

func newSQLNormalizer(maxLength int) *sqlNormalizer {
	return &sqlNormalizer{maxLength: maxLength}
}

// Normalize usa cache por texto bruto: as consultas do código são strings estáticas
// com $n, então o custo de regex é pago uma vez por consulta distinta.
func (n *sqlNormalizer) Normalize(raw string) normalizedSQL {
	if cached, ok := n.cache.Load(raw); ok {
		return cached.(normalizedSQL)
	}
	result := normalizeSQL(raw, n.maxLength)
	n.mu.Lock()
	if n.size < sqlCacheLimit {
		n.size++
		n.cache.Store(raw, result)
	}
	n.mu.Unlock()
	return result
}

func normalizeSQL(raw string, maxLength int) normalizedSQL {
	out := sqlBlockComment.ReplaceAllString(raw, " ")
	out = sqlLineComment.ReplaceAllString(out, " ")
	out = sqlDollarString.ReplaceAllString(out, "?")
	out = sqlString.ReplaceAllString(out, "?")
	out = sqlNumber.ReplaceAllStringFunc(out, func(match string) string { return "?" })
	out = restoreParams(out)
	out = sqlInList.ReplaceAllString(out, "in (?)")
	out = sqlValuesList.ReplaceAllString(out, "values $1 /* ... */")
	out = strings.TrimSpace(sqlSpaces.ReplaceAllString(out, " "))
	out = strings.ToLower(out)

	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(out))
	return normalizedSQL{
		Fingerprint: strconv.FormatUint(hasher.Sum64(), 16),
		Statement:   truncate(out, maxLength),
		Verb:        firstWord(out),
	}
}

// restoreParams desfaz o efeito da troca de números sobre placeholders ($1 → $?).
func restoreParams(value string) string {
	return strings.ReplaceAll(value, "$?", "$n")
}

func firstWord(statement string) string {
	trimmed := strings.TrimLeft(statement, " (")
	if index := strings.IndexByte(trimmed, ' '); index > 0 {
		return trimmed[:index]
	}
	return trimmed
}
