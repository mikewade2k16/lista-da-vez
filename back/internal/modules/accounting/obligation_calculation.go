package accounting

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"time"
)

var obligationRate = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,6})?$`)

func obligationPeriod(value string) (string, string, bool) {
	layout := "2006-01"
	if len(value) == 4 {
		layout = "2006"
	}
	d, e := time.Parse(layout, value)
	if e != nil {
		return "", "", false
	}
	end := d.AddDate(0, 1, 0).AddDate(0, 0, -1)
	if layout == "2006" {
		end = d.AddDate(1, 0, 0).AddDate(0, 0, -1)
	}
	return d.Format("2006-01-02"), end.Format("2006-01-02"), true
}
func obligationCode(value string) bool {
	switch value {
	case "mei.das", "mei.dasn_simei", "company.tax_review", "company.declaration_review":
		return true
	}
	return false
}
func obligationCalculation(base string, p ObligationParameters) (string, json.RawMessage, error) {
	if !moneyPattern.MatchString(base) || !moneyPattern.MatchString(p.Fixed) || !obligationRate.MatchString(p.Rate) || p.Formula != "base_percent_fixed_v1" || p.Rounding != "half_up_2_v1" {
		return "", nil, failure("missing_rule", 422)
	}
	b, _ := new(big.Rat).SetString(base)
	rate, _ := new(big.Rat).SetString(p.Rate)
	fixed, _ := new(big.Rat).SetString(p.Fixed)
	if rate.Cmp(big.NewRat(100, 1)) > 0 {
		return "", nil, failure("unsupported_rate", 422)
	}
	exact := new(big.Rat).Mul(b, rate)
	exact.Quo(exact, big.NewRat(100, 1))
	exact.Add(exact, fixed)
	cents := new(big.Rat).Mul(exact, big.NewRat(100, 1))
	rounded, rem := new(big.Int), new(big.Int)
	rounded.QuoRem(cents.Num(), cents.Denom(), rem)
	if new(big.Int).Mul(rem, big.NewInt(2)).Cmp(cents.Denom()) >= 0 {
		rounded.Add(rounded, big.NewInt(1))
	}
	whole, rest := new(big.Int), new(big.Int)
	whole.QuoRem(rounded, big.NewInt(100), rest)
	amount := fmt.Sprintf("%s.%02d", whole.String(), rest.Int64())
	if !moneyPattern.MatchString(amount) {
		return "", nil, failure("amount_overflow", 422)
	}
	memory, e := json.Marshal(map[string]string{"base": base, "ratePercent": p.Rate, "fixed": p.Fixed, "formula": p.Formula, "rounding": p.Rounding, "unroundedRational": exact.RatString(), "amount": amount})
	return amount, memory, e
}
