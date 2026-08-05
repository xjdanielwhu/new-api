package billingexpr

import (
	"fmt"
	"math"
)

// Quota columns (user/token/log) are 32-bit integers in the database, so an
// oversized product must be rejected instead of wrapping around and turning a
// charge into a credit.
const (
	MaxQuota = math.MaxInt32
	MinQuota = math.MinInt32
)

// QuotaRound converts a float64 quota value to int using half-away-from-zero
// rounding. Every tiered billing path (pre-consume, settlement, breakdown
// validation, log fields) MUST use this function to avoid +-1 discrepancies.
func QuotaRound(f float64) int {
	return int(math.Round(f))
}

// QuotaRoundStrict rejects an unrepresentable pre-consume estimate instead of
// letting a saturated result reach billing.
func QuotaRoundStrict(f float64) (int, error) {
	rounded := math.Round(f)
	switch {
	case math.IsNaN(rounded):
		return 0, fmt.Errorf("quota conversion (QuotaRound) nan: original=%g", f)
	case rounded >= MaxQuota:
		return 0, fmt.Errorf("quota conversion (QuotaRound) overflow: original=%g, max=%d", f, MaxQuota)
	case rounded <= MinQuota:
		return 0, fmt.Errorf("quota conversion (QuotaRound) underflow: original=%g, min=%d", f, MinQuota)
	}
	return int(rounded), nil
}
