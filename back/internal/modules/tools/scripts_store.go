package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type scriptRow struct {
	Audio string `json:"audio"`
	Video string `json:"video"`
}
type scriptDocument struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Rows        []scriptRow `json:"rows"`
	Description string      `json:"description"`
	UpdatedAt   string      `json:"updatedAt"`
	// Cliente sobre o qual o roteiro fala (conta-cliente da mesma organização) e se ele vê.
	ClientAccountID string `json:"clientAccountId"`
	ClientName      string `json:"clientName"`
	ClientVisible   bool   `json:"clientVisible"`
	// Shared: roteiro de outra conta (agência) liberado para a conta atual; somente leitura.
	Shared bool `json:"shared"`
	// Preview: texto puro das primeiras linhas, so na listagem (miniatura da lateral).
	Preview []scriptRow `json:"preview,omitempty"`
}
type scriptInput struct {
	Title           string      `json:"title"`
	Rows            []scriptRow `json:"rows"`
	Description     string      `json:"description"`
	UpdatedAt       string      `json:"updatedAt,omitempty"`
	ClientAccountID string      `json:"clientAccountId"`
	ClientVisible   *bool       `json:"clientVisible"`
}
type scriptListFilter struct {
	Search string
	Client string // "" = todos, "none" = sem cliente, uuid = aquele cliente
	Offset int
}
type scriptClient struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// ScriptCount: roteiros desta conta marcados com o cliente (o filtro da lateral só lista >0).
	ScriptCount int `json:"scriptCount"`
}

var errScriptConflict = errors.New("script conflict")
var errScriptNotFound = errors.New("script not found")
var errInvalidScriptClient = errors.New("invalid script client")

// scriptVisibleTo is the single read scope ($1 = current account): the account's own scripts,
// plus scripts of an agency account of the SAME organization that name $1 as client and keep
// client_visible. The organization check is repeated here so a client moved to another
// organization stops seeing them even if the stored link survives.
const scriptVisibleTo = `(s.account_id=$1::uuid or (s.client_account_id=$1::uuid and s.client_visible and exists(
 select 1 from core.accounts ag join core.accounts cl on cl.organization_id=ag.organization_id
 where ag.id=s.account_id and ag.is_agency and cl.id=s.client_account_id)))`

// scriptColumns matches scanScript; expects tools.scripts as s and the client account as c.
const scriptColumns = `s.id::text,s.title,s.rows,s.description,s.updated_at,
 coalesce(s.client_account_id::text,''),coalesce(c.name,''),s.client_visible,s.account_id<>$1::uuid`

type scriptStore struct{ pool *pgxpool.Pool }

func newScriptStore(pool *pgxpool.Pool) *scriptStore { return &scriptStore{pool: pool} }

type scriptScanner interface{ Scan(...any) error }

func scanScript(row scriptScanner) (scriptDocument, error) {
	var item scriptDocument
	var rows []byte
	var updated time.Time
	if err := row.Scan(&item.ID, &item.Title, &rows, &item.Description, &updated,
		&item.ClientAccountID, &item.ClientName, &item.ClientVisible, &item.Shared); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return item, errScriptNotFound
		}
		return item, err
	}
	if err := json.Unmarshal(rows, &item.Rows); err != nil {
		return item, err
	}
	item.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	return item, nil
}

// list returns the sidebar summaries visible to the account. A non-empty search matches the
// title or the plain text of any row (tags stripped), ignoring case.
func (s *scriptStore) list(ctx context.Context, account, user string, filter scriptListFilter) ([]scriptDocument, error) {
	rows, err := s.pool.Query(ctx, `select s.id::text,s.title,s.updated_at,
 coalesce(s.client_account_id::text,''),coalesce(c.name,''),s.client_visible,s.account_id<>$1::uuid,
 coalesce((select jsonb_agg(jsonb_build_object(
   'audio', left(regexp_replace(coalesce(r.value->>'audio',''), '<[^>]*>', ' ', 'g'),300),
   'video', left(regexp_replace(coalesce(r.value->>'video',''), '<[^>]*>', ' ', 'g'),300)) order by r.ord)
  from jsonb_array_elements(s.rows) with ordinality as r(value,ord) where r.ord <= 6),'[]'::jsonb)
 from tools.scripts s
 left join core.accounts c on c.id=s.client_account_id
 left join tools.script_orders o on o.account_id=$1::uuid and o.user_id=$2::uuid
 where `+scriptVisibleTo+`
   and ($4 = '' or s.title ilike $4 escape '\' or exists (
   select 1 from jsonb_array_elements(s.rows) as m(value)
   where regexp_replace(coalesce(m.value->>'audio','') || ' ' || coalesce(m.value->>'video',''), '<[^>]*>', ' ', 'g')
     ilike $4 escape '\'))
   and ($5 = '' or ($5 = 'none' and s.client_account_id is null) or s.client_account_id::text = $5)
 order by array_position(o.ids,s.id) nulls last,s.created_at desc,s.id desc limit 51 offset $3`,
		account, user, filter.Offset, likePattern(filter.Search), filter.Client)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []scriptDocument{}
	for rows.Next() {
		var item scriptDocument
		var updated time.Time
		var preview []byte
		if err := rows.Scan(&item.ID, &item.Title, &updated, &item.ClientAccountID, &item.ClientName,
			&item.ClientVisible, &item.Shared, &preview); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(preview, &item.Preview); err != nil {
			return nil, err
		}
		item.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
		item.Rows = []scriptRow{}
		result = append(result, item)
	}
	return result, rows.Err()
}

// clients lists the active client accounts of the agency's organization. Only an agency
// account gets options: a client account cannot point its scripts at another client.
func (s *scriptStore) clients(ctx context.Context, account string) ([]scriptClient, error) {
	rows, err := s.pool.Query(ctx, `select cl.id::text,cl.name,
 (select count(*) from tools.scripts s where s.account_id=$1::uuid and s.client_account_id=cl.id)::int
 from core.accounts ag
 join core.accounts cl on cl.organization_id=ag.organization_id and not cl.is_agency and cl.is_active
 where ag.id=$1::uuid and ag.is_agency order by lower(cl.name),cl.id`, account)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []scriptClient{}
	for rows.Next() {
		var item scriptClient
		if err := rows.Scan(&item.ID, &item.Name, &item.ScriptCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *scriptStore) reorder(ctx context.Context, account, user string, ids []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `select id::text from tools.scripts where account_id=$1::uuid and id=any($2::uuid[]) for share`, account, ids)
	if err != nil {
		return err
	}
	count := 0
	for rows.Next() {
		count++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if count != len(ids) {
		return errScriptNotFound
	}
	_, err = tx.Exec(ctx, `insert into tools.script_orders(account_id,user_id,ids) values($1::uuid,$2::uuid,$3::uuid[])
 on conflict(account_id,user_id) do update set ids=excluded.ids || array(select old_id from unnest(tools.script_orders.ids) as old_id where not old_id=any(excluded.ids))`, account, user, ids)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *scriptStore) save(ctx context.Context, account, id string, input scriptInput) (scriptDocument, error) {
	rows, err := json.Marshal(input.Rows)
	if err != nil {
		return scriptDocument{}, err
	}
	if input.ClientAccountID != "" {
		var valid bool
		if err := s.pool.QueryRow(ctx, `select exists(select 1 from core.accounts ag
 join core.accounts cl on cl.organization_id=ag.organization_id
 where ag.id=$1::uuid and ag.is_agency and cl.id=$2::uuid and not cl.is_agency and cl.is_active)`,
			account, input.ClientAccountID).Scan(&valid); err != nil {
			return scriptDocument{}, err
		}
		if !valid {
			return scriptDocument{}, errInvalidScriptClient
		}
	}
	visible := input.ClientVisible == nil || *input.ClientVisible
	if id == "" {
		return scanScript(s.pool.QueryRow(ctx, `with s as (
 insert into tools.scripts(account_id,title,rows,description,client_account_id,client_visible)
 values($1::uuid,$2,$3::jsonb,$4,nullif($5,'')::uuid,$6) returning *)
 select `+scriptColumns+` from s left join core.accounts c on c.id=s.client_account_id`,
			account, input.Title, rows, input.Description, input.ClientAccountID, visible))
	}
	if !isUUID(id) {
		return scriptDocument{}, errScriptNotFound
	}
	item, err := scanScript(s.pool.QueryRow(ctx, `with s as (
 update tools.scripts set title=$3, rows=$4::jsonb, description=$5, client_account_id=nullif($7,'')::uuid,
   client_visible=$8, updated_at=now()
 where account_id=$1::uuid and id=$2::uuid and updated_at=$6::timestamptz returning *)
 select `+scriptColumns+` from s left join core.accounts c on c.id=s.client_account_id`,
		account, id, input.Title, rows, input.Description, input.UpdatedAt, input.ClientAccountID, visible))
	if errors.Is(err, errScriptNotFound) {
		var exists bool
		if checkErr := s.pool.QueryRow(ctx, `select exists(select 1 from tools.scripts where account_id=$1::uuid and id=$2::uuid)`, account, id).Scan(&exists); checkErr != nil {
			return item, checkErr
		}
		if exists {
			return item, errScriptConflict
		}
	}
	return item, err
}
func (s *scriptStore) delete(ctx context.Context, account, id string) error {
	if !isUUID(id) {
		return errScriptNotFound
	}
	result, err := s.pool.Exec(ctx, `delete from tools.scripts where account_id=$1::uuid and id=$2::uuid`, account, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errScriptNotFound
	}
	return nil
}

func (s *scriptStore) get(ctx context.Context, account, id string) (scriptDocument, error) {
	if !isUUID(id) {
		return scriptDocument{}, errScriptNotFound
	}
	return scanScript(s.pool.QueryRow(ctx, `select `+scriptColumns+` from tools.scripts s
 left join core.accounts c on c.id=s.client_account_id where s.id=$2::uuid and `+scriptVisibleTo, account, id))
}

func (s *scriptStore) addImage(ctx context.Context, account string, image scriptImage) error {
	_, err := s.pool.Exec(ctx, `insert into tools.script_images(id,account_id,mime_type,size_bytes) values($1::uuid,$2::uuid,$3,$4)`, image.ID, account, image.MIME, image.Size)
	return err
}

// getImage resolves an image the account may read: its own, or one embedded in a script shared
// with it (scriptVisibleTo). Account in the result is the owner, used to locate the file.
func (s *scriptStore) getImage(ctx context.Context, account, id string) (scriptImage, error) {
	var image scriptImage
	err := s.pool.QueryRow(ctx, `select i.id::text,i.account_id::text,i.mime_type,i.size_bytes from tools.script_images i
 where i.id=$2::uuid and (i.account_id=$1::uuid or exists(select 1 from tools.scripts s
   where s.account_id=i.account_id and `+scriptVisibleTo+`
     and (strpos(s.rows::text,i.id::text)>0 or strpos(s.description,i.id::text)>0)))`,
		account, id).Scan(&image.ID, &image.Account, &image.MIME, &image.Size)
	if errors.Is(err, pgx.ErrNoRows) {
		return image, errScriptNotFound
	}
	return image, err
}

// likePattern turns free text into an ILIKE "contains" pattern with %, _ and \ escaped.
func likePattern(search string) string {
	if search == "" {
		return ""
	}
	return "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(search) + "%"
}
