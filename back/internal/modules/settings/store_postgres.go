package settings

import (
	"context"
	"errors"
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
	optionKindQueueJump      = "queue_jump_reason"
	optionKindLossReason     = "loss_reason"
	optionKindProfession     = "profession"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

type rowQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
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
	record, err := scanConfigRow(repository.pool.QueryRow(ctx, `
		select
			tenant_id::text,
			selected_operation_template_id,
			max_concurrent_services,
			timing_fast_close_minutes,
			timing_long_service_minutes,
			timing_low_sale_amount,
			test_mode_enabled,
			auto_fill_finish_modal,
			alert_min_conversion_rate,
			alert_max_queue_jump_rate,
			alert_min_pa_score,
			alert_min_ticket_average,
			title,
			product_seen_label,
			product_seen_placeholder,
			product_closed_label,
			product_closed_placeholder,
			notes_label,
			notes_placeholder,
			queue_jump_reason_label,
			queue_jump_reason_placeholder,
			loss_reason_label,
			loss_reason_placeholder,
			customer_section_label,
			show_customer_name_field,
			show_customer_phone_field,
			show_email_field,
			show_profession_field,
			show_notes_field,
			show_product_seen_field,
			show_product_seen_notes_field,
			show_product_closed_field,
			show_visit_reason_field,
			show_customer_source_field,
			show_existing_customer_field,
			show_queue_jump_reason_field,
			show_loss_reason_field,
			allow_product_seen_none,
			visit_reason_selection_mode,
			visit_reason_detail_mode,
			loss_reason_selection_mode,
			loss_reason_detail_mode,
			customer_source_selection_mode,
			customer_source_detail_mode,
			require_customer_name_field,
			require_customer_phone_field,
			require_email_field,
			require_profession_field,
			require_notes_field,
			require_product,
			require_product_seen_field,
			require_product_seen_notes_field,
			require_product_closed_field,
			require_visit_reason,
			require_customer_source,
			require_customer_name_phone,
			require_product_seen_notes_when_none,
			product_seen_notes_min_chars,
			require_queue_jump_reason_field,
			require_loss_reason_field,
			created_at,
			updated_at
		from tenant_operation_settings
		where tenant_id = $1::uuid
		limit 1;
	`, tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Record{}, false, nil
		}

		return Record{}, false, err
	}

	visitReasonOptions, err := repository.loadOptionsByKind(ctx, tenantID, optionKindVisitReason)
	if err != nil {
		return Record{}, false, err
	}

	customerSourceOptions, err := repository.loadOptionsByKind(ctx, tenantID, optionKindCustomerSource)
	if err != nil {
		return Record{}, false, err
	}

	pauseReasonOptions, err := repository.loadOptionsByKind(ctx, tenantID, optionKindPauseReason)
	if err != nil {
		return Record{}, false, err
	}

	queueJumpReasonOptions, err := repository.loadOptionsByKind(ctx, tenantID, optionKindQueueJump)
	if err != nil {
		return Record{}, false, err
	}

	lossReasonOptions, err := repository.loadOptionsByKind(ctx, tenantID, optionKindLossReason)
	if err != nil {
		return Record{}, false, err
	}

	professionOptions, err := repository.loadOptionsByKind(ctx, tenantID, optionKindProfession)
	if err != nil {
		return Record{}, false, err
	}

	products, err := repository.loadProducts(ctx, tenantID)
	if err != nil {
		return Record{}, false, err
	}

	record.VisitReasonOptions = visitReasonOptions
	record.CustomerSourceOptions = customerSourceOptions
	record.PauseReasonOptions = pauseReasonOptions
	record.QueueJumpReasonOptions = queueJumpReasonOptions
	record.LossReasonOptions = lossReasonOptions
	record.ProfessionOptions = professionOptions
	record.ProductCatalog = products

	return record, true, nil
}

func (repository *PostgresRepository) Upsert(ctx context.Context, record Record) (Record, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Record{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	savedRecord, err := upsertConfigRow(ctx, tx, record)
	if err != nil {
		return Record{}, err
	}

	optionGroups := []struct {
		kind  string
		items []OptionItem
	}{
		{kind: optionKindVisitReason, items: record.VisitReasonOptions},
		{kind: optionKindCustomerSource, items: record.CustomerSourceOptions},
		{kind: optionKindPauseReason, items: record.PauseReasonOptions},
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

	if err := tx.Commit(ctx); err != nil {
		return Record{}, err
	}

	savedRecord.VisitReasonOptions = cloneOptions(record.VisitReasonOptions)
	savedRecord.CustomerSourceOptions = cloneOptions(record.CustomerSourceOptions)
	savedRecord.PauseReasonOptions = cloneOptions(record.PauseReasonOptions)
	savedRecord.QueueJumpReasonOptions = cloneOptions(record.QueueJumpReasonOptions)
	savedRecord.LossReasonOptions = cloneOptions(record.LossReasonOptions)
	savedRecord.ProfessionOptions = cloneOptions(record.ProfessionOptions)
	savedRecord.ProductCatalog = cloneProducts(record.ProductCatalog)

	return savedRecord, nil
}

func (repository *PostgresRepository) UpsertConfig(ctx context.Context, record Record) (Record, error) {
	return upsertConfigRow(ctx, repository.pool, record)
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

func upsertConfigRow(ctx context.Context, queryer rowQueryer, record Record) (Record, error) {
	return scanConfigRow(queryer.QueryRow(ctx, `
		insert into tenant_operation_settings (
			tenant_id,
			selected_operation_template_id,
			max_concurrent_services,
			timing_fast_close_minutes,
			timing_long_service_minutes,
			timing_low_sale_amount,
			test_mode_enabled,
			auto_fill_finish_modal,
			alert_min_conversion_rate,
			alert_max_queue_jump_rate,
			alert_min_pa_score,
			alert_min_ticket_average,
			title,
			product_seen_label,
			product_seen_placeholder,
			product_closed_label,
			product_closed_placeholder,
			notes_label,
			notes_placeholder,
			queue_jump_reason_label,
			queue_jump_reason_placeholder,
			loss_reason_label,
			loss_reason_placeholder,
			customer_section_label,
			show_customer_name_field,
			show_customer_phone_field,
			show_email_field,
			show_profession_field,
			show_notes_field,
			show_product_seen_field,
			show_product_seen_notes_field,
			show_product_closed_field,
			show_visit_reason_field,
			show_customer_source_field,
			show_existing_customer_field,
			show_queue_jump_reason_field,
			show_loss_reason_field,
			allow_product_seen_none,
			visit_reason_selection_mode,
			visit_reason_detail_mode,
			loss_reason_selection_mode,
			loss_reason_detail_mode,
			customer_source_selection_mode,
			customer_source_detail_mode,
			require_customer_name_field,
			require_customer_phone_field,
			require_email_field,
			require_profession_field,
			require_notes_field,
			require_product,
			require_product_seen_field,
			require_product_seen_notes_field,
			require_product_closed_field,
			require_visit_reason,
			require_customer_source,
			require_customer_name_phone,
			require_product_seen_notes_when_none,
			product_seen_notes_min_chars,
			require_queue_jump_reason_field,
			require_loss_reason_field
		)
		values (
			$1::uuid,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13,
			$14,
			$15,
			$16,
			$17,
			$18,
			$19,
			$20,
			$21,
			$22,
			$23,
			$24,
			$25,
			$26,
			$27,
			$28,
			$29,
			$30,
			$31,
			$32,
			$33,
			$34,
			$35,
			$36,
			$37,
			$38,
			$39,
			$40,
			$41,
			$42,
			$43,
			$44,
			$45,
			$46,
			$47,
			$48,
			$49,
			$50,
			$51,
			$52,
			$53,
			$54,
			$55,
			$56,
			$57,
			$58,
			$59,
			$60
		)
		on conflict (tenant_id) do update
		set
			selected_operation_template_id = excluded.selected_operation_template_id,
			max_concurrent_services = excluded.max_concurrent_services,
			timing_fast_close_minutes = excluded.timing_fast_close_minutes,
			timing_long_service_minutes = excluded.timing_long_service_minutes,
			timing_low_sale_amount = excluded.timing_low_sale_amount,
			test_mode_enabled = excluded.test_mode_enabled,
			auto_fill_finish_modal = excluded.auto_fill_finish_modal,
			alert_min_conversion_rate = excluded.alert_min_conversion_rate,
			alert_max_queue_jump_rate = excluded.alert_max_queue_jump_rate,
			alert_min_pa_score = excluded.alert_min_pa_score,
			alert_min_ticket_average = excluded.alert_min_ticket_average,
			title = excluded.title,
			product_seen_label = excluded.product_seen_label,
			product_seen_placeholder = excluded.product_seen_placeholder,
			product_closed_label = excluded.product_closed_label,
			product_closed_placeholder = excluded.product_closed_placeholder,
			notes_label = excluded.notes_label,
			notes_placeholder = excluded.notes_placeholder,
			queue_jump_reason_label = excluded.queue_jump_reason_label,
			queue_jump_reason_placeholder = excluded.queue_jump_reason_placeholder,
			loss_reason_label = excluded.loss_reason_label,
			loss_reason_placeholder = excluded.loss_reason_placeholder,
			customer_section_label = excluded.customer_section_label,
			show_customer_name_field = excluded.show_customer_name_field,
			show_customer_phone_field = excluded.show_customer_phone_field,
			show_email_field = excluded.show_email_field,
			show_profession_field = excluded.show_profession_field,
			show_notes_field = excluded.show_notes_field,
			show_product_seen_field = excluded.show_product_seen_field,
			show_product_seen_notes_field = excluded.show_product_seen_notes_field,
			show_product_closed_field = excluded.show_product_closed_field,
			show_visit_reason_field = excluded.show_visit_reason_field,
			show_customer_source_field = excluded.show_customer_source_field,
			show_existing_customer_field = excluded.show_existing_customer_field,
			show_queue_jump_reason_field = excluded.show_queue_jump_reason_field,
			show_loss_reason_field = excluded.show_loss_reason_field,
			allow_product_seen_none = excluded.allow_product_seen_none,
			visit_reason_selection_mode = excluded.visit_reason_selection_mode,
			visit_reason_detail_mode = excluded.visit_reason_detail_mode,
			loss_reason_selection_mode = excluded.loss_reason_selection_mode,
			loss_reason_detail_mode = excluded.loss_reason_detail_mode,
			customer_source_selection_mode = excluded.customer_source_selection_mode,
			customer_source_detail_mode = excluded.customer_source_detail_mode,
			require_customer_name_field = excluded.require_customer_name_field,
			require_customer_phone_field = excluded.require_customer_phone_field,
			require_email_field = excluded.require_email_field,
			require_profession_field = excluded.require_profession_field,
			require_notes_field = excluded.require_notes_field,
			require_product = excluded.require_product,
			require_product_seen_field = excluded.require_product_seen_field,
			require_product_seen_notes_field = excluded.require_product_seen_notes_field,
			require_product_closed_field = excluded.require_product_closed_field,
			require_visit_reason = excluded.require_visit_reason,
			require_customer_source = excluded.require_customer_source,
			require_customer_name_phone = excluded.require_customer_name_phone,
			require_product_seen_notes_when_none = excluded.require_product_seen_notes_when_none,
			product_seen_notes_min_chars = excluded.product_seen_notes_min_chars,
			require_queue_jump_reason_field = excluded.require_queue_jump_reason_field,
			require_loss_reason_field = excluded.require_loss_reason_field,
			updated_at = now()
		returning
			tenant_id::text,
			selected_operation_template_id,
			max_concurrent_services,
			timing_fast_close_minutes,
			timing_long_service_minutes,
			timing_low_sale_amount,
			test_mode_enabled,
			auto_fill_finish_modal,
			alert_min_conversion_rate,
			alert_max_queue_jump_rate,
			alert_min_pa_score,
			alert_min_ticket_average,
			title,
			product_seen_label,
			product_seen_placeholder,
			product_closed_label,
			product_closed_placeholder,
			notes_label,
			notes_placeholder,
			queue_jump_reason_label,
			queue_jump_reason_placeholder,
			loss_reason_label,
			loss_reason_placeholder,
			customer_section_label,
			show_customer_name_field,
			show_customer_phone_field,
			show_email_field,
			show_profession_field,
			show_notes_field,
			show_product_seen_field,
			show_product_seen_notes_field,
			show_product_closed_field,
			show_visit_reason_field,
			show_customer_source_field,
			show_existing_customer_field,
			show_queue_jump_reason_field,
			show_loss_reason_field,
			allow_product_seen_none,
			visit_reason_selection_mode,
			visit_reason_detail_mode,
			loss_reason_selection_mode,
			loss_reason_detail_mode,
			customer_source_selection_mode,
			customer_source_detail_mode,
			require_customer_name_field,
			require_customer_phone_field,
			require_email_field,
			require_profession_field,
			require_notes_field,
			require_product,
			require_product_seen_field,
			require_product_seen_notes_field,
			require_product_closed_field,
			require_visit_reason,
			require_customer_source,
			require_customer_name_phone,
			require_product_seen_notes_when_none,
			product_seen_notes_min_chars,
			require_queue_jump_reason_field,
			require_loss_reason_field,
			created_at,
			updated_at;
	`,
		record.TenantID,
		record.SelectedOperationTemplateID,
		record.Settings.MaxConcurrentServices,
		record.Settings.TimingFastCloseMinutes,
		record.Settings.TimingLongServiceMinutes,
		record.Settings.TimingLowSaleAmount,
		record.Settings.TestModeEnabled,
		record.Settings.AutoFillFinishModal,
		record.Settings.AlertMinConversionRate,
		record.Settings.AlertMaxQueueJumpRate,
		record.Settings.AlertMinPaScore,
		record.Settings.AlertMinTicketAverage,
		record.ModalConfig.Title,
		record.ModalConfig.ProductSeenLabel,
		record.ModalConfig.ProductSeenPlaceholder,
		record.ModalConfig.ProductClosedLabel,
		record.ModalConfig.ProductClosedPlaceholder,
		record.ModalConfig.NotesLabel,
		record.ModalConfig.NotesPlaceholder,
		record.ModalConfig.QueueJumpReasonLabel,
		record.ModalConfig.QueueJumpReasonPlaceholder,
		record.ModalConfig.LossReasonLabel,
		record.ModalConfig.LossReasonPlaceholder,
		record.ModalConfig.CustomerSectionLabel,
		record.ModalConfig.ShowCustomerNameField,
		record.ModalConfig.ShowCustomerPhoneField,
		record.ModalConfig.ShowEmailField,
		record.ModalConfig.ShowProfessionField,
		record.ModalConfig.ShowNotesField,
		record.ModalConfig.ShowProductSeenField,
		record.ModalConfig.ShowProductSeenNotesField,
		record.ModalConfig.ShowProductClosedField,
		record.ModalConfig.ShowVisitReasonField,
		record.ModalConfig.ShowCustomerSourceField,
		record.ModalConfig.ShowExistingCustomerField,
		record.ModalConfig.ShowQueueJumpReasonField,
		record.ModalConfig.ShowLossReasonField,
		record.ModalConfig.AllowProductSeenNone,
		record.ModalConfig.VisitReasonSelectionMode,
		record.ModalConfig.VisitReasonDetailMode,
		record.ModalConfig.LossReasonSelectionMode,
		record.ModalConfig.LossReasonDetailMode,
		record.ModalConfig.CustomerSourceSelectionMode,
		record.ModalConfig.CustomerSourceDetailMode,
		record.ModalConfig.RequireCustomerNameField,
		record.ModalConfig.RequireCustomerPhoneField,
		record.ModalConfig.RequireEmailField,
		record.ModalConfig.RequireProfessionField,
		record.ModalConfig.RequireNotesField,
		record.ModalConfig.RequireProduct,
		record.ModalConfig.RequireProductSeenField,
		record.ModalConfig.RequireProductSeenNotesField,
		record.ModalConfig.RequireProductClosedField,
		record.ModalConfig.RequireVisitReason,
		record.ModalConfig.RequireCustomerSource,
		record.ModalConfig.RequireCustomerNamePhone,
		record.ModalConfig.RequireProductSeenNotesWhenNone,
		record.ModalConfig.ProductSeenNotesMinChars,
		record.ModalConfig.RequireQueueJumpReasonField,
		record.ModalConfig.RequireLossReasonField,
	))
}

func scanConfigRow(row pgx.Row) (Record, error) {
	var record Record
	err := row.Scan(
		&record.TenantID,
		&record.SelectedOperationTemplateID,
		&record.Settings.MaxConcurrentServices,
		&record.Settings.TimingFastCloseMinutes,
		&record.Settings.TimingLongServiceMinutes,
		&record.Settings.TimingLowSaleAmount,
		&record.Settings.TestModeEnabled,
		&record.Settings.AutoFillFinishModal,
		&record.Settings.AlertMinConversionRate,
		&record.Settings.AlertMaxQueueJumpRate,
		&record.Settings.AlertMinPaScore,
		&record.Settings.AlertMinTicketAverage,
		&record.ModalConfig.Title,
		&record.ModalConfig.ProductSeenLabel,
		&record.ModalConfig.ProductSeenPlaceholder,
		&record.ModalConfig.ProductClosedLabel,
		&record.ModalConfig.ProductClosedPlaceholder,
		&record.ModalConfig.NotesLabel,
		&record.ModalConfig.NotesPlaceholder,
		&record.ModalConfig.QueueJumpReasonLabel,
		&record.ModalConfig.QueueJumpReasonPlaceholder,
		&record.ModalConfig.LossReasonLabel,
		&record.ModalConfig.LossReasonPlaceholder,
		&record.ModalConfig.CustomerSectionLabel,
		&record.ModalConfig.ShowCustomerNameField,
		&record.ModalConfig.ShowCustomerPhoneField,
		&record.ModalConfig.ShowEmailField,
		&record.ModalConfig.ShowProfessionField,
		&record.ModalConfig.ShowNotesField,
		&record.ModalConfig.ShowProductSeenField,
		&record.ModalConfig.ShowProductSeenNotesField,
		&record.ModalConfig.ShowProductClosedField,
		&record.ModalConfig.ShowVisitReasonField,
		&record.ModalConfig.ShowCustomerSourceField,
		&record.ModalConfig.ShowExistingCustomerField,
		&record.ModalConfig.ShowQueueJumpReasonField,
		&record.ModalConfig.ShowLossReasonField,
		&record.ModalConfig.AllowProductSeenNone,
		&record.ModalConfig.VisitReasonSelectionMode,
		&record.ModalConfig.VisitReasonDetailMode,
		&record.ModalConfig.LossReasonSelectionMode,
		&record.ModalConfig.LossReasonDetailMode,
		&record.ModalConfig.CustomerSourceSelectionMode,
		&record.ModalConfig.CustomerSourceDetailMode,
		&record.ModalConfig.RequireCustomerNameField,
		&record.ModalConfig.RequireCustomerPhoneField,
		&record.ModalConfig.RequireEmailField,
		&record.ModalConfig.RequireProfessionField,
		&record.ModalConfig.RequireNotesField,
		&record.ModalConfig.RequireProduct,
		&record.ModalConfig.RequireProductSeenField,
		&record.ModalConfig.RequireProductSeenNotesField,
		&record.ModalConfig.RequireProductClosedField,
		&record.ModalConfig.RequireVisitReason,
		&record.ModalConfig.RequireCustomerSource,
		&record.ModalConfig.RequireCustomerNamePhone,
		&record.ModalConfig.RequireProductSeenNotesWhenNone,
		&record.ModalConfig.ProductSeenNotesMinChars,
		&record.ModalConfig.RequireQueueJumpReasonField,
		&record.ModalConfig.RequireLossReasonField,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return Record{}, err
	}

	return record, nil
}
