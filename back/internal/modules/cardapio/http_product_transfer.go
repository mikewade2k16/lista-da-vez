package cardapio

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const maxProductImportBodyBytes = 10 << 20

func handleBulkProducts(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if err := requireCardapioPerm(svc, r, accountID, permManage); err != nil {
			writeServiceError(w, r, err)
			return
		}
		var in ProductBulkInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		result, err := svc.BulkProducts(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, result)
	}
}

func handleExportProducts(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if err := requireCardapioPerm(svc, r, accountID, permView); err != nil {
			writeServiceError(w, r, err)
			return
		}
		document, err := svc.ExportProducts(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, document)
	}
}

func handleImportProducts(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if err := requireCardapioPerm(svc, r, accountID, permManage); err != nil {
			writeServiceError(w, r, err)
			return
		}

		var in ProductImportInput
		if err := readProductImportJSON(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_import", "Arquivo de importacao invalido.")
			return
		}
		result, err := svc.ImportProducts(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, result)
	}
}

func handlePreviewProductImport(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if err := requireCardapioPerm(svc, r, accountID, permManage); err != nil {
			writeServiceError(w, r, err)
			return
		}

		var in ProductImportInput
		if err := readProductImportJSON(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_import", "Arquivo de importacao invalido.")
			return
		}
		preview, err := svc.PreviewProductImport(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, preview)
	}
}

func readProductImportJSON(w http.ResponseWriter, r *http.Request, dst *ProductImportInput) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	defer func() {
		_ = r.Body.Close()
	}()
	r.Body = http.MaxBytesReader(w, r.Body, maxProductImportBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}
