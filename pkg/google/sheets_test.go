package google

import "testing"

func TestStoreDataAddShipmentTracksCustomSheetNames(t *testing.T) {
	data := &StoreData{StoreName: "Sample Store"}

	data.AddShipment("新工作表", Shipment{Date: "2026/01/05", Qty: "42"})

	if got := len(data.ProductShipments["新工作表"]); got != 1 {
		t.Fatalf("expected 1 shipment in custom sheet, got %d", got)
	}

	if _, ok := data.ProductShipments["新工作表"]; !ok {
		t.Fatal("custom sheet shipment was not recorded")
	}
}

func TestGetSheetConfigsFromEnvSupportsIndexedSheets(t *testing.T) {
	t.Setenv("GOOGLE_SHEET_ID", "spreadsheet-123")
	t.Setenv("GOOGLE_SHEET_1_GID", "111")
	t.Setenv("GOOGLE_SHEET_1_NAME", "秋葵")
	t.Setenv("GOOGLE_SHEET_2_GID", "222")
	t.Setenv("GOOGLE_SHEET_2_NAME", "產銷絲瓜")

	configs, err := getSheetConfigsFromEnv()
	if err != nil {
		t.Fatalf("expected no error parsing indexed sheets: %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("expected 2 sheet configs, got %d", len(configs))
	}

	if configs[0].GID != "111" || configs[0].Name != "秋葵" {
		t.Fatalf("first config mismatch: %#v", configs[0])
	}

	if configs[1].GID != "222" || configs[1].Name != "產銷絲瓜" {
		t.Fatalf("second config mismatch: %#v", configs[1])
	}
}
