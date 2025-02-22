const CloudProvider = require('../interfaces/CloudProvider');
const tencentcloud = require("tencentcloud-sdk-nodejs-lighthouse");
const LighthouseClient = tencentcloud.lighthouse.v20200324.Client;

class TencentCloudProvider extends CloudProvider {
    constructor(config) {
        super();
        this.config = config;
        this.client = new LighthouseClient({
            credential: {
                secretId: config.secretId,
                secretKey: config.secretKey,
            },
            region: config.lighthouse[0].region,
            profile: {
                httpProfile: {
                    endpoint: "lighthouse.tencentcloudapi.com",
                },
            },
        });
    }

    async updateFirewallRules(currentIp) {
        const cidrBlock = `${currentIp}/32`;
        // console.log(`更新腾讯云防火墙规则，当前IP: ${currentIp}`);

        // 获取现有防火墙规则
        const describeParams = { InstanceId: this.config.lighthouse[0].instanceId };
        const { FirewallRuleSet } = await this.client.DescribeFirewallRules(describeParams);

        // 查找需要更新的规则
        const targetRules = FirewallRuleSet.filter(rule =>
            rule.FirewallRuleDescription === 'DynamicIPRule'
        );

        if (targetRules.some(rule => rule.CidrBlock === cidrBlock)) {
            console.log('腾讯云: IP未变动，防火墙规则无需更新');
            return;
        }

        // 删除旧规则
        if (targetRules.length > 0) {
            await this.client.DeleteFirewallRules({
                InstanceId: this.config.lighthouse[0].instanceId,
                FirewallRules: targetRules.map(rule => ({
                    Protocol: rule.Protocol,
                    Port: rule.Port,
                    CidrBlock: rule.CidrBlock,
                    Action: rule.Action,
                })),
            });
        }

        // 创建新规则
        const newRules = this.config.lighthouse[0].firewallRules.map(rule => ({
            ...rule,
            CidrBlock: cidrBlock,
        }));

        await this.client.CreateFirewallRules({
            InstanceId: this.config.lighthouse[0].instanceId,
            FirewallRules: newRules,
        });

        console.log('腾讯云防火墙规则更新成功');
    }
}

module.exports = TencentCloudProvider;