package api

import (
	"testing"

	"nofx/store"
)

func TestValidateTraderLeverageRangeMatchesManualLimits(t *testing.T) {
	maxLev := store.MaxLeverageLimit

	// 20x should always be accepted
	if msg, code := validateTraderLeverageRange(20, 20); msg != "" || code != "" {
		t.Fatalf("expected 20/20 leverage to be accepted, got msg=%q code=%q", msg, code)
	}

	// max-1 should be accepted
	if msg, code := validateTraderLeverageRange(maxLev-1, maxLev-1); msg != "" || code != "" {
		t.Fatalf("expected max-1/max-1 leverage to be accepted, got msg=%q code=%q", msg, code)
	}

	// max should be accepted
	if msg, code := validateTraderLeverageRange(maxLev, maxLev); msg != "" || code != "" {
		t.Fatalf("expected max/max leverage to be accepted, got msg=%q code=%q", msg, code)
	}

	// max+1 should be rejected
	if msg, code := validateTraderLeverageRange(maxLev+1, maxLev); msg == "" || code != "trader.create.invalid_btc_eth_leverage" {
		t.Fatalf("expected BTC/ETH leverage > max to be rejected, got msg=%q code=%q", msg, code)
	}

	if msg, code := validateTraderLeverageRange(maxLev, maxLev+1); msg == "" || code != "trader.create.invalid_altcoin_leverage" {
		t.Fatalf("expected altcoin leverage > max to be rejected, got msg=%q code=%q", msg, code)
	}
}
