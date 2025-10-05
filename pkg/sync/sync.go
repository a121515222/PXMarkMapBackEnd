package sync

import (
	"database/sql"
	"log"

	"PXMarkMapBackEnd/pkg/database"
	"PXMarkMapBackEnd/pkg/google"
)

// SyncData 執行完整的資料同步流程
func SyncData(db *sql.DB) error {
	log.Println("=== 開始同步資料 ===")

	// 步驟 1: 從 Google Sheets 讀取資料
	log.Println("📄 讀取 Google Sheets 資料...")
	storeMap, err := google.LoadAndOrganizeSheets()
	if err != nil {
		return err
	}
	log.Printf("✓ 成功讀取 %d 個店家\n", len(storeMap))

	// 步驟 2: 使用 Places API 搜尋地點資訊
	log.Println("📍 搜尋店家地點資訊...")
	if err := google.EnrichStoresWithPlaceData(storeMap); err != nil {
		log.Printf("⚠ 搜尋地點資訊時發生錯誤: %v", err)
	}

	// 步驟 3: 轉換資料格式
	stores := convertToStoreInfo(storeMap)

	// 步驟 4: 儲存到資料庫
	log.Println("💾 儲存資料到資料庫...")
	if err := database.SaveStores(db, stores); err != nil {
		return err
	}

	log.Println("✓ 資料同步完成！")
	return nil
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