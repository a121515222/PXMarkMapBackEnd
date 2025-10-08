package sync

import (
	"database/sql"
	"log"

	"PXMarkMapBackEnd/pkg/database"
	"PXMarkMapBackEnd/pkg/google"
)

// SyncData 完整同步（包含 Places API）- 每月執行
func SyncData(db *sql.DB) error {
	log.Println("=== 開始完整同步（含地點資訊） ===")

	// 步驟 1: 從 Google Sheets 讀取資料
	log.Println("[INFO] 讀取 Google Sheets 資料...")
	storeMap, err := google.LoadAndOrganizeSheets()
	if err != nil {
		return err
	}
	log.Printf("[INFO] 成功讀取 %d 個店家\n", len(storeMap))

	// 步驟 2: 使用 Places API 搜尋地點資訊
	log.Println("[INFO] 搜尋店家地點資訊...")
	if err := google.EnrichStoresWithPlaceData(storeMap); err != nil {
		log.Printf("[WARN] 搜尋地點資訊時發生錯誤: %v", err)
	}

	// 步驟 3: 轉換資料格式
	stores := convertToStoreInfo(storeMap)

	// 步驟 4: 儲存到資料庫
	log.Println("[INFO] 儲存資料到資料庫...")
	if err := database.SaveStores(db, stores); err != nil {
		return err
	}

	log.Println("[INFO] 完整同步完成")
	return nil
}

// SyncDataDaily 每日同步（只更新出貨資料，缺少地點的才查詢）
func SyncDataDaily(db *sql.DB) error {
	log.Println("=== 開始每日同步（優先使用現有地點資訊） ===")

	// 步驟 1: 從 Google Sheets 讀取資料
	log.Println("[INFO] 讀取 Google Sheets 資料...")
	storeMap, err := google.LoadAndOrganizeSheets()
	if err != nil {
		return err
	}
	log.Printf("[INFO] 成功讀取 %d 個店家\n", len(storeMap))

	// 步驟 2: 檢查並補充缺少的地點資訊
	log.Println("[INFO] 檢查店家地點資訊...")
	if err := enrichMissingPlaceData(db, storeMap); err != nil {
		log.Printf("[WARN] 補充地點資訊時發生錯誤: %v", err)
	}

	// 步驟 3: 轉換資料格式
	stores := convertToStoreInfo(storeMap)

	// 步驟 4: 儲存到資料庫（會自動更新或插入）
	log.Println("[INFO] 儲存資料到資料庫...")
	if err := database.SaveStores(db, stores); err != nil {
		return err
	}

	log.Println("[INFO] 每日同步完成")
	return nil
}

// enrichMissingPlaceData 只為缺少地點資訊的店家查詢 Places API
func enrichMissingPlaceData(db *sql.DB, storeMap map[string]*google.StoreData) error {
	// 從資料庫查詢已有地點資訊的店家
	existingStores, err := database.GetExistingStoresWithLocation(db)
	if err != nil {
		return err
	}

	log.Printf("[INFO] 資料庫中已有 %d 個店家的地點資訊", len(existingStores))

	// 標記需要查詢的店家
	needPlaceAPI := make(map[string]*google.StoreData)

	for storeName, storeData := range storeMap {
		if existingStore, exists := existingStores[storeName]; exists {
			// 使用資料庫中的地點資訊
			storeData.PlaceID = existingStore.PlaceID
			storeData.FormattedAddress = existingStore.FormattedAddress
			storeData.Latitude = existingStore.Latitude
			storeData.Longitude = existingStore.Longitude
			log.Printf("[INFO] 使用現有地點: %s", storeName)
		} else {
			// 標記為需要查詢
			needPlaceAPI[storeName] = storeData
		}
	}

	// 只為缺少地點的店家查詢 Places API
	if len(needPlaceAPI) > 0 {
		log.Printf("[INFO] 需要查詢 %d 個新店家的地點資訊", len(needPlaceAPI))
		if err := google.EnrichStoresWithPlaceData(needPlaceAPI); err != nil {
			return err
		}
	} else {
		log.Println("[INFO] 所有店家都已有地點資訊，跳過 Places API 查詢")
	}

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