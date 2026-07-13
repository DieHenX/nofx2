package simulated

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/trader/types"
	"strings"
	"sync"
	"time"
)

// SimulatedTrader implements types.Trader interface for simulated trading
type SimulatedTrader struct {
	mu         sync.RWMutex
	balance    float64           // Available balance (USDT)
	positions  map[string]*Position // Current positions (keyed by symbol)
	orders     map[string]*Order    // Pending orders (keyed by orderID)
	initBalance float64            // Initial balance for reference
	exchange   string             // Source exchange for market data (e.g., "binance", "gate")

	// OnStopTriggered is called when a stop-loss or take-profit is triggered
	OnStopTriggered func(symbol string, side string, qty float64, entry float64, exit float64, pnl float64, stopType string)
}

// Position represents a simulated position
type Position struct {
	Symbol        string
	Side          string  // "long" or "short"
	EntryPrice    float64
	Quantity      float64
	Leverage      int
	Margin        float64 // Margin used
	StopLoss      float64
	TakeProfit    float64
	CreatedAt     time.Time
}

// Order represents a simulated order
type Order struct {
	OrderID      string
	Symbol       string
	Side         string  // "open_long", "open_short", "close_long", "close_short"
	Quantity     float64
	Price        float64
	OrderType    string  // "market", "limit"
	Status       string  // "pending", "filled", "cancelled"
	FilledPrice  float64
	CreatedAt    time.Time
}

// NewSimulatedTrader creates a new simulated trader
func NewSimulatedTrader(initialBalance float64, exchange string) *SimulatedTrader {
	if initialBalance <= 0 {
		initialBalance = 10000 // Default 10000 USDT
	}
	if exchange == "" {
		exchange = "binance" // Default to Binance for market data
	}
	return &SimulatedTrader{
		balance:     initialBalance,
		initBalance: initialBalance,
		positions:   make(map[string]*Position),
		orders:      make(map[string]*Order),
		exchange:    exchange,
	}
}

// GetBalance returns the account balance
func (t *SimulatedTrader) GetBalance() (map[string]interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	total := t.balance
	unrealizedPnl := float64(0)

	// Calculate unrealized PnL from all positions
	for _, pos := range t.positions {
		currentPrice, err := t.fetchMarketPrice(pos.Symbol)
		if err != nil {
			continue
		}

		if pos.Side == "long" {
			unrealizedPnl += (currentPrice - pos.EntryPrice) * pos.Quantity
		} else {
			unrealizedPnl += (pos.EntryPrice - currentPrice) * pos.Quantity
		}
	}

	return map[string]interface{}{
		"totalWalletBalance":    total + unrealizedPnl,
		"availableBalance":      total,
		"totalUnrealizedProfit": unrealizedPnl,
	}, nil
}

// GetPositions returns all open positions
func (t *SimulatedTrader) GetPositions() ([]map[string]interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []map[string]interface{}

	for symbol, pos := range t.positions {
		currentPrice, err := t.fetchMarketPrice(symbol)
		if err != nil {
			continue
		}

		var unrealizedPnl float64
		if pos.Side == "long" {
			unrealizedPnl = (currentPrice - pos.EntryPrice) * pos.Quantity
		} else {
			unrealizedPnl = (pos.EntryPrice - currentPrice) * pos.Quantity
		}

		// Calculate liquidation price (rough estimate)
		var liquidationPrice float64
		if pos.Side == "long" {
			liquidationPrice = pos.EntryPrice * (1 - 1/float64(pos.Leverage))
		} else {
			liquidationPrice = pos.EntryPrice * (1 + 1/float64(pos.Leverage))
		}

		result = append(result, map[string]interface{}{
			"symbol":           symbol,
			"positionSide":     strings.ToUpper(pos.Side),
			"size":             pos.Quantity,
			"entryPrice":       pos.EntryPrice,
			"markPrice":        currentPrice,
			"unrealizedPnl":    unrealizedPnl,
			"leverage":         pos.Leverage,
			"margin":           pos.Margin,
			"liquidationPrice": liquidationPrice,
		})
	}

	return result, nil
}

// OpenLong opens a long position
func (t *SimulatedTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.openPosition(symbol, quantity, leverage, "long")
}

// OpenShort opens a short position
func (t *SimulatedTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.openPosition(symbol, quantity, leverage, "short")
}

// openPosition opens a position (internal)
func (t *SimulatedTrader) openPosition(symbol string, quantity float64, leverage int, side string) (map[string]interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Get current market price
	currentPrice, err := t.fetchMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	// Calculate margin required
	margin := (currentPrice * quantity) / float64(leverage)
	if margin > t.balance {
		return nil, fmt.Errorf("insufficient balance: need %.2f, have %.2f", margin, t.balance)
	}

	// Check if position already exists for this symbol
	if existingPos, ok := t.positions[symbol]; ok {
		// Close existing position first
		if err := t.closePositionInternal(symbol, existingPos); err != nil {
			return nil, fmt.Errorf("failed to close existing position: %w", err)
		}
	}

	// Create new position
	pos := &Position{
		Symbol:     symbol,
		Side:       side,
		EntryPrice: currentPrice,
		Quantity:   quantity,
		Leverage:   leverage,
		Margin:     margin,
		CreatedAt:  time.Now(),
	}

	t.positions[symbol] = pos
	t.balance -= margin

	logger.Log.Infof("  [Simulated] Opened %s position: symbol=%s, qty=%s, price=%.4f, lev=%d, margin=%.2f",
		side, symbol, formatQty(quantity), currentPrice, leverage, margin)

	return map[string]interface{}{
		"orderId":   fmt.Sprintf("SIM-%d", time.Now().UnixNano()),
		"symbol":    symbol,
		"status":    "FILLED",
		"fillPrice": currentPrice,
		"avgPrice":  currentPrice,
	}, nil
}

// CloseLong closes a long position
func (t *SimulatedTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return t.closePosition(symbol, quantity, "long")
}

// CloseShort closes a short position
func (t *SimulatedTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return t.closePosition(symbol, quantity, "short")
}

// closePosition closes a position
func (t *SimulatedTrader) closePosition(symbol string, quantity float64, expectedSide string) (map[string]interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	pos, ok := t.positions[symbol]
	if !ok {
		return nil, fmt.Errorf("no position found for %s", symbol)
	}

	if pos.Side != expectedSide {
		return nil, fmt.Errorf("position side mismatch: expected %s, got %s", expectedSide, pos.Side)
	}

	return t.closePositionInternal(symbol, pos)
}

// closePositionInternal closes a position (must be called with lock held)
func (t *SimulatedTrader) closePositionInternal(symbol string, pos *Position) (map[string]interface{}, error) {
	// Get current market price for closing
	currentPrice, err := t.fetchMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	// Calculate PnL
	var pnl float64
	if pos.Side == "long" {
		pnl = (currentPrice - pos.EntryPrice) * pos.Quantity
	} else {
		pnl = (pos.EntryPrice - currentPrice) * pos.Quantity
	}

	// Return margin + PnL
	returnedMargin := pos.Margin + pnl
	t.balance += returnedMargin

	logger.Log.Infof("  [Simulated] Closed %s position: symbol=%s, qty=%s, close=%.4f, pnl=%.2f, returned=%.2f",
		pos.Side, symbol, formatQty(pos.Quantity), currentPrice, pnl, returnedMargin)

	// Remove position
	delete(t.positions, symbol)

	return map[string]interface{}{
		"orderId":   fmt.Sprintf("SIM-%d", time.Now().UnixNano()),
		"symbol":    symbol,
		"status":    "FILLED",
		"fillPrice": currentPrice,
		"avgPrice":  currentPrice,
		"pnl":       pnl,
	}, nil
}

// SetLeverage sets leverage for a symbol (stored with position)
func (t *SimulatedTrader) SetLeverage(symbol string, leverage int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if leverage < 1 || leverage > 125 {
		return fmt.Errorf("leverage must be between 1 and 125")
	}

	// If position exists, update leverage (would require margin rebalance)
	if pos, ok := t.positions[symbol]; ok {
		// Calculate new margin required
		currentPrice, err := t.fetchMarketPrice(symbol)
		if err != nil {
			return err
		}
		newMargin := (currentPrice * pos.Quantity) / float64(leverage)
		oldMargin := pos.Margin

		// Adjust balance
		marginDiff := newMargin - oldMargin
		if marginDiff > t.balance {
			return fmt.Errorf("insufficient balance for leverage change")
		}
		t.balance -= marginDiff
		pos.Margin = newMargin
		pos.Leverage = leverage

		logger.Log.Infof("  [Simulated] Updated leverage for %s to %dx", symbol, leverage)
	}

	return nil
}

// SetMarginMode sets margin mode (cross/isolated) - simplified for simulation
func (t *SimulatedTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// For simulation, we treat all as cross margin
	logger.Log.Infof("  [Simulated] Margin mode set (simulated: cross margin)")
	return nil
}

// GetMarketPrice gets current market price from exchange
func (t *SimulatedTrader) GetMarketPrice(symbol string) (float64, error) {
	return t.fetchMarketPrice(symbol)
}

// fetchMarketPrice internal method to get market price
func (t *SimulatedTrader) fetchMarketPrice(symbol string) (float64, error) {
	// Use market module to get real price
	data, err := market.Get(symbol)
	if err != nil {
		return 0, fmt.Errorf("failed to get market data: %w", err)
	}

	if data.CurrentPrice <= 0 {
		return 0, fmt.Errorf("invalid price for %s", symbol)
	}

	return data.CurrentPrice, nil
}

// SetStopLoss sets stop-loss for a position
func (t *SimulatedTrader) SetStopLoss(symbol string, positionSide string, quantity float64, stopPrice float64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	pos, ok := t.positions[symbol]
	if !ok {
		return fmt.Errorf("no position found for %s", symbol)
	}

	pos.StopLoss = stopPrice
	logger.Log.Infof("  [Simulated] Set stop-loss for %s at %.4f", symbol, stopPrice)
	return nil
}

// SetTakeProfit sets take-profit for a position
func (t *SimulatedTrader) SetTakeProfit(symbol string, positionSide string, quantity float64, takeProfitPrice float64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	pos, ok := t.positions[symbol]
	if !ok {
		return fmt.Errorf("no position found for %s", symbol)
	}

	pos.TakeProfit = takeProfitPrice
	logger.Log.Infof("  [Simulated] Set take-profit for %s at %.4f", symbol, takeProfitPrice)
	return nil
}

// CancelStopLossOrders cancels stop-loss orders (no-op in simulation)
func (t *SimulatedTrader) CancelStopLossOrders(symbol string) error {
	logger.Log.Infof("  [Simulated] Cancel stop-loss orders for %s (no-op)", symbol)
	return nil
}

// CancelTakeProfitOrders cancels take-profit orders (no-op in simulation)
func (t *SimulatedTrader) CancelTakeProfitOrders(symbol string) error {
	logger.Log.Infof("  [Simulated] Cancel take-profit orders for %s (no-op)", symbol)
	return nil
}

// CancelAllOrders cancels all pending orders
func (t *SimulatedTrader) CancelAllOrders(symbol string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// In simulation, we don't have pending limit orders
	logger.Log.Infof("  [Simulated] Cancel all orders for %s (no-op)", symbol)
	return nil
}

// CancelStopOrders cancels stop orders
func (t *SimulatedTrader) CancelStopOrders(symbol string) error {
	logger.Log.Infof("  [Simulated] Cancel stop orders for %s (no-op)", symbol)
	return nil
}

// FormatQuantity formats quantity to appropriate precision
func (t *SimulatedTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return formatQty(quantity), nil
}

// GetOrderStatus gets order status (simplified)
func (t *SimulatedTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// In simulation, orders are immediately filled
	return map[string]interface{}{
		"status":       "FILLED",
		"avgPrice":     0,
		"executedQty": 0,
		"commission":   0,
	}, nil
}

// GetClosedPnL returns empty for simulated trader (no historical data)
func (t *SimulatedTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	// Simulated trader doesn't have closed PnL records
	return []types.ClosedPnLRecord{}, nil
}

// GetOpenOrders returns empty for simulated trader
func (t *SimulatedTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	return []types.OpenOrder{}, nil
}

// CheckAndExecuteStops checks and executes stop-loss/take-profit orders
func (t *SimulatedTrader) CheckAndExecuteStops() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for symbol, pos := range t.positions {
		currentPrice, err := t.fetchMarketPrice(symbol)
		if err != nil {
			continue
		}

		shouldClose := false
		closeSide := pos.Side
		stopType := "SL"

		// Check stop-loss
		if pos.StopLoss > 0 {
			if pos.Side == "long" && currentPrice <= pos.StopLoss {
				shouldClose = true
				stopType = "SL"
			} else if pos.Side == "short" && currentPrice >= pos.StopLoss {
				shouldClose = true
				stopType = "SL"
			}
		}

		// Check take-profit
		if pos.TakeProfit > 0 {
			if pos.Side == "long" && currentPrice >= pos.TakeProfit {
				shouldClose = true
				stopType = "TP"
			} else if pos.Side == "short" && currentPrice <= pos.TakeProfit {
				shouldClose = true
				stopType = "TP"
			}
		}

		if shouldClose {
			var pnl float64
			if pos.Side == "long" {
				pnl = (currentPrice - pos.EntryPrice) * pos.Quantity
			} else {
				pnl = (pos.EntryPrice - currentPrice) * pos.Quantity
			}

			returnedMargin := pos.Margin + pnl
			t.balance += returnedMargin

			// Call callback before deleting position
			if t.OnStopTriggered != nil {
				t.OnStopTriggered(symbol, pos.Side, pos.Quantity, pos.EntryPrice, currentPrice, pnl, stopType)
			}

			delete(t.positions, symbol)

			logger.Log.Infof("  [Simulated] %s triggered: %s %s at %.4f, PnL=%.2f",
				stopType, closeSide, symbol, currentPrice, pnl)
		}
	}

	return nil
}

// GetStats returns trading statistics
func (t *SimulatedTrader) GetStats() (map[string]interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	totalPnl := t.balance - t.initBalance
	winCount := 0
	lossCount := 0

	// Note: In a full implementation, you'd track historical trades
	return map[string]interface{}{
		"initialBalance": t.initBalance,
		"currentBalance": t.balance,
		"totalPnl":       totalPnl,
		"openPositions":  len(t.positions),
		"winCount":       winCount,
		"lossCount":      lossCount,
	}, nil
}

// ============================================================================
// Helper functions
// ============================================================================

// formatQty formats quantity to string with appropriate precision
func formatQty(qty float64) string {
	if qty >= 1 {
		return fmt.Sprintf("%.4f", qty)
	}
	return fmt.Sprintf("%.6f", qty)
}

// Ensure SimulatedTrader implements Trader interface
var _ types.Trader = (*SimulatedTrader)(nil)
