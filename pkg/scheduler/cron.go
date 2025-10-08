package scheduler

import (
	"database/sql"
	"fmt"
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
	s.runSync(false)

	// 建立定時器
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runSync(false)
		}
	}
}

// StartDaily 每天固定時間執行（每日更新）
func (s *Scheduler) StartDaily(hour, minute int, isFullSync bool) {
	syncType := "每日更新"
	if isFullSync {
		syncType = "完整同步"
	}

	log.Printf("[INFO] 排程器啟動,每天 %02d:%02d 執行%s", hour, minute, syncType)

	// 初始化記錄表
	if err := s.InitSyncLogTable(); err != nil {
		log.Printf("[WARN] 無法建立記錄表: %v", err)
	}

	// 檢查上次執行時間
	lastRun, err := s.GetLastSyncTime()
	if err == nil && !lastRun.IsZero() {
		log.Printf("[INFO] 上次同步時間: %s", lastRun.Format("2006-01-02 15:04:05"))
	}

	for {
		now := time.Now()
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

		// 如果今天的執行時間已過,設定為明天
		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		waitDuration := time.Until(nextRun)
		log.Printf("[INFO] 下次執行時間: %s", nextRun.Format("2006-01-02 15:04:05"))
		log.Printf("[INFO] 等待時間: %v", waitDuration.Round(time.Second))

		// 等待到指定時間
		time.Sleep(waitDuration)

		// 執行同步
		s.runSync(isFullSync)
	}
}

// StartMonthly 每月固定日期執行（完整同步）
func (s *Scheduler) StartMonthly(dayOfMonth, hour, minute int) {
	log.Printf("[INFO] 排程器啟動，每月 %d 號 %02d:%02d 執行完整同步", dayOfMonth, hour, minute)

	// 初始化記錄表
	if err := s.InitSyncLogTable(); err != nil {
		log.Printf("[WARN] 無法建立記錄表: %v", err)
	}

	for {
		now := time.Now()

		// 計算下次執行時間
		nextRun := time.Date(now.Year(), now.Month(), dayOfMonth, hour, minute, 0, 0, now.Location())

		// 如果本月的執行時間已過，移到下個月
		if now.After(nextRun) {
			nextRun = nextRun.AddDate(0, 1, 0)
		}

		waitDuration := time.Until(nextRun)
		log.Printf("[INFO] 下次完整同步時間: %s", nextRun.Format("2006-01-02 15:04:05"))
		log.Printf("[INFO] 等待時間: %v", waitDuration.Round(time.Hour))

		time.Sleep(waitDuration)

		// 執行完整同步
		s.runSync(true)
	}
}

// runSync 執行同步任務（根據 isFullSync 決定類型）
func (s *Scheduler) runSync(isFullSync bool) {
	startTime := time.Now()

	syncType := "每日"
	if isFullSync {
		syncType = "完整"
	}

	log.Println("\n" + strings.Repeat("=", 50))
	log.Printf("[INFO] %s同步任務觸發", syncType)
	log.Printf("[INFO] 開始時間: %s", startTime.Format("2006-01-02 15:04:05"))

	// 記錄開始
	logID, err := s.LogSyncStart(startTime)
	if err != nil {
		log.Printf("[WARN] 無法記錄開始時間: %v", err)
	}

	// 執行同步（根據類型）
	var syncErr error
	if isFullSync {
		syncErr = sync.SyncData(s.DB) // 完整同步
	} else {
		syncErr = sync.SyncDataDaily(s.DB) // 每日同步
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// 記錄結束
	if syncErr != nil {
		log.Printf("[ERROR] 同步失敗: %v", syncErr)
		log.Printf("[INFO] 執行時間: %v", duration.Round(time.Second))
		s.LogSyncEnd(logID, endTime, "failed", syncErr.Error())
	} else {
		log.Printf("[INFO] %s同步完成", syncType)
		log.Printf("[INFO] 執行時間: %v", duration.Round(time.Second))
		s.LogSyncEnd(logID, endTime, "success", fmt.Sprintf("%s同步成功", syncType))
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