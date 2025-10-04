package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"PXMarkMapBackEnd/pkg/database"
	"PXMarkMapBackEnd/pkg/google"
)

// func main() {
// 	// 先抓 Sheet
// 	storeMap, err := google.LoadAndOrganizeSheets()
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	// 測試：對每個店名去 Google Places 查資料
// 	for _, store := range storeMap {
// 		result, err := google.SearchPlaceByName(store.StoreName)
// 		if err != nil {
// 			log.Printf("查詢 %s 失敗: %v\n", store.StoreName, err)
// 			continue
// 		}

// 		if len(result.Candidates) > 0 {
// 			place := result.Candidates[0]
// 			fmt.Printf("店名: %s\n地址: %s\n經緯度: (%f, %f)\nPlaceID: %s\n\n",
// 				place.Name,
// 				place.Address,
// 				place.Geometry.Location.Lat,
// 				place.Geometry.Location.Lng,
// 				place.PlaceID,
// 			)
// 		} else {
// 			fmt.Printf("店名: %s → 找不到資料\n", store.StoreName)
// 		}
// 	}
// }
func main() {
	// 步驟 1: 從 Google Sheets 讀取資料
	log.Println("=== 開始讀取 Google Sheets 資料 ===")
	storeMap, err := google.LoadAndOrganizeSheets()
	if err != nil {
		log.Fatalf("讀取 Sheets 失敗: %v", err)
	}
	log.Printf("成功讀取 %d 個店家\n", len(storeMap))

	// 步驟 2: 使用 Places API 搜尋地點資訊
	log.Println("\n=== 開始搜尋店家地點資訊 ===")
	if err := google.EnrichStoresWithPlaceData(storeMap); err != nil {
		log.Printf("搜尋地點資訊時發生錯誤: %v", err)
	}

	// 步驟 3: 連接資料庫
	log.Println("\n=== 連接資料庫 ===")
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	dbConfig := database.DBConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     dbPort,
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", ""),
		DBName:   getEnv("DB_NAME", "px_mark_map_db"),
	}

	db, err := database.ConnectDB(dbConfig)
	if err != nil {
		log.Fatalf("無法連接資料庫: %v", err)
	}
	defer db.Close()

	// 步驟 4: 轉換資料格式並儲存到資料庫
	log.Println("\n=== 儲存資料到資料庫 ===")
	stores := convertToStoreInfo(storeMap)
	if err := database.SaveStores(db, stores); err != nil {
		log.Fatalf("儲存資料失敗: %v", err)
	}

	// 步驟 5: 查詢並顯示近三天的出貨資料
	log.Println("\n=== 查詢近三天出貨資料 ===")
	recentShipments, err := database.GetRecentShipments(db, 3)
	if err != nil {
		log.Fatalf("查詢失敗: %v", err)
	}

	// 整理顯示結果
	displayResults(recentShipments)

	fmt.Println("\n=== 完成 ===")
}

// convertToStoreInfo 將 google.StoreData 轉換為 database.StoreInfo
func convertToStoreInfo(storeMap map[string]*google.StoreData) []database.StoreInfo {
	var stores []database.StoreInfo

	for _, data := range storeMap {
		// 轉換秋葵出貨紀錄
		var okraShipments []database.ShipmentInfo
		for _, s := range data.OkraShipments {
			okraShipments = append(okraShipments, database.ShipmentInfo{
				Date: s.Date,
				Qty:  s.Qty,
			})
		}

		// 轉換絲瓜出貨紀錄
		var gourdShipments []database.ShipmentInfo
		for _, s := range data.SpongeGourdShipments {
			gourdShipments = append(gourdShipments, database.ShipmentInfo{
				Date: s.Date,
				Qty:  s.Qty,
			})
		}

		stores = append(stores, database.StoreInfo{
			StoreName:        data.StoreName,
			PlaceID:          data.PlaceID,
			FormattedAddress: data.FormattedAddress,
			Latitude:         data.Latitude,
			Longitude:        data.Longitude,
			OkraShipments:    okraShipments,
			GourdShipments:   gourdShipments,
		})
	}

	return stores
}

// displayResults 整理並顯示查詢結果
func displayResults(results []map[string]interface{}) {
	if len(results) == 0 {
		fmt.Println("近三天沒有出貨紀錄")
		return
	}

	// 按店家分組顯示
	storeGroup := make(map[string][]map[string]interface{})
	for _, record := range results {
		storeName := record["store_name"].(string)
		storeGroup[storeName] = append(storeGroup[storeName], record)
	}

	for storeName, records := range storeGroup {
		fmt.Printf("\n【%s】\n", storeName)

		// 顯示地址資訊（只需顯示一次）
		if len(records) > 0 {
			fmt.Printf("  地址: %s\n", records[0]["address"])
			fmt.Printf("  座標: %.6f, %.6f\n", records[0]["latitude"], records[0]["longitude"])
		}

		// 分類顯示秋葵和絲瓜
		okraShipments := []map[string]interface{}{}
		gourdShipments := []map[string]interface{}{}

		for _, record := range records {
			productType := record["product_type"].(string)
			if productType == "秋葵" {
				okraShipments = append(okraShipments, record)
			} else if productType == "產銷絲瓜" {
				gourdShipments = append(gourdShipments, record)
			}
		}

		if len(okraShipments) > 0 {
			fmt.Println("\n  📦 近三天秋葵出貨:")
			for _, s := range okraShipments {
				fmt.Printf("    %s: %s\n", s["shipment_date"], s["quantity"])
			}
		}

		if len(gourdShipments) > 0 {
			fmt.Println("\n  📦 近三天絲瓜出貨:")
			for _, s := range gourdShipments {
				fmt.Printf("    %s: %s\n", s["shipment_date"], s["quantity"])
			}
		}
	}
}

// getEnv 取得環境變數，如果不存在則使用預設值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}