package accounting

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *Store) repositoryClaimExecutionQueryRowExecutions(ctx context.Context, arg0 string, arg1 string, out0 *string, out1 *string) error {
	return r.pool.QueryRow(ctx, `select issuer_id::text,actor_id::text from accounting.executions where account_id=$1::uuid and id=$2::uuid`, arg0, arg1).Scan(out0, out1)
}

func repositoryClaimExecutionQueryRowIssuers(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, out0 *int64) error {
	return tx.QueryRow(ctx, `select access_version from accounting.issuers where account_id=$1::uuid and id=$2::uuid for update`, arg0, arg1).Scan(out0)
}

func repositoryClaimExecutionQueryRowSubmissionAuthorizations(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 string, arg4 int64, out0 *string, out1 *bool) error {
	return tx.QueryRow(ctx, `select snapshot_hash,revoked_at is null and consumed_at is null and expires_at>now() and access_version=$5 from accounting.submission_authorizations where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid and id=$4::uuid for update`, arg0, arg1, arg2, arg3, arg4).Scan(out0, out1)
}

func repositoryClaimExecutionExecExecutions(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `update accounting.executions set state='cancelled',safe_error_code='authorization_invalid',version=version+1 where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, arg0, arg1, arg2)
}

func repositoryClaimExecutionExecSubmissionAuthorizations(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `update accounting.submission_authorizations set consumed_at=now(),version=version+1 where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, arg0, arg1, arg2)
}

func repositoryClaimExecutionQueryRowExecutions2(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 string, out0 *int64) error {
	return tx.QueryRow(ctx, `update accounting.executions set state='processing',fence=fence+1,lease_owner=$4,lease_until=now()+interval '90 seconds',version=version+1 where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid returning fence`, arg0, arg1, arg2, arg3).Scan(out0)
}

func repositoryFinishExecExecutions(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 string, arg4 *string, arg5 *string, arg6 string, arg7 int64) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `update accounting.executions set state=$4,remote_ref=$5,protocol=$6,safe_error_code=nullif($7,''),artifacts_pending=($4='authorized'),version=version+1,lease_until=null,updated_at=now() where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid and fence=$8`, arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7)
}

func repositoryRevokeAuthorizationQueryRowSubmissionAuthorizations(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 int64, out0 *string, out1 *int64) error {
	return tx.QueryRow(ctx, `update accounting.submission_authorizations set revoked_at=coalesce(revoked_at,now()),version=version+1 where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid and version=$4 returning document_id::text,version`, arg0, arg1, arg2, arg3).Scan(out0, out1)
}

func repositoryRevokeAuthorizationExecExecutions(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `update accounting.executions set state='cancelled',version=version+1 where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid and state='queued'`, arg0, arg1, arg2)
}

func repositoryPrepareQueryRowDocuments(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 int64, arg3 string, arg4 int64, arg5 string, arg6 []byte, arg7 *string, arg8 json.RawMessage, out0 *string) error {
	return tx.QueryRow(ctx, `insert into accounting.documents(account_id,issuer_id,profile_version,diagnosis_id,diagnosis_version,kind,payload,amount,source_ref) values($1::uuid,$2::uuid,$3,$4::uuid,$5,$6,$7,$8::numeric,$9) returning id::text`, arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8).Scan(out0)
}

func repositoryReviewQueryRowDiagnoses(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, out0 *int64) error {
	return tx.QueryRow(ctx, `select version from accounting.diagnoses where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, arg0, arg1, arg2).Scan(out0)
}

func repositoryDocumentCommandQueryRowExecutions(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, out0 *bool) error {
	return tx.QueryRow(ctx, `select exists(select 1 from accounting.executions where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid and state in ('queued','processing','unknown'))`, arg0, arg1, arg2).Scan(out0)
}

func repositoryDocumentCommandQueryRowDiagnoses(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, out0 *int64) error {
	return tx.QueryRow(ctx, `select version from accounting.diagnoses where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, arg0, arg1, arg2).Scan(out0)
}

func repositoryDocumentCommandExecDocuments(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 []byte, arg4 *string, arg5 int64, arg6 int64) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `update accounting.documents set payload=$4,amount=$5::numeric,profile_version=$6,diagnosis_version=$7,content_version=content_version+1,state='draft',version=version+1,updated_at=now() where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, arg0, arg1, arg2, arg3, arg4, arg5, arg6)
}

func repositoryDocumentCommandQueryRowDocumentReviews(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 int64, arg4 int64, arg5 []byte, arg6 string, arg7 string, out0 *string) error {
	return tx.QueryRow(ctx, `insert into accounting.document_reviews(account_id,issuer_id,document_id,document_version,profile_version,snapshot,snapshot_hash,actor_id) values($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8::uuid) returning id::text`, arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7).Scan(out0)
}

func repositoryDocumentCommandQueryRowDocumentReviews2(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 string, out0 *bool) error {
	return tx.QueryRow(ctx, `select exists(select 1 from accounting.document_reviews where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid and id=$4::uuid)`, arg0, arg1, arg2, arg3).Scan(out0)
}

func repositorySetDocumentStateExecDocuments(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `update accounting.documents set state=$4,version=version+1,updated_at=now() where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, arg0, arg1, arg2, arg3)
}

func repositoryReadDiagnosisQueryRowDiagnoses(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, out0 *json.RawMessage) error {
	return tx.QueryRow(ctx, `select jsonb_build_object('id',id,'issuerId',issuer_id,'version',version,'profileVersion',profile_version,'intent',intent,'competenceDate',competence_date,'answers',answers,'pending',pending) from accounting.diagnoses where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, arg0, arg1, arg2).Scan(out0)
}

func repositoryReadQueryRowIssuers(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, out0 *int64) error {
	return tx.QueryRow(ctx, `select version from accounting.issuers where account_id=$1::uuid and id=$2::uuid`, arg0, arg1).Scan(out0)
}

func repositoryReadQueryExecutionEvents(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 int, arg4 int) (pgx.Rows, error) {
	return tx.Query(ctx, `select jsonb_build_object('sequence',sequence,'eventCode',event_code,'createdAt',created_at) from accounting.execution_events where account_id=$1::uuid and issuer_id=$2::uuid and execution_id=$3::uuid order by sequence limit $4 offset $5`, arg0, arg1, arg2, arg3, arg4)
}

func (r *Store) repositoryListIssuersQueryIssuers(ctx context.Context, arg0 string, arg1 string, arg2 int, arg3 int) (pgx.Rows, error) {
	return r.pool.Query(ctx, `select i.id::text,i.version from accounting.issuers i join accounting.issuer_members m on m.account_id=i.account_id and m.issuer_id=i.id where i.account_id=$1::uuid and m.user_id=$2::uuid and m.status='active' and 'view'=any(m.grants) and i.status='active' order by i.id limit $3 offset $4`, arg0, arg1, arg2, arg3)
}

func repositoryCreateIssuerQueryRowIssuers(ctx context.Context, tx pgx.Tx, arg0 string, out0 *string) error {
	return tx.QueryRow(ctx, `insert into accounting.issuers(account_id) values($1::uuid) returning id::text`, arg0).Scan(out0)
}

func repositoryCreateIssuerExecIssuerMembers(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 []string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `insert into accounting.issuer_members(account_id,issuer_id,user_id,grants) values($1::uuid,$2::uuid,$3::uuid,$4)`, arg0, arg1, arg2, arg3)
}

func repositoryCreateIssuerExecIssuerProfiles(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 []byte, arg3 string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `insert into accounting.issuer_profiles(account_id,issuer_id,version,data,created_by) values($1::uuid,$2::uuid,1,$3,$4::uuid)`, arg0, arg1, arg2, arg3)
}

func repositoryUpdateIssuerQueryRowIssuers(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 int64, out0 *int64) error {
	return tx.QueryRow(ctx, `update accounting.issuers set version=version+1,updated_at=now() where account_id=$1::uuid and id=$2::uuid and version=$3 returning version`, arg0, arg1, arg2).Scan(out0)
}
