package main

import "testing"

func TestExpandSplitItemsMakesUnitsClaimable(t *testing.T) {
	included := true
	items := expandSplitItems([]SplitItemInput{{
		Name:            "Tom Yum Rice",
		Qty:             2,
		LineTotalSen:    3580,
		IncludedInSplit: &included,
	}})
	if len(items) != 2 || items[0].Qty != 1 || items[1].Qty != 1 {
		t.Fatalf("got %+v, want two single units", items)
	}
	if items[0].LineTotalSen+items[1].LineTotalSen != 3580 {
		t.Fatalf("unit totals = %d, want 3580", items[0].LineTotalSen+items[1].LineTotalSen)
	}
}
