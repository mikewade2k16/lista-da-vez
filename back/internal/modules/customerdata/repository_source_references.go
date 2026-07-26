package customerdata

import "context"

func (r *PostgresRepository) ListSourceReferences(
	ctx context.Context,
	scope Scope,
	relationshipID string,
) ([]SourceReference, error) {
	rows, err := r.pool.Query(ctx, `
		select source_module, source_key, source_entity_type, source_entity_id,
		       coalesce(source_version, ''), coalesce(source_hash, '')
		from customer_data.subject_source_links
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and relationship_id = $3::uuid
		  and status = 'active'
		order by source_module, source_key, source_entity_type, source_entity_id
	`, scope.AccountID, scope.ClientAccountID, relationshipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SourceReference, 0)
	for rows.Next() {
		var item SourceReference
		if err := rows.Scan(
			&item.SourceModule,
			&item.SourceKey,
			&item.SourceEntityType,
			&item.SourceEntityID,
			&item.SourceVersion,
			&item.SourceHash,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
