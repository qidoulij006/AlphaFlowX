package main

import (
	"fmt"
	"os"

	"nofx/config"
	"nofx/crypto"
	"nofx/egress"
	"nofx/logger"
	"nofx/store"
	"nofx/trader"
	astertrader "nofx/trader/aster"
	binancetrader "nofx/trader/binance"
	bitgettrader "nofx/trader/bitget"
	bybittrader "nofx/trader/bybit"
	gatetrader "nofx/trader/gate"
	hyperliquidtrader "nofx/trader/hyperliquid"
	indodaxtrader "nofx/trader/indodax"
	kucointrader "nofx/trader/kucoin"
	lightertrader "nofx/trader/lighter"
	okxtrader "nofx/trader/okx"

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

	var tempTrader trader.Trader
	switch exchangeCfg.ExchangeType {
	case "binance":
		binanceTrader := binancetrader.NewFuturesTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), "default", exchangeCfg.ID)
		binanceTrader.BindCooldownStore(st, exchangeCfg.ID)
		tempTrader = binanceTrader
	case "bybit":
		tempTrader = bybittrader.NewBybitTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), exchangeCfg.ID)
	case "okx":
		tempTrader = okxtrader.NewOKXTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), string(exchangeCfg.Passphrase), exchangeCfg.ID)
	case "bitget":
		tempTrader = bitgettrader.NewBitgetTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), string(exchangeCfg.Passphrase), exchangeCfg.ID)
	case "gate":
		tempTrader = gatetrader.NewGateTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), exchangeCfg.ID)
	case "kucoin":
		tempTrader = kucointrader.NewKuCoinTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), string(exchangeCfg.Passphrase), exchangeCfg.ID)
	case "indodax":
		tempTrader = indodaxtrader.NewIndodaxTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), exchangeCfg.ID)
	case "hyperliquid":
		tempTrader, err = hyperliquidtrader.NewHyperliquidTrader(
			string(exchangeCfg.APIKey),
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
			exchangeCfg.HyperliquidUnifiedAcct,
		)
	case "aster":
		tempTrader, err = astertrader.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			string(exchangeCfg.AsterPrivateKey),
			exchangeCfg.ID,
		)
	case "lighter":
		tempTrader, err = lightertrader.NewLighterTraderV2(
			exchangeCfg.LighterWalletAddr,
			string(exchangeCfg.LighterAPIKeyPrivateKey),
			exchangeCfg.LighterAPIKeyIndex,
			exchangeCfg.Testnet,
		)
	default:
		fmt.Fprintf(os.Stderr, "unsupported exchange type %q\n", exchangeCfg.ExchangeType)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "create trader failed: %v\n", err)
		os.Exit(1)
	}

	balance, err := tempTrader.GetBalance()
	if err != nil {
		fmt.Fprintf(os.Stderr, "balance query failed for %s (%s): %v\n", exchangeCfg.AccountName, exchangeCfg.ExchangeType, err)
		os.Exit(2)
	}

	fmt.Printf("OK account=%s exchange=%s id=%s\n", exchangeCfg.AccountName, exchangeCfg.ExchangeType, exchangeCfg.ID)
	fmt.Printf("Balance response: %#v\n", balance)
}
