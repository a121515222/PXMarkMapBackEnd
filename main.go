// package main

// import (
// 	"database/sql"
// 	"fmt"
// 	"log"
// 	"os"
// 	"strconv"

// 	"PXMarkMapBackEnd/pkg/database"
// 	"PXMarkMapBackEnd/pkg/scheduler"
// 	"PXMarkMapBackEnd/pkg/server"
// 	"PXMarkMapBackEnd/pkg/sync"
	
// 	"github.com/joho/godotenv"
// )

// func init() {
// 	// Load .env file only in development
// 	// In production, environment variables are set by the platform
// 	if err := godotenv.Load(); err != nil {
// 		// Check if we're in production
// 		if os.Getenv("GO_ENV") == "production" {
// 			log.Println("[INFO] Running in production mode, using platform environment variables")
// 		} else {
// 			log.Println("[INFO] No .env file found, using system environment variables")
// 		}
// 	}
// }

// func main() {
// 	// Get command parameter
// 	if len(os.Args) < 2 {
// 		printUsage()
// 		os.Exit(1)
// 	}

// 	command := os.Args[1]

// 	// Connect to database
// 	db := connectDatabase()
// 	defer db.Close()

// 	// Execute command
// 	switch command {
// 	case "sync":
// 		handleSync(db)

// 	case "serve":
// 		handleServe(db)

// 	case "schedule":
// 		handleSchedule(db)

// 	case "serve-schedule":
// 		handleServeWithSchedule(db)

// 	default:
// 		fmt.Printf("未知的命令: %s\n", command)
// 		printUsage()
// 		os.Exit(1)
// 	}
// }

// // connectDatabase 連接資料庫
// func connectDatabase() *sql.DB {
// 	log.Println("=== 連接資料庫 ===")
// 	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
// 	dbConfig := database.DBConfig{
// 		Host:     getEnv("DB_HOST", "localhost"),
// 		Port:     dbPort,
// 		User:     getEnv("DB_USER", "postgres"),
// 		Password: getEnv("DB_PASSWORD", ""),
// 		DBName:   getEnv("DB_NAME", "px_mark_map_db"),
// 	}

// 	db, err := database.ConnectDB(dbConfig)
// 	if err != nil {
// 		log.Fatalf("❌ 無法連接資料庫: %v", err)
// 	}

// 	return db
// }

// // handleSync 處理手動同步
// func handleSync(db *sql.DB) {
// 	log.Println("[INFO] 執行手動同步...")
	
// 	if err := sync.SyncData(db); err != nil {
// 		log.Fatalf("[ERROR] 同步失敗: %v", err)
// 	}

// 	log.Println("[INFO] 同步完成")
// }

// // handleServe 處理 API 伺服器
// func handleServe(db *sql.DB) {
// 	port := getEnv("API_PORT", "8080")
// 	corsOrigins := getEnv("CORS_ORIGINS", "*")
// 	recentDays, _ := strconv.Atoi(getEnv("RECENT_DAYS", "3"))
	
// 	// 同步端點設定
// 	enableSync := getEnv("ENABLE_SYNC_API", "false") == "true"
// 	syncSecret := getEnv("SYNC_SECRET", "")
	
// 	if enableSync && syncSecret == "" {
// 		log.Fatal("[ERROR] 啟用同步 API 時必須設定 SYNC_SECRET")
// 	}
	
// 	srv := server.NewServer(db, port, corsOrigins, recentDays, enableSync, syncSecret)

// 	log.Println("[INFO] 啟動 API 伺服器模式")
// 	if err := srv.Start(); err != nil {
// 		log.Fatalf("[ERROR] API 伺服器啟動失敗: %v", err)
// 	}
// }

// // handleSchedule 處理排程器
// func handleSchedule(db *sql.DB) {
// 	log.Println("[INFO] 啟動排程器模式")

// 	// 從環境變數讀取排程時間
// 	scheduleHour, _ := strconv.Atoi(getEnv("SCHEDULE_HOUR", "2"))
// 	scheduleMinute, _ := strconv.Atoi(getEnv("SCHEDULE_MINUTE", "0"))

// 	// 驗證時間範圍
// 	if scheduleHour < 0 || scheduleHour > 23 {
// 		log.Printf("[WARN] 無效的小時設定 %d，使用預設值 2", scheduleHour)
// 		scheduleHour = 2
// 	}
// 	if scheduleMinute < 0 || scheduleMinute > 59 {
// 		log.Printf("[WARN] 無效的分鐘設定 %d，使用預設值 0", scheduleMinute)
// 		scheduleMinute = 0
// 	}

// 	// 每天固定時間執行
// 	s := scheduler.NewScheduler(db, 0)
// 	s.StartDaily(scheduleHour, scheduleMinute)

// }

// // handleServeWithSchedule API and scheduler together
// func handleServeWithSchedule(db *sql.DB) {
// 	log.Println("[INFO] 啟動 API 伺服器 + 排程器模式")

// 	// 從環境變數讀取排程時間
// 	scheduleHour, _ := strconv.Atoi(getEnv("SCHEDULE_HOUR", "2"))
// 	scheduleMinute, _ := strconv.Atoi(getEnv("SCHEDULE_MINUTE", "0"))

// 	// 驗證時間範圍
// 	if scheduleHour < 0 || scheduleHour > 23 {
// 		log.Printf("[WARN] 無效的小時設定 %d，使用預設值 2", scheduleHour)
// 		scheduleHour = 2
// 	}
// 	if scheduleMinute < 0 || scheduleMinute > 59 {
// 		log.Printf("[WARN] 無效的分鐘設定 %d，使用預設值 0", scheduleMinute)
// 		scheduleMinute = 0
// 	}

// 	// Start scheduler in background
// 	go func() {
// 		s := scheduler.NewScheduler(db, 0)
// 		s.StartDaily(scheduleHour, scheduleMinute)
// 	}()

// 	// Start API server in main thread
// 	port := getEnv("API_PORT", "8080")
// 	corsOrigins := getEnv("CORS_ORIGINS", "*")
// 	recentDays, _ := strconv.Atoi(getEnv("RECENT_DAYS", "3"))
	
// 	// Sync API settings
// 	enableSync := getEnv("ENABLE_SYNC_API", "false") == "true"
// 	syncSecret := getEnv("SYNC_SECRET", "")
	
// 	if enableSync && syncSecret == "" {
// 		log.Fatal("[ERROR] 啟用同步 API 時必須設定 SYNC_SECRET")
// 	}
	
// 	srv := server.NewServer(db, port, corsOrigins, recentDays, enableSync, syncSecret)
	
// 	if err := srv.Start(); err != nil {
// 		log.Fatalf("[ERROR] API 伺服器啟動失敗: %v", err)
// 	}
// }

// // printUsage 印出使用說明
// func printUsage() {
// 	fmt.Println(`
// PXMarkMap Backend - 使用說明

// 命令:
//   sync              立即執行一次資料同步
//   serve             啟動 API 伺服器
//   schedule          啟動排程器（每天自動同步）
//   serve-schedule    啟動 API 伺服器 + 排程器

// 範例:
//   go run main.go sync              # 手動同步資料
//   go run main.go serve             # 啟動 API (http://localhost:8080)
//   go run main.go schedule          # 啟動排程器
//   go run main.go serve-schedule    # API + 排程一起跑

// 環境變數（.env）:
//   GOOGLE_SHEET_ID          Google Sheets ID
//   GOOGLE_SHEET_GIDS        GID 列表（逗號分隔）
//   GOOGLE_SHEET_NAMES       Sheet 名稱（逗號分隔）
//   GOOGLE_PLACES_API_KEY    Google Places API Key
//   DB_HOST                  資料庫主機
//   DB_PORT                  資料庫埠號
//   DB_USER                  資料庫使用者
//   DB_PASSWORD              資料庫密碼
//   DB_NAME                  資料庫名稱
//   API_PORT                 API 伺服器埠號（預設 8080）
//   CORS_ORIGINS             CORS 允許的來源（預設 *，可設定如 http://localhost:3000）
//   RECENT_DAYS              查詢近幾天的出貨資料（預設 3）
//   SCHEDULE_HOUR            排程執行的小時（0-23，預設 2）
//   SCHEDULE_MINUTE          排程執行的分鐘（0-59，預設 0）
//   ENABLE_SYNC_API          是否啟用手動同步 API（true/false，預設 false）
//   SYNC_SECRET              同步 API 的密鑰（啟用時必填）
// 	`)
// }

// // getEnv 取得環境變數，如果不存在則使用預設值
// func getEnv(key, defaultValue string) string {
// 	if value := os.Getenv(key); value != "" {
// 		return value
// 	}
// 	return defaultValue
// }

package main

import (
	"database/sql"
	"log"
	"os"
	"strconv"

	"PXMarkMapBackEnd/pkg/database"
	"PXMarkMapBackEnd/pkg/scheduler"
	"PXMarkMapBackEnd/pkg/server"
	"PXMarkMapBackEnd/pkg/sync"

	"github.com/joho/godotenv"
	"github.com/gin-gonic/gin"
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
		log.Fatalf("未知的命令: %s", command)
	}
}

func connectDatabase() *sql.DB {
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

func handleServe(db *sql.DB) {
	srv := createGinServer(db)
	log.Println("[INFO] 啟動 API 伺服器模式")
	if err := srv.Run(":" + getEnv("API_PORT", "8080")); err != nil {
		log.Fatalf("[ERROR] API 伺服器啟動失敗: %v", err)
	}
}

func handleServeWithSchedule(db *sql.DB) {
	// 啟動排程
	go func() {
		scheduleHour, _ := strconv.Atoi(getEnv("SCHEDULE_HOUR", "2"))
		scheduleMinute, _ := strconv.Atoi(getEnv("SCHEDULE_MINUTE", "0"))
		s := scheduler.NewScheduler(db, 0)
		s.StartDaily(scheduleHour, scheduleMinute)
	}()

	srv := createGinServer(db)
	log.Println("[INFO] 啟動 API + 排程模式")
	if err := srv.Run(":" + getEnv("API_PORT", "8080")); err != nil {
		log.Fatalf("[ERROR] API 伺服器啟動失敗: %v", err)
	}
}

// 建立 Gin Server，提供 static + API
func createGinServer(db *sql.DB) *gin.Engine {
	r := gin.Default()

	// 提供 static 資料夾
	r.Static("/", "./static") // Docker WORKDIR /app + COPY static ./static

	// API: /api/shopeMap
	r.GET("/api/shopeMap", func(c *gin.Context) {
		// 這裡可以改成原本 server.NewServer 的邏輯
		data, err := server.GetShopMap(db)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, data)
	})

	return r
}

// printUsage 同原本
func printUsage() {
	log.Println(`
PXMarkMap Backend - 使用說明
命令:
  sync              立即執行一次資料同步
  serve             啟動 API 伺服器
  schedule          啟動排程器（每天自動同步）
  serve-schedule    啟動 API 伺服器 + 排程器
`)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
