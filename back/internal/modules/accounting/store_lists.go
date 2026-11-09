package accounting

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func listRecords(ctx context.Context, tx pgx.Tx, s Scope, kind string, page, limit int) ([]json.RawMessage, error) {
	// Both queries are fixed owner-owned statements. No identifiers come from input.
	query := `select jsonb_build_object('id',id,'version',version,'kind',kind,'state',state) from accounting.documents where deleted_at is null and account_id=$1::uuid and issuer_id=$2::uuid order by updated_at desc,id limit $3 offset $4`
	if kind == "diagnoses" {
		query = `select jsonb_build_object('id',id,'version',version,'intent',intent,'competenceDate',competence_date,'pending',pending) from accounting.diagnoses where account_id=$1::uuid and issuer_id=$2::uuid order by updated_at desc,id limit $3 offset $4`
	}
	rows, e := tx.Query(ctx, query, s.AccountID, s.IssuerID, limit, (page-1)*limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func recordValidation(ctx context.Context, tx pgx.Tx, s Scope, id, connector, environment string) error {
	_, e := tx.Exec(ctx, `update accounting.documents set state='validated',version=version+1,validated_connector=$4,validated_environment=$5,updated_at=now() where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, s.AccountID, s.IssuerID, id, connector, environment)
	return e
}

func artifactViews(ctx context.Context, tx pgx.Tx, s Scope, id string) ([]json.RawMessage, error) {
	rows, e := tx.Query(ctx, `select jsonb_build_object('id',id,'storageObjectId',storage_object_id,'kind',kind,'sha256',sha256,'size',size,'protocol',protocol) from accounting.artifacts where account_id=$1::uuid and issuer_id=$2::uuid and execution_id=$3::uuid order by id`, s.AccountID, s.IssuerID, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func updateDocumentSource(ctx context.Context, tx pgx.Tx, s Scope, id string, source json.RawMessage) error {
	_, e := tx.Exec(ctx, `update accounting.documents set source_ref=$4 where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, s.AccountID, s.IssuerID, id, source)
	return e
}

func authorizationVersion(ctx context.Context, tx pgx.Tx, s Scope, id, document string) (int64, error) {
	var version int64
	e := tx.QueryRow(ctx, `select version from accounting.submission_authorizations where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid and ($4='' or document_id=nullif($4,'')::uuid)`, s.AccountID, s.IssuerID, id, document).Scan(&version)
	return version, dbError(e)
}

func activeIssueGrant(ctx context.Context, tx pgx.Tx, s Scope) (bool, error) {
	var active bool
	e := tx.QueryRow(ctx, `select exists(select 1 from accounting.issuer_members where account_id=$1::uuid and issuer_id=$2::uuid and status='active' and 'issue'=any(grants))`, s.AccountID, s.IssuerID).Scan(&active)
	return active, e
}
