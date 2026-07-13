package notify

import (
	"sync"
)

// Manager manages all notification channels
type Manager struct {
	mu          sync.RWMutex
	email       *EmailNotifier
	telegramBot string // Telegram bot token (optional)
}

// NewManager creates a new notification manager
func NewManager() *Manager {
	return &Manager{}
}

// Global notification manager instance
var globalManager *Manager
var managerOnce sync.Once

// GetManager returns the global notification manager (deprecated, use NewManager for per-trader manager)
func GetManager() *Manager {
	managerOnce.Do(func() {
		globalManager = NewManager()
	})
	return globalManager
}

// SetEmailNotifier sets the email notifier configuration
func (m *Manager) SetEmailNotifier(config *EmailConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config != nil && config.Enabled {
		m.email = NewEmailNotifier(config)
	} else {
		m.email = nil
	}
}

// GetEmailNotifier returns the email notifier
func (m *Manager) GetEmailNotifier() *EmailNotifier {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.email
}

// IsEmailEnabled returns whether email notifications are enabled
func (m *Manager) IsEmailEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.email != nil && m.email.IsEnabled()
}

// ============================================================================
// Convenience methods that delegate to email notifier
// ============================================================================

// SendOpenPositionNotification sends notification for opening a position
func (m *Manager) SendOpenPositionNotification(traderName, exchange, symbol, side string, quantity, price float64, leverage int, confidence int) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.email != nil && m.email.IsEnabled() {
		return m.email.SendOpenPositionNotification(traderName, exchange, symbol, side, quantity, price, leverage, confidence)
	}
	return nil
}

// SendClosePositionNotification sends notification for closing a position
func (m *Manager) SendClosePositionNotification(traderName, exchange, symbol, side string, quantity, openPrice, closePrice float64, pnl float64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.email != nil && m.email.IsEnabled() {
		return m.email.SendClosePositionNotification(traderName, exchange, symbol, side, quantity, openPrice, closePrice, pnl)
	}
	return nil
}

// SendStopLossNotification sends notification when stop-loss is triggered
func (m *Manager) SendStopLossNotification(traderName, exchange, symbol string, quantity, entryPrice, stopPrice, currentPrice float64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.email != nil && m.email.IsEnabled() {
		return m.email.SendStopLossNotification(traderName, exchange, symbol, quantity, entryPrice, stopPrice, currentPrice)
	}
	return nil
}

// SendTakeProfitNotification sends notification when take-profit is triggered
func (m *Manager) SendTakeProfitNotification(traderName, exchange, symbol string, quantity, entryPrice, tpPrice, currentPrice float64, pnl float64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.email != nil && m.email.IsEnabled() {
		return m.email.SendTakeProfitNotification(traderName, exchange, symbol, quantity, entryPrice, tpPrice, currentPrice, pnl)
	}
	return nil
}

// SendErrorNotification sends notification for runtime errors
func (m *Manager) SendErrorNotification(traderName, exchange, errorMsg string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.email != nil && m.email.IsEnabled() {
		return m.email.SendErrorNotification(traderName, exchange, errorMsg)
	}
	return nil
}

// SendDailySummary sends a daily trading summary
func (m *Manager) SendDailySummary(traderName, exchange string, totalTrades, winningTrades int, totalPnl float64, openPositions int) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.email != nil && m.email.IsEnabled() {
		return m.email.SendDailySummary(traderName, exchange, totalTrades, winningTrades, totalPnl, openPositions)
	}
	return nil
}

// SendTestEmail sends a test email to verify configuration
func (m *Manager) SendTestEmail() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.email != nil && m.email.IsEnabled() {
		return m.email.SendTestEmail()
	}
	return nil
}
