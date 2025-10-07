package server

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"PXMarkMapBackEnd/pkg/database"
	"PXMarkMapBackEnd/pkg/sync"
)

// StoreMapResponse API 回應結構
type StoreMapResponse struct {
	StoreName string              `json:"storeName"`
	Address   string              `json:"address"`
	Latitude  float64             `json:"latitude"`
	Longitude float64             `json:"longitude"`
	Shipments []ShipmentResponse  `json:"shipments"`
}

// ShipmentResponse 出貨資料結構
type ShipmentResponse struct {
	ProductType string `json:"productType"`
	Date        string `json:"date"`
	Quantity    string `json:"quantity"`
}

// Server API 伺服器
type Server struct {
	DB              *sql.DB
	Port            string
	CORSOrigins     []string // CORS 允許的來源列表
	AllowAllOrigins bool     // 是否允許所有來源
	RecentDays      int      // 查詢近幾天的資料
	EnableSync      bool     // 是否啟用手動同步端點
	SyncSecret      string   // 同步端點的密鑰
}

// NewServer 建立新的 API 伺服器
func NewServer(db *sql.DB, port string, corsOrigins string, recentDays int, enableSync bool, syncSecret string) *Server {
	server := &Server{
		DB:         db,
		Port:       port,
		RecentDays: recentDays,
		EnableSync: enableSync,
		SyncSecret: syncSecret,
	}

	// 解析 CORS 設定
	if corsOrigins == "" || corsOrigins == "*" {
		server.AllowAllOrigins = true
		server.CORSOrigins = []string{"*"}
	} else {
		// 用逗號分割多個來源
		origins := strings.Split(corsOrigins, ",")
		for i, origin := range origins {
			origins[i] = strings.TrimSpace(origin)
		}
		server.CORSOrigins = origins
		server.AllowAllOrigins = false
	}

	return server
}

// Start 啟動 API 伺服器
// func (s *Server) Start() error {
// 	http.HandleFunc("/api/shopeMap", s.handleShopeMap)
	
// 	// 只有啟用時才註冊同步端點
// 	if s.EnableSync {
// 		http.HandleFunc("/api/triggerSync", s.handleTriggerSync)
// 		log.Printf("[INFO] 手動同步端點: http://localhost:%s/api/triggerSync", s.Port)
// 		log.Printf("[INFO] 同步端點已啟用（需要密鑰驗證）")
// 	} else {
// 		log.Printf("[WARN] 手動同步端點已停用")
// 	}

// 	log.Printf("[INFO] API 伺服器啟動於 http://localhost:%s", s.Port)
// 	log.Printf("[INFO] 店家地圖端點: http://localhost:%s/api/shopeMap", s.Port)
// 	log.Printf("[INFO] 查詢近 %d 天的出貨資料", s.RecentDays)
	
// 	if s.AllowAllOrigins {
// 		log.Printf("[INFO] CORS 設定: 允許所有來源 (*)")
// 	} else {
// 		log.Printf("[INFO] CORS 設定: %v", s.CORSOrigins)
// 	}
	
// 	return http.ListenAndServe(":"+s.Port, nil)
// }
func (s *Server) Start() error {
	// API 路由
	http.HandleFunc("/api/shopeMap", s.handleShopeMap)

	if s.EnableSync {
		http.HandleFunc("/api/triggerSync", s.handleTriggerSync)
		log.Printf("[INFO] 手動同步端點: http://localhost:%s/api/triggerSync", s.Port)
		log.Printf("[INFO] 同步端點已啟用（需要密鑰驗證）")
	} else {
		log.Printf("[WARN] 手動同步端點已停用")
	}

	// 靜態檔案服務
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	log.Printf("[INFO] API 伺服器啟動於 http://localhost:%s", s.Port)
	log.Printf("[INFO] 店家地圖端點: http://localhost:%s/api/shopeMap", s.Port)
	log.Printf("[INFO] 查詢近 %d 天的出貨資料", s.RecentDays)

	if s.AllowAllOrigins {
		log.Printf("[INFO] CORS 設定: 允許所有來源 (*)")
	} else {
		log.Printf("[INFO] CORS 設定: %v", s.CORSOrigins)
	}

	return http.ListenAndServe(":"+s.Port, nil)
}
// handleShopeMap 處理店家地圖請求
func (s *Server) handleShopeMap(w http.ResponseWriter, r *http.Request) {
	// 設定 CORS
	s.setCORSHeaders(w, r)

	// 處理 OPTIONS 請求（CORS preflight）
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 只接受 GET 請求
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 從資料庫查詢近 N 天的出貨資料
	data, err := database.GetRecentShipments(s.DB, s.RecentDays)
	if err != nil {
		log.Printf("[ERROR] 查詢資料失敗: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 整理成前端需要的格式
	response := s.formatResponse(data)

	// 回傳 JSON
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[ERROR] 編碼 JSON 失敗: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] 回傳 %d 個店家的資料", len(response))
}

// setCORSHeaders 設定 CORS 標頭
func (s *Server) setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")

	if s.AllowAllOrigins {
		// 允許所有來源
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		// 檢查來源是否在允許列表中
		allowed := false
		for _, allowedOrigin := range s.CORSOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin") // 重要：告訴瀏覽器快取要考慮 Origin
		}
	}

	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
}

// formatResponse 將資料庫查詢結果格式化為 API 回應
func (s *Server) formatResponse(data []map[string]interface{}) []StoreMapResponse {
	// 按店家分組
	storeMap := make(map[string]*StoreMapResponse)

	for _, record := range data {
		storeName := record["store_name"].(string)

		// 如果店家還沒建立，初始化
		if _, exists := storeMap[storeName]; !exists {
			storeMap[storeName] = &StoreMapResponse{
				StoreName: storeName,
				Address:   record["address"].(string),
				Latitude:  record["latitude"].(float64),
				Longitude: record["longitude"].(float64),
				Shipments: []ShipmentResponse{},
			}
		}

		// 加入出貨紀錄
		storeMap[storeName].Shipments = append(storeMap[storeName].Shipments, ShipmentResponse{
			ProductType: record["product_type"].(string),
			Date:        record["shipment_date"].(string),
			Quantity:    record["quantity"].(string),
		})
	}

	// 轉換成陣列
	var response []StoreMapResponse
	for _, store := range storeMap {
		response = append(response, *store)
	}

	return response
}

// handleTriggerSync 處理手動觸發同步（需要密鑰驗證）
func (s *Server) handleTriggerSync(w http.ResponseWriter, r *http.Request) {
	// 設定 CORS
	s.setCORSHeaders(w, r)

	// 處理 OPTIONS 請求
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 只接受 POST 請求
	if r.Method != "POST" {
		http.Error(w, "Method not allowed. Use POST.", http.StatusMethodNotAllowed)
		return
	}

	// 驗證密鑰
	secret := r.Header.Get("X-Sync-Secret")
	if secret == "" {
		// 也支援從 query parameter 讀取（方便測試）
		secret = r.URL.Query().Get("secret")
	}

	if secret != s.SyncSecret {
		log.Printf("[WARN] 同步請求被拒絕：密鑰錯誤")
		http.Error(w, "Unauthorized: Invalid secret", http.StatusUnauthorized)
		return
	}

	log.Println("[INFO] 收到手動同步請求（密鑰驗證通過）")

	// 在背景執行同步（避免阻塞 API）
	go func() {
		if err := sync.SyncData(s.DB); err != nil {
			log.Printf("[ERROR] 同步失敗: %v", err)
		} else {
			log.Println("[INFO] 手動同步完成")
		}
	}()

	// 立即回應
	response := map[string]string{
		"status":  "triggered",
		"message": "同步任務已觸發，正在背景執行",
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
	log.Println("[INFO] 已回應同步請求，同步任務在背景執行中")
}