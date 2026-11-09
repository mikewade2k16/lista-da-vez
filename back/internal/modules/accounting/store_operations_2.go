package accounting

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"time"
)

func repositoryUpdateIssuerQueryRowIssuerProfiles(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, out0 *int64) error {
	return tx.QueryRow(ctx, `select coalesce(max(version),0)+1 from accounting.issuer_profiles where account_id=$1::uuid and issuer_id=$2::uuid`, arg0, arg1).Scan(out0)
}

func repositoryUpdateIssuerExecIssuerProfiles(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 int64, arg3 []byte, arg4 string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `insert into accounting.issuer_profiles(account_id,issuer_id,version,data,created_by) values($1::uuid,$2::uuid,$3,$4,$5::uuid)`, arg0, arg1, arg2, arg3, arg4)
}

func repositorySaveMemberQueryRowIssuerMembers(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 []string, arg4 string, arg5 int64, out0 *int64) error {
	return tx.QueryRow(ctx, `insert into accounting.issuer_members(account_id,issuer_id,user_id,grants,status) select $1::uuid,$2::uuid,$3::uuid,$4,$5 where $6::bigint=0
  on conflict(account_id,issuer_id,user_id) do update set grants=excluded.grants,status=excluded.status,version=accounting.issuer_members.version+1,updated_at=now() where accounting.issuer_members.version=$6
  returning version`, arg0, arg1, arg2, arg3, arg4, arg5).Scan(out0)
}

func repositorySaveMemberQueryRowIssuerMembers2(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 []string, arg4 string, arg5 int64, out0 *int64) error {
	return tx.QueryRow(ctx, `update accounting.issuer_members set grants=$4,status=$5,version=version+1,updated_at=now() where account_id=$1::uuid and issuer_id=$2::uuid and user_id=$3::uuid and version=$6 returning version`, arg0, arg1, arg2, arg3, arg4, arg5).Scan(out0)
}

func repositorySaveMemberExecIssuers(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `update accounting.issuers set access_version=access_version+1 where account_id=$1::uuid and id=$2::uuid`, arg0, arg1)
}

func repositorySaveDiagnosisQueryRowDiagnoses(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 int64, arg3 string, arg4 *string, arg5 []byte, arg6 []byte, out0 *string, out1 *int64) error {
	return tx.QueryRow(ctx, `insert into accounting.diagnoses(account_id,issuer_id,profile_version,intent,competence_date,answers,pending) values($1::uuid,$2::uuid,$3,$4,$5::date,$6,$7) returning id::text,version`, arg0, arg1, arg2, arg3, arg4, arg5, arg6).Scan(out0, out1)
}

func repositorySaveDiagnosisQueryRowDiagnoses2(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 int64, arg4 string, arg5 *string, arg6 []byte, arg7 []byte, arg8 int64, out0 *int64) error {
	return tx.QueryRow(ctx, `update accounting.diagnoses set version=version+1,profile_version=$4,intent=$5,competence_date=$6::date,answers=$7,pending=$8,updated_at=now() where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid and version=$9 returning version`, arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8).Scan(out0)
}

func (r *Store) repositoryRecoverArtifactsExecArtifacts(ctx context.Context, arg0 string, arg1 string, arg2 string, arg3 string, arg4 string, arg5 string, arg6 int) (pgconn.CommandTag, error) {
	return r.pool.Exec(ctx, `insert into accounting.artifacts(account_id,issuer_id,execution_id,storage_object_id,kind,sha256,size) values($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7) on conflict do nothing`, arg0, arg1, arg2, arg3, arg4, arg5, arg6)
}

func (r *Store) repositoryRecoverArtifactsExecExecutions(ctx context.Context, arg0 string, arg1 string, arg2 string) (pgconn.CommandTag, error) {
	return r.pool.Exec(ctx, `update accounting.executions set artifacts_pending=false,version=version+1 where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid and state='authorized'`, arg0, arg1, arg2)
}

func repositoryDeleteQueryRowExecutions(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, out0 *bool) error {
	return tx.QueryRow(ctx, `select exists(select 1 from accounting.executions where account_id=$1::uuid and issuer_id=$2::uuid and state in ('processing','unknown'))`, arg0, arg1).Scan(out0)
}

func repositoryDeleteQueryRowIssuers(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 int64, out0 *int64) error {
	return tx.QueryRow(ctx, `update accounting.issuers set status='inactive',version=version+1,access_version=access_version+1,updated_at=now() where account_id=$1::uuid and id=$2::uuid and version=$3 returning version`, arg0, arg1, arg2).Scan(out0)
}

func repositoryDeleteExecDocuments(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `update accounting.documents set deleted_at=now(),version=version+1 where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, arg0, arg1, arg2)
}

func (r *Store) repositoryReconcileStaleQueryExecutions(ctx context.Context) (pgx.Rows, error) {
	return r.pool.Query(ctx, `select account_id::text,issuer_id::text,id::text,document_id::text,fence from accounting.executions where (state='processing' and lease_until<now() or state='unknown') and fence<10 order by updated_at,id limit 20`)
}

func repositoryAuthorizeQueryRowDocumentReviews(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 string, arg4 string, out0 *bool) error {
	return tx.QueryRow(ctx, `select exists(select 1 from accounting.document_reviews where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid and id=$4::uuid and snapshot_hash=$5 and revoked_at is null)`, arg0, arg1, arg2, arg3, arg4).Scan(out0)
}

func repositoryAuthorizeQueryRowSubmissionAuthorizations(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 string, arg4 string, arg5 string, arg6 string, arg7 string, arg8 int64, out0 *string, out1 *time.Time) error {
	return tx.QueryRow(ctx, `insert into accounting.submission_authorizations(account_id,issuer_id,document_id,review_id,actor_id,snapshot_hash,connector_id,environment,access_version,expires_at) values($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,$6,$7,$8,$9,now()+interval '15 minutes') returning id::text,expires_at`, arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8).Scan(out0, out1)
}

func repositoryInvalidateAuthorizationsExecSubmissionAuthorizations(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `update accounting.submission_authorizations set revoked_at=coalesce(revoked_at,now()),version=version+1 where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid and consumed_at is null and revoked_at is null`, arg0, arg1, arg2)
}

func repositoryExecuteQueryRowExecutions(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, out0 *bool) error {
	return tx.QueryRow(ctx, `select exists(select 1 from accounting.executions where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid and state in ('queued','processing','unknown'))`, arg0, arg1, arg2).Scan(out0)
}

func repositoryExecuteQueryRowSubmissionAuthorizations(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 string, arg4 string, out0 *string, out1 *string, out2 *string) error {
	return tx.QueryRow(ctx, `select connector_id,environment,snapshot_hash from accounting.submission_authorizations where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid and id=$4::uuid and actor_id=$5::uuid and revoked_at is null and consumed_at is null and expires_at>now()`, arg0, arg1, arg2, arg3, arg4).Scan(out0, out1, out2)
}

func repositoryExecuteQueryRowExecutions2(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 string, arg4 string, arg5 string, arg6 string, arg7 string, arg8 string, out0 *string) error {
	return tx.QueryRow(ctx, `insert into accounting.executions(account_id,issuer_id,document_id,authorization_id,actor_id,operation,environment,connector_id,request_hash,business_key) values($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,'issue',$6,$7,$8,$9) on conflict do nothing returning id::text`, arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8).Scan(out0)
}

func repositoryExecuteExecExecutions(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `update accounting.executions set job_id=$4::uuid,remote_identity=id::text where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, arg0, arg1, arg2, arg3)
}

func repositoryEventExecExecutionEvents(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, arg3 string, arg4 string) (commandTag pgconn.CommandTag, err error) {
	return tx.Exec(ctx, `insert into accounting.execution_events(account_id,issuer_id,execution_id,sequence,event_code,actor_id) select $1::uuid,$2::uuid,$3::uuid,coalesce(max(sequence),0)+1,$4,$5::uuid from accounting.execution_events where account_id=$1::uuid and issuer_id=$2::uuid and execution_id=$3::uuid`, arg0, arg1, arg2, arg3, arg4)
}

func repositoryLoadExecutionQueryRowExecutions(ctx context.Context, tx pgx.Tx, arg0 string, arg1 string, arg2 string, out0 *string, out1 **string, out2 *string, out3 *string, out4 *string, out5 *string, out6 *string, out7 *int64, out8 *string, out9 *string, out10 *string, out11 **string, out12 **string, out13 **string, out14 *bool, out15 *int64, out16 **time.Time) error {
	return tx.QueryRow(ctx, `select id::text,job_id::text,issuer_id::text,document_id::text,actor_id::text,authorization_id::text,state,version,connector_id,environment,remote_identity,remote_ref,protocol,safe_error_code,artifacts_pending,fence,lease_until from accounting.executions where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid for update`, arg0, arg1, arg2).Scan(out0, out1, out2, out3, out4, out5, out6, out7, out8, out9, out10, out11, out12, out13, out14, out15, out16)
}
