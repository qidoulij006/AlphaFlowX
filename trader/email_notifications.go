package trader

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"nofx/logger"
	"nofx/store"
	"os"
	"strconv"
	"strings"
	"time"
)

type tradeEventEmailConfig struct {
	Enabled         bool
	DisplayTimezone string
	SMTPHost        string
	SMTPPort        int
	SMTPUsername    string
	SMTPPassword    string
	EmailFrom       string
	EmailTo         string
}

type tradeEventSnapshot struct {
	TraderName    string
	UserEmail     string
	TotalEquity   float64
	Balance       float64
	UnrealizedPnL float64
	PositionCount int
	MarginUsedPct float64
	OpenPositions []*store.TraderPosition
}

func sendTradeEventEmailAsync(st *store.Store, userID, traderID, traderName, eventType, symbol, side string, quantity, price float64, leverage int, fee float64, orderID string) {
	cfg := loadTradeEventEmailConfig()
	if !cfg.Enabled || st == nil {
		return
	}

	go func() {
		if err := sendTradeEventEmail(st, cfg, userID, traderID, traderName, eventType, symbol, side, quantity, price, leverage, fee, orderID); err != nil {
			logger.Infof("  ⚠️ Trade event email failed: %v", err)
		}
	}()
}

func sendTradeEventEmail(st *store.Store, cfg tradeEventEmailConfig, userID, traderID, traderName, eventType, symbol, side string, quantity, price float64, leverage int, fee float64, orderID string) error {
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		return fmt.Errorf("smtp host is empty")
	}
	if strings.TrimSpace(cfg.EmailFrom) == "" {
		return fmt.Errorf("from address is empty")
	}

	to := strings.TrimSpace(cfg.EmailTo)
	if to == "" {
		user, err := st.User().GetByID(userID)
		if err != nil {
			return err
		}
		to = strings.TrimSpace(user.Email)
	}
	if to == "" {
		return fmt.Errorf("recipient email is empty")
	}

	snapshot, err := buildTradeEventSnapshot(st, userID, traderID, traderName)
	if err != nil {
		return err
	}

	loc, err := time.LoadLocation(blankFallback(cfg.DisplayTimezone, "Asia/Shanghai"))
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}

	now := time.Now().UTC()
	subject := fmt.Sprintf("[NOFX] %s %s %s %s", snapshot.TraderName, tradeEventActionLabel(eventType), symbol, side)
	body := buildTradeEventEmailBody(loc, snapshot, eventType, symbol, side, quantity, price, leverage, fee, orderID, now)
	if err := sendSMTPMail(cfg, to, subject, body); err != nil {
		return err
	}
	logger.Infof("  📧 Trade event email sent to %s (%s)", to, subject)
	return nil
}

func buildTradeEventSnapshot(st *store.Store, userID, traderID, traderName string) (*tradeEventSnapshot, error) {
	snapshot := &tradeEventSnapshot{TraderName: traderName}
	user, err := st.User().GetByID(userID)
	if err == nil {
		snapshot.UserEmail = strings.TrimSpace(user.Email)
	}

	if traderRow, err := st.Trader().GetByID(traderID); err == nil && strings.TrimSpace(traderRow.Name) != "" {
		snapshot.TraderName = traderRow.Name
	}

	equity, err := st.Equity().GetLatest(traderID, 1)
	if err == nil && len(equity) > 0 {
		snapshot.TotalEquity = equity[0].TotalEquity
		snapshot.Balance = equity[0].Balance
		snapshot.UnrealizedPnL = equity[0].UnrealizedPnL
		snapshot.PositionCount = equity[0].PositionCount
		snapshot.MarginUsedPct = equity[0].MarginUsedPct
	}

	openPositions, err := st.Position().GetOpenPositions(traderID)
	if err != nil {
		return nil, err
	}
	snapshot.OpenPositions = openPositions
	if snapshot.PositionCount == 0 {
		snapshot.PositionCount = len(openPositions)
	}
	return snapshot, nil
}

func buildTradeEventEmailBody(loc *time.Location, snapshot *tradeEventSnapshot, eventType, symbol, side string, quantity, price float64, leverage int, fee float64, orderID string, now time.Time) string {
	lines := []string{
		"[NOFX Trade Event Report]",
		"",
		"事件摘要",
		"----------------------------------------",
		fmt.Sprintf("交易员: %s", snapshot.TraderName),
		fmt.Sprintf("事件: %s", tradeEventActionLabel(eventType)),
		fmt.Sprintf("币种: %s", symbol),
		fmt.Sprintf("方向: %s", side),
		fmt.Sprintf("数量: %.6f", quantity),
		fmt.Sprintf("价格: %.6f", price),
		fmt.Sprintf("杠杆: %dx", maxInt(leverage, 1)),
		fmt.Sprintf("手续费: %.6f", fee),
		fmt.Sprintf("订单号: %s", blankFallback(orderID, "-")),
		fmt.Sprintf("时间: %s", formatInLocation(now, loc)),
		"",
		"账户快照",
		"----------------------------------------",
		fmt.Sprintf("总权益: %.4f USDT", snapshot.TotalEquity),
		fmt.Sprintf("余额: %.4f USDT", snapshot.Balance),
		fmt.Sprintf("未实现盈亏: %+0.4f USDT", snapshot.UnrealizedPnL),
		fmt.Sprintf("持仓数: %d", snapshot.PositionCount),
		fmt.Sprintf("保证金使用率: %.2f%%", snapshot.MarginUsedPct*100),
		"",
		"当前持仓",
		"----------------------------------------",
	}

	if len(snapshot.OpenPositions) == 0 {
		lines = append(lines, "- 当前无持仓")
	} else {
		for _, pos := range snapshot.OpenPositions {
			entryQty := pos.EntryQuantity
			if entryQty == 0 {
				entryQty = pos.Quantity
			}
			lines = append(lines,
				fmt.Sprintf("- %s %s", pos.Symbol, pos.Side),
				fmt.Sprintf("  数量: %.6f / 开仓总量: %.6f", pos.Quantity, entryQty),
				fmt.Sprintf("  开仓价: %.6f", pos.EntryPrice),
				fmt.Sprintf("  杠杆: %dx", maxInt(pos.Leverage, 1)),
				fmt.Sprintf("  开仓时间: %s", formatUnixMilliInLocation(pos.EntryTime, loc)),
				fmt.Sprintf("  持仓时长: %s", formatDurationMinutes(minutesSince(pos.EntryTime, true))),
			)
		}
	}

	return strings.Join(lines, "\n")
}

func tradeEventActionLabel(eventType string) string {
	switch eventType {
	case "open_long":
		return "开多成交"
	case "open_short":
		return "开空成交"
	case "close_long":
		return "平多成交"
	case "close_short":
		return "平空成交"
	default:
		return eventType
	}
}

func loadTradeEventEmailConfig() tradeEventEmailConfig {
	return tradeEventEmailConfig{
		Enabled:         envBool("TRADE_EVENT_EMAIL_ENABLED") || envBool("PROMPT_OPT_EMAIL_ENABLED"),
		DisplayTimezone: envOrDefault("TRADE_EVENT_DISPLAY_TIMEZONE", envOrDefault("PROMPT_OPT_DISPLAY_TIMEZONE", "Asia/Shanghai")),
		SMTPHost:        strings.TrimSpace(envOrDefault("TRADE_EVENT_SMTP_HOST", envOrDefault("PROMPT_OPT_SMTP_HOST", ""))),
		SMTPPort:        envInt("TRADE_EVENT_SMTP_PORT", envInt("PROMPT_OPT_SMTP_PORT", 587)),
		SMTPUsername:    strings.TrimSpace(envOrDefault("TRADE_EVENT_SMTP_USERNAME", envOrDefault("PROMPT_OPT_SMTP_USERNAME", ""))),
		SMTPPassword:    envOrDefault("TRADE_EVENT_SMTP_PASSWORD", envOrDefault("PROMPT_OPT_SMTP_PASSWORD", "")),
		EmailFrom:       strings.TrimSpace(envOrDefault("TRADE_EVENT_EMAIL_FROM", envOrDefault("PROMPT_OPT_EMAIL_FROM", ""))),
		EmailTo:         strings.TrimSpace(envOrDefault("TRADE_EVENT_EMAIL_TO", envOrDefault("PROMPT_OPT_EMAIL_TO", ""))),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func sendSMTPMail(cfg tradeEventEmailConfig, to, subject, body string) error {
	addr := net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(cfg.SMTPPort))
	client, err := smtpDial(cfg.SMTPHost, addr, cfg.SMTPPort)
	if err != nil {
		return err
	}
	defer client.Quit()

	if cfg.SMTPUsername != "" {
		auth := smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(cfg.EmailFrom); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	defer wc.Close()

	headers := map[string]string{
		"From":         cfg.EmailFrom,
		"To":           to,
		"Subject":      subject,
		"Date":         time.Now().UTC().Format(time.RFC1123Z),
		"Message-ID":   makeMessageID(cfg.EmailFrom, cfg.SMTPHost),
		"MIME-Version": "1.0",
		"Content-Type": `text/plain; charset="UTF-8"`,
	}

	var b strings.Builder
	for key, value := range headers {
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(value)
		b.WriteString("\r\n")
	}
	b.WriteString("\r\n")
	b.WriteString(body)

	if _, err := wc.Write([]byte(b.String())); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	return nil
}

func smtpDial(host, addr string, port int) (*smtp.Client, error) {
	if port == 465 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
		if err != nil {
			return nil, fmt.Errorf("smtp tls dial: %w", err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return nil, fmt.Errorf("smtp new client: %w", err)
		}
		return client, nil
	}

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("smtp dial: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return nil, fmt.Errorf("smtp new client: %w", err)
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return nil, fmt.Errorf("smtp starttls: %w", err)
		}
	}
	return client, nil
}

func makeMessageID(from, smtpHost string) string {
	domain := smtpHost
	if parts := strings.Split(strings.TrimSpace(from), "@"); len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		domain = strings.TrimSpace(parts[1])
	}
	if domain == "" {
		domain = "localhost"
	}
	return fmt.Sprintf("<%s@%s>", randomHex(12), textproto.TrimString(domain))
}

func randomHex(n int) string {
	if n <= 0 {
		n = 8
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func formatInLocation(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return "-"
	}
	return t.In(loc).Format("2006-01-02 15:04:05 MST")
}

func formatUnixMilliInLocation(ms int64, loc *time.Location) string {
	if ms <= 0 {
		return "-"
	}
	return time.UnixMilli(ms).In(loc).Format("2006-01-02 15:04:05 MST")
}

func minutesSince(ms int64, roundUp bool) int64 {
	if ms <= 0 {
		return 0
	}
	diffMs := time.Now().UTC().UnixMilli() - ms
	if diffMs <= 0 {
		return 0
	}
	minutes := diffMs / 60000
	if roundUp && diffMs%60000 != 0 {
		minutes++
	}
	return minutes
}

func formatDurationMinutes(minutes int64) string {
	if minutes <= 0 {
		return "0m"
	}
	hours := minutes / 60
	remainingMinutes := minutes % 60
	if hours == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	if remainingMinutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, remainingMinutes)
}

func formatDurationMs(ms int64) string {
	if ms <= 0 {
		return "0m"
	}
	seconds := ms / 1000
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	if hours < 24 {
		if minutes%60 == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes%60)
	}
	if hours%24 == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd%dh", days, hours%24)
}

func blankFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
