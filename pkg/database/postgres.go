package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// DBConfig 資料庫連線設定
type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// ConnectDB 連接資料庫
func ConnectDB(config DBConfig) (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.DBName,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("[INFO] 成功連接到 PostgreSQL")
	return db, nil
}

// StoreInfo 用於接收店家資料的介面
type StoreInfo struct {
	StoreName        string
	PlaceID          string
	FormattedAddress string
	Latitude         float64
	Longitude        float64
	OkraShipments    []ShipmentInfo
	GourdShipments   []ShipmentInfo
}

// ShipmentInfo 出貨資訊
type ShipmentInfo struct {
	Date string
	Qty  string
}

// SaveStores 儲存店家資料到資料庫
func SaveStores(db *sql.DB, stores []StoreInfo) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, store := range stores {
		// 插入或更新店家資料
		var storeID int
		err := tx.QueryRow(`
			INSERT INTO stores (store_name, place_id, formatted_address, latitude, longitude, updated_at)
			VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
			ON CONFLICT (store_name) 
			DO UPDATE SET 
				place_id = EXCLUDED.place_id,
				formatted_address = EXCLUDED.formatted_address,
				latitude = EXCLUDED.latitude,
				longitude = EXCLUDED.longitude,
				updated_at = CURRENT_TIMESTAMP
			RETURNING id
		`, store.StoreName, store.PlaceID, store.FormattedAddress, store.Latitude, store.Longitude).Scan(&storeID)

		if err != nil {
			return fmt.Errorf("儲存店家 %s 失敗: %v", store.StoreName, err)
		}

		// 儲存秋葵出貨紀錄
		for _, shipment := range store.OkraShipments {
			if err := saveShipment(tx, storeID, "秋葵", shipment); err != nil {
				log.Printf("儲存秋葵出貨紀錄失敗: %v", err)
			}
		}

		// 儲存絲瓜出貨紀錄
		for _, shipment := range store.GourdShipments {
			if err := saveShipment(tx, storeID, "產銷絲瓜", shipment); err != nil {
				log.Printf("儲存絲瓜出貨紀錄失敗: %v", err)
			}
		}

		log.Printf("[INFO] 已儲存 %s 的資料", store.StoreName)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Println("[INFO] 所有資料已成功儲存到資料庫")
	return nil
}

// saveShipment 儲存單筆出貨紀錄
func saveShipment(tx *sql.Tx, storeID int, productType string, shipment ShipmentInfo) error {
	date, err := parseShipmentDate(shipment.Date)
	if err != nil {
		log.Printf("跳過無效日期 %s: %v", shipment.Date, err)
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO shipments (store_id, product_type, shipment_date, quantity)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (store_id, product_type, shipment_date) 
		DO UPDATE SET quantity = EXCLUDED.quantity
	`, storeID, productType, date, shipment.Qty)

	return err
}

// parseShipmentDate 解析多種日期格式
func parseShipmentDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006/01/02",
		"2006-01-02",
		"01/02/2006",
		"2006/1/2",
		"1/2/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("無法解析日期: %s", dateStr)
}

// GetRecentShipments 查詢近 N 天有出貨的店家
func GetRecentShipments(db *sql.DB, days int) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			s.store_name,
			s.formatted_address,
			s.latitude,
			s.longitude,
			sh.product_type,
			sh.shipment_date,
			sh.quantity
		FROM stores s
		JOIN shipments sh ON s.id = sh.store_id
		WHERE sh.shipment_date >= CURRENT_DATE - INTERVAL '%d days'
		  AND sh.quantity IS NOT NULL 
		  AND sh.quantity != ''
		  AND sh.quantity != '0'
		ORDER BY s.store_name, sh.product_type, sh.shipment_date DESC
	`

	rows, err := db.Query(fmt.Sprintf(query, days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var storeName, address, productType, quantity string
		var lat, lng sql.NullFloat64
		var shipmentDate time.Time

		err := rows.Scan(&storeName, &address, &lat, &lng, &productType, &shipmentDate, &quantity)
		if err != nil {
			return nil, err
		}

		// 處理可能為 NULL 的座標值
		latitude := 0.0
		longitude := 0.0
		if lat.Valid {
			latitude = lat.Float64
		}
		if lng.Valid {
			longitude = lng.Float64
		}

		// 額外檢查:確保數量不為空且不為 0
		if quantity == "" || quantity == "0" {
			continue
		}

		results = append(results, map[string]interface{}{
			"store_name":    storeName,
			"address":       address,
			"latitude":      latitude,
			"longitude":     longitude,
			"product_type":  productType,
			"shipment_date": shipmentDate.Format("2006-01-02"),
			"quantity":      quantity,
		})
	}

	return results, nil
}
type ExistingStoreInfo struct {
	PlaceID          string
	FormattedAddress string
	Latitude         float64
	Longitude        float64
}
// ExistingStoreInfo 現有店家資訊
// GetExistingStoresWithLocation 取得已有地點資訊的店家
func GetExistingStoresWithLocation(db *sql.DB) (map[string]ExistingStoreInfo, error) {
	query := `
		SELECT store_name, place_id, formatted_address, latitude, longitude
		FROM stores
		WHERE place_id IS NOT NULL 
		  AND place_id != ''
		  AND latitude IS NOT NULL
		  AND longitude IS NOT NULL
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]ExistingStoreInfo)

	for rows.Next() {
		var storeName, placeID, address string
		var lat, lng float64

		if err := rows.Scan(&storeName, &placeID, &address, &lat, &lng); err != nil {
			continue
		}

		result[storeName] = ExistingStoreInfo{
			PlaceID:          placeID,
			FormattedAddress: address,
			Latitude:         lat,
			Longitude:        lng,
		}
	}

	return result, nil
}