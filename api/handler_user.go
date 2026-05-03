package api

import (
	"fmt"
	"net/http"
	"net/netip"
	"sort"
	"strings"
	"time"

	"nofx/auth"
	"nofx/config"
	"nofx/logger"
	"nofx/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const adminEmail = "admin@localhost"
const registrationEnabledConfigKey = "registration_enabled"

func (s *Server) recordAdminAudit(c *gin.Context, action, targetType, targetID, summary string) {
	if err := s.store.AuditLog().Create(&store.AuditLog{
		ActorID:    c.GetString("user_id"),
		ActorEmail: c.GetString("email"),
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Summary:    summary,
	}); err != nil {
		logger.Warnf("failed to record admin audit log: %v", err)
	}
}

// handleLogout Add current token to blacklist
func (s *Server) handleLogout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization format"})
		return
	}
	tokenString := parts[1]
	claims, err := auth.ValidateJWT(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}
	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	} else {
		exp = time.Now().Add(24 * time.Hour)
	}
	auth.BlacklistToken(tokenString, exp)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

// handleRegister Handle user registration request.
// handleRegister allows registration only when no users exist yet (first-time setup).
// Admin can later re-open registration through the protected registration toggle.
func (s *Server) handleRegister(c *gin.Context) {
	cfg := config.Get()
	userCount, err := s.store.User().Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user count"})
		return
	}

	registrationEnabled := s.isRegistrationEnabled(cfg, userCount)
	if !registrationEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Registration is currently disabled"})
		return
	}

	if userCount == 0 && !cfg.AllowPublicRegistration && !isLoopbackClient(c.ClientIP()) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Remote registration is disabled. Complete first-time setup from localhost or set ALLOW_PUBLIC_REGISTRATION=true temporarily.",
		})
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Check if email already exists
	_, err = s.store.User().GetByEmail(req.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Generate password hash
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password processing failed"})
		return
	}

	// Create user
	userID := uuid.New().String()
	user := &store.User{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: passwordHash,
	}

	err = s.store.User().Create(user)
	if err != nil {
		SafeInternalError(c, "Failed to create user", err)
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Initialize default model and exchange configs for user
	err = s.initUserDefaultConfigs(user.ID)
	if err != nil {
		logger.Infof("Failed to initialize user default configs: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "Registration successful",
	})
}

// handleLogin Handle user login request
func (s *Server) handleLogin(c *gin.Context) {
	clientIP := c.ClientIP()
	if !s.loginLimiter.allow(clientIP) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many login attempts, please try again later"})
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Get user information
	user, err := s.store.User().GetByEmail(req.Email)
	if err != nil {
		s.loginLimiter.registerFailure(clientIP)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password incorrect"})
		return
	}

	// Verify password
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		s.loginLimiter.registerFailure(clientIP)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password incorrect"})
		return
	}
	if user.ArchivedAt != nil {
		s.loginLimiter.registerFailure(clientIP)
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is archived"})
		return
	}
	if !user.IsActive {
		s.loginLimiter.registerFailure(clientIP)
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is disabled"})
		return
	}

	// Issue token directly after password verification.
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	s.loginLimiter.registerSuccess(clientIP)

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "Login successful",
	})
}

// handleAdminLogin handles password-only login for single-user self-hosted admin mode.
func (s *Server) handleAdminLogin(c *gin.Context) {
	clientIP := c.ClientIP()
	if !s.loginLimiter.allow(clientIP) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many login attempts, please try again later"})
		return
	}

	cfg := config.Get()
	if !cfg.AdminMode {
		c.JSON(http.StatusNotFound, gin.H{"error": "Admin mode is not enabled"})
		return
	}
	if strings.TrimSpace(cfg.AdminPassword) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Admin password is not configured"})
		return
	}

	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	if req.Password != cfg.AdminPassword {
		s.loginLimiter.registerFailure(clientIP)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin password incorrect"})
		return
	}

	if err := s.store.User().EnsureAdmin(); err != nil {
		SafeInternalError(c, "Failed to initialize admin user", err)
		return
	}

	token, err := auth.GenerateJWT("admin", adminEmail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	s.loginLimiter.registerSuccess(clientIP)

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": "admin",
		"email":   adminEmail,
		"message": "Admin login successful",
	})
}

// handleChangePassword changes the password for the currently authenticated user.
func (s *Server) handleChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "new_password is required (min 8 chars)")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		SafeInternalError(c, "Password processing failed", err)
		return
	}
	if err := s.store.User().UpdatePassword(userID, hash); err != nil {
		SafeInternalError(c, "Failed to update password", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password updated"})
}

// handleResetPassword Reset password via email and new password
func (s *Server) handleResetPassword(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"error": "Password reset is disabled. Please sign in and use the authenticated password change flow.",
	})
}

// initUserDefaultConfigs Initialize default model and exchange configs for new user
func (s *Server) initUserDefaultConfigs(userID string) error {
	// Commented out auto-creation of default configs, let users add manually
	// This way new users won't have config items automatically after registration
	logger.Infof("User %s registration completed, waiting for manual AI model and exchange configuration", userID)
	return nil
}

func (s *Server) isRegistrationEnabled(cfg *config.Config, userCount int) bool {
	if userCount == 0 {
		return true
	}

	if value, err := s.store.GetSystemConfig(registrationEnabledConfigKey); err == nil {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "true" {
			return true
		}
		if value == "false" {
			return false
		}
	}

	return false
}

func (s *Server) ensureAdmin(c *gin.Context) bool {
	if c.GetString("user_id") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return false
	}
	return true
}

func (s *Server) handleGetRegistrationSettings(c *gin.Context) {
	if !s.ensureAdmin(c) {
		return
	}

	cfg := config.Get()
	userCount, _ := s.store.User().Count()
	value, _ := s.store.GetSystemConfig(registrationEnabledConfigKey)
	enabled := s.isRegistrationEnabled(cfg, userCount)

	c.JSON(http.StatusOK, gin.H{
		"registration_enabled": enabled,
		"source":               strings.TrimSpace(value),
	})
}

func (s *Server) handleUpdateRegistrationSettings(c *gin.Context) {
	if !s.ensureAdmin(c) {
		return
	}

	var req struct {
		RegistrationEnabled bool `json:"registration_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	value := "false"
	if req.RegistrationEnabled {
		value = "true"
	}
	if err := s.store.SetSystemConfig(registrationEnabledConfigKey, value); err != nil {
		SafeInternalError(c, "Failed to update registration setting", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "Registration setting updated",
		"registration_enabled": req.RegistrationEnabled,
	})

	s.recordAdminAudit(
		c,
		"registration.update",
		"system_config",
		registrationEnabledConfigKey,
		fmt.Sprintf("registration_enabled=%t", req.RegistrationEnabled),
	)
}

func (s *Server) handleAdminUpdateUserStatus(c *gin.Context) {
	if !s.ensureAdmin(c) {
		return
	}

	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		SafeBadRequest(c, "User id is required")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	user, err := s.store.User().GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.ID == "admin" || user.Email == adminEmail {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Admin account cannot be disabled"})
		return
	}

	if err := s.store.User().UpdateActive(userID, req.Enabled); err != nil {
		SafeInternalError(c, "Failed to update user status", err)
		return
	}
	if !req.Enabled {
		if err := s.store.User().RevokeSessions(userID); err != nil {
			SafeInternalError(c, "Failed to revoke user sessions", err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User status updated",
		"enabled": req.Enabled,
	})

	s.recordAdminAudit(
		c,
		"user.status.update",
		"user",
		user.ID,
		fmt.Sprintf("%s enabled=%t", user.Email, req.Enabled),
	)
}

func (s *Server) handleAdminRevokeUserSessions(c *gin.Context) {
	if !s.ensureAdmin(c) {
		return
	}

	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		SafeBadRequest(c, "User id is required")
		return
	}

	user, err := s.store.User().GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.ID == "admin" || user.Email == adminEmail {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Admin sessions cannot be revoked from this endpoint"})
		return
	}

	if err := s.store.User().RevokeSessions(userID); err != nil {
		SafeInternalError(c, "Failed to revoke user sessions", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User sessions revoked"})
	s.recordAdminAudit(c, "user.sessions.revoke", "user", user.ID, fmt.Sprintf("sessions revoked for %s", user.Email))
}

func (s *Server) handleAdminArchiveUser(c *gin.Context) {
	if !s.ensureAdmin(c) {
		return
	}

	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		SafeBadRequest(c, "User id is required")
		return
	}

	user, err := s.store.User().GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.ID == "admin" || user.Email == adminEmail {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Admin account cannot be archived"})
		return
	}

	var req struct {
		Archived bool `json:"archived"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	if err := s.store.User().SetArchived(userID, req.Archived); err != nil {
		SafeInternalError(c, "Failed to update archive state", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "User archive state updated",
		"archived": req.Archived,
	})
	s.recordAdminAudit(c, "user.archive.update", "user", user.ID, fmt.Sprintf("%s archived=%t", user.Email, req.Archived))
}

func (s *Server) handleAdminResetUserPassword(c *gin.Context) {
	if !s.ensureAdmin(c) {
		return
	}

	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		SafeBadRequest(c, "User id is required")
		return
	}

	user, err := s.store.User().GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.ID == "admin" || user.Email == adminEmail {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Admin password is managed separately"})
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "new_password is required (min 8 chars)")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		SafeInternalError(c, "Password processing failed", err)
		return
	}
	if err := s.store.User().UpdatePassword(userID, hash); err != nil {
		SafeInternalError(c, "Failed to reset user password", err)
		return
	}
	if err := s.store.User().RevokeSessions(userID); err != nil {
		SafeInternalError(c, "Failed to revoke user sessions", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User password reset"})

	s.recordAdminAudit(
		c,
		"user.password.reset",
		"user",
		user.ID,
		fmt.Sprintf("password reset for %s", user.Email),
	)
}

func (s *Server) handleAdminDeleteUser(c *gin.Context) {
	if !s.ensureAdmin(c) {
		return
	}

	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		SafeBadRequest(c, "User id is required")
		return
	}

	user, err := s.store.User().GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.ID == "admin" || user.Email == adminEmail {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Admin account cannot be deleted"})
		return
	}

	traders, err := s.store.Trader().ListAll()
	if err != nil {
		SafeInternalError(c, "Failed to inspect traders", err)
		return
	}
	models, err := s.store.AIModel().ListAll()
	if err != nil {
		SafeInternalError(c, "Failed to inspect models", err)
		return
	}
	exchanges, err := s.store.Exchange().ListAll()
	if err != nil {
		SafeInternalError(c, "Failed to inspect exchanges", err)
		return
	}

	resourceCount := 0
	for _, trader := range traders {
		if trader.UserID == userID {
			resourceCount++
		}
	}
	for _, model := range models {
		if model.UserID == userID {
			resourceCount++
		}
	}
	for _, exchange := range exchanges {
		if exchange.UserID == userID {
			resourceCount++
		}
	}
	if resourceCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "User still owns traders, models, or exchanges. Archive the account instead."})
		return
	}

	if err := s.store.User().DeleteByID(userID); err != nil {
		SafeInternalError(c, "Failed to delete user", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
	s.recordAdminAudit(c, "user.delete", "user", user.ID, fmt.Sprintf("deleted %s", user.Email))
}

func (s *Server) handleAdminOverview(c *gin.Context) {
	if !s.ensureAdmin(c) {
		return
	}

	cfg := config.Get()
	totalUsers, err := s.store.User().Count()
	if err != nil {
		SafeInternalError(c, "Failed to load user count", err)
		return
	}
	newUsers7d, err := s.store.User().CountCreatedAfter(time.Now().Add(-7 * 24 * time.Hour))
	if err != nil {
		SafeInternalError(c, "Failed to load recent user count", err)
		return
	}
	totalTraders, err := s.store.Trader().CountAll()
	if err != nil {
		SafeInternalError(c, "Failed to load trader count", err)
		return
	}
	runningTraders, err := s.store.Trader().CountRunning()
	if err != nil {
		SafeInternalError(c, "Failed to load running trader count", err)
		return
	}
	totalModels, err := s.store.AIModel().CountAll()
	if err != nil {
		SafeInternalError(c, "Failed to load model count", err)
		return
	}
	enabledModels, err := s.store.AIModel().CountEnabled()
	if err != nil {
		SafeInternalError(c, "Failed to load enabled model count", err)
		return
	}
	totalExchanges, err := s.store.Exchange().CountAll()
	if err != nil {
		SafeInternalError(c, "Failed to load exchange count", err)
		return
	}
	enabledExchanges, err := s.store.Exchange().CountEnabled()
	if err != nil {
		SafeInternalError(c, "Failed to load enabled exchange count", err)
		return
	}

	userCount, _ := s.store.User().Count()
	registrationEnabled := s.isRegistrationEnabled(cfg, userCount)

	c.JSON(http.StatusOK, gin.H{
		"admin_mode":                cfg.AdminMode,
		"admin_password_configured": strings.TrimSpace(cfg.AdminPassword) != "",
		"db_type":                   cfg.DBType,
		"transport_encryption":      cfg.TransportEncryption,
		"nofxos_enabled":            cfg.NofxOSEnabled,
		"cors_origins":              len(cfg.CORSAllowedOrigins),
		"registration_enabled":      registrationEnabled,
		"users_total":               totalUsers,
		"new_users_7d":              newUsers7d,
		"traders_total":             totalTraders,
		"traders_running":           runningTraders,
		"models_total":              totalModels,
		"models_enabled":            enabledModels,
		"exchanges_total":           totalExchanges,
		"exchanges_enabled":         enabledExchanges,
		"server_time":               time.Now().UTC(),
	})
}

func (s *Server) handleAdminUsers(c *gin.Context) {
	if !s.ensureAdmin(c) {
		return
	}

	users, err := s.store.User().GetAll()
	if err != nil {
		SafeInternalError(c, "Failed to load users", err)
		return
	}
	traders, err := s.store.Trader().ListAll()
	if err != nil {
		SafeInternalError(c, "Failed to load traders", err)
		return
	}
	models, err := s.store.AIModel().ListAll()
	if err != nil {
		SafeInternalError(c, "Failed to load models", err)
		return
	}
	exchanges, err := s.store.Exchange().ListAll()
	if err != nil {
		SafeInternalError(c, "Failed to load exchanges", err)
		return
	}

	type adminUserRow struct {
		ID               string     `json:"id"`
		Email            string     `json:"email"`
		IsAdmin          bool       `json:"is_admin"`
		IsActive         bool       `json:"is_active"`
		IsArchived       bool       `json:"is_archived"`
		CreatedAt        time.Time  `json:"created_at"`
		UpdatedAt        time.Time  `json:"updated_at"`
		SessionRevokedAt *time.Time `json:"session_revoked_at,omitempty"`
		TraderCount      int        `json:"trader_count"`
		RunningTraders   int        `json:"running_traders"`
		ModelCount       int        `json:"model_count"`
		EnabledModels    int        `json:"enabled_models"`
		ExchangeCount    int        `json:"exchange_count"`
		EnabledExchanges int        `json:"enabled_exchanges"`
	}

	rows := make([]adminUserRow, 0, len(users))
	for _, user := range users {
		row := adminUserRow{
			ID:               user.ID,
			Email:            user.Email,
			IsAdmin:          user.ID == "admin" || user.Email == adminEmail,
			IsActive:         user.IsActive,
			IsArchived:       user.ArchivedAt != nil,
			CreatedAt:        user.CreatedAt,
			UpdatedAt:        user.UpdatedAt,
			SessionRevokedAt: user.SessionRevokedAt,
		}

		for _, trader := range traders {
			if trader.UserID != user.ID {
				continue
			}
			row.TraderCount++
			if trader.IsRunning {
				row.RunningTraders++
			}
		}
		for _, model := range models {
			if model.UserID != user.ID {
				continue
			}
			row.ModelCount++
			if model.Enabled {
				row.EnabledModels++
			}
		}
		for _, exchange := range exchanges {
			if exchange.UserID != user.ID {
				continue
			}
			row.ExchangeCount++
			if exchange.Enabled {
				row.EnabledExchanges++
			}
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].IsAdmin != rows[j].IsAdmin {
			return rows[i].IsAdmin
		}
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})

	c.JSON(http.StatusOK, gin.H{"users": rows})
}

func (s *Server) handleAdminAuditLogs(c *gin.Context) {
	if !s.ensureAdmin(c) {
		return
	}

	limit := 100
	if q := strings.TrimSpace(c.Query("limit")); q != "" {
		fmt.Sscanf(q, "%d", &limit)
	}
	action := c.Query("action")
	search := c.Query("q")
	logs, err := s.store.AuditLog().ListFiltered(limit, action, search)
	if err != nil {
		SafeInternalError(c, "Failed to load audit logs", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func isLoopbackClient(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	return addr.IsLoopback()
}
