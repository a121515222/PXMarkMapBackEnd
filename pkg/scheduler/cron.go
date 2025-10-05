package scheduler

import (
	"database/sql"
	"log"
	"time"

	"PXMarkMapBackEnd/pkg/sync"
)

// Scheduler 排程器
type Scheduler struct {
	DB       *sql.DB
	Interval time.Duration
}

// NewScheduler 建立新的排程器
func NewScheduler(db *sql.DB, interval time.Duration) *Scheduler {
	return &Scheduler{
		DB:       db,
		Interval: interval,
	}
}

// Start 啟動排程器
func (s *Scheduler) Start() {
	log.Printf("⏰ 排程器啟動，每 %v 執行一次同步", s.Interval)

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

// runSync 執行同步任務
func (s *Scheduler) runSync() {
	log.Println("\n 排程任務觸發")
	log.Printf("執行時間: %s", time.Now().Format("2006-01-02 15:04:05"))

	if err := sync.SyncData(s.DB); err != nil {
		log.Printf("同步失敗: %v", err)
	} else {
		log.Println("✓ 排程同步完成")
	}
}

// StartDaily 每天固定時間執行（例如：每天凌晨 2:00）
func (s *Scheduler) StartDaily(hour, minute int) {
	log.Printf("排程器啟動，每天 %02d:%02d 執行同步", hour, minute)

	// 計算下次執行時間
	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	
	// 如果今天的執行時間已過，設定為明天
	if nextRun.Before(now) {
		nextRun = nextRun.Add(24 * time.Hour)
	}

	log.Printf("下次執行時間: %s", nextRun.Format("2006-01-02 15:04:05"))

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