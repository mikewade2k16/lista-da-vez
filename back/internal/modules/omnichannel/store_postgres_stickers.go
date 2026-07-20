package omnichannel

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Persistencia dos stickers salvos (messaging.saved_stickers) — spec F12 C1.
//
// REGRA DA CASA, sem excecao: TODO select/delete filtra por account_id (defesa em
// profundidade, principio 2), mesmo com o id ja validado no service. IDs sao string + cast
// no SQL ($1::uuid); nao importamos pacote de uuid (padrao da casa).
//
// O base64 NUNCA vai para o banco (D-B/D2): a coluna data_url grava a string vazia e os bytes
// vivem em disco. storage_key (coluna ADITIVA desta fase, criada pelo orquestrador) guarda o
// path RELATIVO — remontado em data URL na serializacao. data_url e NOT NULL no schema 0200;
// por isso o insert grava vazio (sem base64, satisfazendo a constraint independente de a
// migration aditiva relaxar ou nao o NOT NULL).

// stickerRow e a linha crua de messaging.saved_stickers. Sem data_url: ele nunca sai do banco
// (fica vazio) — o data URL servido ao front e remontado do disco pelo service.
type stickerRow struct {
	ID              string
	Name            string
	MimeType        string
	SizeBytes       int64
	StorageKey      *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CreatedByUserID *string
}

// stickerInsert e o payload de InsertSticker. account_id/created_by_user_id vem SEMPRE do
// Principal, nunca do body (principio 2).
type stickerInsert struct {
	AccountID       string
	CreatedByUserID string
	Name            string
	MimeType        string
	SizeBytes       int64
	StorageKey      string
}

// prunedSticker identifica um sticker removido na poda FIFO: id (log) + storage_key (para
// apagar o arquivo de disco JUNTO — linha e arquivo nunca se separam, senao vira orfao que
// nenhum purge coleta).
type prunedSticker struct {
	ID         string
	StorageKey *string
}

// stickerCols sai na ordem esperada por scanSticker. uuid como ::text (scan em string/*string
// sem pacote de uuid).
const stickerCols = `id::text, name, mime_type, size_bytes, storage_key, created_at, updated_at, created_by_user_id::text`

func scanSticker(row rowScanner) (stickerRow, error) {
	var s stickerRow
	err := row.Scan(&s.ID, &s.Name, &s.MimeType, &s.SizeBytes, &s.StorageKey,
		&s.CreatedAt, &s.UpdatedAt, &s.CreatedByUserID)
	return s, err
}

// ListStickers devolve os stickers da conta, mais novos primeiro, limitados a `limit`
// (contrato do front: created_at desc). Sticker de outra conta cai no filtro de account_id.
func (s *Store) ListStickers(ctx context.Context, accountID string, limit int) ([]stickerRow, error) {
	rows, err := s.pool.Query(ctx, `select `+stickerCols+`
		from messaging.saved_stickers
		where account_id = $1::uuid
		order by created_at desc, id desc
		limit $2`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]stickerRow, 0, limit)
	for rows.Next() {
		row, err := scanSticker(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// InsertSticker grava a linha e devolve o row completo. data_url grava a string vazia (o
// base64 fica em disco, nunca no banco). created_by_user_id vazio => NULL (nullif).
func (s *Store) InsertSticker(ctx context.Context, in stickerInsert) (stickerRow, error) {
	return scanSticker(s.pool.QueryRow(ctx, `
		insert into messaging.saved_stickers
			(account_id, created_by_user_id, name, data_url, mime_type, size_bytes, storage_key)
		values ($1::uuid, nullif($2, '')::uuid, $3, '', $4, $5, $6)
		returning `+stickerCols,
		in.AccountID, in.CreatedByUserID, in.Name, in.MimeType, in.SizeBytes, in.StorageKey))
}

// DeleteSticker apaga a linha escopada por conta e devolve o storage_key para o service
// remover o arquivo. found=false (sticker inexistente OU de outra conta — indistinguiveis)
// => o handler responde 404, nunca 403 (403 confirmaria a existencia = enumeration).
func (s *Store) DeleteSticker(ctx context.Context, accountID, id string) (storageKey *string, found bool, err error) {
	err = s.pool.QueryRow(ctx, `
		delete from messaging.saved_stickers
		where account_id = $1::uuid and id = $2::uuid
		returning storage_key`, accountID, id).Scan(&storageKey)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	default:
		return storageKey, true, nil
	}
}

// PruneStickers apaga tudo alem dos `keep` mais recentes da conta (poda FIFO > 200) e devolve
// os storage_key removidos para o service apagar os arquivos JUNTO. Uma unica query (CTE de
// delete com subselect ordenado + offset): sem corrida entre "listar antigos" e "apagar".
func (s *Store) PruneStickers(ctx context.Context, accountID string, keep int) ([]prunedSticker, error) {
	rows, err := s.pool.Query(ctx, `
		delete from messaging.saved_stickers
		where account_id = $1::uuid
		  and id in (
			select id from messaging.saved_stickers
			where account_id = $1::uuid
			order by created_at desc, id desc
			offset $2
		  )
		returning id::text, storage_key`, accountID, keep)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]prunedSticker, 0)
	for rows.Next() {
		var p prunedSticker
		if err := rows.Scan(&p.ID, &p.StorageKey); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
