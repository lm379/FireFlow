package core

import (
	"fmt"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

// CronManager 管理定时任务
type CronManager struct {
	cron          *cron.Cron
	firewallJobID cron.EntryID
	ruleJobs      map[uint]cron.EntryID // 存储每个规则的任务ID
	mutex         sync.RWMutex          // 保护并发访问
	updateFunc    func()
	isRunning     bool
}

// NewCronManager 创建新的定时任务管理器
func NewCronManager() *CronManager {
	return &CronManager{
		cron:          cron.New(cron.WithSeconds()), // 支持包含秒的6字段格式
		firewallJobID: 0,
		ruleJobs:      make(map[uint]cron.EntryID),
		isRunning:     false,
	}
}

// SetUpdateFunc 设置更新函数
func (cm *CronManager) SetUpdateFunc(updateFunc func()) {
	cm.updateFunc = updateFunc
}

// StartRuleUpdateJob 为特定规则启动定时更新任务
func (cm *CronManager) StartRuleUpdateJob(ruleID uint, intervalMinutes int, updateFunc func()) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// 如果该规则已经有任务在运行，先停止
	if jobID, exists := cm.ruleJobs[ruleID]; exists {
		cm.cron.Remove(jobID)
		delete(cm.ruleJobs, ruleID)
	}

	// 创建cron表达式：每N分钟执行一次
	cronExpr := fmt.Sprintf("0 */%d * * * *", intervalMinutes)

	// 添加新任务
	jobID, err := cm.cron.AddFunc(cronExpr, updateFunc)
	if err != nil {
		return err
	}

	cm.ruleJobs[ruleID] = jobID
	log.Printf("Rule %d update job scheduled with expression: %s (every %d minutes)", ruleID, cronExpr, intervalMinutes)
	return nil
}

// StopRuleUpdateJob 停止特定规则的定时更新任务
func (cm *CronManager) StopRuleUpdateJob(ruleID uint) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if jobID, exists := cm.ruleJobs[ruleID]; exists {
		cm.cron.Remove(jobID)
		delete(cm.ruleJobs, ruleID)
		log.Printf("Rule %d update job stopped", ruleID)
	}
}

// StartFirewallUpdateJob 根据配置启动防火墙更新任务 (保留兼容性)
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

// StopFirewallUpdateJob 停止防火墙更新任务 (保留兼容性)
func (cm *CronManager) StopFirewallUpdateJob() {
	if cm.firewallJobID != 0 {
		cm.cron.Remove(cm.firewallJobID)
		cm.firewallJobID = 0
		cm.isRunning = false
		log.Println("Firewall update job stopped")
	}
}

// StopAllRuleJobs 停止所有规则的定时任务
func (cm *CronManager) StopAllRuleJobs() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	for ruleID, jobID := range cm.ruleJobs {
		cm.cron.Remove(jobID)
		log.Printf("Rule %d update job stopped", ruleID)
	}
	cm.ruleJobs = make(map[uint]cron.EntryID)
	log.Println("All rule update jobs stopped")
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
	cm.StopAllRuleJobs()
	cm.cron.Stop()
	log.Println("Cron manager stopped")
}
