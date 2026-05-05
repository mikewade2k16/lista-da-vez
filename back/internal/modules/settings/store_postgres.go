package settings

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const (
	optionKindVisitReason    = "visit_reason"
	optionKindCustomerSource = "customer_source"
	optionKindPauseReason    = "pause_reason"
	optionKindCancelReason   = "cancel_reason"
	optionKindStopReason     = "stop_reason"
	optionKindQueueJump      = "queue_jump_reason"
	optionKindLossReason     = "loss_reason"
	optionKindProfession     = "profession"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

type execQueryer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) TenantExists(ctx context.Context, tenantID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists(
			select 1
			from tenants
			where id = $1::uuid
		);
	`, tenantID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (repository *PostgresRepository) CanAccessTenant(ctx context.Context, principal auth.Principal, tenantID string) (bool, error) {
	normalizedTenantID := strings.TrimSpace(tenantID)
	if normalizedTenantID == "" {
		return false, nil
	}

	if principalTenantID := strings.TrimSpace(principal.TenantID); principalTenantID != "" {
		return principalTenantID == normalizedTenantID, nil
	}

	var (
		query string
		args  []any
	)

	switch principal.Role {
	case auth.RolePlatformAdmin:
		query = `
			select exists(
				select 1
				from tenants t
				where t.id::text = $1
					and t.is_active = true
			);
		`
		args = []any{normalizedTenantID}
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		query = `
			select exists(
				select 1
				from tenants t
				join user_tenant_roles utr on utr.tenant_id = t.id
				where t.id::text = $1
					and utr.user_id::text = $2
					and t.is_active = true
			);
		`
		args = []any{normalizedTenantID, strings.TrimSpace(principal.UserID)}
	default:
		query = `
			select exists(
				select 1
				from tenants t
				join stores s on s.tenant_id = t.id
				join user_store_roles usr on usr.store_id = s.id
				where t.id::text = $1
					and usr.user_id::text = $2
					and t.is_active = true
					and s.is_active = true
			);
		`
		args = []any{normalizedTenantID, strings.TrimSpace(principal.UserID)}
	}

	var allowed bool
	if err := repository.pool.QueryRow(ctx, query, args...).Scan(&allowed); err != nil {
		return false, err
	}

	return allowed, nil
}

func (repository *PostgresRepository) ResolveDefaultTenantID(ctx context.Context, principal auth.Principal) (string, error) {
	if tenantID := strings.TrimSpace(principal.TenantID); tenantID != "" {
		return tenantID, nil
	}

	var (
		query string
		args  []any
	)

	switch principal.Role {
	case auth.RolePlatformAdmin:
		query = `
			select t.id::text
			from tenants t
			where t.is_active = true
			order by t.name asc, t.created_at asc, t.id asc
			limit 2;
		`
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		query = `
			select distinct t.id::text
			from tenants t
			join user_tenant_roles utr on utr.tenant_id = t.id
			where utr.user_id = $1::uuid
				and t.is_active = true
			order by t.id asc
			limit 2;
		`
		args = []any{principal.UserID}
	default:
		query = `
			select distinct t.id::text
			from tenants t
			join stores s on s.tenant_id = t.id
			join user_store_roles usr on usr.store_id = s.id
			where usr.user_id = $1::uuid
				and t.is_active = true
				and s.is_active = true
			order by t.id asc
			limit 2;
		`
		args = []any{principal.UserID}
	}

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	tenantIDs := make([]string, 0, 2)
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return "", err
		}

		tenantIDs = append(tenantIDs, tenantID)
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	if len(tenantIDs) != 1 {
		return "", ErrTenantRequired
	}

	return tenantIDs[0], nil
}

func (repository *PostgresRepository) GetByTenant(ctx context.Context, tenantID string) (Record, bool, error) {
	operationSection, found, err := repository.GetOperationSection(ctx, tenantID)
	if err != nil {
		return Record{}, false, err
	}
	if !found {
		return Record{}, false, nil
	}

	modalSection, modalFound, err := repository.GetModalSection(ctx, tenantID)
	if err != nil {
		return Record{}, false, err
	}
	if !modalFound {
		modalSection = defaultModalSectionRecord(tenantID, operationSection.SelectedOperationTemplateID)
		modalSection.CreatedAt = operationSection.CreatedAt
		modalSection.UpdatedAt = operationSection.UpdatedAt
	}

	visitReasonOptions, err := repository.GetOptionGroup(ctx, tenantID, optionKindVisitReason)
	if err != nil {
		return Record{}, false, err
	}

	customerSourceOptions, err := repository.GetOptionGroup(ctx, tenantID, optionKindCustomerSource)
	if err != nil {
		return Record{}, false, err
	}

	pauseReasonOptions, err := repository.GetOptionGroup(ctx, tenantID, optionKindPauseReason)
	if err != nil {
		return Record{}, false, err
	}

	cancelReasonOptions, err := repository.GetOptionGroup(ctx, tenantID, optionKindCancelReason)
	if err != nil {
		return Record{}, false, err
	}

	stopReasonOptions, err := repository.GetOptionGroup(ctx, tenantID, optionKindStopReason)
	if err != nil {
		return Record{}, false, err
	}

	queueJumpReasonOptions, err := repository.GetOptionGroup(ctx, tenantID, optionKindQueueJump)
	if err != nil {
		return Record{}, false, err
	}

	lossReasonOptions, err := repository.GetOptionGroup(ctx, tenantID, optionKindLossReason)
	if err != nil {
		return Record{}, false, err
	}

	professionOptions, err := repository.GetOptionGroup(ctx, tenantID, optionKindProfession)
	if err != nil {
		return Record{}, false, err
	}

	products, err := repository.GetProductCatalog(ctx, tenantID)
	if err != nil {
		return Record{}, false, err
	}

	return Record{
		TenantID:                    tenantID,
		SelectedOperationTemplateID: operationSection.SelectedOperationTemplateID,
		Settings:                    composeAppSettings(operationSection.CoreSettings, operationSection.AlertSettings),
		ModalConfig:                 modalSection.ModalConfig,
		VisitReasonOptions:          visitReasonOptions,
		CustomerSourceOptions:       customerSourceOptions,
		PauseReasonOptions:          pauseReasonOptions,
		CancelReasonOptions:         cancelReasonOptions,
		StopReasonOptions:           stopReasonOptions,
		QueueJumpReasonOptions:      queueJumpReasonOptions,
		LossReasonOptions:           lossReasonOptions,
		ProfessionOptions:           professionOptions,
		ProductCatalog:              products,
		CreatedAt:                   operationSection.CreatedAt,
		UpdatedAt:                   operationSection.UpdatedAt,
	}, true, nil
}

// Upsert salva o bundle completo nas tabelas novas.
// Fase 9: escrita legada em tenant_operation_settings removida;
// a linha ancora FK permanece via ensureConfigRow para opcoes e catalogo.
func (repository *PostgresRepository) Upsert(ctx context.Context, record Record) (Record, error) {
	coreSettings, alertSettings := splitAppSettings(record.Settings)
	operationSection := normalizeOperationSectionRecord(OperationSectionRecord{
		TenantID:                    record.TenantID,
		SelectedOperationTemplateID: record.SelectedOperationTemplateID,
		CoreSettings:                coreSettings,
		AlertSettings:               alertSettings,
	})
	modalSection := normalizeModalSectionRecord(ModalSectionRecord{
		TenantID:                    record.TenantID,
		SelectedOperationTemplateID: record.SelectedOperationTemplateID,
		ModalConfig:                 record.ModalConfig,
	})

	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Record{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := upsertAlertSettingsToNew(ctx, tx, record.TenantID, operationSection.AlertSettings); err != nil {
		return Record{}, err
	}
	if err := upsertCoreSettingsToNew(ctx, tx, operationSection); err != nil {
		return Record{}, err
	}
	if err := upsertModalSectionToNew(ctx, tx, modalSection); err != nil {
		return Record{}, err
	}

	// Garante linha ancora em tenant_operation_settings para FK de opcoes e catalogo.
	if err := ensureConfigRow(ctx, tx, record.TenantID); err != nil {
		return Record{}, err
	}

	optionGroups := []struct {
		kind  string
		items []OptionItem
	}{
		{kind: optionKindVisitReason, items: record.VisitReasonOptions},
		{kind: optionKindCustomerSource, items: record.CustomerSourceOptions},
		{kind: optionKindPauseReason, items: record.PauseReasonOptions},
		{kind: optionKindCancelReason, items: record.CancelReasonOptions},
		{kind: optionKindStopReason, items: record.StopReasonOptions},
		{kind: optionKindQueueJump, items: record.QueueJumpReasonOptions},
		{kind: optionKindLossReason, items: record.LossReasonOptions},
		{kind: optionKindProfession, items: record.ProfessionOptions},
	}
	for _, group := range optionGroups {
		if err := replaceOptionGroupTx(ctx, tx, record.TenantID, group.kind, group.items); err != nil {
			return Record{}, err
		}
	}

	if err := replaceProductsTx(ctx, tx, record.TenantID, record.ProductCatalog); err != nil {
		return Record{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, record.TenantID)
	if err != nil {
		return Record{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Record{}, err
	}

	return Record{
		TenantID:                    operationSection.TenantID,
		SelectedOperationTemplateID: operationSection.SelectedOperationTemplateID,
		Settings:                    composeAppSettings(operationSection.CoreSettings, operationSection.AlertSettings),
		ModalConfig:                 modalSection.ModalConfig,
		VisitReasonOptions:          cloneOptions(record.VisitReasonOptions),
		CustomerSourceOptions:       cloneOptions(record.CustomerSourceOptions),
		PauseReasonOptions:          cloneOptions(record.PauseReasonOptions),
		CancelReasonOptions:         cloneOptions(record.CancelReasonOptions),
		StopReasonOptions:           cloneOptions(record.StopReasonOptions),
		QueueJumpReasonOptions:      cloneOptions(record.QueueJumpReasonOptions),
		LossReasonOptions:           cloneOptions(record.LossReasonOptions),
		ProfessionOptions:           cloneOptions(record.ProfessionOptions),
		ProductCatalog:              cloneProducts(record.ProductCatalog),
		UpdatedAt:                   updatedAt,
	}, nil
}

func (repository *PostgresRepository) ReplaceOptionGroup(ctx context.Context, tenantID string, kind string, options []OptionItem) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, tenantID); err != nil {
		return time.Time{}, err
	}

	if err := replaceOptionGroupTx(ctx, tx, tenantID, kind, options); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, tenantID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) UpsertOption(ctx context.Context, tenantID string, kind string, option OptionItem) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, tenantID); err != nil {
		return time.Time{}, err
	}

	if err := upsertOptionTx(ctx, tx, tenantID, kind, option); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, tenantID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) DeleteOption(ctx context.Context, tenantID string, kind string, optionID string) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, tenantID); err != nil {
		return time.Time{}, err
	}

	if _, err := tx.Exec(ctx, `
		delete from tenant_setting_options
		where tenant_id = $1::uuid
		  and kind = $2
		  and option_id = $3;
	`, tenantID, kind, strings.TrimSpace(optionID)); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, tenantID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) ReplaceProducts(ctx context.Context, tenantID string, products []ProductItem) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, tenantID); err != nil {
		return time.Time{}, err
	}

	if err := replaceProductsTx(ctx, tx, tenantID, products); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, tenantID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) UpsertProduct(ctx context.Context, tenantID string, product ProductItem) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, tenantID); err != nil {
		return time.Time{}, err
	}

	if err := upsertProductTx(ctx, tx, tenantID, product); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, tenantID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) DeleteProduct(ctx context.Context, tenantID string, productID string) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, tenantID); err != nil {
		return time.Time{}, err
	}

	if _, err := tx.Exec(ctx, `
		delete from tenant_catalog_products
		where tenant_id = $1::uuid
		  and product_id = $2;
	`, tenantID, strings.TrimSpace(productID)); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, tenantID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) loadOptionsByKind(ctx context.Context, tenantID string, kind string) ([]OptionItem, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			option_id,
			label
		from tenant_setting_options
		where tenant_id = $1::uuid
		  and kind = $2
		order by sort_order asc, label asc;
	`, tenantID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	options := make([]OptionItem, 0)
	for rows.Next() {
		var option OptionItem
		if err := rows.Scan(&option.ID, &option.Label); err != nil {
			return nil, err
		}

		options = append(options, option)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return options, nil
}

func (repository *PostgresRepository) loadProducts(ctx context.Context, tenantID string) ([]ProductItem, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			product_id,
			name,
			code,
			category,
			base_price
		from tenant_catalog_products
		where tenant_id = $1::uuid
		order by sort_order asc, name asc;
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]ProductItem, 0)
	for rows.Next() {
		var product ProductItem
		if err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Code,
			&product.Category,
			&product.BasePrice,
		); err != nil {
			return nil, err
		}

		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return products, nil
}

func ensureConfigRow(ctx context.Context, queryer execQueryer, tenantID string) error {
	_, err := queryer.Exec(ctx, `
		insert into tenant_operation_settings (tenant_id)
		values ($1::uuid)
		on conflict (tenant_id) do nothing;
	`, tenantID)
	return err
}

func touchConfigRow(ctx context.Context, queryer execQueryer, tenantID string) (time.Time, error) {
	var updatedAt time.Time
	err := queryer.QueryRow(ctx, `
		update tenant_operation_settings
		set updated_at = now()
		where tenant_id = $1::uuid
		returning updated_at;
	`, tenantID).Scan(&updatedAt)
	if err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func replaceOptionGroupTx(ctx context.Context, tx pgx.Tx, tenantID string, kind string, options []OptionItem) error {
	if _, err := tx.Exec(ctx, `
		delete from tenant_setting_options
		where tenant_id = $1::uuid
		  and kind = $2;
	`, tenantID, kind); err != nil {
		return err
	}

	for index, option := range options {
		if _, err := tx.Exec(ctx, `
			insert into tenant_setting_options (
				tenant_id,
				kind,
				option_id,
				label,
				sort_order
			)
			values ($1::uuid, $2, $3, $4, $5);
		`,
			tenantID,
			kind,
			strings.TrimSpace(option.ID),
			strings.TrimSpace(option.Label),
			index,
		); err != nil {
			return err
		}
	}

	return nil
}

func upsertOptionTx(ctx context.Context, tx pgx.Tx, tenantID string, kind string, option OptionItem) error {
	_, err := tx.Exec(ctx, `
		insert into tenant_setting_options (
			tenant_id,
			kind,
			option_id,
			label,
			sort_order
		)
		values (
			$1::uuid,
			$2,
			$3,
			$4,
			coalesce(
				(
					select sort_order
					from tenant_setting_options
					where tenant_id = $1::uuid
					  and kind = $2
					  and option_id = $3
				),
				(
					select coalesce(max(sort_order) + 1, 0)
					from tenant_setting_options
					where tenant_id = $1::uuid
					  and kind = $2
				)
			)
		)
		on conflict (tenant_id, kind, option_id) do update
		set label = excluded.label;
	`,
		tenantID,
		kind,
		strings.TrimSpace(option.ID),
		strings.TrimSpace(option.Label),
	)
	return err
}

func replaceProductsTx(ctx context.Context, tx pgx.Tx, tenantID string, products []ProductItem) error {
	if _, err := tx.Exec(ctx, `
		delete from tenant_catalog_products
		where tenant_id = $1::uuid;
	`, tenantID); err != nil {
		return err
	}

	for index, product := range products {
		if _, err := tx.Exec(ctx, `
			insert into tenant_catalog_products (
				tenant_id,
				product_id,
				name,
				code,
				category,
				base_price,
				sort_order
			)
			values ($1::uuid, $2, $3, $4, $5, $6, $7);
		`,
			tenantID,
			strings.TrimSpace(product.ID),
			strings.TrimSpace(product.Name),
			strings.ToUpper(strings.TrimSpace(product.Code)),
			strings.TrimSpace(product.Category),
			product.BasePrice,
			index,
		); err != nil {
			return err
		}
	}

	return nil
}

func upsertProductTx(ctx context.Context, tx pgx.Tx, tenantID string, product ProductItem) error {
	_, err := tx.Exec(ctx, `
		insert into tenant_catalog_products (
			tenant_id,
			product_id,
			name,
			code,
			category,
			base_price,
			sort_order
		)
		values (
			$1::uuid,
			$2,
			$3,
			$4,
			$5,
			$6,
			coalesce(
				(
					select sort_order
					from tenant_catalog_products
					where tenant_id = $1::uuid
					  and product_id = $2
				),
				(
					select coalesce(max(sort_order) + 1, 0)
					from tenant_catalog_products
					where tenant_id = $1::uuid
				)
			)
		)
		on conflict (tenant_id, product_id) do update
		set
			name = excluded.name,
			code = excluded.code,
			category = excluded.category,
			base_price = excluded.base_price;
	`,
		tenantID,
		strings.TrimSpace(product.ID),
		strings.TrimSpace(product.Name),
		strings.ToUpper(strings.TrimSpace(product.Code)),
		strings.TrimSpace(product.Category),
		product.BasePrice,
	)
	return err
}
