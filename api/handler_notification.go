package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/notify"
	"nofx/store"

	"github.com/gin-gonic/gin"
)

// NotificationConfigResponse is the safe response structure for notification config
type NotificationConfigResponse struct {
	EmailEnabled bool   `json:"email_enabled"`
	EmailHost    string `json:"email_host"`
	EmailPort    int    `json:"email_port"`
	EmailUser    string `json:"email_user"`
	HasEmailPass bool   `json:"has_email_pass"`
	EmailFromName string `json:"email_from_name"`
	EmailTo      string `json:"email_to"`

	NotifyOnOpen       bool `json:"notify_on_open"`
	NotifyOnClose      bool `json:"notify_on_close"`
	NotifyOnStopLoss   bool `json:"notify_on_stop_loss"`
	NotifyOnTakeProfit bool `json:"notify_on_take_profit"`
	NotifyOnError      bool `json:"notify_on_error"`
	NotifyDailySummary bool `json:"notify_daily_summary"`
}

// UpdateNotificationConfigRequest is the request to update notification config
type UpdateNotificationConfigRequest struct {
	EmailEnabled   bool   `json:"email_enabled"`
	EmailHost      string `json:"email_host"`
	EmailPort      int    `json:"email_port"`
	EmailUser      string `json:"email_user"`
	EmailPass      string `json:"email_pass"`
	EmailFromName  string `json:"email_from_name"`
	EmailTo        string `json:"email_to"`

	NotifyOnOpen       bool `json:"notify_on_open"`
	NotifyOnClose      bool `json:"notify_on_close"`
	NotifyOnStopLoss   bool `json:"notify_on_stop_loss"`
	NotifyOnTakeProfit bool `json:"notify_on_take_profit"`
	NotifyOnError      bool `json:"notify_on_error"`
	NotifyDailySummary bool `json:"notify_daily_summary"`
}

// TestEmailRequest is the request to send a test email
type TestEmailRequest struct {
	ToEmail string `json:"to_email"`
}

func safeNotificationConfigFromStore(config *store.NotificationConfig) NotificationConfigResponse {
	return NotificationConfigResponse{
		EmailEnabled:     config.EmailEnabled,
		EmailHost:        config.EmailHost,
		EmailPort:        config.EmailPort,
		EmailUser:        config.EmailUser,
		HasEmailPass:     config.EmailPass != "",
		EmailFromName:    config.EmailFromName,
		EmailTo:          config.EmailTo,
		NotifyOnOpen:     config.NotifyOnOpen,
		NotifyOnClose:    config.NotifyOnClose,
		NotifyOnStopLoss: config.NotifyOnStopLoss,
		NotifyOnTakeProfit: config.NotifyOnTakeProfit,
		NotifyOnError:    config.NotifyOnError,
		NotifyDailySummary: config.NotifyDailySummary,
	}
}

// handleGetNotificationConfig gets the global notification configuration
func (s *Server) handleGetNotificationConfig(c *gin.Context) {
	userID := c.GetString("user_id")

	config, err := s.store.NotificationConfig().GetGlobal(userID)
	if err != nil {
		SafeInternalError(c, "Failed to get notification config", err)
		return
	}

	c.JSON(http.StatusOK, safeNotificationConfigFromStore(config))
}

// handleUpdateNotificationConfig updates the global notification configuration
func (s *Server) handleUpdateNotificationConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	cfg := config.Get()

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req UpdateNotificationConfigRequest

	// Check if transport encryption is enabled
	if !cfg.TransportEncryption {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			logger.Infof("❌ Failed to parse plain JSON request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		logger.Infof("📝 Received plain text notification config (UserID: %s)", userID)
	} else {
		// Transport encryption enabled, require encrypted payload
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
			logger.Infof("❌ Failed to parse encrypted payload: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format, encrypted transmission required"})
			return
		}

		if encryptedPayload.WrappedKey == "" {
			logger.Infof("❌ Detected unencrypted request (UserID: %s)", userID)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "This endpoint only supports encrypted transmission",
				"code":    "ENCRYPTION_REQUIRED",
				"message": "Encrypted transmission is required for security reasons",
			})
			return
		}

		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			logger.Infof("❌ Failed to decrypt notification config (UserID: %s): %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			logger.Infof("❌ Failed to parse decrypted data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
		logger.Infof("🔓 Decrypted notification config data (UserID: %s)", userID)
	}

	// Get existing config or create new one
	existing, err := s.store.NotificationConfig().GetGlobal(userID)
	if err != nil {
		SafeInternalError(c, "Failed to get existing notification config", err)
		return
	}

	// Preserve existing password if new one is empty
	emailPass := crypto.EncryptedString(req.EmailPass)
	if req.EmailPass == "" && existing.ID != "" {
		emailPass = existing.EmailPass
	}

	// Set default SMTP host
	emailHost := req.EmailHost
	if emailHost == "" {
		emailHost = "smtp.qq.com"
	}

	// Set default SMTP port
	emailPort := req.EmailPort
	if emailPort <= 0 {
		emailPort = 465
	}

	// Set default FromName
	emailFromName := req.EmailFromName
	if emailFromName == "" {
		emailFromName = "NOFX Trading"
	}

	existing.EmailEnabled = req.EmailEnabled
	existing.EmailHost = emailHost
	existing.EmailPort = emailPort
	existing.EmailUser = req.EmailUser
	existing.EmailPass = emailPass
	existing.EmailFromName = emailFromName
	existing.EmailTo = req.EmailTo
	existing.NotifyOnOpen = req.NotifyOnOpen
	existing.NotifyOnClose = req.NotifyOnClose
	existing.NotifyOnStopLoss = req.NotifyOnStopLoss
	existing.NotifyOnTakeProfit = req.NotifyOnTakeProfit
	existing.NotifyOnError = req.NotifyOnError
	existing.NotifyDailySummary = req.NotifyDailySummary

	if err := s.store.NotificationConfig().Save(existing); err != nil {
		SafeInternalError(c, "Failed to save notification config", err)
		return
	}

	// Refresh notification config for all running traders
	s.traderManager.RefreshAllTradersNotificationConfig()

	logger.Infof("✅ Notification config updated for user %s (email: %v)", userID, req.EmailEnabled)

	c.JSON(http.StatusOK, gin.H{"message": "Notification config updated successfully"})
}

// handleTestEmail sends a test email to verify the notification configuration
func (s *Server) handleTestEmail(c *gin.Context) {
	userID := c.GetString("user_id")

	var req TestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request format")
		return
	}

	// Get notification config
	config, err := s.store.NotificationConfig().GetGlobal(userID)
	if err != nil {
		SafeInternalError(c, "Failed to get notification config", err)
		return
	}

	if !config.EmailEnabled || config.EmailUser == "" || config.EmailPass == "" {
		SafeBadRequest(c, "Email notifications are not configured. Please enable and configure email settings first.")
		return
	}

	toEmail := req.ToEmail
	if toEmail == "" {
		toEmail = config.EmailTo
	}
	if toEmail == "" {
		SafeBadRequest(c, "No recipient email specified. Please provide a to_email or configure email_to in settings.")
		return
	}

	// Create a temporary email notifier to send the test
	emailNotifier := notify.NewEmailNotifier(&notify.EmailConfig{
		SMTPHost: config.EmailHost,
		SMTPPort: config.EmailPort,
		Username: config.EmailUser,
		Password: string(config.EmailPass),
		FromName: config.EmailFromName,
		ToEmail:  toEmail,
		Enabled:  true,
	})

	if err := emailNotifier.SendTestEmail(); err != nil {
		errMsg := fmt.Sprintf("Failed to send test email: %v", err)
		logger.Errorf("❌ %s (UserID: %s)", errMsg, userID)
		SafeInternalError(c, errMsg, err)
		return
	}

	logger.Infof("✅ Test email sent successfully (UserID: %s, to: %s)", userID, toEmail)
	c.JSON(http.StatusOK, gin.H{"message": "Test email sent successfully"})
}