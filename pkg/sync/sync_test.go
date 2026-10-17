package sync

import (
	"testing"

	"PXMarkMapBackEnd/pkg/google"
)

func TestConvertToStoreInfoIncludesCustomSheetNames(t *testing.T) {
	storeMap := map[string]*google.StoreData{
		"Sample Store": {
			StoreName: "Sample Store",
			ProductShipments: map[string][]google.Shipment{
				"新工作表": []google.Shipment{{Date: "2026/01/05", Qty: "42"}},
			},
		},
	}

	stores := convertToStoreInfo(storeMap)
	if len(stores) != 1 {
		t.Fatalf("expected 1 store, got %d", len(stores))
	}

	if got := len(stores[0].ProductShipments["新工作表"]); got != 1 {
		t.Fatalf("expected custom sheet shipments to be preserved, got %d", got)
	}
}
