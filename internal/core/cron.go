package core

import (
	"FireFlow/internal/logger"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// CronManager 管理定时任务
type CronManager struct {
	cron            *cron.Cron
	firewallJobID   cron.EntryID
	mutex           sync.RWMutex // 保护并发访问
	updateFunc      func()
	isRunning       bool
	intervalMinutes int // 存储当前的时间间隔
}

// NewCronManager 创建新的定时任务管理器
func NewCronManager() *CronManager {
	return &CronManager{
		cron:          cron.New(cron.WithSeconds()), // 支持包含秒的6字段格式
		firewallJobID: 0,
		isRunning:     false,
	}
}

// SetUpdateFunc 设置更新函数
func (cm *CronManager) SetUpdateFunc(updateFunc func()) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.updateFunc = updateFunc
}

// ValidateInterval rejects invalid intervals before replacing an existing job.
func ValidateInterval(intervalMinutes int) error {
	if intervalMinutes <= 0 || int64(intervalMinutes) > int64((1<<63-1)/time.Minute) {
		return fmt.Errorf("interval must be a positive number of minutes within the supported duration")
	}
	return nil
}

// StartFirewallUpdateJob 启动防火墙更新任务
func (cm *CronManager) StartFirewallUpdateJob(intervalMinutes int) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.updateFunc == nil {
		return fmt.Errorf("update function not set")
	}
	if err := ValidateInterval(intervalMinutes); err != nil {
		return err
	}

	// 如果已经有任务在运行，先停止
	if cm.firewallJobID != 0 {
		cm.cron.Remove(cm.firewallJobID)
		cm.firewallJobID = 0
	}

	// Use a duration so intervals remain consistent across hour boundaries.
	jobID := cm.cron.Schedule(cron.Every(time.Duration(intervalMinutes)*time.Minute), cron.FuncJob(cm.updateFunc))

	cm.firewallJobID = jobID
	cm.isRunning = true
	cm.intervalMinutes = intervalMinutes // 保存间隔时间
	return nil
}

// StopFirewallUpdateJob 停止防火墙更新任务
func (cm *CronManager) StopFirewallUpdateJob() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.firewallJobID != 0 {
		cm.cron.Remove(cm.firewallJobID)
		cm.firewallJobID = 0
		cm.isRunning = false
		cm.intervalMinutes = 0 // 重置间隔时间
		logger.Println("Firewall update job stopped")
	}
}

// ExecuteNow 立即执行一次更新任务
func (cm *CronManager) ExecuteNow() error {
	cm.mutex.RLock()
	updateFunc := cm.updateFunc
	interval := cm.intervalMinutes
	cm.mutex.RUnlock()
	if updateFunc == nil {
		return fmt.Errorf("update function not set")
	}

	if interval > 0 {
		logger.Printf("Executing firewall update job immediately... (scheduled interval: %d minutes)", interval)
	} else {
		logger.Println("Executing firewall update job immediately...")
	}

	go updateFunc() // 异步执行，避免阻塞
	return nil
}

// IsRunning 检查防火墙更新任务是否正在运行
func (cm *CronManager) IsRunning() bool {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.isRunning
}

// Start 启动定时任务引擎
func (cm *CronManager) Start() {
	cm.cron.Start()
	// logger.Println("Cron manager started")
}

// Stop 停止定时任务引擎
func (cm *CronManager) Stop() {
	cm.StopFirewallUpdateJob()
	cm.cron.Stop()
	logger.Println("Cron manager stopped")
}
