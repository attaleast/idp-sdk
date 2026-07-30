package health

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthChecker struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *HealthChecker { return &HealthChecker{db: db} }

func (h *HealthChecker) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *HealthChecker) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	if h.db != nil {
		if err := h.db.Ping(r.Context()); err != nil {
			http.Error(w, "DB not ready", http.StatusServiceUnavailable)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
