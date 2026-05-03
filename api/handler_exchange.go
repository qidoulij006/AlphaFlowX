package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/security"

	"github.com/gin-gonic/gin"
)

type ExchangeConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "cex" or "dex"
	Enabled   bool   `json:"enabled"`
	APIKey    string `json:"apiKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	Testnet   bool   `json:"testnet,omitempty"`
}

// SafeExchangeConfig Safe exchange configuration structure (does not contain sensitive information)
type SafeExchangeConfig struct {
	ID                    string `json:"id"`            // UUID
	ExchangeType          string `json:"exchange_type"` // "binance", "bybit", "okx", "hyperliquid", "aster", "lighter"
	AccountName           string `json:"account_name"`  // User-defined account name
	Name                  string `json:"name"`          // Display name
	Type                  string `json:"type"`          // "cex" or "dex"
	Enabled               bool   `json:"enabled"`
	ProxyConfigured       bool   `json:"proxy_configured"`
	ProxyServerID         string `json:"proxy_server_id"`
	ProxyName             string `json:"proxy_name,omitempty"`
	ProxyExitIP           string `json:"proxy_exit_ip,omitempty"`
	Testnet               bool   `json:"testnet,omitempty"`
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"` // Hyperliquid wallet address (not sensitive)
	AsterUser             string `json:"asterUser"`             // Aster username (not sensitive)
	AsterSigner           string `json:"asterSigner"`           // Aster signer (not sensitive)
	LighterWalletAddr     string `json:"lighterWalletAddr"`     // LIGHTER wallet address (not sensitive)
}

type UpdateExchangeConfigRequest struct {
	Exchanges map[string]struct {
		Enabled                 bool    `json:"enabled"`
		APIKey                  string  `json:"api_key"`
		SecretKey               string  `json:"secret_key"`
		Passphrase              string  `json:"passphrase"` // OKX specific
		ProxyURL                *string `json:"proxy_url"`
		ProxyServerID           *string `json:"proxy_server_id"`
		Testnet                 bool    `json:"testnet"`
		HyperliquidWalletAddr   string  `json:"hyperliquid_wallet_addr"`
		HyperliquidUnifiedAcct  bool    `json:"hyperliquid_unified_account"` // Unified Account mode
		AsterUser               string  `json:"aster_user"`
		AsterSigner             string  `json:"aster_signer"`
		AsterPrivateKey         string  `json:"aster_private_key"`
		LighterWalletAddr       string  `json:"lighter_wallet_addr"`
		LighterPrivateKey       string  `json:"lighter_private_key"`
		LighterAPIKeyPrivateKey string  `json:"lighter_api_key_private_key"`
		LighterAPIKeyIndex      int     `json:"lighter_api_key_index"`
	} `json:"exchanges"`
}

// CreateExchangeRequest request structure for creating a new exchange account
type CreateExchangeRequest struct {
	ExchangeType            string `json:"exchange_type" binding:"required"` // "binance", "bybit", "okx", "hyperliquid", "aster", "lighter"
	AccountName             string `json:"account_name"`                     // User-defined account name
	Enabled                 bool   `json:"enabled"`
	APIKey                  string `json:"api_key"`
	SecretKey               string `json:"secret_key"`
	Passphrase              string `json:"passphrase"`
	ProxyURL                string `json:"proxy_url"`
	ProxyServerID           string `json:"proxy_server_id"`
	Testnet                 bool   `json:"testnet"`
	HyperliquidWalletAddr   string `json:"hyperliquid_wallet_addr"`
	HyperliquidUnifiedAcct  bool   `json:"hyperliquid_unified_account"` // Unified Account mode: Spot as Perp collateral
	AsterUser               string `json:"aster_user"`
	AsterSigner             string `json:"aster_signer"`
	AsterPrivateKey         string `json:"aster_private_key"`
	LighterWalletAddr       string `json:"lighter_wallet_addr"`
	LighterPrivateKey       string `json:"lighter_private_key"`
	LighterAPIKeyPrivateKey string `json:"lighter_api_key_private_key"`
	LighterAPIKeyIndex      int    `json:"lighter_api_key_index"`
}

type TestProxyRequest struct {
	ProxyURL string `json:"proxy_url"`
}

type SafeProxyServerConfig struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Enabled           bool       `json:"enabled"`
	Configured        bool       `json:"configured"`
	LastTestStatus    string     `json:"last_test_status"`
	LastExitIP        string     `json:"last_exit_ip"`
	LastTestAt        *time.Time `json:"last_test_at,omitempty"`
	BoundExchangeID   string     `json:"bound_exchange_id,omitempty"`
	BoundExchangeName string     `json:"bound_exchange_name,omitempty"`
}

type CreateProxyServerRequest struct {
	Name     string `json:"name"`
	ProxyURL string `json:"proxy_url"`
}

type UpdateProxyServerRequest struct {
	Name     string `json:"name"`
	ProxyURL string `json:"proxy_url"`
	Enabled  bool   `json:"enabled"`
}

// handleGetExchangeConfigs Get exchange configurations
func (s *Server) handleGetExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	logger.Infof("🔍 Querying exchange configs for user %s", userID)
	exchanges, err := s.store.Exchange().List(userID)
	if err != nil {
		SafeInternalError(c, "Failed to get exchange configs", err)
		return
	}

	// If no exchanges in database, return empty array (user needs to create accounts)
	if len(exchanges) == 0 {
		logger.Infof("⚠️ No exchanges in database for user %s", userID)
		c.JSON(http.StatusOK, []SafeExchangeConfig{})
		return
	}

	logger.Infof("✅ Found %d exchange configs", len(exchanges))

	// Convert to safe response structure, remove sensitive information
	safeExchanges := make([]SafeExchangeConfig, len(exchanges))
	for i, exchange := range exchanges {
		proxyName := ""
		proxyExitIP := ""
		if exchange.ProxyServerID != "" {
			if proxy, err := s.store.ProxyServer().GetByID(userID, exchange.ProxyServerID); err == nil {
				proxyName = proxy.Name
				proxyExitIP = proxy.LastExitIP
			}
		}
		safeExchanges[i] = SafeExchangeConfig{
			ID:                    exchange.ID,
			ExchangeType:          exchange.ExchangeType,
			AccountName:           exchange.AccountName,
			Name:                  exchange.Name,
			Type:                  exchange.Type,
			Enabled:               exchange.Enabled,
			ProxyConfigured:       exchange.ProxyServerID != "" || string(exchange.ProxyURL) != "",
			ProxyServerID:         exchange.ProxyServerID,
			ProxyName:             proxyName,
			ProxyExitIP:           proxyExitIP,
			Testnet:               exchange.Testnet,
			HyperliquidWalletAddr: exchange.HyperliquidWalletAddr,
			AsterUser:             exchange.AsterUser,
			AsterSigner:           exchange.AsterSigner,
			LighterWalletAddr:     exchange.LighterWalletAddr,
		}
	}

	c.JSON(http.StatusOK, safeExchanges)
}

// handleUpdateExchangeConfigs Update exchange configurations (supports both encrypted and plain text based on config)
func (s *Server) handleUpdateExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	cfg := config.Get()

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req UpdateExchangeConfigRequest

	// Check if transport encryption is enabled
	if !cfg.TransportEncryption {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			logger.Infof("❌ Failed to parse plain JSON request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		logger.Infof("📝 Received plain text exchange config (UserID: %s)", userID)
	} else {
		// Transport encryption enabled, require encrypted payload
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
			logger.Infof("❌ Failed to parse encrypted payload: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format, encrypted transmission required"})
			return
		}

		// Verify encrypted data
		if encryptedPayload.WrappedKey == "" {
			logger.Infof("❌ Detected unencrypted request (UserID: %s)", userID)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "This endpoint only supports encrypted transmission, please use encrypted client",
				"code":    "ENCRYPTION_REQUIRED",
				"message": "Encrypted transmission is required for security reasons",
			})
			return
		}

		// Decrypt data
		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			logger.Infof("❌ Failed to decrypt exchange config (UserID: %s): %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		// Parse decrypted data
		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			logger.Infof("❌ Failed to parse decrypted data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
		logger.Infof("🔓 Decrypted exchange config data (UserID: %s)", userID)
	}

	// Update each exchange's configuration and track traders that need reload
	tradersToReload := make(map[string]bool)
	for exchangeID, exchangeData := range req.Exchanges {
		currentExchange, getErr := s.store.Exchange().GetByID(userID, exchangeID)
		if getErr != nil {
			SafeInternalError(c, fmt.Sprintf("Get exchange %s", exchangeID), getErr)
			return
		}

		// Find traders using this exchange BEFORE updating
		traders, _ := s.store.Trader().ListByExchangeID(userID, exchangeID)
		for _, t := range traders {
			tradersToReload[t.ID] = true
		}

		proxyURL := string(currentExchange.ProxyURL)
		if exchangeData.ProxyURL != nil {
			if err := security.ValidateProxyURL(*exchangeData.ProxyURL); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid proxy_url for exchange %s: %s", exchangeID, err.Error())})
				return
			}
			proxyURL = *exchangeData.ProxyURL
		}
		proxyServerID := currentExchange.ProxyServerID
		if exchangeData.ProxyServerID != nil {
			proxyServerID = strings.TrimSpace(*exchangeData.ProxyServerID)
			if proxyServerID != "" {
				if _, err := s.store.ProxyServer().GetByID(userID, proxyServerID); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid proxy_server_id for exchange %s", exchangeID)})
					return
				}
			}
		}

		err := s.store.Exchange().Update(userID, exchangeID, exchangeData.Enabled, exchangeData.APIKey, exchangeData.SecretKey, exchangeData.Passphrase, exchangeData.Testnet, proxyURL, proxyServerID, exchangeData.HyperliquidWalletAddr, exchangeData.HyperliquidUnifiedAcct, exchangeData.AsterUser, exchangeData.AsterSigner, exchangeData.AsterPrivateKey, exchangeData.LighterWalletAddr, exchangeData.LighterPrivateKey, exchangeData.LighterAPIKeyPrivateKey, exchangeData.LighterAPIKeyIndex)
		if err != nil {
			SafeInternalError(c, fmt.Sprintf("Update exchange %s", exchangeID), err)
			return
		}
	}

	// Remove affected traders from memory BEFORE reloading to pick up new config
	for traderID := range tradersToReload {
		logger.Infof("🔄 Removing trader %s from memory to reload with new exchange config", traderID)
		s.traderManager.RemoveTrader(traderID)
	}

	// Reload all traders for this user to make new config take effect immediately
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
		// Don't return error here since exchange config was successfully updated to database
	}

	logger.Infof("✓ Exchange config updated: %+v", SanitizeExchangeConfigForLog(req.Exchanges))
	c.JSON(http.StatusOK, gin.H{"message": "Exchange configuration updated"})
}

// handleCreateExchange Create a new exchange account
func (s *Server) handleCreateExchange(c *gin.Context) {
	userID := c.GetString("user_id")
	cfg := config.Get()

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req CreateExchangeRequest

	// Check if transport encryption is enabled
	if !cfg.TransportEncryption {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			logger.Infof("❌ Failed to parse plain JSON request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
	} else {
		// Transport encryption enabled, require encrypted payload
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format, encrypted transmission required"})
			return
		}

		if encryptedPayload.WrappedKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "This endpoint only supports encrypted transmission",
				"code":    "ENCRYPTION_REQUIRED",
				"message": "Encrypted transmission is required for security reasons",
			})
			return
		}

		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
	}

	// Validate exchange type
	validTypes := map[string]bool{
		"binance": true, "bybit": true, "okx": true, "bitget": true,
		"hyperliquid": true, "aster": true, "lighter": true, "gate": true, "kucoin": true, "indodax": true,
	}
	if !validTypes[req.ExchangeType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid exchange type: %s", req.ExchangeType)})
		return
	}

	if err := security.ValidateProxyURL(req.ProxyURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid proxy_url: %s", err.Error())})
		return
	}
	if req.ProxyServerID != "" {
		if _, err := s.store.ProxyServer().GetByID(userID, req.ProxyServerID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy_server_id"})
			return
		}
	}

	// Create new exchange account
	id, err := s.store.Exchange().Create(
		userID, req.ExchangeType, req.AccountName, req.Enabled,
		req.APIKey, req.SecretKey, req.Passphrase, req.ProxyURL, req.ProxyServerID, req.Testnet,
		req.HyperliquidWalletAddr, req.HyperliquidUnifiedAcct,
		req.AsterUser, req.AsterSigner, req.AsterPrivateKey,
		req.LighterWalletAddr, req.LighterPrivateKey, req.LighterAPIKeyPrivateKey, req.LighterAPIKeyIndex,
	)
	if err != nil {
		logger.Infof("❌ Failed to create exchange account: %v", err)
		SafeInternalError(c, "Failed to create exchange account", err)
		return
	}

	logger.Infof("✓ Created exchange account: type=%s, name=%s, id=%s", req.ExchangeType, req.AccountName, id)
	c.JSON(http.StatusOK, gin.H{
		"message": "Exchange account created",
		"id":      id,
	})
}

func (s *Server) handleTestProxy(c *gin.Context) {
	var req TestProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	publicIP, message, err := testProxyURL(c.Request.Context(), req.ProxyURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"public_ip": publicIP,
		"message":   message,
	})
}

func testProxyURL(parent context.Context, rawProxyURL string) (string, string, error) {
	rawProxyURL = strings.TrimSpace(rawProxyURL)
	if rawProxyURL == "" {
		return "", "", fmt.Errorf("Proxy URL is required")
	}

	if err := security.ValidateProxyURL(rawProxyURL); err != nil {
		return "", "", fmt.Errorf("Invalid proxy_url: %s", err.Error())
	}

	parsedProxyURL, err := url.Parse(rawProxyURL)
	if err != nil {
		return "", "", fmt.Errorf("Invalid proxy URL format")
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(parsedProxyURL),
	}
	client := &http.Client{
		Timeout:   12 * time.Second,
		Transport: transport,
	}

	requestCtx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()

	testReq, err := http.NewRequestWithContext(requestCtx, http.MethodGet, "https://api.ipify.org?format=text", nil)
	if err != nil {
		return "", "", fmt.Errorf("Failed to build proxy test request")
	}

	resp, err := client.Do(testReq)
	if err != nil {
		return "", "", fmt.Errorf("Proxy test failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("Proxy test failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", "", fmt.Errorf("Failed to read proxy test response")
	}

	publicIP := strings.TrimSpace(string(body))
	if publicIP == "" {
		return "", "", fmt.Errorf("Proxy test returned an empty public IP")
	}
	return publicIP, fmt.Sprintf("Proxy is reachable and returned public IP %s", publicIP), nil
}

func (s *Server) handleGetProxyServers(c *gin.Context) {
	userID := c.GetString("user_id")
	proxies, err := s.store.ProxyServer().List(userID)
	if err != nil {
		SafeInternalError(c, "Failed to get proxy servers", err)
		return
	}

	exchanges, _ := s.store.Exchange().List(userID)
	boundByProxyID := make(map[string]*SafeExchangeConfig)
	for _, exchange := range exchanges {
		if exchange.ProxyServerID == "" {
			continue
		}
		boundByProxyID[exchange.ProxyServerID] = &SafeExchangeConfig{
			ID:          exchange.ID,
			AccountName: exchange.AccountName,
			Name:        exchange.Name,
		}
	}

	response := make([]SafeProxyServerConfig, 0, len(proxies))
	for _, proxy := range proxies {
		item := SafeProxyServerConfig{
			ID:             proxy.ID,
			Name:           proxy.Name,
			Enabled:        proxy.Enabled,
			Configured:     string(proxy.ProxyURL) != "",
			LastTestStatus: proxy.LastTestStatus,
			LastExitIP:     proxy.LastExitIP,
			LastTestAt:     proxy.LastTestAt,
		}
		if bound := boundByProxyID[proxy.ID]; bound != nil {
			item.BoundExchangeID = bound.ID
			item.BoundExchangeName = bound.AccountName
			if item.BoundExchangeName == "" {
				item.BoundExchangeName = bound.Name
			}
		}
		response = append(response, item)
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) handleCreateProxyServer(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateProxyServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.ProxyURL = strings.TrimSpace(req.ProxyURL)
	publicIP, _, err := testProxyURL(c.Request.Context(), req.ProxyURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id, err := s.store.ProxyServer().Create(userID, req.Name, req.ProxyURL, publicIP)
	if err != nil {
		SafeInternalError(c, "Failed to create proxy server", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "public_ip": publicIP, "message": "Proxy server created"})
}

func (s *Server) handleUpdateProxyServer(c *gin.Context) {
	userID := c.GetString("user_id")
	proxyID := c.Param("id")
	var req UpdateProxyServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.ProxyURL = strings.TrimSpace(req.ProxyURL)
	lastExitIP := ""
	if req.ProxyURL != "" {
		publicIP, _, err := testProxyURL(c.Request.Context(), req.ProxyURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		lastExitIP = publicIP
	}
	if err := s.store.ProxyServer().Update(userID, proxyID, req.Name, req.ProxyURL, lastExitIP, req.Enabled); err != nil {
		SafeInternalError(c, "Failed to update proxy server", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Proxy server updated", "public_ip": lastExitIP})
}

func (s *Server) handleTestSavedProxyServer(c *gin.Context) {
	userID := c.GetString("user_id")
	proxyID := c.Param("id")
	proxy, err := s.store.ProxyServer().GetByID(userID, proxyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Proxy server not found"})
		return
	}
	publicIP, message, err := testProxyURL(c.Request.Context(), string(proxy.ProxyURL))
	if err != nil {
		_ = s.store.ProxyServer().UpdateTestResult(userID, proxyID, "failed", "")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_ = s.store.ProxyServer().UpdateTestResult(userID, proxyID, "passed", publicIP)
	c.JSON(http.StatusOK, gin.H{"public_ip": publicIP, "message": message})
}

func (s *Server) handleDeleteProxyServer(c *gin.Context) {
	userID := c.GetString("user_id")
	proxyID := c.Param("id")
	count, err := s.store.Exchange().CountByProxyServerID(userID, proxyID)
	if err != nil {
		SafeInternalError(c, "Failed to check proxy bindings", err)
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete proxy server that is bound to an exchange"})
		return
	}
	if err := s.store.ProxyServer().Delete(userID, proxyID); err != nil {
		SafeInternalError(c, "Failed to delete proxy server", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Proxy server deleted"})
}

// handleDeleteExchange Delete an exchange account
func (s *Server) handleDeleteExchange(c *gin.Context) {
	userID := c.GetString("user_id")
	exchangeID := c.Param("id")

	if exchangeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Exchange ID is required"})
		return
	}

	// Check if any traders are using this exchange
	traders, err := s.store.Trader().List(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check traders"})
		return
	}

	for _, trader := range traders {
		if trader.ExchangeID == exchangeID {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":       "Cannot delete exchange account that is in use by traders",
				"trader_id":   trader.ID,
				"trader_name": trader.Name,
			})
			return
		}
	}

	// Delete exchange account
	err = s.store.Exchange().Delete(userID, exchangeID)
	if err != nil {
		logger.Infof("❌ Failed to delete exchange account: %v", err)
		SafeInternalError(c, "Failed to delete exchange account", err)
		return
	}

	logger.Infof("✓ Deleted exchange account: id=%s", exchangeID)
	c.JSON(http.StatusOK, gin.H{"message": "Exchange account deleted"})
}

// handleGetSupportedExchanges Get list of exchanges supported by the system
func (s *Server) handleGetSupportedExchanges(c *gin.Context) {
	// Return static list of supported exchange types
	// Note: ID is empty for supported exchanges (they are templates, not actual accounts)
	supportedExchanges := []SafeExchangeConfig{
		{ExchangeType: "binance", Name: "Binance Futures", Type: "cex"},
		{ExchangeType: "bybit", Name: "Bybit Futures", Type: "cex"},
		{ExchangeType: "okx", Name: "OKX Futures", Type: "cex"},
		{ExchangeType: "gate", Name: "Gate.io Futures", Type: "cex"},
		{ExchangeType: "kucoin", Name: "KuCoin Futures", Type: "cex"},
		{ExchangeType: "hyperliquid", Name: "Hyperliquid", Type: "dex"},
		{ExchangeType: "aster", Name: "Aster DEX", Type: "dex"},
		{ExchangeType: "lighter", Name: "LIGHTER DEX", Type: "dex"},
		{ExchangeType: "alpaca", Name: "Alpaca (US Stocks)", Type: "stock"},
		{ExchangeType: "forex", Name: "Forex (TwelveData)", Type: "forex"},
		{ExchangeType: "metals", Name: "Metals (TwelveData)", Type: "metals"},
	}

	c.JSON(http.StatusOK, supportedExchanges)
}
