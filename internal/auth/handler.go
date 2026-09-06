package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/go-playground/validator/v10"
)

type Handler struct {
	service  *Service
	validate *validator.Validate
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
	}
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

type loginResponse struct {
	User struct {
		ID     string `json:"id"`
		Email  string `json:"email"`
		Status string `json:"status"`
	} `json:"user"`

	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req loginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
		return
	}

	ip := net.ParseIP(r.RemoteAddr)

	if ip == nil {
		// Remove port if RemoteAddr is "IP:PORT".
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			ip = net.ParseIP(host)
		} else {
			ip = net.ParseIP(r.RemoteAddr)
		}
	}

	result, err := h.service.Login(
		r.Context(),
		LoginInput{
			Email:     req.Email,
			Password:  req.Password,
			UserAgent: r.UserAgent(),
			IPAddress: ip,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid email or password",
			})

		case errors.Is(err, ErrAccountLocked):
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "account temporarily locked",
			})

		case errors.Is(err, ErrAccountSuspended):
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "account suspended",
			})

		case errors.Is(err, ErrAccountDisabled):
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "account disabled",
			})

		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "internal server error",
			})
		}

		return
	}

	response := loginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}

	response.User.ID = result.User.ID.String()
	response.User.Email = result.User.Email
	response.User.Status = string(result.User.Status)

	writeJSON(w, http.StatusOK, response)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	value any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}
