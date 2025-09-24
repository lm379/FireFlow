package core

import (
	"fmt"
	"log"

	"github.com/robfig/cron/v3"
)

// CronManager 管理定时任务
type CronManager struct {
	cron          *cron.Cron
	firewallJobID cron.EntryID
	updateFunc    func()
	isRunning     bool
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
	cm.updateFunc = updateFunc
}

// StartFirewallUpdateJob 根据配置启动防火墙更新任务
func (cm *CronManager) StartFirewallUpdateJob(intervalMinutes int) error {
	if cm.updateFunc == nil {
		return fmt.Errorf("update function not set")
	}

	// 如果已经有任务在运行，先停止
	if cm.firewallJobID != 0 {
		cm.cron.Remove(cm.firewallJobID)
		cm.firewallJobID = 0
	}

	// 创建cron表达式：每N分钟执行一次
	cronExpr := fmt.Sprintf("0 */%d * * * *", intervalMinutes)

	// 添加新任务
	jobID, err := cm.cron.AddFunc(cronExpr, cm.updateFunc)
	if err != nil {
		return err
	}

	cm.firewallJobID = jobID
	cm.isRunning = true
	log.Printf("Firewall update job scheduled with expression: %s (every %d minutes)", cronExpr, intervalMinutes)
	return nil
}

// StopFirewallUpdateJob 停止防火墙更新任务
func (cm *CronManager) StopFirewallUpdateJob() {
	if cm.firewallJobID != 0 {
		cm.cron.Remove(cm.firewallJobID)
		cm.firewallJobID = 0
		cm.isRunning = false
		log.Println("Firewall update job stopped")
	}
}

// IsRunning 检查防火墙更新任务是否正在运行
func (cm *CronManager) IsRunning() bool {
	return cm.isRunning
}

// Start 启动定时任务
func (cm *CronManager) Start() {
	cm.cron.Start()
	log.Println("Cron manager started")
}

// Stop 停止定时任务
func (cm *CronManager) Stop() {
	cm.cron.Stop()
	log.Println("Cron manager stopped")
}
