package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type userRecord struct {
	User
	PasswordHash string
	Salt         string
}

type AuthService struct {
	users     map[string]*userRecord
	tokens    map[string]string
	jwtSecret []byte
	mu        sync.RWMutex
}

func NewAuthService() *AuthService {
	secret := make([]byte, 32)
	rand.Read(secret)
	return &AuthService{
		users:     make(map[string]*userRecord),
		tokens:    make(map[string]string),
		jwtSecret: secret,
	}
}

type contextKey string

const userContextKey contextKey = "auth_user"

func hashPassword(password, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

func generateSalt() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *AuthService) createToken(userID string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	now := time.Now().Unix()
	exp := time.Now().Add(72 * time.Hour).Unix()
	payload := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf(`{"sub":"%s","iat":%d,"exp":%d}`, userID, now, exp),
	))
	mac := hmac.New(sha256.New, a.jwtSecret)
	mac.Write([]byte(header + "." + payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return header + "." + payload + "." + sig
}

func (a *AuthService) validateToken(token string) (string, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}
	mac := hmac.New(sha256.New, a.jwtSecret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expected)) {
		return "", fmt.Errorf("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid token payload: %w", err)
	}
	var claims struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("invalid token claims: %w", err)
	}
	if time.Now().Unix() > claims.Exp {
		return "", fmt.Errorf("token expired")
	}
	return claims.Sub, nil
}

func (a *AuthService) Register(email, password, name string) (*User, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.users[email]; exists {
		return nil, "", fmt.Errorf("email already registered")
	}
	if len(password) < 4 {
		return nil, "", fmt.Errorf("password must be at least 4 characters")
	}

	id := newID()
	salt := generateSalt()
	rec := &userRecord{
		User: User{
			ID:        id,
			Email:     email,
			Name:      name,
			CreatedAt: time.Now(),
		},
		PasswordHash: hashPassword(password, salt),
		Salt:         salt,
	}
	a.users[email] = rec
	token := a.createToken(id)
	return &rec.User, token, nil
}

func (a *AuthService) Login(email, password string) (*User, string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	rec, ok := a.users[email]
	if !ok {
		return nil, "", fmt.Errorf("invalid email or password")
	}
	if rec.PasswordHash != hashPassword(password, rec.Salt) {
		return nil, "", fmt.Errorf("invalid email or password")
	}
	token := a.createToken(rec.ID)
	return &rec.User, token, nil
}

func (a *AuthService) ValidateToken(token string) (*User, error) {
	if token == "" {
		return nil, fmt.Errorf("no token provided")
	}
	userID, err := a.validateToken(token)
	if err != nil {
		return nil, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, rec := range a.users {
		if rec.ID == userID {
			u := rec.User
			return &u, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth {
			next.ServeHTTP(w, r)
			return
		}
		user, err := h.auth.LoginWithToken(token)
		if err == nil && user != nil {
			ctx := context.WithValue(r.Context(), userContextKey, user)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getUserFromContext(r)
		if user == nil {
			h.sendJSON(w, http.StatusUnauthorized, map[string]string{"error": "Authentication required"})
			return
		}
		next(w, r)
	}
}

func getUserFromContext(r *http.Request) *User {
	user, _ := r.Context().Value(userContextKey).(*User)
	return user
}

func (h *Handler) currentUserID(r *http.Request) string {
	if user := getUserFromContext(r); user != nil {
		return user.ID
	}
	return ""
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.Email == "" || body.Password == "" {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Email and password are required"})
		return
	}
	if body.Name == "" {
		body.Name = strings.Split(body.Email, "@")[0]
	}

	user, token, err := h.auth.Register(body.Email, body.Password, body.Name)
	if err != nil {
		h.sendJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	h.sendJSON(w, http.StatusCreated, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.Email == "" || body.Password == "" {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Email and password are required"})
		return
	}

	user, token, err := h.auth.Login(body.Email, body.Password)
	if err != nil {
		h.sendJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r)
	if user == nil {
		h.sendJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not authenticated"})
		return
	}
	h.sendJSON(w, http.StatusOK, user)
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (a *AuthService) LoginWithToken(token string) (*User, error) {
	return a.ValidateToken(token)
}

func (a *AuthService) HasUsers() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.users) > 0
}
