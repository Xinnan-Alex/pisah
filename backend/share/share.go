// Package share holds Pisah's bill-split math. Pure functions, stdlib only —
// so `go test ./share` runs without the DB/AWS deps the rest of the service needs.
//
// All money is integer sen (1 sen = RM 0.01). Floats are never used for money.
package share

// Item is one receipt line plus how many participants are splitting it.
// Multi-unit receipt lines are normalised to one Item per unit by the owner at
// review time, so equal-split-among-claimants is exact for exclusive items and
// correct for genuinely shared ones (e.g. one dish split three ways).
type Item struct {
	LineTotalSen int64
	Claimants    int // participants sharing this item; <1 treated as 1
}

// ClaimedSen sums a participant's portion of the items they claimed, each item
// split equally among its claimants (round half-up per item).
func ClaimedSen(items []Item) int64 {
	var sum int64
	for _, it := range items {
		n := int64(it.Claimants)
		if n < 1 {
			n = 1
		}
		sum += roundDiv(it.LineTotalSen, n)
	}
	return sum
}

// Owed returns what a participant owes in sen: their claimed item portions plus
// a proportional cut of the bill's tax + service + rounding.
//
//	owed = claimed + round( claimed / subtotal * taxTotal )
//
// ponytail: each participant's tax cut and per-item portion round independently,
// so summing every participant can drift a sen or two from the printed bill
// total. Acceptable for P2P splitting — the owner's bill total stays
// authoritative. Upgrade path if exactness ever matters: largest-remainder
// allocation computed across all participants in one pass.
func Owed(claimedSen, subtotalSen, taxTotalSen int64) int64 {
	if subtotalSen <= 0 {
		return claimedSen
	}
	taxShare := roundDiv(claimedSen*taxTotalSen, subtotalSen)
	return claimedSen + taxShare
}

// roundDiv divides a by b (b > 0) rounding half-up.
func roundDiv(a, b int64) int64 {
	if b <= 0 {
		return 0
	}
	return (a + b/2) / b
}
