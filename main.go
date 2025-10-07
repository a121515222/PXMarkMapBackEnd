package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

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
		log.Printf("未知命令: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// connectDatabase 連接資料庫
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

// handleSync 執行手動同步
func handleSync(db *sql.DB) {
	log.Println("[INFO] 執行手動同步...")
	if err := sync.SyncData(db); err != nil {
		log.Fatalf("[ERROR] 同步失敗: %v", err)
	}
	log.Println("[INFO] 同步完成")
}

// handleServe 啟動 Gin API
func handleServe(db *sql.DB) {
	runGinServer(db)
}

// handleSchedule 啟動排程器
func handleSchedule(db *sql.DB) {
	log.Println("[INFO] 啟動排程器模式")
	scheduleHour, _ := strconv.Atoi(getEnv("SCHEDULE_HOUR", "0"))
	scheduleMinute, _ := strconv.Atoi(getEnv("SCHEDULE_MINUTE", "0"))

	s := scheduler.NewScheduler(db, 0)
	s.StartDaily(scheduleHour, scheduleMinute)
}

// handleServeWithSchedule 同時啟動 API + 排程
func handleServeWithSchedule(db *sql.DB) {
	log.Println("[INFO] 啟動 API + 排程器模式")

	scheduleHour, _ := strconv.Atoi(getEnv("SCHEDULE_HOUR", "0"))
	scheduleMinute, _ := strconv.Atoi(getEnv("SCHEDULE_MINUTE", "0"))

	// 啟動排程器
	go func() {
		s := scheduler.NewScheduler(db, 0)
		s.StartDaily(scheduleHour, scheduleMinute)
	}()

	// 啟動 Gin API
	runGinServer(db)
}

// runGinServer Gin API 伺服器
func runGinServer(db *sql.DB) {
	port := getEnv("API_PORT", "8080")
	corsOrigins := getEnv("CORS_ORIGINS", "*")
	enableSync := getEnv("ENABLE_SYNC_API", "false") == "true"
	syncSecret := getEnv("SYNC_SECRET", "")

	if enableSync && syncSecret == "" {
		log.Fatal("[ERROR] 啟用同步 API 時必須設定 SYNC_SECRET")
	}

	router := gin.Default()

	// CORS Middleware
	router.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if corsOrigins == "*" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			allowed := false
			for _, o := range strings.Split(corsOrigins, ",") {
				if strings.TrimSpace(o) == origin {
					allowed = true
					break
				}
			}
			if allowed {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Vary", "Origin")
			}
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Sync-Secret")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}
		c.Next()
	})

	// 靜態 HTML
	router.Static("/static", "./static")
	router.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	// /api/shopeMap
	router.GET("/api/shopeMap", func(c *gin.Context) {
		recentDays, err := strconv.Atoi(getEnv("RECENT_DAYS", "5"))
		if err != nil {
		recentDays = 5 // 若轉換失敗，預設為 5
		}
		data, err := database.GetRecentShipments(db, recentDays)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, formatResponse(data))
	})

	// /api/triggerSync
	if enableSync {
		router.POST("/api/triggerSync", func(c *gin.Context) {
			secret := c.GetHeader("X-Sync-Secret")
			if secret == "" {
				secret = c.Query("secret")
			}
			if secret != syncSecret {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid secret"})
				return
			}
			go func() {
				if err := sync.SyncData(db); err != nil {
					log.Printf("[ERROR] 同步失敗: %v", err)
				} else {
					log.Println("[INFO] 手動同步完成")
				}
			}()
			c.JSON(http.StatusAccepted, gin.H{"status": "triggered", "message": "同步任務已觸發，正在背景執行"})
		})
	}

	log.Printf("[INFO] API 伺服器啟動於 http://localhost:%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("[ERROR] API 伺服器啟動失敗: %v", err)
	}
}

// formatResponse 將資料整理成前端需要格式
func formatResponse(data []map[string]interface{}) []map[string]interface{} {
	storeMap := make(map[string]map[string]interface{})
	for _, record := range data {
		name := record["store_name"].(string)
		if _, exists := storeMap[name]; !exists {
			storeMap[name] = map[string]interface{}{
				"storeName": name,
				"address":   record["address"].(string),
				"latitude":  record["latitude"].(float64),
				"longitude": record["longitude"].(float64),
				"shipments": []map[string]string{},
			}
		}
		store := storeMap[name]
		shipments := store["shipments"].([]map[string]string)
		shipments = append(shipments, map[string]string{
			"productType": record["product_type"].(string),
			"date":        record["shipment_date"].(string),
			"quantity":    record["quantity"].(string),
		})
		store["shipments"] = shipments
	}
	response := []map[string]interface{}{}
	for _, v := range storeMap {
		response = append(response, v)
	}
	return response
}

// 環境變數取得
func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

// 使用說明
func printUsage() {
	log.Println("PXMarkMap Backend - 使用說明")
	log.Println("命令:")
	log.Println("  sync             立即執行一次資料同步")
	log.Println("  serve            啟動 API 伺服器")
	log.Println("  schedule         啟動排程器")
	log.Println("  serve-schedule   啟動 API 伺服器 + 排程器")
	log.Println("範例:")
	log.Println("  go run main.go sync")
	log.Println("  go run main.go serve")
	log.Println("  go run main.go schedule")
	log.Println("  go run main.go serve-schedule")
}
