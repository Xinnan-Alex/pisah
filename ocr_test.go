package main

import (
	"testing"

	"github.com/pisah/share"
)

func TestNormalizeParsedReceipt_taxInclusiveKFC(t *testing.T) {
	rec := ParsedReceipt{
		SubtotalSen: 2168,
		SstSen:      130,
		TotalSen:    2298,
		Items: []ParsedItem{
			{Name: "SEAWEED SHAKER FRIES (M)", Qty: 1, UnitPriceSen: 599, LineTotalSen: 599},
			{Name: "1-PC NASI LEMAK KFC COMBO", Qty: 1, UnitPriceSen: 1699, LineTotalSen: 1699},
		},
	}
	normalizeParsedReceipt(&rec)

	var itemsSum int64
	for _, it := range rec.Items {
		itemsSum += it.LineTotalSen
	}
	if itemsSum != rec.SubtotalSen {
		t.Fatalf("items sum = %d, want subtotal %d", itemsSum, rec.SubtotalSen)
	}

	claimed := share.ClaimedSen([]share.Item{
		{LineTotalSen: rec.Items[0].LineTotalSen, Claimants: 1},
		{LineTotalSen: rec.Items[1].LineTotalSen, Claimants: 1},
	})
	owed := share.Owed(claimed, rec.SubtotalSen, rec.SstSen)
	if owed != rec.TotalSen {
		t.Fatalf("owed = %d, want total %d", owed, rec.TotalSen)
	}
}

func TestNormalizeParsedReceipt_taxExclusiveUnchanged(t *testing.T) {
	rec := ParsedReceipt{
		SubtotalSen: 9690,
		SstSen:      1550,
		TotalSen:    11240,
		Items: []ParsedItem{
			{Name: "Nasi Lemak Ayam", Qty: 1, LineTotalSen: 4500},
			{Name: "Teh Tarik", Qty: 1, LineTotalSen: 5190},
		},
	}
	before := rec.Items
	normalizeParsedReceipt(&rec)
	for i, it := range rec.Items {
		if it.LineTotalSen != before[i].LineTotalSen {
			t.Fatalf("item %d changed from %d to %d", i, before[i].LineTotalSen, it.LineTotalSen)
		}
	}
}
