package erp

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (repository *PostgresRepository) ListConsultantERPLinks(ctx context.Context, store StoreScope, employeeIDs []string) (ConsultantERPLinksResponse, error) {
	manualLinks, err := repository.listCRMManualConsultantLinks(ctx, store.TenantID)
	if err != nil {
		return ConsultantERPLinksResponse{}, err
	}
	profiles, err := repository.listCRMConsultantLinkProfiles(ctx, store.TenantID)
	if err != nil {
		return ConsultantERPLinksResponse{}, err
	}
	employees, err := repository.listCRMERPEmployeeLinkCandidates(ctx, store, employeeIDs)
	if err != nil {
		return ConsultantERPLinksResponse{}, err
	}

	response := ConsultantERPLinksResponse{
		Store:       store,
		Employees:   make([]ConsultantERPLinkEmployeeRow, 0, len(employees)),
		Consultants: make([]ConsultantERPLinkConsultantOption, 0, len(profiles)),
	}

	for _, profile := range profiles {
		response.Consultants = append(response.Consultants, ConsultantERPLinkConsultantOption{
			ConsultantID:   profile.ConsultantID,
			ConsultantName: profile.ConsultantName,
			StoreID:        profile.StoreID,
			StoreCode:      profile.StoreCode,
			StoreName:      profile.StoreName,
			EmployeeCode:   profile.EmployeeCode,
		})
	}

	for _, employee := range employees {
		link := resolveCRMConsultantLink(employee.ERPStoreCode, employee.ERPEmployeeID, employee.ERPEmployeeName, manualLinks, profiles)
		manualLink, hasManualLink := findCRMManualConsultantLink(employee.ERPStoreCode, employee.ERPEmployeeID, manualLinks)
		row := ConsultantERPLinkEmployeeRow{
			ERPEmployeeID:        employee.ERPEmployeeID,
			ERPEmployeeName:      employee.ERPEmployeeName,
			ERPStoreCode:         employee.ERPStoreCode,
			ERPStoreLabel:        employee.ERPStoreLabel,
			ERPStoreRawCode:      employee.ERPStoreRawCode,
			LinkedConsultantID:   link.Profile.ConsultantID,
			LinkedConsultantName: link.Profile.ConsultantName,
			LinkedStoreID:        link.Profile.StoreID,
			LinkedStoreName:      link.Profile.StoreName,
			LinkStatus:           link.Status,
			LinkConfidence:       link.Confidence,
			LinkCandidates:       link.Candidates,
		}
		if hasManualLink {
			row.LinkID = manualLink.LinkID
			row.Note = manualLink.Note
		}
		response.Employees = append(response.Employees, row)
	}

	sort.Slice(response.Employees, func(left int, right int) bool {
		if response.Employees[left].ERPStoreLabel != response.Employees[right].ERPStoreLabel {
			return response.Employees[left].ERPStoreLabel < response.Employees[right].ERPStoreLabel
		}
		return response.Employees[left].ERPEmployeeName < response.Employees[right].ERPEmployeeName
	})

	return response, nil
}

func (repository *PostgresRepository) UpsertConsultantERPLink(ctx context.Context, store StoreScope, input ConsultantERPLinkUpsertInput, userID string) error {
	erpEmployeeID := strings.TrimSpace(input.ERPEmployeeID)
	consultantID := strings.TrimSpace(input.ConsultantID)
	if erpEmployeeID == "" || consultantID == "" {
		return ErrValidation
	}

	erpStoreCode := normalizeCRMManualStoreCode(input.ERPStoreCode)
	erpEmployeeName := strings.TrimSpace(input.ERPEmployeeName)
	note := strings.TrimSpace(input.Note)
	userID = strings.TrimSpace(userID)

	var consultantStoreID string
	err := repository.pool.QueryRow(ctx, `
		select store_id::text
		from queue.consultants
		where tenant_id = $1::uuid
		  and id = $2::uuid
		  and is_active = true
		limit 1;
	`, store.TenantID, consultantID).Scan(&consultantStoreID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrValidation
		}
		return err
	}

	tag, err := repository.pool.Exec(ctx, `
		update consultant_erp_links
		set
			store_id = $4::uuid,
			consultant_id = $5::uuid,
			erp_employee_name = $6,
			note = $7,
			is_active = true,
			updated_by_user_id = nullif($8, '')::uuid,
			updated_at = now()
		where tenant_id = $1::uuid
		  and is_active = true
		  and lower(trim(erp_store_code)) = lower(trim($2))
		  and lower(trim(erp_employee_id)) = lower(trim($3));
	`, store.TenantID, erpStoreCode, erpEmployeeID, consultantStoreID, consultantID, erpEmployeeName, note, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	_, err = repository.pool.Exec(ctx, `
		insert into consultant_erp_links (
			tenant_id,
			store_id,
			consultant_id,
			erp_store_code,
			erp_employee_id,
			erp_employee_name,
			note,
			created_by_user_id,
			updated_by_user_id
		)
		values (
			$1::uuid,
			$2::uuid,
			$3::uuid,
			$4,
			$5,
			$6,
			$7,
			nullif($8, '')::uuid,
			nullif($8, '')::uuid
		);
	`, store.TenantID, consultantStoreID, consultantID, erpStoreCode, erpEmployeeID, erpEmployeeName, note, userID)
	return err
}

func (repository *PostgresRepository) AutoLinkConsultantERP(ctx context.Context, store StoreScope, userID string, employeeIDs []string) error {
	manualLinks, err := repository.listCRMManualConsultantLinks(ctx, store.TenantID)
	if err != nil {
		return err
	}
	profiles, err := repository.listCRMConsultantLinkProfiles(ctx, store.TenantID)
	if err != nil {
		return err
	}
	employees, err := repository.listCRMERPEmployeeLinkCandidates(ctx, store, employeeIDs)
	if err != nil {
		return err
	}

	for _, input := range buildAutoConsultantERPLinkInputs(manualLinks, profiles, employees) {
		if err := repository.UpsertConsultantERPLink(ctx, store, input, userID); err != nil {
			return err
		}
	}

	return nil
}

func (repository *PostgresRepository) DeleteConsultantERPLink(ctx context.Context, store StoreScope, linkID string, userID string) error {
	linkID = strings.TrimSpace(linkID)
	if linkID == "" {
		return ErrValidation
	}

	tag, err := repository.pool.Exec(ctx, `
		update consultant_erp_links
		set
			is_active = false,
			updated_by_user_id = nullif($3, '')::uuid,
			updated_at = now()
		where tenant_id = $1::uuid
		  and id = $2::uuid
		  and is_active = true;
	`, store.TenantID, linkID, strings.TrimSpace(userID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrValidation
	}
	return nil
}

func (repository *PostgresRepository) listCRMERPEmployeeLinkCandidates(ctx context.Context, store StoreScope, employeeIDs []string) ([]crmERPEmployeeLinkCandidate, error) {
	normalizedEmployeeIDs := normalizeCRMEmployeeIDFilters(employeeIDs)

	query := `
		with latest as (
			select distinct on (
				lower(trim(original_id)),
				coalesce(nullif(trim(store_id_raw), ''), nullif(trim(store_cnpj), ''), nullif(trim(store_code), ''), '')
			)
				trim(original_id) as erp_employee_id,
				coalesce(nullif(trim(name), ''), 'Funcionario ERP ' || trim(original_id)) as erp_employee_name,
				coalesce(nullif(trim(store_id_raw), ''), '') as store_id_raw,
				coalesce(nullif(trim(store_code), ''), '') as store_code,
				coalesce(nullif(trim(store_cnpj), ''), '') as store_cnpj
			from erp_employee_raw
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
			  and nullif(trim(original_id), '') is not null
	`
	args := []any{store.TenantID, store.StoreID}
	if len(normalizedEmployeeIDs) > 0 {
		query += `
			  and lower(trim(original_id)) = any($3::text[])
		`
		args = append(args, normalizedEmployeeIDs)
	}
	query += `
			order by
				lower(trim(original_id)),
				coalesce(nullif(trim(store_id_raw), ''), nullif(trim(store_cnpj), ''), nullif(trim(store_code), ''), ''),
				source_batch_date desc,
				created_at_imported desc,
				id desc
		)
		select erp_employee_id, erp_employee_name, store_id_raw, store_code, store_cnpj
		from latest;
	`
	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	employees := make([]crmERPEmployeeLinkCandidate, 0, 64)
	for rows.Next() {
		var employeeID string
		var employeeName string
		var rawStoreCode string
		var sourceStoreCode string
		var storeCNPJ string
		if err := rows.Scan(&employeeID, &employeeName, &rawStoreCode, &sourceStoreCode, &storeCNPJ); err != nil {
			return nil, err
		}

		storeKey := resolveCRMEmployeeLinkStoreKey(rawStoreCode, sourceStoreCode, storeCNPJ)
		employees = append(employees, crmERPEmployeeLinkCandidate{
			ERPEmployeeID:   strings.TrimSpace(employeeID),
			ERPEmployeeName: strings.TrimSpace(employeeName),
			ERPStoreRawCode: strings.TrimSpace(rawStoreCode),
			ERPStoreCode:    storeKey,
			ERPStoreLabel:   formatCRMEmployeeLinkStoreLabel(storeKey),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return employees, nil
}

func normalizeCRMEmployeeIDFilters(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		key := normalizeCRMEmployeeID(value)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func findCRMManualConsultantLink(storeCode string, employeeID string, manualLinks map[string]crmConsultantManualLink) (crmConsultantManualLink, bool) {
	for _, key := range []string{
		crmManualConsultantLinkKey(storeCode, employeeID),
		crmManualConsultantLinkKey("", employeeID),
	} {
		if key == "" {
			continue
		}
		if link, ok := manualLinks[key]; ok {
			return link, true
		}
	}
	return crmConsultantManualLink{}, false
}

func buildAutoConsultantERPLinkInputs(manualLinks map[string]crmConsultantManualLink, profiles []crmConsultantLinkProfile, employees []crmERPEmployeeLinkCandidate) []ConsultantERPLinkUpsertInput {
	inputs := make([]ConsultantERPLinkUpsertInput, 0, len(employees))

	for _, employee := range employees {
		if _, ok := findCRMManualConsultantLink(employee.ERPStoreCode, employee.ERPEmployeeID, manualLinks); ok {
			continue
		}

		link := resolveCRMConsultantLink(employee.ERPStoreCode, employee.ERPEmployeeID, employee.ERPEmployeeName, manualLinks, profiles)
		if link.Profile.ConsultantID == "" {
			continue
		}
		if link.Status != crmConsultantLinkStatusEmployeeCode && link.Status != crmConsultantLinkStatusNameExact {
			continue
		}

		inputs = append(inputs, ConsultantERPLinkUpsertInput{
			ERPStoreCode:    employee.ERPStoreCode,
			ERPEmployeeID:   employee.ERPEmployeeID,
			ERPEmployeeName: employee.ERPEmployeeName,
			ConsultantID:    link.Profile.ConsultantID,
			Note:            crmConsultantAutoLinkNote(link.Status),
		})
	}

	return inputs
}

func resolveCRMEmployeeLinkStoreKey(rawStoreCode string, sourceStoreCode string, storeCNPJ string) string {
	for _, value := range []string{rawStoreCode, storeCNPJ, sourceStoreCode} {
		if digits := onlyDigits(value); digits != "" {
			if _, ok := resolveCRMStoreAlias(digits); ok {
				return digits
			}
		}
		if storeKey := crmStoreKeyFromOperationalStore(value, value); storeKey != "" {
			return storeKey
		}
	}
	return ""
}

func formatCRMEmployeeLinkStoreLabel(storeKey string) string {
	if alias, ok := resolveCRMStoreAlias(storeKey); ok {
		return alias.Label
	}
	return "Loja nao identificada"
}
