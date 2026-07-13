package notify

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
	"time"

	"nofx/logger"
)

// EmailConfig holds QQ email SMTP configuration
type EmailConfig struct {
	SMTPHost     string // SMTP server (e.g., "smtp.qq.com")
	SMTPPort     int    // SMTP port (e.g., 465 or 587)
	Username     string // QQ email address
	Password     string // QQ email授权码 (not the login password)
	FromName     string // Display name for sender
	ToEmail      string // Recipient email (can be same as Username)
	Enabled      bool   // Whether email notifications are enabled

	// Notification triggers
	NotifyOnOpen       bool
	NotifyOnClose      bool
	NotifyOnStopLoss   bool
	NotifyOnTakeProfit bool
	NotifyOnError      bool
}

// EmailNotification represents a single notification
type EmailNotification struct {
	Subject      string
	Body         string
	BodyHTML     string // Optional HTML body
	TraderName   string
	Exchange     string
	TemplateType string // "open_position", "close_position", "stop_loss", "take_profit", "error"
	Data         map[string]interface{}
}

// EmailNotifier handles sending email notifications
type EmailNotifier struct {
	config *EmailConfig
}

// NewEmailNotifier creates a new email notifier
func NewEmailNotifier(config *EmailConfig) *EmailNotifier {
	if config.SMTPPort == 0 {
		config.SMTPPort = 465 // Default to SSL port
	}
	if config.FromName == "" {
		config.FromName = "NOFX Trading"
	}
	return &EmailNotifier{config: config}
}

// IsEnabled returns whether email notifications are enabled
func (n *EmailNotifier) IsEnabled() bool {
	return n.config != nil && n.config.Enabled && n.config.Username != "" && n.config.Password != ""
}

// SendOpenPositionNotification sends notification for opening a position
func (n *EmailNotifier) SendOpenPositionNotification(traderName, exchange, symbol, side string, quantity, price float64, leverage int, confidence int) error {
	if !n.IsEnabled() || !n.config.NotifyOnOpen {
		return nil
	}

	sideEmoji := "📈"
	if side == "short" {
		sideEmoji = "📉"
	}

	subject := fmt.Sprintf("【NOFX】%s - 开仓通知 %s %s", traderName, sideEmoji, symbol)
	body := fmt.Sprintf(`【NOFX 交易通知】

📊 策略: %s
📋 交易所: %s
🔔 类型: 开仓

%s 多头入场
   币种: %s
   数量: %.6f
   价格: $%.4f
   杠杆: %dx
   信心度: %d%%

🕐 时间: %s

---
NOFX AI Trading System
`,
		traderName, exchange, sideEmoji, symbol, quantity, price, leverage, confidence, time.Now().Format("2006-01-02 15:04:05"))

	return n.sendEmail(subject, body)
}

// SendClosePositionNotification sends notification for closing a position
func (n *EmailNotifier) SendClosePositionNotification(traderName, exchange, symbol, side string, quantity, openPrice, closePrice float64, pnl float64) error {
	if !n.IsEnabled() || !n.config.NotifyOnClose {
		return nil
	}

	sideEmoji := "📈"
	if side == "short" {
		sideEmoji = "📉"
	}

	pnlStr := fmt.Sprintf("+%.2f", pnl)
	if pnl < 0 {
		pnlStr = fmt.Sprintf("%.2f", pnl)
	}

	subject := fmt.Sprintf("【NOFX】%s - 平仓通知 %s %s", traderName, sideEmoji, symbol)
	body := fmt.Sprintf(`【NOFX 交易通知】

📊 策略: %s
📋 交易所: %s
🔔 类型: 平仓

%s %s
   币种: %s
   数量: %.6f
   开仓价: $%.4f
   平仓价: $%.4f
   盈亏: %s USDT

🕐 时间: %s

---
NOFX AI Trading System
`,
		traderName, exchange, sideEmoji, side, symbol, quantity, openPrice, closePrice, pnlStr, time.Now().Format("2006-01-02 15:04:05"))

	return n.sendEmail(subject, body)
}

// SendStopLossNotification sends notification when stop-loss is triggered
func (n *EmailNotifier) SendStopLossNotification(traderName, exchange, symbol string, quantity, entryPrice, stopPrice, currentPrice float64) error {
	if !n.IsEnabled() || !n.config.NotifyOnStopLoss {
		return nil
	}

	subject := fmt.Sprintf("【NOFX】%s - 止损触发 ⚠️ %s", traderName, symbol)
	body := fmt.Sprintf(`【NOFX 交易通知】

📊 策略: %s
📋 交易所: %s
🔔 类型: 止损触发

⚠️ 止损触发
   币种: %s
   数量: %.6f
   入场价: $%.4f
   止损价: $%.4f
   当前价: $%.4f

🕐 时间: %s

---
NOFX AI Trading System
`,
		traderName, exchange, symbol, quantity, entryPrice, stopPrice, currentPrice, time.Now().Format("2006-01-02 15:04:05"))

	return n.sendEmail(subject, body)
}

// SendTakeProfitNotification sends notification when take-profit is triggered
func (n *EmailNotifier) SendTakeProfitNotification(traderName, exchange, symbol string, quantity, entryPrice, tpPrice, currentPrice float64, pnl float64) error {
	if !n.IsEnabled() || !n.config.NotifyOnTakeProfit {
		return nil
	}

	pnlStr := fmt.Sprintf("+%.2f", pnl)
	if pnl < 0 {
		pnlStr = fmt.Sprintf("%.2f", pnl)
	}

	subject := fmt.Sprintf("【NOFX】%s - 止盈触发 🎯 %s", traderName, symbol)
	body := fmt.Sprintf(`【NOFX 交易通知】

📊 策略: %s
📋 交易所: %s
🔔 类型: 止盈触发

🎯 止盈触发
   币种: %s
   数量: %.6f
   入场价: $%.4f
   止盈价: $%.4f
   当前价: $%.4f
   盈亏: %s USDT

🕐 时间: %s

---
NOFX AI Trading System
`,
		traderName, exchange, symbol, quantity, entryPrice, tpPrice, currentPrice, pnlStr, time.Now().Format("2006-01-02 15:04:05"))

	return n.sendEmail(subject, body)
}

// SendErrorNotification sends notification for runtime errors
func (n *EmailNotifier) SendErrorNotification(traderName, exchange, errorMsg string) error {
	if !n.IsEnabled() || !n.config.NotifyOnError {
		return nil
	}

	subject := fmt.Sprintf("【NOFX】%s - 运行错误 ❌", traderName)
	body := fmt.Sprintf(`【NOFX 交易通知】

📊 策略: %s
📋 交易所: %s
🔔 类型: 运行错误

❌ 错误信息:
%s

🕐 时间: %s

---
NOFX AI Trading System
`,
		traderName, exchange, errorMsg, time.Now().Format("2006-01-02 15:04:05"))

	return n.sendEmail(subject, body)
}

// SendDailySummary sends a daily trading summary
func (n *EmailNotifier) SendDailySummary(traderName, exchange string, totalTrades, winningTrades int, totalPnl float64, openPositions int) error {
	if !n.IsEnabled() {
		return nil
	}

	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(winningTrades) / float64(totalTrades) * 100
	}

	pnlStr := fmt.Sprintf("+%.2f", totalPnl)
	if totalPnl < 0 {
		pnlStr = fmt.Sprintf("%.2f", totalPnl)
	}

	subject := fmt.Sprintf("【NOFX】%s - 每日交易摘要 📊 %s", traderName, time.Now().Format("2006-01-02"))
	body := fmt.Sprintf(`【NOFX 每日交易摘要】

📊 策略: %s
📋 交易所: %s
📅 日期: %s

📈 交易统计:
   总交易数: %d
   盈利交易: %d
   亏损交易: %d
   胜率: %.1f%%
   总盈亏: %s USDT
   当前持仓: %d

🕐 生成时间: %s

---
NOFX AI Trading System
`,
		traderName, exchange, time.Now().Format("2006-01-02"),
		totalTrades, winningTrades, totalTrades-winningTrades, winRate, pnlStr, openPositions,
		time.Now().Format("2006-01-02 15:04:05"))

	return n.sendEmail(subject, body)
}

// sendEmail sends an email
func (n *EmailNotifier) sendEmail(subject, body string) error {
	if !n.IsEnabled() {
		return fmt.Errorf("email notifications are disabled")
	}

	// Build email headers
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", n.config.FromName, n.config.Username)
	headers["To"] = n.config.ToEmail
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=UTF-8"

	var message bytes.Buffer
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(body)

	// Send email based on port
	var err error
	if n.config.SMTPPort == 465 {
		err = n.sendWithSSL(subject, message.String())
	} else {
		err = n.sendWithTLS(subject, message.String())
	}

	if err != nil {
		logger.Errorf("Failed to send email notification: %v", err)
	} else {
		logger.Infof("Email notification sent: %s", subject)
	}

	return err
}

// sendWithSSL sends email using SSL (port 465)
func (n *EmailNotifier) sendWithSSL(subject, message string) error {
	auth := smtp.PlainAuth("", n.config.Username, n.config.Password, n.config.SMTPHost)

	err := smtp.SendMail(
		n.config.SMTPHost+":465",
		auth,
		n.config.Username,
		[]string{n.config.ToEmail},
		[]byte(message),
	)

	return err
}

// sendWithTLS sends email using STARTTLS (port 587)
func (n *EmailNotifier) sendWithTLS(subject, message string) error {
	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", n.config.SMTPHost, n.config.SMTPPort)

	// Set up TLS config
	tlsConfig := &tls.Config{
		ServerName: n.config.SMTPHost,
	}

	// Connect and start TLS
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, n.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Authenticate
	auth := smtp.PlainAuth("", n.config.Username, n.config.Password, n.config.SMTPHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP auth failed: %w", err)
	}

	// Set sender and recipient
	if err := client.Mail(n.config.Username); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	if err := client.Rcpt(n.config.ToEmail); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// Send email body
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get email writer: %w", err)
	}
	_, err = writer.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write email: %w", err)
	}
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close email writer: %w", err)
	}

	return client.Quit()
}

// SendTestEmail sends a test email to verify configuration
func (n *EmailNotifier) SendTestEmail() error {
	if !n.IsEnabled() {
		return fmt.Errorf("email notifications are disabled")
	}

	subject := "【NOFX】测试邮件 - Email Notification Test"
	body := fmt.Sprintf(`【NOFX 邮件通知测试】

您好,

这是一封来自 NOFX AI Trading System 的测试邮件。

如果您收到这封邮件，说明您的邮箱通知配置正确无误。

🕐 测试时间: %s

---
NOFX AI Trading System
`, time.Now().Format("2006-01-02 15:04:05"))

	return n.sendEmail(subject, body)
}

// ============================================================================
// Template-based email (for future use)
// ============================================================================

// notificationTemplates holds HTML templates for notifications
var notificationTemplates = map[string]*template.Template{}

// HTMLNotificationData holds data for HTML template rendering
type HTMLNotificationData struct {
	TraderName   string
	Exchange     string
	Symbol       string
	Side         string
	Quantity     float64
	Price        float64
	Leverage     int
	Confidence   int
	PnL          float64
	Timestamp    time.Time
}

// renderHTMLTemplate renders an HTML template with given data
func renderHTMLTemplate(templateName string, data *HTMLNotificationData) (string, error) {
	tmpl, ok := notificationTemplates[templateName]
	if !ok {
		return "", fmt.Errorf("template not found: %s", templateName)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
