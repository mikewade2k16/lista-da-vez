package omnichannel

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// F13 — Purge de MIDIA em disco (o gap que o banco nao alcanca, C5)
// ============================================================================
//
// A midia vive em disco (F6): {rootDir}/{accountId}/{conversationId}/{random}.{ext}, apontada
// por messaging.messages.media_storage_key. DELETE de mensagem NAO libera byte nenhum — o
// arquivo com o audio do cliente fica no volume para sempre. Esta camada apaga o arquivo.
//
// SEGURANCA: so apaga SOB {rootDir}/{accountId}/. O path e resolvido e o prefixo CONFERIDO
// antes de qualquer os.Remove — media_storage_key/accountId sao dado de banco, e um traversal
// aqui apagaria arquivo de outra conta. Segmentos passam por sanitizeSegment (media_storage.go).

// mediaOrphanGrace e a janela de carencia da varredura de orfaos (C5): arquivo com mtime mais
// novo que isso NAO e apagado — pode ser upload em voo cuja transacao ainda nao commitou.
const mediaOrphanGrace = 24 * time.Hour

// mediaPurger apaga arquivos de midia sob uma raiz privada. rootDir espelha o do
// DiskMediaStorage (MediaDirFromEnv) — a mesma raiz que a F6 grava.
type mediaPurger struct {
	rootDir string
}

// newMediaPurger monta o purger. rootDir vazio => raiz privada default (igual a F6).
func newMediaPurger(rootDir string) *mediaPurger {
	root := strings.TrimSpace(rootDir)
	if root == "" {
		root = defaultMediaDir
	}
	return &mediaPurger{rootDir: root}
}

// purgeConversationDir apaga o diretorio de UMA conversa ({root}/{acct}/{conv}) e devolve
// (arquivos, bytes) liberados. dryRun => apenas conta, sem apagar. Diretorio inexistente =>
// (0,0,nil). Apaga a arvore inteira da conversa de uma vez (todos os arquivos daquela conversa).
func (m *mediaPurger) purgeConversationDir(accountID, conversationID string, dryRun bool) (int64, int64, error) {
	dir, ok := m.containedDir(sanitizeSegment(accountID), sanitizeSegment(conversationID))
	if !ok {
		return 0, 0, ErrMediaInvalid
	}
	files, bytes, err := walkAccumulate(dir)
	if err != nil || dryRun || files == 0 {
		return files, bytes, err
	}
	if rmErr := os.RemoveAll(dir); rmErr != nil { //nolint:gosec // dir contido em rootDir (containedDir)
		return files, bytes, rmErr
	}
	return files, bytes, nil
}

// scanOrphans varre {root}/{acct} e apaga arquivos SEM media_storage_key correspondente
// (known) e com mtime anterior a now-carencia. Cobre o upload que gravou o arquivo e falhou
// antes do insert (F6). Devolve (arquivos, bytes) removidos. dryRun => so conta.
func (m *mediaPurger) scanOrphans(accountID string, known map[string]bool, now time.Time, dryRun bool) (int64, int64, error) {
	acct := sanitizeSegment(accountID)
	dir, ok := m.containedDir(acct, "")
	if !ok {
		return 0, 0, ErrMediaInvalid
	}
	rootAbs, err := filepath.Abs(m.rootDir)
	if err != nil {
		return 0, 0, err
	}
	cutoff := now.Add(-mediaOrphanGrace)
	var files, bytes int64
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // conta sem diretorio de midia — nada a varrer
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.ModTime().After(cutoff) {
			return nil // ilegivel ou dentro da carencia: nao mexe
		}
		key := storageKeyFor(rootAbs, path)
		if key == "" || known[key] {
			return nil // referenciado por mensagem: nao e orfao
		}
		files++
		bytes += info.Size()
		if !dryRun {
			if rmErr := os.Remove(path); rmErr != nil { //nolint:gosec // path sob rootDir (WalkDir a partir de containedDir)
				return rmErr
			}
		}
		return nil
	})
	return files, bytes, walkErr
}

// containedDir resolve {rootDir}/{acct}[/{conv}] e CONFERE que o resultado fica sob rootDir
// (defesa contra traversal por dado de banco). acct vazio ou path fora da raiz => ok=false.
func (m *mediaPurger) containedDir(acct, conv string) (string, bool) {
	if acct == "" {
		return "", false
	}
	rootAbs, err := filepath.Abs(m.rootDir)
	if err != nil {
		return "", false
	}
	target := filepath.Join(m.rootDir, acct)
	if conv != "" {
		target = filepath.Join(m.rootDir, acct, conv)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", false
	}
	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, rootAbs+string(filepath.Separator)) {
		return "", false
	}
	return targetAbs, true
}

// storageKeyFor devolve o media_storage_key ({acct}/{conv}/{name}, com barras) de um arquivo
// absoluto sob rootAbs. Fora da raiz => "".
func storageKeyFor(rootAbs, fullPath string) string {
	rel, err := filepath.Rel(rootAbs, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel)
}

// walkAccumulate soma arquivos e bytes sob dir (toda a arvore). Diretorio ausente => (0,0,nil).
func walkAccumulate(dir string) (int64, int64, error) {
	var files, bytes int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files++
		bytes += info.Size()
		return nil
	})
	return files, bytes, err
}
