package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/nguyentanvinh/auth-be/backend/internal/platform/config"
)

type Handler struct {
	Config *config.Config
}

func (h *Handler) ReturnWithJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if statusCode == http.StatusNoContent {
		return
	}

	if resp, ok := v.(config.SuccessResponse); ok && resp.Count == nil {
		one := int64(1)
		resp.Count = &one
		v = resp
	}

	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status is already on the wire, so logging is all that is left.
		log.Printf("handler: encode %T: %v", v, err)
	}
}

// maxBodyBytes caps a request body. These endpoints carry a handful of short
// strings, so anything larger is a mistake or an attempt to make us allocate.
const maxBodyBytes = 1 << 20

// DecodeJSON reads exactly one JSON object and rejects unknown fields, so a
// mistyped field name fails loudly instead of being ignored.
func (h *Handler) DecodeJSON(w http.ResponseWriter, r *http.Request, dest any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	if err := decoder.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		return errors.New("body must contain a single JSON object")
	}
	return nil
}
