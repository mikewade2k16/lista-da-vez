package commission

import "encoding/json"

// ParsePolicyJSON desserializa o JSONB cru da politica e aplica NormalizePolicy
// (defaults v2 + retrocompat com o campo legado "manager"). So stdlib.
//
// O consumidor (ex.: crm/erp) passa o conteudo bruto de
// queue.tenant_operation_core_settings.crm_goal_payout_policy. Em caso de JSON
// invalido, retorna o erro — cabe ao chamador cair na DefaultPolicy().
func ParsePolicyJSON(raw []byte) (Policy, error) {
	if len(raw) == 0 {
		return DefaultPolicy(), nil
	}

	var policy Policy
	if err := json.Unmarshal(raw, &policy); err != nil {
		return Policy{}, err
	}

	return NormalizePolicy(policy), nil
}
