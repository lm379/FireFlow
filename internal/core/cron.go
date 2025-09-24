package core

import (
	"log"

	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

// CronManager 管理定时任务
type CronManager struct {
	cron *cron.Cron
}

// NewCronManager 创建新的定时任务管理器
func NewCronManager() *CronManager {
	return &CronManager{
		cron: cron.New(),
	}
}

// AddFirewallUpdateJob 添加防火墙更新任务
func (cm *CronManager) AddFirewallUpdateJob(updateFunc func()) error {
	cronExpr := viper.GetString("cron.schedule")
	if cronExpr == "" {
		cronExpr = "0 */5 * * * *" // 默认每5分钟执行一次
	}

	_, err := cm.cron.AddFunc(cronExpr, updateFunc)
	if err != nil {
		return err
	}

	log.Printf("Firewall update job scheduled with expression: %s", cronExpr)
	return nil
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
