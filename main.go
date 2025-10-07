package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"

	"PXMarkMapBackEnd/pkg/database"
	"PXMarkMapBackEnd/pkg/scheduler"
	"PXMarkMapBackEnd/pkg/sync"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil {
		if os.Getenv("GO_ENV") == "production" {
			log.Println("[INFO] Running in production mode, using platform environment variables")
		} else {
			log.Println("[INFO] No .env file found, using system environment variables")
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	db := connectDatabase()
	defer db.Close()

	switch command {
	case "sync":
		handleSync(db)
	case "serve":
		handleServe(db)
	case "schedule":
		handleSchedule(db)
	case "serve-schedule":
		handleServeWithSchedule(db)
	default:
		fmt.Printf("未知的命令: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func connectDatabase() *sql.DB {
	log.Println("=== 連接資料庫 ===")
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
		log.Fatalf("❌ 無法連接資料庫: %v", err)
	}
	return db
}

func handleSync(db *sql.DB) {
	log.Println("[INFO] 執行手動同步...")
	if err := sync.SyncData(db); err != nil {
		log.Fatalf("[ERROR] 同步失敗: %v", err)
	}
	log.Println("[INFO] 同步完成")
}

// --------------------- Gin 版本 ---------------------

func handleServe(db *sql.DB) {
	runGinServer(db, false)
}

func handleServeWithSchedule(db *sql.DB) {
	// 從環境變數讀取排程時間
	scheduleHour, _ := strconv.Atoi(getEnv("SCHEDULE_HOUR", "2"))
	scheduleMinute, _ := strconv.Atoi(getEnv("SCHEDULE_MINUTE", "0"))

	if scheduleHour < 0 || scheduleHour > 23 {
		log.Printf("[WARN] 無效的小時設定 %d，使用預設值 2", scheduleHour)
		scheduleHour = 2
	}
	if scheduleMinute < 0 || scheduleMinute > 59 {
		log.Printf("[WARN] 無效的分鐘設定 %d，使用預設值 0", scheduleMinute)
		scheduleMinute = 0
	}

	// 啟動排程器背景
	go func() {
		s := scheduler.NewScheduler(db, 0)
		s.StartDaily(scheduleHour, scheduleMinute)
	}()

	runGinServer(db, true)
}

func runGinServer(db *sql.DB, enableSync bool) {
	port := getEnv("API_PORT", "8080")
	// corsOrigins := getEnv("CORS_ORIGINS", "*")
	recentDays, _ := strconv.Atoi(getEnv("RECENT_DAYS", "3"))
	syncSecret := getEnv("SYNC_SECRET", "")

	r := gin.Default()

	// 靜態檔案服務，只掛 /static
	r.Static("/static", "./static")

	// 前端入口
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	// API 群組
	api := r.Group("/api")
	{
		api.GET("/shopeMap", func(c *gin.Context) {
			data, err := database.GetRecentShipments(db, recentDays)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			response := formatResponse(data)
			c.JSON(200, response)
		})

		if enableSync {
			api.POST("/triggerSync", func(c *gin.Context) {
				secret := c.GetHeader("X-Sync-Secret")
				if secret == "" {
					secret = c.Query("secret")
				}

				if secret != syncSecret {
					c.JSON(401, gin.H{"error": "Unauthorized"})
					return
				}

				go func() {
					if err := sync.SyncData(db); err != nil {
						log.Printf("[ERROR] 同步失敗: %v", err)
					} else {
						log.Println("[INFO] 手動同步完成")
					}
				}()

				c.JSON(202, gin.H{"status": "triggered", "message": "同步任務已觸發"})
			})
		}
	}

	log.Printf("[INFO] API 伺服器啟動於 http://localhost:%s", port)
	log.Printf("[INFO] 靜態檔案路徑: /static")
	log.Printf("[INFO] 店家地圖端點: http://localhost:%s/api/shopeMap", port)
	log.Printf("[INFO] 查詢近 %d 天的出貨資料", recentDays)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("[ERROR] Gin 伺服器啟動失敗: %v", err)
	}
}

// 格式化資料庫資料
func formatResponse(data []map[string]interface{}) []map[string]interface{} {
	storeMap := make(map[string]*map[string]interface{})

	for _, record := range data {
		storeName := record["store_name"].(string)
		if _, exists := storeMap[storeName]; !exists {
			storeMap[storeName] = &map[string]interface{}{
				"storeName": storeName,
				"address":   record["address"].(string),
				"latitude":  record["latitude"].(float64),
				"longitude": record["longitude"].(float64),
				"shipments": []map[string]string{},
			}
		}
		shipments := (*storeMap[storeName])["shipments"].([]map[string]string)
		shipments = append(shipments, map[string]string{
			"productType": record["product_type"].(string),
			"date":        record["shipment_date"].(string),
			"quantity":    record["quantity"].(string),
		})
		(*storeMap[storeName])["shipments"] = shipments
	}

	var response []map[string]interface{}
	for _, store := range storeMap {
		response = append(response, *store)
	}

	return response
}

// --------------------- 原本排程 ---------------------
func handleSchedule(db *sql.DB) {
	log.Println("[INFO] 啟動排程器模式")

	scheduleHour, _ := strconv.Atoi(getEnv("SCHEDULE_HOUR", "2"))
	scheduleMinute, _ := strconv.Atoi(getEnv("SCHEDULE_MINUTE", "0"))

	if scheduleHour < 0 || scheduleHour > 23 {
		log.Printf("[WARN] 無效的小時設定 %d，使用預設值 2", scheduleHour)
		scheduleHour = 2
	}
	if scheduleMinute < 0 || scheduleMinute > 59 {
		log.Printf("[WARN] 無效的分鐘設定 %d，使用預設值 0", scheduleMinute)
		scheduleMinute = 0
	}

	s := scheduler.NewScheduler(db, 0)
	s.StartDaily(scheduleHour, scheduleMinute)
}

// --------------------- 輔助 ---------------------
func printUsage() {
	fmt.Println(`
PXMarkMap Backend - 使用說明

命令:
  sync              立即執行一次資料同步
  serve             啟動 API 伺服器
  schedule          啟動排程器（每天自動同步）
  serve-schedule    啟動 API 伺服器 + 排程器

範例:
  go run main.go sync              # 手動同步資料
  go run main.go serve             # 啟動 API (http://localhost:8080)
  go run main.go schedule          # 啟動排程器
  go run main.go serve-schedule    # API + 排程一起跑

環境變數（.env）:
  GOOGLE_SHEET_ID          Google Sheets ID
  GOOGLE_SHEET_GIDS        GID 列表（逗號分隔）
  GOOGLE_SHEET_NAMES       Sheet 名稱（逗號分隔）
  GOOGLE_PLACES_API_KEY    Google Places API Key
  DB_HOST                  資料庫主機
  DB_PORT                  資料庫埠號
  DB_USER                  資料庫使用者
  DB_PASSWORD              資料庫密碼
  DB_NAME                  資料庫名稱
  API_PORT                 API 伺服器埠號（預設 8080）
  CORS_ORIGINS             CORS 允許的來源（預設 *，可設定如 http://localhost:3000）
  RECENT_DAYS              查詢近幾天的出貨資料（預設 3）
  SCHEDULE_HOUR            排程執行的小時（0-23，預設 2）
  SCHEDULE_MINUTE          排程執行的分鐘（0-59，預設 0）
  ENABLE_SYNC_API          是否啟用手動同步 API（true/false，預設 false）
  SYNC_SECRET              同步 API 的密鑰（啟用時必填）
`)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
