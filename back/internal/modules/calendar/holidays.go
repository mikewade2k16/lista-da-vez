package calendar

import (
	"fmt"
	"sort"
	"time"
)

// Holiday e um feriado / data comemorativa devolvido pelo endpoint read-only
// GET /v1/calendar/holidays. Set e SEMPRE um de: brNational, sergipe, aracaju,
// luxuryIntl (os mesmos toggles de HolidayConfig).
type Holiday struct {
	Date string `json:"date"` // 'YYYY-MM-DD'
	Name string `json:"name"`
	Set  string `json:"set"`
}

// Identificadores dos conjuntos (batem 1:1 com os campos de HolidayConfig).
const (
	setBrNational = "brNational"
	setSergipe    = "sergipe"
	setAracaju    = "aracaju"
	setLuxuryIntl = "luxuryIntl"
)

// fixedHoliday e uma data comemorativa de dia/mes fixos (independe do ano).
type fixedHoliday struct {
	month int
	day   int
	name  string
	set   string
}

// fixedHolidays sao as datas de dia/mes fixos, replicadas a cada ano do range.
//
// Curadoria:
//   - brNational: feriados nacionais oficiais + datas comemorativas uteis para a
//     agencia (Dia dos Namorados, comercial forte no Brasil em 12/06).
//   - sergipe: feriado estadual.
//   - aracaju: feriados/datas municipais.
//   - luxuryIntl: datas globais fortes para marcas de luxo (Valentine's, Dia da
//     Mulher, Halloween, Natal e Reveillon). Cyber Monday e Black Friday sao
//     moveis (calculadas por ano) e ficam fora desta tabela.
var fixedHolidays = []fixedHoliday{
	// --- brNational: feriados nacionais oficiais ---
	{1, 1, "Confraternizacao Universal", setBrNational},
	{4, 21, "Tiradentes", setBrNational},
	{5, 1, "Dia do Trabalho", setBrNational},
	{9, 7, "Independencia do Brasil", setBrNational},
	{10, 12, "Nossa Senhora Aparecida", setBrNational},
	{11, 2, "Finados", setBrNational},
	{11, 15, "Proclamacao da Republica", setBrNational},
	{12, 25, "Natal", setBrNational},
	// --- brNational: datas comemorativas uteis para a agencia ---
	{6, 12, "Dia dos Namorados", setBrNational},

	// --- sergipe: feriado estadual ---
	{7, 8, "Emancipacao Politica de Sergipe", setSergipe},

	// --- aracaju: feriados / datas municipais ---
	{3, 17, "Emancipacao Politica de Aracaju", setAracaju},
	{12, 8, "Nossa Senhora da Conceicao (Padroeira de Aracaju)", setAracaju},

	// --- luxuryIntl: datas globais fortes para marcas de luxo ---
	{2, 14, "Valentine's Day", setLuxuryIntl},
	{3, 8, "Dia Internacional da Mulher", setLuxuryIntl},
	{10, 31, "Halloween", setLuxuryIntl},
	{12, 25, "Natal (Luxo)", setLuxuryIntl},
	{12, 31, "Reveillon", setLuxuryIntl},
}

// HolidaysInRange gera as datas comemorativas dentro de [from, to] (inclusive),
// mantendo apenas os conjuntos ligados em cfg. Retorna slice nao-nil (mesmo
// vazio), ordenado por date e sem duplicatas por (date+name).
//
// A comparacao de janela usa strings 'YYYY-MM-DD', que sao cronologicas nesse
// formato (zero-padded), dispensando parse de time.Time no filtro.
func HolidaysInRange(from, to string, cfg CalendarConfig) []Holiday {
	out := make([]Holiday, 0)
	if from > to {
		return out
	}

	fromYear, okFrom := yearOf(from)
	toYear, okTo := yearOf(to)
	if !okFrom || !okTo {
		return out
	}

	seen := make(map[string]bool)
	add := func(date, name, set string) {
		if !setEnabled(cfg, set) {
			return
		}
		if date < from || date > to {
			return
		}
		key := date + "|" + name
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Holiday{Date: date, Name: name, Set: set})
	}

	for year := fromYear; year <= toYear; year++ {
		for _, f := range fixedHolidays {
			add(ymd(year, f.month, f.day), f.name, f.set)
		}
		for _, m := range movableHolidays(year) {
			add(m.Date, m.Name, m.Set)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// movableHolidays calcula as datas moveis de um ano (dependem da Pascoa ou da
// posicao de um dia-da-semana no mes).
//
// Curadoria dos sets:
//   - Carnaval / Sexta-feira Santa / Corpus Christi / Dia das Maes / Dia dos Pais
//     -> brNational (feriados e datas comemorativas nacionais).
//   - Black Friday / Cyber Monday -> luxuryIntl (eventos de varejo global de
//     alto giro para marcas de luxo).
func movableHolidays(year int) []Holiday {
	em, ed := easterSunday(year)
	easter := time.Date(year, time.Month(em), ed, 0, 0, 0, 0, time.UTC)

	// Carnaval: terca-feira = Pascoa - 47 (a segunda seria -48).
	carnaval := easter.AddDate(0, 0, -47)
	// Sexta-feira Santa: Pascoa - 2.
	goodFriday := easter.AddDate(0, 0, -2)
	// Corpus Christi: Pascoa + 60.
	corpusChristi := easter.AddDate(0, 0, 60)

	// Dia das Maes: 2o domingo de maio. Dia dos Pais: 2o domingo de agosto.
	mothersDay := nthWeekdayOfMonth(year, 5, time.Sunday, 2)
	fathersDay := nthWeekdayOfMonth(year, 8, time.Sunday, 2)

	// Black Friday: sexta seguinte a 4a quinta de novembro (Thanksgiving + 1).
	thanksgiving := nthWeekdayOfMonth(year, 11, time.Thursday, 4)
	blackFriday := time.Date(year, 11, thanksgiving, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	// Cyber Monday: segunda seguinte a Black Friday (Black Friday + 3).
	cyberMonday := blackFriday.AddDate(0, 0, 3)

	return []Holiday{
		{Date: fromTime(carnaval), Name: "Carnaval", Set: setBrNational},
		{Date: fromTime(goodFriday), Name: "Sexta-feira Santa", Set: setBrNational},
		{Date: fromTime(corpusChristi), Name: "Corpus Christi", Set: setBrNational},
		{Date: ymd(year, 5, mothersDay), Name: "Dia das Maes", Set: setBrNational},
		{Date: ymd(year, 8, fathersDay), Name: "Dia dos Pais", Set: setBrNational},
		{Date: fromTime(blackFriday), Name: "Black Friday", Set: setLuxuryIntl},
		{Date: fromTime(cyberMonday), Name: "Cyber Monday", Set: setLuxuryIntl},
	}
}

// easterSunday devolve mes e dia do Domingo de Pascoa do ano pelo algoritmo
// "Computus" de Meeus/Jones/Butcher (calendario gregoriano).
func easterSunday(year int) (month, day int) {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month = (h + l - 7*m + 114) / 31
	day = ((h + l - 7*m + 114) % 31) + 1
	return month, day
}

// nthWeekdayOfMonth devolve o dia-do-mes da n-esima ocorrencia de weekday em
// (year, month). Ex.: 2o domingo de maio. Assume que a ocorrencia existe (n in
// 1..5 para meses reais), o que e verdade para os feriados usados aqui.
func nthWeekdayOfMonth(year, month int, weekday time.Weekday, n int) int {
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	offset := (int(weekday) - int(first.Weekday()) + 7) % 7
	return 1 + offset + (n-1)*7
}

// ============================================================================
// Helpers
// ============================================================================

// setEnabled diz se o conjunto esta ligado na config da conta.
func setEnabled(cfg CalendarConfig, set string) bool {
	switch set {
	case setBrNational:
		return cfg.Holidays.BrNational
	case setSergipe:
		return cfg.Holidays.Sergipe
	case setAracaju:
		return cfg.Holidays.Aracaju
	case setLuxuryIntl:
		return cfg.Holidays.LuxuryIntl
	default:
		return false
	}
}

// ymd formata (ano, mes, dia) como 'YYYY-MM-DD'.
func ymd(y, m, d int) string {
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

// fromTime formata um time.Time como 'YYYY-MM-DD'.
func fromTime(t time.Time) string {
	return ymd(t.Year(), int(t.Month()), t.Day())
}

// yearOf extrai o ano de uma data 'YYYY-MM-DD' (ok=false se malformada).
func yearOf(date string) (int, bool) {
	if !dateRe.MatchString(date) {
		return 0, false
	}
	var y, m, d int
	if _, err := fmt.Sscanf(date, "%d-%d-%d", &y, &m, &d); err != nil {
		return 0, false
	}
	return y, true
}
