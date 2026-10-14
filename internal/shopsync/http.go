package shopsync

import (
	"errors"
	"html"
	"io"
	"net/http"
	"strings"

	"github.com/hon94csl/qingfan/internal/auth"
	"github.com/hon94csl/qingfan/internal/tiktok"
)

type handler struct {
	svc *Service
}

// Mount serves the seller redirect. The account that started the link is stored with the state.
// Adding and syncing shops happens in the signed-in desktop app.
func Mount(mux *http.ServeMux, svc *Service) {
	h := &handler{svc: svc}
	mux.HandleFunc("GET /v1/tiktok/callback", h.callback)
}

func (h *handler) callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if _, err := h.svc.Complete(r.Context(), code, state); err != nil {
		writePage(w, false, publicMessage(err))
		return
	}
	writePage(w, true, "店铺已写入后台数据库，可以回到桌面应用。")
}

func publicMessage(err error) string {
	if statusOf(err) == http.StatusInternalServerError {
		return "服务暂时不可用"
	}
	return err.Error()
}

func statusOf(err error) int {
	var api *tiktok.APIError
	switch {
	case errors.Is(err, auth.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, tiktok.ErrShop):
		return http.StatusNotFound
	case errors.Is(err, tiktok.ErrOwned), errors.Is(err, tiktok.ErrPartial):
		return http.StatusConflict
	case errors.Is(err, tiktok.ErrConfig), errors.Is(err, tiktok.ErrState), errors.Is(err, tiktok.ErrRange), errors.Is(err, tiktok.ErrSeller), errors.Is(err, tiktok.ErrCode), errors.Is(err, tiktok.ErrNoShop):
		return http.StatusBadRequest
	case errors.As(err, &api), strings.HasPrefix(err.Error(), "TikTok Shop"):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func writePage(w http.ResponseWriter, ok bool, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	code := http.StatusOK
	title := "店铺已写入后台数据库"
	if !ok {
		code = http.StatusBadRequest
		title = "店铺没有写入"
	}
	w.WriteHeader(code)
	_, _ = io.WriteString(w, "<!doctype html><meta charset=\"utf-8\"><title>"+html.EscapeString(title)+"</title><p>"+html.EscapeString(message)+"</p>")
}
