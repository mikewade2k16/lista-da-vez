package cardapio

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterCatalogRoutes monta os endpoints de catalogo do painel (categorias,
// produtos e avaliacoes).
func RegisterCatalogRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}

	mux.Handle("GET /v1/cardapio/restaurants/{id}/categories", wrap(handleListCategories(svc)))
	mux.Handle("POST /v1/cardapio/restaurants/{id}/categories", wrap(handleCreateCategory(svc)))
	mux.Handle("PATCH /v1/cardapio/categories/{id}", wrap(handleUpdateCategory(svc)))
	mux.Handle("DELETE /v1/cardapio/categories/{id}", wrap(handleDeleteCategory(svc)))

	mux.Handle("GET /v1/cardapio/restaurants/{id}/products", wrap(handleListProducts(svc)))
	mux.Handle("POST /v1/cardapio/restaurants/{id}/products", wrap(handleCreateProduct(svc)))
	mux.Handle("POST /v1/cardapio/restaurants/{id}/products/bulk-action", wrap(handleBulkProducts(svc)))
	mux.Handle("GET /v1/cardapio/restaurants/{id}/products/export", wrap(handleExportProducts(svc)))
	mux.Handle("POST /v1/cardapio/restaurants/{id}/products/import/preview", wrap(handlePreviewProductImport(svc)))
	mux.Handle("POST /v1/cardapio/restaurants/{id}/products/import", wrap(handleImportProducts(svc)))
	mux.Handle("GET /v1/cardapio/products/{id}", wrap(handleGetProduct(svc)))
	mux.Handle("PATCH /v1/cardapio/products/{id}", wrap(handleUpdateProduct(svc)))
	mux.Handle("DELETE /v1/cardapio/products/{id}", wrap(handleDeleteProduct(svc)))

	mux.Handle("GET /v1/cardapio/products/{id}/reviews", wrap(handleListReviews(svc)))
	mux.Handle("POST /v1/cardapio/products/{id}/reviews", wrap(handleCreateReview(svc)))
	mux.Handle("GET /v1/cardapio/restaurants/{id}/reviews", wrap(handleListEstablishmentReviews(svc)))
	mux.Handle("POST /v1/cardapio/restaurants/{id}/reviews", wrap(handleCreateEstablishmentReview(svc)))
	mux.Handle("PATCH /v1/cardapio/reviews/{id}", wrap(handleUpdateReview(svc)))
	mux.Handle("DELETE /v1/cardapio/reviews/{id}", wrap(handleDeleteReview(svc)))
}

// ============================================================================
// Categories
// ============================================================================

func handleListCategories(svc *Service) http.HandlerFunc {
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
		items, err := svc.ListCategories(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"categories": items})
	}
}

func handleCreateCategory(svc *Service) http.HandlerFunc {
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
		var in CategoryInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.CreateCategory(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleUpdateCategory(svc *Service) http.HandlerFunc {
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
		var in CategoryInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.UpdateCategory(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDeleteCategory(svc *Service) http.HandlerFunc {
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
		if err := svc.DeleteCategory(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ============================================================================
// Products
// ============================================================================

func handleListProducts(svc *Service) http.HandlerFunc {
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
		items, err := svc.ListProducts(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"products": items})
	}
}

func handleCreateProduct(svc *Service) http.HandlerFunc {
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
		var in ProductInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.CreateProduct(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleGetProduct(svc *Service) http.HandlerFunc {
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
		view, err := svc.GetProduct(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleUpdateProduct(svc *Service) http.HandlerFunc {
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
		var in ProductInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.UpdateProduct(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDeleteProduct(svc *Service) http.HandlerFunc {
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
		if err := svc.DeleteProduct(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ============================================================================
// Reviews
// ============================================================================

func handleListReviews(svc *Service) http.HandlerFunc {
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
		items, err := svc.ListReviews(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"reviews": items})
	}
}

func handleCreateReview(svc *Service) http.HandlerFunc {
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
		var in ReviewInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.CreateReview(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

// handleListEstablishmentReviews lista as avaliacoes do estabelecimento (F2):
// reviews proprias (product_id NULL) + reviews de produto marcadas para a vitrine.
func handleListEstablishmentReviews(svc *Service) http.HandlerFunc {
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
		items, err := svc.ListEstablishmentReviews(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"reviews": items})
	}
}

// handleCreateEstablishmentReview cria uma avaliacao do estabelecimento (F2,
// product_id NULL). Escopo validado via scopedAccountID; defesa em profundidade no repo.
func handleCreateEstablishmentReview(svc *Service) http.HandlerFunc {
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
		var in ReviewInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.CreateEstablishmentReview(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleUpdateReview(svc *Service) http.HandlerFunc {
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
		var in ReviewInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.UpdateReview(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDeleteReview(svc *Service) http.HandlerFunc {
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
		if err := svc.DeleteReview(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
