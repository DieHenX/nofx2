package store

import (
	"nofx/crypto"
	"time"

	"gorm.io/gorm"
)

// NotificationConfigStore notification settings storage
type NotificationConfigStore struct {
	db *gorm.DB
}

// NewNotificationConfigStore creates a new notification config store
func NewNotificationConfigStore(db *gorm.DB) *NotificationConfigStore {
	return &NotificationConfigStore{db: db}
}

// NotificationConfig notification settings
type NotificationConfig struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"column:user_id;not null;default:default;index" json:"user_id"`
	TraderID  string    `gorm:"column:trader_id;not null;index" json:"trader_id"` // Empty for global settings

	// Email settings
	EmailEnabled  bool                   `gorm:"column:email_enabled;default:false" json:"email_enabled"`
	EmailHost    string                 `gorm:"column:email_host;default:'smtp.qq.com'" json:"email_host"`
	EmailPort    int                    `gorm:"column:email_port;default:465" json:"email_port"`
	EmailUser    string                 `gorm:"column:email_user;default:''" json:"email_user"`
	EmailPass    crypto.EncryptedString `gorm:"column:email_pass;default:''" json:"-"` // Encrypted, never expose
	EmailFromName string                 `gorm:"column:email_from_name;default:'NOFX Trading'" json:"email_from_name"`
	EmailTo      string                 `gorm:"column:email_to;default:''" json:"email_to"`

	// Notification triggers
	NotifyOnOpen         bool `gorm:"column:notify_on_open;default:true" json:"notify_on_open"`
	NotifyOnClose        bool `gorm:"column:notify_on_close;default:true" json:"notify_on_close"`
	NotifyOnStopLoss     bool `gorm:"column:notify_on_stop_loss;default:true" json:"notify_on_stop_loss"`
	NotifyOnTakeProfit   bool `gorm:"column:notify_on_take_profit;default:true" json:"notify_on_take_profit"`
	NotifyOnError        bool `gorm:"column:notify_on_error;default:true" json:"notify_on_error"`
	NotifyDailySummary   bool `gorm:"column:notify_daily_summary;default:false" json:"notify_daily_summary"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName returns the table name
func (NotificationConfig) TableName() string {
	return "notification_configs"
}

func (s *NotificationConfigStore) initTables() error {
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'notification_configs'`).Scan(&tableExists)
		if tableExists > 0 {
			return nil
		}
	}
	return s.db.AutoMigrate(&NotificationConfig{}).Error
}

// GetByTraderID gets notification config for a trader
func (s *NotificationConfigStore) GetByTraderID(userID, traderID string) (*NotificationConfig, error) {
	var config NotificationConfig
	err := s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).First(&config).Error
	if err != nil {
		// Return default config if not found
		if err == gorm.ErrRecordNotFound {
			return &NotificationConfig{
				UserID:    userID,
				TraderID:  traderID,
				EmailHost: "smtp.qq.com",
				EmailPort: 465,
				NotifyOnOpen:       true,
				NotifyOnClose:      true,
				NotifyOnStopLoss:   true,
				NotifyOnTakeProfit: true,
				NotifyOnError:      true,
			}, nil
		}
		return nil, err
	}
	return &config, nil
}

// GetGlobal gets global notification config for a user
func (s *NotificationConfigStore) GetGlobal(userID string) (*NotificationConfig, error) {
	return s.GetByTraderID(userID, "")
}

// Save saves notification config
func (s *NotificationConfigStore) Save(config *NotificationConfig) error {
	if config.ID == "" {
		// Create new
		return s.db.Create(config).Error
	}
	// Update existing
	return s.db.Save(config).Error
}

// Delete deletes notification config
func (s *NotificationConfigStore) Delete(userID, traderID string) error {
	return s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).Delete(&NotificationConfig{}).Error
}
