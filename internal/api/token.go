package api
import (
	"encoding/json"
	"github.com/owenmoloney/chronos/internal/auth"
	"net/http"
	"time"
	"errors"
	"strings"
)

func (h *Handler) IssueToken(w http.ResponseWriter, r *http.Request){
	if r.Method != "POST"{
		http.Error(w, "method not allowed", 405)
		return
	}

	var req TokenRequest

	err:= json.NewDecoder(r.Body).Decode(&req)

	if err != nil{
		http.Error(w, "invalid JSON body", 400)
		return
	}

	if req.TenantId <= 0{
		http.Error(w, "invalid tenant_id", 400)
		return 
	}

	token, expiresAt, err := auth.IssueToken(h.JWTSecret, req.TenantId, time.Hour)

	if err != nil {
		http.Error(w, "failed to issue token", 500)
		return
	}

	resp := TokenResponse{Token: token, ExpiresAt: expiresAt}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	err = json.NewEncoder(w).Encode(resp)

}
func (h *Handler) tenantIDFromRequest(r *http.Request) (tenantID int64, err error){
	header := r.Header.Get("Authorization")

	if header == ""{
		return 0, errors.New("missing authorization")
	}

	tokenString, ok := strings.CutPrefix(header, "Bearer ")

	if !ok || tokenString == ""{
		return 0, errors.New("Invalid Authorization")
	}

	claims, err := auth.ParseToken(h.JWTSecret, tokenString)

	if err != nil{
		return  0, errors.New("Invalid Token")
	}

	return claims.TenantId, nil

}