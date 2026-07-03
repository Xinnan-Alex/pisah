package share

import "testing"

// Numbers below are the exact figures from the Pisah prototype, verified against
// the SwiftUI app: Nasi Lemak House, subtotal RM 96.90, tax+service+rounding
// RM 15.50, bill total RM 112.40.
const (
	subtotalSen = 9690
	taxTotalSen = 1550 // 11240 - 9690
)

func TestOwed_singleItem(t *testing.T) {
	// Sara claims one Nasi Lemak Ayam unit @ RM 12.50.
	claimed := ClaimedSen([]Item{{LineTotalSen: 1250, Claimants: 1}})
	if claimed != 1250 {
		t.Fatalf("claimed = %d, want 1250", claimed)
	}
	owed := Owed(claimed, subtotalSen, taxTotalSen)
	if owed != 1450 { // RM 14.50 — matches prototype
		t.Fatalf("owed = %d, want 1450", owed)
	}
}

func TestOwed_threeItemsWithShared(t *testing.T) {
	// Nasi Lemak Ayam (1250) + Teh Tarik (280) + Sambal Sotong (1800) split 3 ways.
	claimed := ClaimedSen([]Item{
		{LineTotalSen: 1250, Claimants: 1},
		{LineTotalSen: 280, Claimants: 1},
		{LineTotalSen: 1800, Claimants: 3},
	})
	if claimed != 2130 { // 1250 + 280 + 600
		t.Fatalf("claimed = %d, want 2130", claimed)
	}
	owed := Owed(claimed, subtotalSen, taxTotalSen)
	if owed != 2471 { // RM 24.71 — matches prototype
		t.Fatalf("owed = %d, want 2471", owed)
	}
}

func TestOwed_nothingClaimed(t *testing.T) {
	if got := Owed(ClaimedSen(nil), subtotalSen, taxTotalSen); got != 0 {
		t.Fatalf("owed = %d, want 0", got)
	}
}

func TestOwed_splittableSubtotal(t *testing.T) {
	// Bill subtotal RM 100, but RM 20 is owner-only; friends split RM 80.
	// Tax RM 10 total. Friend claims RM 40 of splittable pool.
	// Tax share = round(40 * 10 / 80) = 5 → owed RM 45.00.
	splittableSub := int64(8000)
	claimed := ClaimedSen([]Item{{LineTotalSen: 4000, Claimants: 1}})
	if claimed != 4000 {
		t.Fatalf("claimed = %d, want 4000", claimed)
	}
	owed := Owed(claimed, splittableSub, 1000)
	if owed != 4500 {
		t.Fatalf("owed = %d, want 4500", owed)
	}
}

func TestOwed_zeroSubtotalDoesNotPanic(t *testing.T) {
	if got := Owed(500, 0, 1550); got != 500 {
		t.Fatalf("owed = %d, want 500 (no tax when subtotal unknown)", got)
	}
}
