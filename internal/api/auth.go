package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"insightpilot/internal/store"
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
}

type customClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"sub"`
}

type AuthService struct {
	users         map[string]*userRecord
	revokedTokens map[string]bool
	jwtSecret     []byte
	db            *store.DB
	mu            sync.RWMutex
}

func NewAuthService(db *store.DB) *AuthService {
	var secret []byte

	// Try to load existing JWT secret from database
	if db != nil {
		if stored, err := db.LoadJWTSecret(); err == nil && len(stored) == 32 {
			secret = stored
		}
	}

	// Generate a new secret if none was loaded
	if len(secret) != 32 {
		secret = make([]byte, 32)
		_, _ = rand.Read(secret)
		// Persist the new secret
		if db != nil {
			if err := db.SaveJWTSecret(secret); err != nil {
				log.Printf("[auth] Failed to persist JWT secret: %v", err)
			}
		}
	}

	svc := &AuthService{
		users:         make(map[string]*userRecord),
		revokedTokens: make(map[string]bool),
		jwtSecret:     secret,
		db:            db,
	}

	// Load users from database on startup
	if db != nil {
		if records, err := db.LoadUsers(); err == nil {
			for _, rec := range records {
				svc.users[rec.Email] = &userRecord{
					User: User{
						ID:        rec.ID,
						Email:     rec.Email,
						Name:      rec.Name,
						CreatedAt: rec.CreatedAt,
					},
					PasswordHash: rec.PasswordHash,
				}
			}
			if len(records) > 0 {
				log.Printf("[auth] Restored %d users from database", len(records))
			}
		}
	}

	return svc
}

type contextKey string

const userContextKey contextKey = "auth_user"

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func validateEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func (a *AuthService) createToken(userID string) string {
	claims := customClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        userID + "_" + fmt.Sprint(time.Now().UnixNano()),
		},
		UserID: userID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(a.jwtSecret)
	return tokenString
}

func (a *AuthService) validateToken(tokenString string) (string, error) {
	if a.isRevoked(tokenString) {
		return "", fmt.Errorf("token has been revoked")
	}
	token, err := jwt.ParseWithClaims(tokenString, &customClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.jwtSecret, nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := token.Claims.(*customClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	return claims.UserID, nil
}

func (a *AuthService) RevokeToken(tokenString string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revokedTokens[tokenString] = true
}

func (a *AuthService) isRevoked(tokenString string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.revokedTokens[tokenString]
}

func (a *AuthService) Register(email, password, name string) (*User, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !validateEmail(email) {
		return nil, "", fmt.Errorf("invalid email format")
	}
	if _, exists := a.users[email]; exists {
		return nil, "", fmt.Errorf("email already registered")
	}
	if len(password) < 8 {
		return nil, "", fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := hashPassword(password)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password")
	}

	id := newID()
	now := time.Now()
	rec := &userRecord{
		User: User{
			ID:        id,
			Email:     email,
			Name:      name,
			CreatedAt: now,
		},
		PasswordHash: hash,
	}
	a.users[email] = rec

	// Persist to database
	if a.db != nil {
		if err := a.db.SaveUser(id, email, name, hash, now); err != nil {
			log.Printf("[auth] Failed to persist user %s: %v", email, err)
		}
	}

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
	if err := bcrypt.CompareHashAndPassword([]byte(rec.PasswordHash), []byte(password)); err != nil {
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
		token := extractTokenFromRequest(r)
		if token == "" {
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

// setAuthCookie sets an HttpOnly cookie with the JWT token (72h expiry).
func setAuthCookie(w http.ResponseWriter, token string) {
	if token == "" {
		return
	}
	secure := isProduction()
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
		MaxAge:   72 * 60 * 60,
	})
}

// setSessionCookie sets a session-scoped HttpOnly cookie (deleted on browser close).
func setSessionCookie(w http.ResponseWriter, token string) {
	if token == "" {
		return
	}
	secure := isProduction()
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
		MaxAge:   0,
	})
}

// clearAuthCookie clears the auth cookie (used on logout).
func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// extractTokenFromRequest extracts a JWT token from the Authorization header
// or from an HttpOnly cookie as a fallback.
func extractTokenFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")
	if token != auth && token != "" {
		return token
	}
	if cookie, err := r.Cookie("auth_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
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
		status := http.StatusConflict
		msg := err.Error()
		if strings.Contains(msg, "invalid email") || strings.Contains(msg, "password must be") {
			status = http.StatusBadRequest
		}
		h.sendJSON(w, status, map[string]string{"error": msg})
		return
	}
	setAuthCookie(w, token)
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
	if !decodeJSON(w, r, &body) {
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
	setAuthCookie(w, token)
	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

func (h *Handler) handleGuestLogin(w http.ResponseWriter, r *http.Request) {
	id := "guest_" + newID()
	now := time.Now()
	user := &User{
		ID:        id,
		Email:     id + "@guest.local",
		Name:      "Guest User",
		CreatedAt: now,
	}
	h.auth.mu.Lock()
	h.auth.users[user.Email] = &userRecord{
		User:         *user,
		PasswordHash: "",
	}
	h.auth.mu.Unlock()
	token := h.auth.createToken(user.ID)
	setSessionCookie(w, token)
	h.sendJSON(w, http.StatusCreated, map[string]interface{}{
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
	token := extractTokenFromRequest(r)
	if token != "" {
		h.auth.RevokeToken(token)
		user := getUserFromContext(r)
		if user != nil && isGuestUser(user.ID) {
			h.auth.mu.Lock()
			delete(h.auth.users, user.Email)
			h.auth.mu.Unlock()
			h.mu.Lock()
			for id, ds := range h.datasets {
				if ds.OwnerID == user.ID {
					delete(h.datasets, id)
				}
			}
			h.mu.Unlock()
		}
	}
	clearAuthCookie(w)
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

func isGuestUser(userID string) bool {
	return strings.HasPrefix(userID, "guest_")
}

// cleanupGuests removes users and datasets associated with expired guest sessions.
func (h *Handler) startGuestCleanup() {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.cleanupExpiredGuests()
			case <-h.stopCh:
				return
			}
		}
	}()
}

func (h *Handler) cleanupExpiredGuests() {
	h.auth.mu.Lock()
	defer h.auth.mu.Unlock()

	now := time.Now()
	for email, rec := range h.auth.users {
		if !isGuestUser(rec.ID) {
			continue
		}
		// Guest tokens expire after 24 hours
		if now.Sub(rec.CreatedAt) > 24*time.Hour {
			delete(h.auth.users, email)
			h.mu.Lock()
			for id, ds := range h.datasets {
				if ds.OwnerID == rec.ID {
					delete(h.datasets, id)
				}
			}
			h.mu.Unlock()
		}
	}
}
