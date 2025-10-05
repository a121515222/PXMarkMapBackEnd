package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"

	"PXMarkMapBackEnd/pkg/database"
	"PXMarkMapBackEnd/pkg/scheduler"
	"PXMarkMapBackEnd/pkg/server"
	"PXMarkMapBackEnd/pkg/sync"
)

func main() {
	// 取得命令參數
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// 連接資料庫
	db := connectDatabase()
	defer db.Close()

	// 根據命令執行不同功能
	switch command {
	case "sync":
		// 手動執行同步
		handleSync(db)

	case "serve":
		// 啟動 API 伺服器
		handleServe(db)

	case "schedule":
		// 啟動排程器
		handleSchedule(db)

	case "serve-schedule":
		// API + 排程一起執行
		handleServeWithSchedule(db)

	default:
		fmt.Printf("未知的命令: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// connectDatabase 連接資料庫
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
		log.Fatalf("無法連接資料庫: %v", err)
	}

	return db
}

// handleSync 處理手動同步
func handleSync(db *sql.DB) {
	log.Println("執行手動同步...")
	
	if err := sync.SyncData(db); err != nil {
		log.Fatalf("同步失敗: %v", err)
	}

	log.Println("同步完成！")
}

// handleServe 處理 API 伺服器
func handleServe(db *sql.DB) {
	port := getEnv("API_PORT", "8080")
	corsOrigins := getEnv("CORS_ORIGINS", "*")
	recentDays, _ := strconv.Atoi(getEnv("RECENT_DAYS", "3"))
	
	srv := server.NewServer(db, port, corsOrigins, recentDays)

	log.Println("啟動 API 伺服器模式")
	if err := srv.Start(); err != nil {
		log.Fatalf("API 伺服器啟動失敗: %v", err)
	}
}

// handleSchedule 處理排程器
func handleSchedule(db *sql.DB) {
	log.Println("啟動排程器模式")

	// 方式 1: 每隔固定時間執行（例如每 1 小時）
	// interval := 1 * time.Hour
	// s := scheduler.NewScheduler(db, interval)
	// s.Start()

	// 方式 2: 每天固定時間執行（例如每天凌晨 2:00）
	s := scheduler.NewScheduler(db, 0)
	s.StartDaily(2, 0) // 每天 02:00 執行
}

// handleServeWithSchedule API + 排程一起執行
func handleServeWithSchedule(db *sql.DB) {
	log.Println("啟動 API 伺服器 + 排程器模式")

	// 啟動排程器（在背景執行）
	go func() {
		s := scheduler.NewScheduler(db, 0)
		s.StartDaily(2, 0) // 每天 02:00 執行
	}()

	// 啟動 API 伺服器（主執行緒）
	port := getEnv("API_PORT", "8080")
	corsOrigins := getEnv("CORS_ORIGINS", "*")
	recentDays, _ := strconv.Atoi(getEnv("RECENT_DAYS", "3"))
	srv := server.NewServer(db, port, corsOrigins, recentDays)
	
	if err := srv.Start(); err != nil {
		log.Fatalf("API 伺服器啟動失敗: %v", err)
	}
}

// printUsage 印出使用說明
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
	`)
}

// getEnv 取得環境變數，如果不存在則使用預設值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}