package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"nofx/config"
	"nofx/crypto"
	"nofx/egress"
	"nofx/hook"
	"nofx/logger"
	"nofx/store"

	"github.com/joho/godotenv"
)

func main() {
	accountName := "qidou02"
	if len(os.Args) > 1 && os.Args[1] != "" {
		accountName = os.Args[1]
	}

	_ = godotenv.Load()
	logger.Init(nil)
	config.Init()
	cfg := config.Get()

	cryptoService, err := crypto.NewCryptoService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "init crypto failed: %v\n", err)
		os.Exit(1)
	}
	crypto.SetGlobalCryptoService(cryptoService)

	dbType := store.DBTypeSQLite
	if cfg.DBType == "postgres" {
		dbType = store.DBTypePostgres
	}
	st, err := store.NewWithConfig(store.DBConfig{
		Type:     dbType,
		Path:     cfg.DBPath,
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		DBName:   cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "init store failed: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	egress.InitHooks(st)

	var exchangeCfg *store.Exchange
	if err := st.GormDB().Where("account_name = ?", accountName).First(&exchangeCfg).Error; err != nil {
		fmt.Fprintf(os.Stderr, "lookup exchange account %q failed: %v\n", accountName, err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	hookRes := hook.HookExec[hook.SetHttpClientResult](hook.SET_HTTP_CLIENT, exchangeCfg.ID, client)
	if hookRes != nil && hookRes.GetResult() != nil {
		client = hookRes.GetResult()
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.ipify.org?format=text", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create request failed: %v\n", err)
		os.Exit(1)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "proxy ip request failed for %s (%s): %v\n", exchangeCfg.AccountName, exchangeCfg.ExchangeType, err)
		os.Exit(2)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read response failed: %v\n", err)
		os.Exit(1)
	}

	ip := strings.TrimSpace(string(body))
	fmt.Printf("OK account=%s exchange=%s id=%s\n", exchangeCfg.AccountName, exchangeCfg.ExchangeType, exchangeCfg.ID)
	fmt.Printf("Proxy egress IP: %s\n", ip)
}
