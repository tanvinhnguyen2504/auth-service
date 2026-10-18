package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-playground/mold/v4"
	"github.com/nguyentanvinh/my-claude/backend/internal/platform/config"
)

type Handler struct {
	Config      *config.Config
	transformer *mold.Transformer
}

func New(config *config.Config) *Handler {
	return &Handler{
		Config: config,
	}
}

func (h *Handler) ReturnWithJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if statusCode == http.StatusNoContent {
		return
	}

	if resp, ok := v.(config.SuccessResponse); ok {
		if resp.Count == nil {
			var defaultCount int64 = 1
			resp.Count = &defaultCount
		}

		v = resp
	}

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(v); err != nil {
		// need logger in handler level
		// h.Logger.WithError(err).Errorf("writing JSON to ResponseWriter failed on %T", v)
	}
	fmt.Println("By pass Return With JSON")
}

func (h *Handler) FromJSON(dest interface{}, r *http.Request) error {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dest); err != nil {
		return err
	}

	if err := h.transformer.Struct(context.Background(), dest); err != nil {
		return err
	}

	// TODO vincent: Later
	// validate
	// return h.validate.Struct(dest)
	return nil
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

func (h *Handler) ExtractUserIDFromContext(ctx context.Context) *int64 {
	if userID, ok := ctx.Value(config.ContextKeyUserID).(int64); ok && userID > 0 {
		return &userID
	}
	return nil
}

func (h *Handler) ExtractAppIDFromContext(ctx context.Context) *string {
	if appID, ok := ctx.Value(config.ContextKeyAppID).(string); ok && appID != "" {
		return &appID
	}
	return nil
}
