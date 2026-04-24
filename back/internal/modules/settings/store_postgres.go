package settings

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	optionKindVisitReason    = "visit_reason"
	optionKindCustomerSource = "customer_source"
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

func (repository *PostgresRepository) StoreExists(ctx context.Context, storeID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists(
			select 1
			from stores
			where id = $1::uuid
		);
	`, storeID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (repository *PostgresRepository) GetByStore(ctx context.Context, storeID string) (Record, bool, error) {
	record, err := scanConfigRow(repository.pool.QueryRow(ctx, `
		select
			store_id::text,
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
			show_email_field,
			show_profession_field,
			show_notes_field,
			visit_reason_selection_mode,
			visit_reason_detail_mode,
			loss_reason_selection_mode,
			loss_reason_detail_mode,
			customer_source_selection_mode,
			customer_source_detail_mode,
			require_product,
			require_visit_reason,
			require_customer_source,
			require_customer_name_phone,
			created_at,
			updated_at
		from store_operation_settings
		where store_id = $1::uuid
		limit 1;
	`, storeID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Record{}, false, nil
		}

		return Record{}, false, err
	}

	visitReasonOptions, err := repository.loadOptionsByKind(ctx, storeID, optionKindVisitReason)
	if err != nil {
		return Record{}, false, err
	}

	customerSourceOptions, err := repository.loadOptionsByKind(ctx, storeID, optionKindCustomerSource)
	if err != nil {
		return Record{}, false, err
	}

	queueJumpReasonOptions, err := repository.loadOptionsByKind(ctx, storeID, optionKindQueueJump)
	if err != nil {
		return Record{}, false, err
	}

	lossReasonOptions, err := repository.loadOptionsByKind(ctx, storeID, optionKindLossReason)
	if err != nil {
		return Record{}, false, err
	}

	professionOptions, err := repository.loadOptionsByKind(ctx, storeID, optionKindProfession)
	if err != nil {
		return Record{}, false, err
	}

	products, err := repository.loadProducts(ctx, storeID)
	if err != nil {
		return Record{}, false, err
	}

	record.VisitReasonOptions = visitReasonOptions
	record.CustomerSourceOptions = customerSourceOptions
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
		{kind: optionKindQueueJump, items: record.QueueJumpReasonOptions},
		{kind: optionKindLossReason, items: record.LossReasonOptions},
		{kind: optionKindProfession, items: record.ProfessionOptions},
	}

	for _, group := range optionGroups {
		if err := replaceOptionGroupTx(ctx, tx, record.StoreID, group.kind, group.items); err != nil {
			return Record{}, err
		}
	}

	if err := replaceProductsTx(ctx, tx, record.StoreID, record.ProductCatalog); err != nil {
		return Record{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Record{}, err
	}

	savedRecord.VisitReasonOptions = cloneOptions(record.VisitReasonOptions)
	savedRecord.CustomerSourceOptions = cloneOptions(record.CustomerSourceOptions)
	savedRecord.QueueJumpReasonOptions = cloneOptions(record.QueueJumpReasonOptions)
	savedRecord.LossReasonOptions = cloneOptions(record.LossReasonOptions)
	savedRecord.ProfessionOptions = cloneOptions(record.ProfessionOptions)
	savedRecord.ProductCatalog = cloneProducts(record.ProductCatalog)

	return savedRecord, nil
}

func (repository *PostgresRepository) UpsertConfig(ctx context.Context, record Record) (Record, error) {
	return upsertConfigRow(ctx, repository.pool, record)
}

func (repository *PostgresRepository) ReplaceOptionGroup(ctx context.Context, storeID string, kind string, options []OptionItem) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, storeID); err != nil {
		return time.Time{}, err
	}

	if err := replaceOptionGroupTx(ctx, tx, storeID, kind, options); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, storeID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) UpsertOption(ctx context.Context, storeID string, kind string, option OptionItem) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, storeID); err != nil {
		return time.Time{}, err
	}

	if err := upsertOptionTx(ctx, tx, storeID, kind, option); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, storeID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) DeleteOption(ctx context.Context, storeID string, kind string, optionID string) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, storeID); err != nil {
		return time.Time{}, err
	}

	if _, err := tx.Exec(ctx, `
		delete from store_setting_options
		where store_id = $1::uuid
		  and kind = $2
		  and option_id = $3;
	`, storeID, kind, strings.TrimSpace(optionID)); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, storeID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) ReplaceProducts(ctx context.Context, storeID string, products []ProductItem) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, storeID); err != nil {
		return time.Time{}, err
	}

	if err := replaceProductsTx(ctx, tx, storeID, products); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, storeID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) UpsertProduct(ctx context.Context, storeID string, product ProductItem) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, storeID); err != nil {
		return time.Time{}, err
	}

	if err := upsertProductTx(ctx, tx, storeID, product); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, storeID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) DeleteProduct(ctx context.Context, storeID string, productID string) (time.Time, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ensureConfigRow(ctx, tx, storeID); err != nil {
		return time.Time{}, err
	}

	if _, err := tx.Exec(ctx, `
		delete from store_catalog_products
		where store_id = $1::uuid
		  and product_id = $2;
	`, storeID, strings.TrimSpace(productID)); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, storeID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repository *PostgresRepository) loadOptionsByKind(ctx context.Context, storeID string, kind string) ([]OptionItem, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			option_id,
			label
		from store_setting_options
		where store_id = $1::uuid
		  and kind = $2
		order by sort_order asc, label asc;
	`, storeID, kind)
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

func (repository *PostgresRepository) loadProducts(ctx context.Context, storeID string) ([]ProductItem, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			product_id,
			name,
			code,
			category,
			base_price
		from store_catalog_products
		where store_id = $1::uuid
		order by sort_order asc, name asc;
	`, storeID)
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

func ensureConfigRow(ctx context.Context, queryer execQueryer, storeID string) error {
	_, err := queryer.Exec(ctx, `
		insert into store_operation_settings (store_id)
		values ($1::uuid)
		on conflict (store_id) do nothing;
	`, storeID)
	return err
}

func touchConfigRow(ctx context.Context, queryer execQueryer, storeID string) (time.Time, error) {
	var updatedAt time.Time
	err := queryer.QueryRow(ctx, `
		update store_operation_settings
		set updated_at = now()
		where store_id = $1::uuid
		returning updated_at;
	`, storeID).Scan(&updatedAt)
	if err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func replaceOptionGroupTx(ctx context.Context, tx pgx.Tx, storeID string, kind string, options []OptionItem) error {
	if _, err := tx.Exec(ctx, `
		delete from store_setting_options
		where store_id = $1::uuid
		  and kind = $2;
	`, storeID, kind); err != nil {
		return err
	}

	for index, option := range options {
		if _, err := tx.Exec(ctx, `
			insert into store_setting_options (
				store_id,
				kind,
				option_id,
				label,
				sort_order
			)
			values ($1::uuid, $2, $3, $4, $5);
		`,
			storeID,
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

func upsertOptionTx(ctx context.Context, tx pgx.Tx, storeID string, kind string, option OptionItem) error {
	_, err := tx.Exec(ctx, `
		insert into store_setting_options (
			store_id,
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
					from store_setting_options
					where store_id = $1::uuid
					  and kind = $2
					  and option_id = $3
				),
				(
					select coalesce(max(sort_order) + 1, 0)
					from store_setting_options
					where store_id = $1::uuid
					  and kind = $2
				)
			)
		)
		on conflict (store_id, kind, option_id) do update
		set label = excluded.label;
	`,
		storeID,
		kind,
		strings.TrimSpace(option.ID),
		strings.TrimSpace(option.Label),
	)
	return err
}

func replaceProductsTx(ctx context.Context, tx pgx.Tx, storeID string, products []ProductItem) error {
	if _, err := tx.Exec(ctx, `
		delete from store_catalog_products
		where store_id = $1::uuid;
	`, storeID); err != nil {
		return err
	}

	for index, product := range products {
		if _, err := tx.Exec(ctx, `
			insert into store_catalog_products (
				store_id,
				product_id,
				name,
				code,
				category,
				base_price,
				sort_order
			)
			values ($1::uuid, $2, $3, $4, $5, $6, $7);
		`,
			storeID,
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

func upsertProductTx(ctx context.Context, tx pgx.Tx, storeID string, product ProductItem) error {
	_, err := tx.Exec(ctx, `
		insert into store_catalog_products (
			store_id,
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
					from store_catalog_products
					where store_id = $1::uuid
					  and product_id = $2
				),
				(
					select coalesce(max(sort_order) + 1, 0)
					from store_catalog_products
					where store_id = $1::uuid
				)
			)
		)
		on conflict (store_id, product_id) do update
		set
			name = excluded.name,
			code = excluded.code,
			category = excluded.category,
			base_price = excluded.base_price;
	`,
		storeID,
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
		insert into store_operation_settings (
			store_id,
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
			show_email_field,
			show_profession_field,
			show_notes_field,
			visit_reason_selection_mode,
			visit_reason_detail_mode,
			loss_reason_selection_mode,
			loss_reason_detail_mode,
			customer_source_selection_mode,
			customer_source_detail_mode,
			require_product,
			require_visit_reason,
			require_customer_source,
			require_customer_name_phone
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
			$37
		)
		on conflict (store_id) do update
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
			show_email_field = excluded.show_email_field,
			show_profession_field = excluded.show_profession_field,
			show_notes_field = excluded.show_notes_field,
			visit_reason_selection_mode = excluded.visit_reason_selection_mode,
			visit_reason_detail_mode = excluded.visit_reason_detail_mode,
			loss_reason_selection_mode = excluded.loss_reason_selection_mode,
			loss_reason_detail_mode = excluded.loss_reason_detail_mode,
			customer_source_selection_mode = excluded.customer_source_selection_mode,
			customer_source_detail_mode = excluded.customer_source_detail_mode,
			require_product = excluded.require_product,
			require_visit_reason = excluded.require_visit_reason,
			require_customer_source = excluded.require_customer_source,
			require_customer_name_phone = excluded.require_customer_name_phone,
			updated_at = now()
		returning
			store_id::text,
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
			show_email_field,
			show_profession_field,
			show_notes_field,
			visit_reason_selection_mode,
			visit_reason_detail_mode,
			loss_reason_selection_mode,
			loss_reason_detail_mode,
			customer_source_selection_mode,
			customer_source_detail_mode,
			require_product,
			require_visit_reason,
			require_customer_source,
			require_customer_name_phone,
			created_at,
			updated_at;
	`,
		record.StoreID,
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
		record.ModalConfig.ShowEmailField,
		record.ModalConfig.ShowProfessionField,
		record.ModalConfig.ShowNotesField,
		record.ModalConfig.VisitReasonSelectionMode,
		record.ModalConfig.VisitReasonDetailMode,
		record.ModalConfig.LossReasonSelectionMode,
		record.ModalConfig.LossReasonDetailMode,
		record.ModalConfig.CustomerSourceSelectionMode,
		record.ModalConfig.CustomerSourceDetailMode,
		record.ModalConfig.RequireProduct,
		record.ModalConfig.RequireVisitReason,
		record.ModalConfig.RequireCustomerSource,
		record.ModalConfig.RequireCustomerNamePhone,
	))
}

func scanConfigRow(row pgx.Row) (Record, error) {
	var record Record
	err := row.Scan(
		&record.StoreID,
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
		&record.ModalConfig.ShowEmailField,
		&record.ModalConfig.ShowProfessionField,
		&record.ModalConfig.ShowNotesField,
		&record.ModalConfig.VisitReasonSelectionMode,
		&record.ModalConfig.VisitReasonDetailMode,
		&record.ModalConfig.LossReasonSelectionMode,
		&record.ModalConfig.LossReasonDetailMode,
		&record.ModalConfig.CustomerSourceSelectionMode,
		&record.ModalConfig.CustomerSourceDetailMode,
		&record.ModalConfig.RequireProduct,
		&record.ModalConfig.RequireVisitReason,
		&record.ModalConfig.RequireCustomerSource,
		&record.ModalConfig.RequireCustomerNamePhone,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return Record{}, err
	}

	return record, nil
}
