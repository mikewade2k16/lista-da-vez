package erp

import (
	"context"
	"strings"
)

func crmPrimaryStoreKeyFromSlug(slug string) string {
	switch strings.TrimSpace(slug) {
	case "riomar":
		return "12583959000186"
	case "jardins":
		return "56173889000163"
	case "garcia":
		return "53578278000107"
	case "treze":
		return "43068099000176"
	default:
		return ""
	}
}

func crmStoreKeyFromOperationalStore(code string, name string) string {
	slug, _ := crmStoreSlugFromOperationalStore(code, name)
	return crmPrimaryStoreKeyFromSlug(slug)
}

func crmEmployeeSpecialStoreKey(employeeID string) string {
	return strings.TrimSpace(crmEmployeeSpecialStoreKeys[strings.TrimSpace(employeeID)])
}

func resolveCRMOrderStoreKey(explicitStoreKey string, fallbackStoreCNPJ string, employeeID string, employeeStoreFallbacks map[string]string, employeeDominantStoreKeys map[string]string) string {
	if normalized := onlyDigits(strings.TrimSpace(explicitStoreKey)); normalized != "" {
		return normalized
	}

	normalizedEmployeeID := strings.TrimSpace(employeeID)
	if normalizedEmployeeID != "" {
		if specialKey := crmEmployeeSpecialStoreKey(normalizedEmployeeID); specialKey != "" {
			return specialKey
		}
		if normalized := onlyDigits(employeeStoreFallbacks[normalizedEmployeeID]); normalized != "" {
			return normalized
		}
		if normalized := onlyDigits(employeeDominantStoreKeys[normalizedEmployeeID]); normalized != "" {
			return normalized
		}
	}

	return onlyDigits(strings.TrimSpace(fallbackStoreCNPJ))
}

func (repository *PostgresRepository) listCRMStoreTargets(ctx context.Context, tenantID string) (map[string]crmStoreTarget, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			code,
			name,
			coalesce(round(monthly_goal * 100), 0)::bigint,
			coalesce(round(avg_ticket_goal * 100), 0)::bigint,
			coalesce(pa_goal, 0)::float8
		from stores
		where tenant_id = $1::uuid
		  and is_active = true;
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := make(map[string]crmStoreTarget, 4)
	for rows.Next() {
		var (
			code               string
			name               string
			monthlyGoalCents   int64
			avgTicketGoalCents int64
			paGoal             float64
		)
		if err := rows.Scan(&code, &name, &monthlyGoalCents, &avgTicketGoalCents, &paGoal); err != nil {
			return nil, err
		}

		slug, label := crmStoreSlugFromOperationalStore(code, name)
		if slug == "" {
			continue
		}

		targets[slug] = crmStoreTarget{
			Slug:               slug,
			Label:              label,
			Code:               code,
			Name:               name,
			MonthlyGoalCents:   monthlyGoalCents,
			AvgTicketGoalCents: avgTicketGoalCents,
			PAGoal:             paGoal,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return targets, nil
}

func (repository *PostgresRepository) listCRMEmployeeStoreFallbacks(ctx context.Context, tenantID string) (map[string]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			trim(u.employee_code) as employee_code,
			coalesce(max(nullif(consultant_store.code, '')), max(nullif(role_store.code, '')), '') as store_code,
			coalesce(max(nullif(consultant_store.name, '')), max(nullif(role_store.name, '')), '') as store_name
		from users u
		left join consultants c
			on c.user_id = u.id
		   and c.tenant_id = $1::uuid
		left join stores consultant_store
			on consultant_store.id = c.store_id
		   and consultant_store.tenant_id = $1::uuid
		left join user_store_roles usr
			on usr.user_id = u.id
		left join stores role_store
			on role_store.id = usr.store_id
		   and role_store.tenant_id = $1::uuid
		where nullif(trim(u.employee_code), '') is not null
		group by trim(u.employee_code);
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fallbacks := make(map[string]string)
	for rows.Next() {
		var employeeCode string
		var storeCode string
		var storeName string
		if err := rows.Scan(&employeeCode, &storeCode, &storeName); err != nil {
			return nil, err
		}

		employeeCode = strings.TrimSpace(employeeCode)
		if employeeCode == "" {
			continue
		}

		if storeKey := crmStoreKeyFromOperationalStore(storeCode, storeName); storeKey != "" {
			fallbacks[employeeCode] = storeKey
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return fallbacks, nil
}

func (repository *PostgresRepository) listCRMDominantEmployeeStoreKeys(ctx context.Context, store StoreScope) (map[string]string, error) {
	rows, err := repository.pool.Query(ctx, `
		with known_orders as (
			select
				order_id,
				coalesce(max(nullif(trim(store_id_raw), '')), '') as explicit_store_key,
				coalesce(max(nullif(trim(employee_id), '')), '') as employee_id,
				case
					when max(total_amount_cents) > 0 then max(total_amount_cents)::bigint
					else sum(amount_cents)::bigint
				end as order_total_cents
			from erp_order_raw
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
			  and nullif(trim(order_id), '') is not null
			  and coalesce(nullif(trim(store_id_raw), ''), '') <> ''
			group by order_id
		), canceled_orders as (
			select distinct order_id
			from erp_order_canceled_raw
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
		), active_known_orders as (
			select *
			from known_orders known
			where not exists (
				select 1
				from canceled_orders canceled
				where canceled.order_id = known.order_id
			)
		), ranked as (
			select
				employee_id,
				explicit_store_key,
				row_number() over (
					partition by employee_id
					order by count(*) desc, sum(order_total_cents) desc, explicit_store_key asc
				) as row_number
			from active_known_orders
			where employee_id <> ''
			group by employee_id, explicit_store_key
		)
		select employee_id, explicit_store_key
		from ranked
		where row_number = 1;
	`, store.TenantID, store.StoreID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dominantKeys := make(map[string]string)
	for rows.Next() {
		var employeeID string
		var storeKey string
		if err := rows.Scan(&employeeID, &storeKey); err != nil {
			return nil, err
		}

		employeeID = strings.TrimSpace(employeeID)
		storeKey = onlyDigits(strings.TrimSpace(storeKey))
		if employeeID == "" || storeKey == "" {
			continue
		}

		dominantKeys[employeeID] = storeKey
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return dominantKeys, nil
}

func (repository *PostgresRepository) listCRMEmployeeNames(ctx context.Context, store StoreScope) (map[string]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select distinct on (original_id)
			original_id,
			name
		from erp_employee_raw
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  and nullif(trim(original_id), '') is not null
		order by original_id, created_at_imported desc, source_batch_date desc, id desc;
	`, store.TenantID, store.StoreID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make(map[string]string)
	for rows.Next() {
		var employeeID string
		var name string
		if err := rows.Scan(&employeeID, &name); err != nil {
			return nil, err
		}

		employeeID = strings.TrimSpace(employeeID)
		name = strings.TrimSpace(name)
		if employeeID == "" || name == "" {
			continue
		}

		names[employeeID] = name
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return names, nil
}

func (repository *PostgresRepository) listCRMManualConsultantLinks(ctx context.Context, tenantID string) (map[string]crmConsultantManualLink, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			l.id::text,
			trim(l.erp_employee_id) as erp_employee_id,
			coalesce(nullif(trim(l.erp_employee_name), ''), '') as erp_employee_name,
			coalesce(nullif(trim(l.erp_store_code), ''), '') as erp_store_code,
			coalesce(nullif(trim(l.note), ''), '') as note,
			c.id::text,
			c.name,
			coalesce(c.user_id::text, '') as user_id,
			s.id::text,
			s.code,
			s.name,
			coalesce(nullif(trim(u.employee_code), ''), '') as employee_code
		from consultant_erp_links l
		join consultants c
			on c.id = l.consultant_id
		   and c.tenant_id = l.tenant_id
		join stores s
			on s.id = coalesce(l.store_id, c.store_id)
		   and s.tenant_id = l.tenant_id
		left join users u
			on u.id = c.user_id
		where l.tenant_id = $1::uuid
		  and l.is_active = true
		  and c.is_active = true;
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := make(map[string]crmConsultantManualLink)
	for rows.Next() {
		var link crmConsultantManualLink
		if err := rows.Scan(
			&link.LinkID,
			&link.ERPEmployeeID,
			&link.ERPEmployeeName,
			&link.ERPStoreCode,
			&link.Note,
			&link.Profile.ConsultantID,
			&link.Profile.ConsultantName,
			&link.Profile.UserID,
			&link.Profile.StoreID,
			&link.Profile.StoreCode,
			&link.Profile.StoreName,
			&link.Profile.EmployeeCode,
		); err != nil {
			return nil, err
		}

		key := crmManualConsultantLinkKey(link.ERPStoreCode, link.ERPEmployeeID)
		if key == "" {
			continue
		}
		links[key] = link
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return links, nil
}

func (repository *PostgresRepository) listCRMConsultantLinkProfiles(ctx context.Context, tenantID string) ([]crmConsultantLinkProfile, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			c.id::text,
			c.name,
			coalesce(c.user_id::text, '') as user_id,
			c.store_id::text,
			s.code,
			s.name,
			coalesce(nullif(trim(u.employee_code), ''), '') as employee_code
		from consultants c
		join stores s
			on s.id = c.store_id
		   and s.tenant_id = c.tenant_id
		left join users u
			on u.id = c.user_id
		where c.tenant_id = $1::uuid
		  and c.is_active = true
		order by c.name asc;
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := make([]crmConsultantLinkProfile, 0, 64)
	for rows.Next() {
		var profile crmConsultantLinkProfile
		if err := rows.Scan(
			&profile.ConsultantID,
			&profile.ConsultantName,
			&profile.UserID,
			&profile.StoreID,
			&profile.StoreCode,
			&profile.StoreName,
			&profile.EmployeeCode,
		); err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return profiles, nil
}

func resolveCRMConsultantLink(storeKey string, employeeID string, consultantName string, manualLinks map[string]crmConsultantManualLink, profiles []crmConsultantLinkProfile) crmConsultantResolvedLink {
	normalizedStoreKey := normalizeCRMManualStoreCode(storeKey)
	employeeKey := normalizeCRMEmployeeID(employeeID)
	if employeeKey != "" {
		for _, key := range []string{
			crmManualConsultantLinkKey(normalizedStoreKey, employeeKey),
			crmManualConsultantLinkKey("", employeeKey),
		} {
			if key == "" {
				continue
			}
			if manualLink, ok := manualLinks[key]; ok {
				return crmConsultantResolvedLink{
					Profile:    manualLink.Profile,
					Status:     crmConsultantManualLinkStatus(manualLink),
					Confidence: 1,
					Candidates: 1,
				}
			}
		}

		employeeMatches := make([]crmConsultantLinkProfile, 0, 1)
		for _, profile := range profiles {
			if normalizeCRMEmployeeID(profile.EmployeeCode) == employeeKey {
				employeeMatches = append(employeeMatches, profile)
			}
		}
		if len(employeeMatches) > 1 {
			sameStoreMatches := filterCRMProfilesByStoreKey(employeeMatches, normalizedStoreKey)
			if len(sameStoreMatches) == 1 {
				return crmConsultantResolvedLink{
					Profile:    sameStoreMatches[0],
					Status:     crmConsultantLinkStatusEmployeeCode,
					Confidence: 0.9,
					Candidates: len(employeeMatches),
				}
			}
			if len(sameStoreMatches) > 1 {
				return crmConsultantResolvedLink{
					Status:     crmConsultantLinkStatusAmbiguous,
					Confidence: 0.4,
					Candidates: len(sameStoreMatches),
				}
			}
		}
		if len(employeeMatches) == 1 {
			return crmConsultantResolvedLink{
				Profile:    employeeMatches[0],
				Status:     crmConsultantLinkStatusEmployeeCode,
				Confidence: 0.95,
				Candidates: 1,
			}
		}
		if len(employeeMatches) > 1 {
			return crmConsultantResolvedLink{
				Status:     crmConsultantLinkStatusAmbiguous,
				Confidence: 0.4,
				Candidates: len(employeeMatches),
			}
		}
	}

	nameKey := normalizeCRMConsultantName(consultantName)
	if nameKey != "" {
		nameMatches := make([]crmConsultantLinkProfile, 0, 1)
		for _, profile := range profiles {
			if normalizeCRMConsultantName(profile.ConsultantName) == nameKey {
				nameMatches = append(nameMatches, profile)
			}
		}
		if len(nameMatches) > 1 {
			sameStoreMatches := filterCRMProfilesByStoreKey(nameMatches, normalizedStoreKey)
			if len(sameStoreMatches) == 1 {
				return crmConsultantResolvedLink{
					Profile:    sameStoreMatches[0],
					Status:     crmConsultantLinkStatusNameExact,
					Confidence: 0.8,
					Candidates: len(nameMatches),
				}
			}
			if len(sameStoreMatches) > 1 {
				return crmConsultantResolvedLink{
					Status:     crmConsultantLinkStatusAmbiguous,
					Confidence: 0.35,
					Candidates: len(sameStoreMatches),
				}
			}
		}
		if len(nameMatches) == 1 {
			return crmConsultantResolvedLink{
				Profile:    nameMatches[0],
				Status:     crmConsultantLinkStatusNameExact,
				Confidence: 0.85,
				Candidates: 1,
			}
		}
		if len(nameMatches) > 1 {
			return crmConsultantResolvedLink{
				Status:     crmConsultantLinkStatusAmbiguous,
				Confidence: 0.35,
				Candidates: len(nameMatches),
			}
		}
	}

	return crmConsultantResolvedLink{
		Status: crmConsultantLinkStatusUnmatched,
	}
}

func normalizeCRMEmployeeID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func crmConsultantManualLinkStatus(link crmConsultantManualLink) string {
	switch normalizeCRMEmployeeID(link.Note) {
	case crmConsultantLinkNoteAutoEmployee:
		return crmConsultantLinkStatusEmployeeCode
	case crmConsultantLinkNoteAutoName:
		return crmConsultantLinkStatusNameExact
	default:
		return crmConsultantLinkStatusManual
	}
}

func crmConsultantAutoLinkNote(status string) string {
	switch strings.TrimSpace(status) {
	case crmConsultantLinkStatusEmployeeCode:
		return crmConsultantLinkNoteAutoEmployee
	case crmConsultantLinkStatusNameExact:
		return crmConsultantLinkNoteAutoName
	default:
		return ""
	}
}

func crmManualConsultantLinkKey(storeCode string, employeeID string) string {
	employeeKey := normalizeCRMEmployeeID(employeeID)
	if employeeKey == "" {
		return ""
	}
	return normalizeCRMManualStoreCode(storeCode) + "\x00" + employeeKey
}

func normalizeCRMManualStoreCode(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if digits := onlyDigits(normalized); len(digits) >= 8 {
		return digits
	}
	return normalized
}

func filterCRMProfilesByStoreKey(profiles []crmConsultantLinkProfile, storeKey string) []crmConsultantLinkProfile {
	storeKey = normalizeCRMManualStoreCode(storeKey)
	if storeKey == "" {
		return nil
	}

	matches := make([]crmConsultantLinkProfile, 0, len(profiles))
	for _, profile := range profiles {
		if normalizeCRMManualStoreCode(crmStoreKeyFromOperationalStore(profile.StoreCode, profile.StoreName)) == storeKey {
			matches = append(matches, profile)
		}
	}
	return matches
}

func normalizeCRMConsultantName(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	var builder strings.Builder
	lastWasSpace := true
	for _, char := range normalized {
		char = normalizeCRMConsultantNameRune(char)
		isAlphaNum := (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')
		if isAlphaNum {
			builder.WriteRune(char)
			lastWasSpace = false
			continue
		}
		if !lastWasSpace {
			builder.WriteByte(' ')
			lastWasSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func normalizeCRMConsultantNameRune(char rune) rune {
	switch char {
	case '\u00C1', '\u00C0', '\u00C2', '\u00C3', '\u00C4':
		return 'A'
	case '\u00C9', '\u00C8', '\u00CA', '\u00CB':
		return 'E'
	case '\u00CD', '\u00CC', '\u00CE', '\u00CF':
		return 'I'
	case '\u00D3', '\u00D2', '\u00D4', '\u00D5', '\u00D6':
		return 'O'
	case '\u00DA', '\u00D9', '\u00DB', '\u00DC':
		return 'U'
	case '\u00C7':
		return 'C'
	case '\u00D1':
		return 'N'
	default:
		return char
	}
}
