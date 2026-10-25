package router

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func (r *Router) registerHealthRoute(api *mux.Router) {
	api.HandleFunc("/health", r.healthCheck).Methods(http.MethodGet)
}

// HealthResponse is the /api/v1/health body, per ../../../CLAUDE.md section 6.
type HealthResponse struct {
	OK  bool   `json:"ok"`
	Env string `json:"env"`
}

// healthCheck reports that the process is up. It deliberately does not touch
// the database: a liveness probe that fails on one slow query takes the whole
// app out of rotation.
//
//	@Summary  Liveness check
//	@Tags     health
//	@Produce  json
//	@Success  200 {object} router.HealthResponse
//	@Router   /health [get]
func (r *Router) healthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(HealthResponse{OK: true, Env: r.Config.Env}); err != nil {
		log.Printf("router: encode health: %v", err)
	}
}
