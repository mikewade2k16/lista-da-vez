package app

import (
	"net/http"
	"path"
	"strings"
)

// newUploadsHandler serve os arquivos de /uploads/ sem expor listagem de
// diretorio (A-02). Requisicao cujo alvo e diretorio, ou arquivo oculto
// (nome iniciado em "."), responde 404. Traversal continua barrado por
// http.Dir (rejeita ".." e separadores invalidos) e por path.Clean aqui.
func newUploadsHandler(root string) http.Handler {
	dir := http.Dir(root)
	files := http.StripPrefix("/uploads/", http.FileServer(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/uploads/")
		if name == "" || strings.HasSuffix(name, "/") || hasHiddenSegment(name) {
			http.NotFound(w, r)
			return
		}
		file, err := dir.Open(path.Clean("/" + name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		info, statErr := file.Stat()
		_ = file.Close()
		if statErr != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}

func hasHiddenSegment(name string) bool {
	for _, segment := range strings.Split(name, "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}
