# FireFlow - 动态防火墙规则管理系统

## 🔧 功能特性

### ✅ 已实现
- SQLite数据库存储
- 腾讯云Lighthouse防火墙规则管理
- 定时任务自动更新IP
- Web管理界面
- RESTful API

## 使用方法

### Docker

```
docker run -d \
    --name fireflow \
    -p 9686:9686 \
    -v ./configs:/app/configs \
    lm379/fireflow:latest
```

### 二进制直接运行
前往 [Release](https://github.com/lm379/FireFlow/releases) 下载对应架构的二进制包，解压后直接运行即可