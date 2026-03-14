package tenant

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	t, err := h.svc.GetByID(r.Context(), tenantID)
	if err != nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (h *Handler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	existing, err := h.svc.GetByID(r.Context(), tenantID)
	if err != nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	var update struct {
		Name        *string `json:"name"`
		EmailDomain *string `json:"email_domain"`
		IsActive    *bool   `json:"is_active"`
		Plan        *string `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if update.Name != nil {
		existing.Name = *update.Name
	}
	if update.EmailDomain != nil {
		existing.EmailDomain = *update.EmailDomain
	}
	if update.IsActive != nil {
		existing.IsActive = *update.IsActive
	}
	if update.Plan != nil {
		existing.Plan = *update.Plan
	}

	if err := h.svc.Update(r.Context(), existing); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}
