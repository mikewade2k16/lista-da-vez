package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/finance"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/storage"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/config"
)

// privateOriginals guarda originais privados (comprovantes do Finance e fontes do
// Knowledge). Com R2 ligado usa o Storage canonico (ledger, cotas, sem URL publica).
// Com R2 desligado e PRIVATE_FILES_DIR definido, grava em disco FORA de UPLOADS_DIR,
// que e servido publicamente em /uploads/. Sem nenhum dos dois, falha de forma
// explicita: o chamador mostra "original nao guardado" em vez de fingir que guardou.
type privateOriginals struct {
	storage *storage.Module
	dir     string
}

var (
	errPrivateOriginalsUnavailable = errors.New("private original storage unavailable")
	errPrivateOriginalNotFound     = errors.New("private original not found")
	privateUUID                    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	privateHex                     = regexp.MustCompile(`^[0-9a-f]{32}$`)
	privateSource                  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,30}$`)
)

// financeOriginalFiles só liga a porta quando existe destino: R2 configurado ou
// PRIVATE_FILES_DIR. Sem nenhum, a tela informa que os originais não são guardados.
func financeOriginalFiles(module *storage.Module, cfg config.Config) finance.OriginalFiles {
	if !cfg.R2Enabled && cfg.PrivateFilesDir == "" {
		return nil
	}
	return privateOriginals{storage: module, dir: cfg.PrivateFilesDir}
}

// uuidFromHex formata o ID do Storage (32 hex) como UUID, formato exigido pelas
// tabelas que referenciam o original (ex.: knowledge original_storage_id uuid).
func uuidFromHex(id string) string {
	if !privateHex.MatchString(id) {
		return ""
	}
	return id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:32]
}

// Put devolve uma referencia "r2:<uuid>" ou "disk:<uuid>". A chave torna a gravacao
// idempotente: repetir a mesma chave nao cria outro objeto.
func (p privateOriginals) Put(ctx context.Context, account, actor, source, key, name, mime string, content []byte) (string, error) {
	if !privateUUID.MatchString(account) || !privateSource.MatchString(source) || key == "" || len(content) == 0 {
		return "", errPrivateOriginalsUnavailable
	}
	if svc := p.storage.Service(); svc != nil {
		object, err := svc.Upload(ctx, storage.UploadInput{AccountID: account, SourceModule: source, IdempotencyKey: key, FileName: name, ContentType: mime, Content: content, CreatedBy: actor})
		switch {
		case err == nil:
			if id := uuidFromHex(object.ID); id != "" {
				return "r2:" + id, nil
			}
			return "", errPrivateOriginalsUnavailable
		case !errors.Is(err, storage.ErrDisabled) && !errors.Is(err, storage.ErrUploadsDisabled):
			return "", err
		}
	}
	if p.dir == "" {
		return "", errPrivateOriginalsUnavailable
	}
	sum := sha256.Sum256([]byte(source + "\x00" + account + "\x00" + key))
	id := uuidFromHex(hex.EncodeToString(sum[:16]))
	dir := filepath.Join(p.dir, source, account)
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // segmentos validados por regex, sem traversal
		return "", err
	}
	path := filepath.Join(dir, id)
	if _, err := os.Stat(path); err == nil {
		return "disk:" + id, nil
	}
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return "", err
	}
	_, werr := tmp.Write(content)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmp.Name())
		return "", errors.Join(werr, cerr)
	}
	if err = os.Chmod(tmp.Name(), 0o600); err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return "disk:" + id, nil
}

func (p privateOriginals) locate(account, source, ref string) (kind, id string, err error) {
	kind, id, ok := strings.Cut(ref, ":")
	if !ok || !privateUUID.MatchString(id) || !privateUUID.MatchString(account) || !privateSource.MatchString(source) || (kind != "r2" && kind != "disk") {
		return "", "", errPrivateOriginalNotFound
	}
	return kind, id, nil
}

func (p privateOriginals) Open(ctx context.Context, account, source, ref string) (io.ReadCloser, error) {
	kind, id, err := p.locate(account, source, ref)
	if err != nil {
		return nil, err
	}
	if kind == "r2" {
		if p.storage.Service() == nil {
			return nil, errPrivateOriginalsUnavailable
		}
		_, content, err := p.storage.Service().DownloadForSource(ctx, account, strings.ReplaceAll(id, "-", ""), source, "")
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, errPrivateOriginalNotFound
		}
		if err != nil {
			return nil, err
		}
		return content.Body, nil
	}
	if p.dir == "" {
		return nil, errPrivateOriginalsUnavailable
	}
	file, err := os.Open(filepath.Join(p.dir, source, account, id)) //nolint:gosec // segmentos validados por regex
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errPrivateOriginalNotFound
	}
	return file, err
}

// Delete remove o binario. Ja removido conta como sucesso (idempotente).
func (p privateOriginals) Delete(ctx context.Context, account, source, ref string) error {
	kind, id, err := p.locate(account, source, ref)
	if err != nil {
		return err
	}
	if kind == "r2" {
		if p.storage.Service() == nil {
			return errPrivateOriginalsUnavailable
		}
		err = p.storage.Service().DeleteForSource(ctx, account, strings.ReplaceAll(id, "-", ""), source)
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil
		}
		return err
	}
	if p.dir == "" {
		return errPrivateOriginalsUnavailable
	}
	err = os.Remove(filepath.Join(p.dir, source, account, id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove private original: %w", err)
	}
	return nil
}
