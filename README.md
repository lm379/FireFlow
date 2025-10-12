# FireFlow - 动态防火墙规则管理系统

一个专为多云环境设计的智能防火墙规则管理工具，支持动态 IP 更新和统一管理界面。

## 🔧 功能特性

### ✅ 已实现
- **数据存储**：使用 SQLite 数据库存储配置和规则
- **多云支持**：腾讯云轻量应用服务器、阿里云 ECS、华为云 ECS/Flexus（仅支持中国站）
- **智能更新**：定时任务自动检测并更新 IP 地址
- **管理界面**：直观的 Web 管理控制台
- **API 接口**：完整的 RESTful API 支持

## 🌐 前端界面

FireFlow 提供了现代化的 Web 管理界面，基于 Vue 3 + TypeScript 开发。

- **GitHub**: [fireflow-frontend](https://github.com/lm379/fireflow-frontend)
- **技术栈**: Vue 3 + TypeScript + Vite

## 🚀 快速开始

### Docker 部署（推荐）

```bash
docker run -d \
    --name fireflow \
    -p 9686:9686 \
    -v ./configs:/app/configs \
    lm379/fireflow:latest
```

### 二进制部署

1. 前往 [Release 页面](https://github.com/lm379/FireFlow/releases) 下载对应架构的二进制包
2. 解压并运行：
   ```bash
   tar -xzf fireflow-linux-amd64.tar.gz
   mv fireflow-linux-amd64 fireflow
   ./fireflow
   ```

## ⚠️ 重要提醒

> **安全须知**
> - 请妥善保管各云厂商的 API 密钥
> - 不要将服务端口暴露到公网
> - 密钥泄露造成的后果与本项目及作者无关


## ⚙️ 配置指南

> **说明**：本程序提供的区域信息可能不完全准确，请以云厂商控制台为准。如果选项中没有目标区域，可以手动输入。

### 🔹 腾讯云（轻量应用服务器）

#### 1. 获取访问密钥（AK/SK）

**推荐使用子账号，安全性更高**

1. 访问 [腾讯云 CAM 控制台](https://console.cloud.tencent.com/cam)
2. 创建子用户，选择「编程访问」
3. 授予 `QcloudLighthouseFullAccess` 权限
4. 保存生成的 `secret_id`（AK）和 `secret_key`（SK）

![腾讯云子账号配置](./docs/images/tencent/tencent1.png)

#### 2. 获取实例 ID

1. 进入腾讯云轻量应用服务器控制台
2. 找到目标服务器，复制实例 ID

![腾讯云实例 ID](./docs/images/tencent/tencent2.png)

### 🔹 阿里云（ECS）

#### 1. 获取访问密钥（AK/SK）

**推荐使用 RAM 子账号**

1. 访问 [阿里云 RAM 控制台](https://ram.console.aliyun.com/users)
2. 创建用户，勾选「使用永久 AccessKey 访问」

![阿里云用户创建](./docs/images/ali/ali1.png)

3. 为用户授予 ECS 管理权限

![阿里云权限配置](./docs/images/ali/ali2.png)

#### 2. 获取安全组 ID

1. 访问 [阿里云 ECS 控制台](https://ecs.console.aliyun.com/)
2. 选择对应地域，进入「网络与安全」→「安全组」
3. 复制安全组 ID（格式：`sg-xxxxxxxxx`）

![阿里云安全组](./docs/images/ali/ali3.png)

### 🔹 华为云（ECS/Flexus）

> **兼容性说明**：华为云 ECS 和 Flexus 云服务器使用相同的 API，理论上支持所有使用 VPC 网络的云服务器。

#### 1. 获取访问密钥（AK/SK）

**强烈推荐使用 IAM 子用户**

1. 访问 [华为云 IAM 控制台](https://console.huaweicloud.com/iam/#/iam/users)
2. 创建用户，勾选「编程访问」和「访问密钥」

![华为云用户创建](./docs/images/huawei/huawei3.png)

3. 创建用户组并授权 ECS 相关权限

![华为云权限配置](./docs/images/huawei/huawei5.png)

**主账号密钥（不推荐）**

如需使用主账号，请访问 [我的凭证](https://console.huaweicloud.com/iam/#/mine/accessKey) 创建访问密钥。

> ⚠️ **安全警告**：主账号 Token 权限过高，泄露风险极大，强烈建议使用子账号。

#### 2. 获取 Project ID

访问 [我的凭证](https://console.huaweicloud.com/iam/#/mine/accessKey)，复制与服务器区域对应的项目 ID。

![华为云项目 ID](./docs/images/huawei/huawei6.png)

#### 3. 获取安全组 ID

1. 进入服务器所在地域的「虚拟私有云 VPC」控制台
2. 选择「访问控制」→「安全组」
3. 复制实例关联的安全组 ID

![华为云安全组](./docs/images/huawei/huawei7.png)