package google

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// 出貨紀錄
type Shipment struct {
	Date string
	Qty  string
}

// 每個店名的資料
type StoreData struct {
	StoreName            string
	OkraShipments        []Shipment
	SpongeGourdShipments []Shipment
	// 地點資訊
	PlaceID          string
	FormattedAddress string
	Latitude         float64
	Longitude        float64
}

// 抓單個 CSV
func LoadSheetByGID(sheetID, gid string) ([][]string, error) {
	csvURL := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?format=csv&gid=%s", sheetID, gid)
	resp, err := http.Get(csvURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	// 去掉空格
	for i := range records {
		for j := range records[i] {
			records[i][j] = strings.TrimSpace(records[i][j])
		}
	}
	return records, nil
}

// 抓所有 sheet 並整理
func LoadAndOrganizeSheets() (map[string]*StoreData, error) {
	sheetID := os.Getenv("GOOGLE_SHEET_ID")
	gidsEnv := os.Getenv("GOOGLE_SHEET_GIDS")   // 例如 "0,123456789"
	namesEnv := os.Getenv("GOOGLE_SHEET_NAMES") // 對應名稱 "秋葵,產銷絲瓜"

	if sheetID == "" || gidsEnv == "" || namesEnv == "" {
		return nil, fmt.Errorf("GOOGLE_SHEET_ID or GOOGLE_SHEET_GIDS or GOOGLE_SHEET_NAMES not set")
	}

	gids := strings.Split(gidsEnv, ",")
	names := strings.Split(namesEnv, ",")
	if len(gids) != len(names) {
		return nil, fmt.Errorf("GIDs count and Names count do not match")
	}

	storeMap := make(map[string]*StoreData)

	for i, gid := range gids {
		sheetName := strings.TrimSpace(names[i])
		records, err := LoadSheetByGID(sheetID, strings.TrimSpace(gid))
		if err != nil {
			log.Printf("failed to load sheet %s: %v\n", sheetName, err)
			continue
		}

		if len(records) < 2 {
			continue
		}

		// 交叉表: 第一列是日期
		header := records[0]

		for j := 1; j < len(records); j++ {
			row := records[j]
			storeName := row[0]
			if _, ok := storeMap[storeName]; !ok {
				storeMap[storeName] = &StoreData{StoreName: storeName}
			}

			for k := 1; k < len(row) && k < len(header); k++ {
				date := header[k]
				qty := row[k]

				shipment := Shipment{Date: date, Qty: qty}
				if sheetName == "秋葵" {
					storeMap[storeName].OkraShipments = append(storeMap[storeName].OkraShipments, shipment)
				} else if sheetName == "產銷絲瓜" {
					storeMap[storeName].SpongeGourdShipments = append(storeMap[storeName].SpongeGourdShipments, shipment)
				}
			}
		}
	}

	return storeMap, nil
}