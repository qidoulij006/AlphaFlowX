package auth

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// JWTSecret is the JWT secret key, will be dynamically set from config
var JWTSecret []byte

// tokenBlacklist for logged out tokens (memory only, cleaned by expiration time)
var tokenBlacklist = struct {
	sync.RWMutex
	items map[string]time.Time
}{items: make(map[string]time.Time)}

// maxBlacklistEntries is the maximum capacity threshold for blacklist
const maxBlacklistEntries = 100_000

// SetJWTSecret sets the JWT secret key
func SetJWTSecret(secret string) {
	JWTSecret = []byte(secret)
}

// BlacklistToken adds token to blacklist until expiration
func BlacklistToken(token string, exp time.Time) {
	tokenBlacklist.Lock()
	defer tokenBlacklist.Unlock()
	tokenBlacklist.items[token] = exp

	// If exceeds capacity threshold, perform expired cleanup; if still over limit, log warning
	if len(tokenBlacklist.items) > maxBlacklistEntries {
		now := time.Now()
		for t, e := range tokenBlacklist.items {
			if now.After(e) {
				delete(tokenBlacklist.items, t)
			}
		}
		if len(tokenBlacklist.items) > maxBlacklistEntries {
			log.Printf("auth: token blacklist size (%d) exceeds limit (%d) after sweep; consider reducing JWT TTL or using a shared persistent store",
				len(tokenBlacklist.items), maxBlacklistEntries)
		}
	}
}

// IsTokenBlacklisted checks if token is in blacklist (auto cleanup on expiration)
func IsTokenBlacklisted(token string) bool {
	tokenBlacklist.Lock()
	defer tokenBlacklist.Unlock()
	if exp, ok := tokenBlacklist.items[token]; ok {
		if time.Now().After(exp) {
			delete(tokenBlacklist.items, token)
			return false
		}
		return true
	}
	return false
}

// Claims represents JWT claims
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// ShareClaims represents read-only dashboard share token claims.
type ShareClaims struct {
	UserID   string `json:"user_id"`
	TraderID string `json:"trader_id"`
	Scope    string `json:"scope"`
	jwt.RegisteredClaims
}

// HashPassword hashes the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword verifies the password
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateJWT generates JWT token
func GenerateJWT(userID, email string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)), // Expires in 24 hours
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "nofxAI",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// ValidateJWT validates JWT token
func ValidateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return JWTSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// GenerateShareJWT generates a long-lived read-only share token for a single trader dashboard.
func GenerateShareJWT(userID, traderID string) (string, error) {
	now := time.Now()
	claims := ShareClaims{
		UserID:   userID,
		TraderID: traderID,
		Scope:    "dashboard:read",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(365 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "nofxAI-share",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// ValidateShareJWT validates a read-only dashboard share token.
func ValidateShareJWT(tokenString string) (*ShareClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ShareClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return JWTSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*ShareClaims); ok && token.Valid {
		if claims.Scope != "dashboard:read" || claims.TraderID == "" || claims.UserID == "" {
			return nil, fmt.Errorf("invalid share token scope")
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid share token")
}

// IsIssuedBefore checks whether a token was issued before the given time.
func IsIssuedBefore(claims *Claims, t *time.Time) bool {
	if claims == nil || t == nil || claims.IssuedAt == nil {
		return false
	}
	return claims.IssuedAt.Time.Before(t.UTC())
}
