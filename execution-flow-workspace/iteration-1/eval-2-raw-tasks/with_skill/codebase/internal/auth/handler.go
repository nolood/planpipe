package auth

import (
	"encoding/json"
	"net/http"

	"github.com/example/multi-tenant-app/internal/user"
	"github.com/golang-jwt/jwt/v5"
)

type Handler struct {
	userService *user.Service
	jwtSecret   string
}

func NewHandler(userService *user.Service, jwtSecret string) *Handler {
	return &Handler{
		userService: userService,
		jwtSecret:   jwtSecret,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	u, err := h.userService.Authenticate(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := h.generateToken(u)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResponse{Token: token})
}

func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	// simplified
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) generateToken(u *user.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":   u.ID,
		"email":     u.Email,
		"tenant_id": u.TenantID,
		"role":      u.Role,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
