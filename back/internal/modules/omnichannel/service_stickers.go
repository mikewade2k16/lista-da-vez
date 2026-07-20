package omnichannel

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// Service dos stickers salvos (spec F12 C1): validacao (sniff de mime + teto PROPRIO de
// ~1MB), storage em DISCO reusando o media_storage.go da F6 e poda FIFO > 200/conta.
//
// Reuso da F6, sem tocar media_storage.go: sticker NAO tem conversa, entao o path
// {root}/{accountId}/{conversationId}/... vira {root}/{accountId}/stickers/{random}.{ext}
// passando o segmento fixo "stickers" no lugar do conversationId de SaveReader.
//
// O DADO SERVIDO ao front e o data URL base64 REAL, relido do disco na serializacao — JAMAIS
// uma URL de endpoint. O front (normalizeSavedStickerItem) DESCARTA em silencio o item cujo
// dataUrl nao comeca com "data:image/": devolver URL faria a figurinha sumir da grade sem
// erro. Por isso o base64 volta inline (o teto proprio de ~1MB limita o tamanho da resposta).
//
// account_id/userID vem SEMPRE do Principal (nunca do body). Sticker de outra conta no delete
// => nao-encontrado (404 no handler, nunca 403 = enumeration).

const (
	// stickerMaxDecodedBytes e o teto PROPRIO do sticker (~1MB DECODIFICADO — decisao do dono
	// 2026-07-18), NAO o min(max_upload_mb, 20MB) da midia. O WhatsApp usa ~512KB; 1MiB limita
	// a resposta base64 (que infla ~33%) mesmo no limit maximo. Acima => 413.
	stickerMaxDecodedBytes = 1 << 20
	// stickerDefaultLimit/stickerMaxLimit: contrato do front (anexo F7). 1..200, default 36.
	stickerDefaultLimit = 36
	stickerMaxLimit     = 200
	// stickerFIFOKeep: teto de stickers por conta; a poda apaga tudo alem disso (linha + arquivo).
	stickerFIFOKeep = 200
	// stickerStorageSegment ocupa o 2o segmento do path da F6 (o conversationId de SaveReader):
	// sticker nao tem conversa => {root}/{accountId}/stickers/{random}.{ext}.
	stickerStorageSegment = "stickers"
)

// stickerAllowedMime e o allowlist por SNIFF (image/webp|png|jpeg|gif). "jpg" canoniza para
// image/jpeg no http.DetectContentType — sem entrada propria. Fora daqui => 415.
var stickerAllowedMime = map[string]struct{}{
	"image/webp": {},
	"image/png":  {},
	"image/jpeg": {},
	"image/gif":  {},
}

// StickerView e o item servido ao front (web-reference .../useInboxChatStickerAssets.ts:39-48).
// dataUrl e SEMPRE data URL base64 real (nunca URL de endpoint) — ver doc do arquivo.
type StickerView struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	DataURL         string    `json:"dataUrl"`
	MimeType        string    `json:"mimeType"`
	SizeBytes       int64     `json:"sizeBytes"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	CreatedByUserID *string   `json:"createdByUserId"`
}

// StickerInput e o body do POST /stickers (contrato do front :138-146). sizeBytes vem do
// cliente e NAO e confiavel: o teto e medido no servidor sobre o tamanho DECODIFICADO.
type StickerInput struct {
	Name      string `json:"name"`
	DataURL   string `json:"dataUrl"`
	MimeType  string `json:"mimeType"`
	SizeBytes *int   `json:"sizeBytes"`
}

// StickerService orquestra os stickers. media = o DiskMediaStorage da F6 (a MESMA instancia
// do envio, construida no Build); store = a persistencia messaging.*.
type StickerService struct {
	store  *Store
	media  *DiskMediaStorage
	logger *slog.Logger
}

// NewStickerService monta o service. logger nil => slog.Default().
func NewStickerService(store *Store, media *DiskMediaStorage, logger *slog.Logger) *StickerService {
	if logger == nil {
		logger = slog.Default()
	}
	return &StickerService{store: store, media: media, logger: logger}
}

// normalizeStickerLimit aplica o contrato 1..200, default 36 (fora da faixa nao e erro: <=0
// cai no default, acima do teto satura — igual ao clamp do front).
func normalizeStickerLimit(limit int) int {
	switch {
	case limit <= 0:
		return stickerDefaultLimit
	case limit > stickerMaxLimit:
		return stickerMaxLimit
	default:
		return limit
	}
}

// List devolve os stickers da conta (mais novos primeiro) ja com o data URL remontado do
// disco. Item cujo arquivo sumiu do disco e PULADO (o front descartaria o dataUrl quebrado de
// qualquer jeito) — a linha orfa e podada no proximo insert. Nao derruba a grade toda.
func (s *StickerService) List(ctx context.Context, accountID string, limit int) ([]StickerView, error) {
	rows, err := s.store.ListStickers(ctx, accountID, normalizeStickerLimit(limit))
	if err != nil {
		return nil, err
	}
	out := make([]StickerView, 0, len(rows))
	for _, row := range rows {
		view, err := s.viewFromRow(row)
		if err != nil {
			s.logger.Warn("omnichannel_sticker_read_skip", "account_id", accountID, "sticker_id", row.ID)
			continue
		}
		out = append(out, view)
	}
	return out, nil
}

// Create valida (sniff + teto), grava em disco (streaming), insere a linha (data_url vazio) e
// poda FIFO. Devolve a view com o data URL real (relido do disco). account_id/userID = Principal.
func (s *StickerService) Create(ctx context.Context, accountID, userID string, in StickerInput) (StickerView, error) {
	// O mime CONFIAVEL vem do sniff dos bytes decodificados, nao do mimeType/data URL declarados.
	_, payload, ok := parseDataURL(strings.TrimSpace(in.DataURL))
	if !ok {
		return StickerView{}, ErrMediaInvalid
	}

	// Decode base64 em STREAMING (nunca materializa o arquivo em []byte). bufio permite espiar
	// os primeiros bytes para o sniff sem consumi-los: o mesmo reader segue para o disco.
	buffered := bufio.NewReader(base64.NewDecoder(base64.StdEncoding, strings.NewReader(payload)))
	sniffed, err := sniffStickerMime(buffered)
	if err != nil {
		return StickerView{}, err
	}

	stored, err := s.media.SaveReader(accountID, stickerStorageSegment, sniffed, in.Name, buffered, stickerMaxDecodedBytes)
	if err != nil {
		return StickerView{}, err
	}

	row, err := s.store.InsertSticker(ctx, stickerInsert{
		AccountID:       accountID,
		CreatedByUserID: userID,
		Name:            stickerName(in.Name, stored.MimeType),
		MimeType:        stored.MimeType,
		SizeBytes:       stored.SizeBytes,
		StorageKey:      stored.StorageKey,
	})
	if err != nil {
		// Insert falhou: remove o arquivo recem-gravado para nao deixar orfao em disco.
		s.removeFile(stored.StorageKey)
		return StickerView{}, err
	}

	s.pruneFIFO(ctx, accountID)

	return s.viewFromRow(row)
}

// Delete apaga a linha (escopada por conta) e o arquivo de disco JUNTOS. found=false =>
// ErrNotFound (404, nunca 403): sticker de outra conta e indistinguivel de inexistente.
func (s *StickerService) Delete(ctx context.Context, accountID, id string) error {
	storageKey, found, err := s.store.DeleteSticker(ctx, accountID, id)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if storageKey != nil {
		s.removeFile(*storageKey)
	}
	return nil
}

// pruneFIFO apaga tudo alem dos 200 mais recentes da conta — linha E arquivo juntos. Best-
// effort: falha na poda nao derruba o insert (o sticker ja foi criado); so registra.
func (s *StickerService) pruneFIFO(ctx context.Context, accountID string) {
	pruned, err := s.store.PruneStickers(ctx, accountID, stickerFIFOKeep)
	if err != nil {
		s.logger.Error("omnichannel_sticker_prune", "account_id", accountID, "error", err.Error())
		return
	}
	for _, p := range pruned {
		if p.StorageKey != nil {
			s.removeFile(*p.StorageKey)
		}
	}
}

// viewFromRow remonta o data URL base64 REAL relendo o arquivo do disco (o front exige
// data:image/...). Erro de leitura sobe para o chamador decidir (pular na lista, 500 no create).
func (s *StickerService) viewFromRow(row stickerRow) (StickerView, error) {
	if row.StorageKey == nil || strings.TrimSpace(*row.StorageKey) == "" {
		return StickerView{}, ErrMediaInvalid
	}
	dataURL, err := s.dataURLFromDisk(*row.StorageKey, row.MimeType)
	if err != nil {
		return StickerView{}, err
	}
	return StickerView{
		ID:              row.ID,
		Name:            row.Name,
		DataURL:         dataURL,
		MimeType:        row.MimeType,
		SizeBytes:       row.SizeBytes,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		CreatedByUserID: row.CreatedByUserID,
	}, nil
}

// dataURLFromDisk le o arquivo pelo storage_key (containment ja checado no Open da F6) e monta
// "data:<mime>;base64,<...>" com o encoder em STREAMING (io.Copy do arquivo para o base64).
func (s *StickerService) dataURLFromDisk(storageKey, mime string) (string, error) {
	file, _, err := s.media.Open(storageKey)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	var sb strings.Builder
	sb.WriteString("data:")
	sb.WriteString(mime)
	sb.WriteString(";base64,")
	enc := base64.NewEncoder(base64.StdEncoding, &sb)
	if _, err := io.Copy(enc, file); err != nil {
		_ = enc.Close()
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// removeFile apaga o arquivo do sticker do disco reusando o Open da F6 (que ja faz o
// containment sob a raiz privada): pega o path absoluto seguro e os.Remove. Arquivo ausente
// ou path invalido = no-op silencioso (idempotente — poda/delete nunca falham por isso).
func (s *StickerService) removeFile(storageKey string) {
	file, _, err := s.media.Open(storageKey)
	if err != nil {
		return
	}
	path := file.Name()
	_ = file.Close()
	if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
		s.logger.Warn("omnichannel_sticker_file_remove", "error", rmErr.Error())
	}
}

// sniffStickerMime detecta o mime pelos PRIMEIROS bytes DECODIFICADOS (nao pelo mimeType
// declarado no body) e valida contra o allowlist de sticker. Fora do allowlist => 415; corpo
// vazio/base64 malformado no inicio => 400.
func sniffStickerMime(r *bufio.Reader) (string, error) {
	head, err := r.Peek(512)
	if err != nil && err != io.EOF {
		return "", ErrMediaInvalid
	}
	if len(head) == 0 {
		return "", ErrMediaInvalid
	}
	mime := normalizeMime(http.DetectContentType(head))
	if _, ok := stickerAllowedMime[mime]; !ok {
		return "", ErrMediaUnsupported
	}
	return mime, nil
}

// stickerName escolhe o rotulo: o nome enviado (base, limitado a 120) ou "figurinha"+ext do
// mime. name e NOT NULL no schema — nunca devolvemos vazio.
func stickerName(name, mime string) string {
	clean := strings.TrimSpace(name)
	if clean != "" {
		if len(clean) > 120 {
			clean = clean[:120]
		}
		return clean
	}
	if spec, ok := allowedMedia[mime]; ok {
		return "figurinha" + spec.ext
	}
	return "figurinha"
}
