const axios = require('axios');
const fs = require('fs');
const CloudProviderFactory = require('./factories/CloudProviderFactory');

// log添加时间输出
const originalConsole = {
    log: console.log,
    error: console.error,
}

function getTimeStamp() {
    return new Date().toLocaleString('zh-CN', { hour12: false });
}

console.log = function () {
    const args = Array.from(arguments);
    originalConsole.log(`[${getTimeStamp()}]`, ...args);
}

console.error = function () {
    const args = Array.from(arguments);
    originalConsole.error(`[${getTimeStamp()}]`, ...args);
}

// 获取当前公网IPv4地址
async function getCurrentIp() {
    try {
        const response = await axios.get('https://4.ipw.cn/', { timeout: 5000 });
        return response.data.trim();
    } catch (error) {
        throw new Error(`获取IP失败: ${error.message}`);
    }
}

async function updateAllFirewallRules() {
    try {
        // 读取配置文件
        const config = JSON.parse(fs.readFileSync('config.json', 'utf8'));
        if (config.tag == null || config.tag == "") {
            throw new Error('请在配置文件中填写tag');
        }
        // 获取当前IP
        const currentIp = await getCurrentIp();
        console.log(`当前IP地址: ${currentIp}`);

        // 创建所有云提供商实例
        const providers = CloudProviderFactory.createProviders(config);

        // 并行更新所有云提供商的防火墙规则
        await Promise.all(providers.map(provider =>
            provider.updateFirewallRules(currentIp).catch(error => {
                console.error(`更新失败 [${provider.constructor.name}]:`, error.message);
            })
        ));

        console.log('所有防火墙规则更新完成');
    } catch (error) {
        console.error('操作失败:', error.message);
        process.exit(1);
    }
}

updateAllFirewallRules();