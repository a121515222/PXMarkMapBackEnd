package google

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
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
	ProductShipments     map[string][]Shipment
	// 地點資訊
	PlaceID          string
	FormattedAddress string
	Latitude         float64
	Longitude        float64
}

func (s *StoreData) AddShipment(sheetName string, shipment Shipment) {
	if s.ProductShipments == nil {
		s.ProductShipments = make(map[string][]Shipment)
	}
	if sheetName == "" {
		return
	}

	s.ProductShipments[sheetName] = append(s.ProductShipments[sheetName], shipment)

	switch sheetName {
	case "秋葵":
		s.OkraShipments = append(s.OkraShipments, shipment)
	case "產銷絲瓜":
		s.SpongeGourdShipments = append(s.SpongeGourdShipments, shipment)
	}
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
type sheetConfig struct {
	GID  string
	Name string
}

func getSheetConfigsFromEnv() ([]sheetConfig, error) {
	if gidsEnv := strings.TrimSpace(os.Getenv("GOOGLE_SHEET_GIDS")); gidsEnv != "" {
		namesEnv := strings.TrimSpace(os.Getenv("GOOGLE_SHEET_NAMES"))
		if namesEnv == "" {
			return nil, fmt.Errorf("GOOGLE_SHEET_NAMES is required when GOOGLE_SHEET_GIDS is configured")
		}

		gids := splitCSV(gidsEnv)
		names := splitCSV(namesEnv)
		if len(gids) != len(names) {
			return nil, fmt.Errorf("GOOGLE_SHEET_GIDS count and GOOGLE_SHEET_NAMES count do not match")
		}

		configs := make([]sheetConfig, 0, len(gids))
		for i := range gids {
			configs = append(configs, sheetConfig{GID: gids[i], Name: names[i]})
		}
		return configs, nil
	}

	indexedConfigs, err := getIndexedSheetConfigsFromEnv()
	if err != nil {
		return nil, err
	}
	if len(indexedConfigs) > 0 {
		return indexedConfigs, nil
	}

	return nil, fmt.Errorf("Google Sheet configuration not found: set GOOGLE_SHEET_ID with GOOGLE_SHEET_GIDS/GOOGLE_SHEET_NAMES or GOOGLE_SHEET_1_GID + GOOGLE_SHEET_1_NAME")
}

func getIndexedSheetConfigsFromEnv() ([]sheetConfig, error) {
	indexed := make(map[int]sheetConfig)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value == "" {
			continue
		}

		index, kind, ok := parseIndexedSheetKey(key)
		if !ok {
			continue
		}

		config := indexed[index]
		switch kind {
		case "GID", "ID":
			config.GID = value
		case "NAME", "SHEET_NAME", "TITLE":
			config.Name = value
		}
		indexed[index] = config
	}

	if len(indexed) == 0 {
		return nil, nil
	}

	keys := make([]int, 0, len(indexed))
	for idx := range indexed {
		keys = append(keys, idx)
	}
	sort.Ints(keys)

	configs := make([]sheetConfig, 0, len(keys))
	for _, idx := range keys {
		cfg := indexed[idx]
		if cfg.GID == "" || cfg.Name == "" {
			continue
		}
		configs = append(configs, cfg)
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("Sheet index env vars were found, but no complete GID + NAME pairs were configured")
	}

	return configs, nil
}

func parseIndexedSheetKey(key string) (int, string, bool) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^GOOGLE_SHEET_(\d+)_(ID|GID|NAME|SHEET_NAME|TITLE)$`),
		regexp.MustCompile(`^GOOGLE_SHEET_(ID|GID|NAME|SHEET_NAME|TITLE)_(\d+)$`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(key)
		if len(matches) != 3 {
			continue
		}

		if idx, err := strconv.Atoi(matches[1]); err == nil {
			return idx, strings.ToUpper(matches[2]), true
		}
		if idx, err := strconv.Atoi(matches[2]); err == nil {
			return idx, strings.ToUpper(matches[1]), true
		}
	}

	return 0, "", false
}

func splitCSV(value string) []string {
	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func LoadAndOrganizeSheets() (map[string]*StoreData, error) {
	sheetID := strings.TrimSpace(os.Getenv("GOOGLE_SHEET_ID"))
	if sheetID == "" {
		return nil, fmt.Errorf("GOOGLE_SHEET_ID is required")
	}

	sheetConfigs, err := getSheetConfigsFromEnv()
	if err != nil {
		return nil, err
	}
	if len(sheetConfigs) == 0 {
		return nil, fmt.Errorf("no Google Sheet config found")
	}

	storeMap := make(map[string]*StoreData)

	for _, config := range sheetConfigs {
		sheetName := strings.TrimSpace(config.Name)
		gid := strings.TrimSpace(config.GID)
		if sheetName == "" || gid == "" {
			continue
		}

		records, err := LoadSheetByGID(sheetID, gid)
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
				storeMap[storeName].AddShipment(sheetName, shipment)
			}
		}
	}

	return storeMap, nil
}