package scheduler

import (
	"database/sql"
	"log"
	"strings"
	"time"

	"PXMarkMapBackEnd/pkg/sync"
)

// Scheduler 排程器
type Scheduler struct {
	DB       *sql.DB
	Interval time.Duration
}

// SyncLog 同步執行記錄
type SyncLog struct {
	ID        int
	StartTime time.Time
	EndTime   sql.NullTime
	Status    string // 'running', 'success', 'failed'
	Message   string
}

// NewScheduler 建立新的排程器
func NewScheduler(db *sql.DB, interval time.Duration) *Scheduler {
	return &Scheduler{
		DB:       db,
		Interval: interval,
	}
}

// InitSyncLogTable 初始化同步記錄表
func (s *Scheduler) InitSyncLogTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS sync_logs (
			id SERIAL PRIMARY KEY,
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP,
			status VARCHAR(20) NOT NULL,
			message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_sync_logs_start_time ON sync_logs(start_time);
	`
	_, err := s.DB.Exec(query)
	if err != nil {
		return err
	}
	log.Println("[INFO] 同步記錄表已初始化")
	return nil
}

// Start 啟動排程器（每隔固定時間）
func (s *Scheduler) Start() {
	log.Printf("[INFO] 排程器啟動，每 %v 執行一次同步", s.Interval)

	// 初始化記錄表
	if err := s.InitSyncLogTable(); err != nil {
		log.Printf("[WARN] 無法建立記錄表: %v", err)
	}

	// 立即執行一次
	s.runSync()

	// 建立定時器
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runSync()
		}
	}
}

// StartDaily 每天固定時間執行
func (s *Scheduler) StartDaily(hour, minute int) {
	log.Printf("[INFO] 排程器啟動，每天 %02d:%02d 執行同步", hour, minute)

	// 初始化記錄表
	if err := s.InitSyncLogTable(); err != nil {
		log.Printf("[WARN] 無法建立記錄表: %v", err)
	}

	// 檢查上次執行時間
	lastRun, err := s.GetLastSyncTime()
	if err == nil && !lastRun.IsZero() {
		log.Printf("[INFO] 上次同步時間: %s", lastRun.Format("2006-01-02 15:04:05"))
	}

	// 計算下次執行時間
	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	
	// 如果今天的執行時間已過，設定為明天
	if nextRun.Before(now) {
		nextRun = nextRun.Add(24 * time.Hour)
	}

	log.Printf("[INFO] 下次執行時間: %s", nextRun.Format("2006-01-02 15:04:05"))
	log.Printf("[INFO] 等待時間: %v", time.Until(nextRun).Round(time.Second))

	// 等待到執行時間
	time.Sleep(time.Until(nextRun))

	// 執行第一次
	s.runSync()

	// 之後每 24 小時執行一次
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.runSync()
	}
}

// runSync 執行同步任務並記錄
func (s *Scheduler) runSync() {
	startTime := time.Now()
	log.Println("\n" + strings.Repeat("=", 50))
	log.Println("[INFO] 排程任務觸發")
	log.Printf("[INFO] 開始時間: %s", startTime.Format("2006-01-02 15:04:05"))

	// 記錄開始
	logID, err := s.LogSyncStart(startTime)
	if err != nil {
		log.Printf("[WARN] 無法記錄開始時間: %v", err)
	}

	// 執行同步
	syncErr := sync.SyncData(s.DB)
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// 記錄結束
	if syncErr != nil {
		log.Printf("[ERROR] 同步失敗: %v", syncErr)
		log.Printf("[INFO] 執行時間: %v", duration.Round(time.Second))
		s.LogSyncEnd(logID, endTime, "failed", syncErr.Error())
	} else {
		log.Println("[INFO] 排程同步完成")
		log.Printf("[INFO] 執行時間: %v", duration.Round(time.Second))
		s.LogSyncEnd(logID, endTime, "success", "同步成功")
	}

	log.Println(strings.Repeat("=", 50))
}

// LogSyncStart 記錄同步開始
func (s *Scheduler) LogSyncStart(startTime time.Time) (int, error) {
	var id int
	query := `
		INSERT INTO sync_logs (start_time, status, message)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	err := s.DB.QueryRow(query, startTime, "running", "同步開始").Scan(&id)
	return id, err
}

// LogSyncEnd 記錄同步結束
func (s *Scheduler) LogSyncEnd(id int, endTime time.Time, status, message string) error {
	query := `
		UPDATE sync_logs
		SET end_time = $1, status = $2, message = $3
		WHERE id = $4
	`
	_, err := s.DB.Exec(query, endTime, status, message, id)
	return err
}

// GetLastSyncTime 取得上次同步時間
func (s *Scheduler) GetLastSyncTime() (time.Time, error) {
	var lastSync time.Time
	query := `
		SELECT start_time
		FROM sync_logs
		WHERE status = 'success'
		ORDER BY start_time DESC
		LIMIT 1
	`
	err := s.DB.QueryRow(query).Scan(&lastSync)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return lastSync, err
}

// GetSyncHistory 取得同步歷史記錄
func (s *Scheduler) GetSyncHistory(limit int) ([]SyncLog, error) {
	query := `
		SELECT id, start_time, end_time, status, message
		FROM sync_logs
		ORDER BY start_time DESC
		LIMIT $1
	`
	rows, err := s.DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []SyncLog
	for rows.Next() {
		var log SyncLog
		err := rows.Scan(&log.ID, &log.StartTime, &log.EndTime, &log.Status, &log.Message)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}