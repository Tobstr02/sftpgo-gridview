package thumbnail

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/drakkan/sftpgo/v2/internal/logger"
)

const (
	thumbBasePath = "/thumb"
	thumbGetPath  = thumbBasePath + "/{key}"
	thumbGenPath  = thumbBasePath + "/generate"
)

type Handlers struct {
	service *ThumbnailService
}

func NewHandlers(svc *ThumbnailService) *Handlers {
	return &Handlers{service: svc}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get(thumbGetPath, h.handleGetThumbnail)
	r.Post(thumbGenPath, h.handleGenerateThumbnail)
	r.Delete(thumbGetPath, h.handleDeleteThumbnail)
}

func (h *Handlers) handleGetThumbnail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := chi.URLParam(r, "key")

	data, contentType, err := h.service.GetCachedThumbnail(ctx, key)
	if err != nil {
		if err.Error() == "thumbnail not found" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("ETag", key)
	w.Write(data)
}

type generateRequest struct {
	URL string `json:"url"`
}

func (h *Handlers) handleGenerateThumbnail(w http.ResponseWriter, r *http.Request) {
	var req generateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.JSON(w, r, map[string]string{"error": "invalid request"})
		return
	}

	ctx := context.Background()
	resp, err := h.service.GetThumbnail(ctx, ThumbRequest{
		Path: req.URL,
	})
	if err != nil {
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, resp)
}

func (h *Handlers) handleDeleteThumbnail(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	ctx := context.Background()

	if err := h.service.DeleteThumbnail(ctx, key); err != nil {
		logger.Warn("thumbnail", "", "Failed to delete thumbnail: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}
