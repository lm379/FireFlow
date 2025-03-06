const SWAS_OPEN20200601 = require('@alicloud/swas-open20200601');
const OpenApi = require('@alicloud/openapi-client');
const AliyunProvider = require('./AliyunProvider');

class AliyunSwasProvider extends AliyunProvider {
    constructor(config) {
        super(config);
        this.client = new SWAS_OPEN20200601.default(new OpenApi.Config({
            accessKeyId: config.accessKeyId,
            accessKeySecret: config.accessKeySecret,
            endpoint: "swas." + config.RegionId + ".aliyuncs.com"
        }));
    }

    // 获取现有规则
    async describeSecurityGroupAttribute(securityGroupId, runtime) {
        let resp = await this.client.listFirewallRulesWithOptions(new SWAS_OPEN20200601.ListFirewallRulesRequest({
            regionId: this.config.RegionId,
            instanceId: this.config.InstanceId
        }), runtime);
        return resp.body.firewallRules;
    }

    // 删除旧规则
    async revokeSecurityGroup(securityGroupId, rule, runtime) {
        await this.client.deleteFirewallRulesWithOptions(new SWAS_OPEN20200601.DeleteFirewallRulesRequest({
            instanceId: this.config.InstanceId,
            ruleId: rule.ruleId,
            regionId: this.config.RegionId
        }), runtime);
    }

    // 增加新规则
    async authorizeSecurityGroup(securityGroupId, rule, cidrBlock, runtime) {
        if (rule.RuleProtocol != 'TCP' && rule.RuleProtocol != 'UDP' && rule.RuleProtocol != 'TCP+UDP' && rule.RuleProtocol != 'ICMP') {
            console.error('阿里云轻量应用云服务器: 不支持的协议类型');
            return;
        }
        if (rule.RuleProtocol == 'ICMP') {
            rule.Port = '-1/-1';
        }
        // 如果端口范围相同，则只取前一个，阿里云接口是这么写的，不然会报错
        const regex = /^(\d+)\/\1$/;
        if (regex.test(rule.Port)) {
            rule.Port = rule.Port.match(regex)[1];
        }
        let firewallRules = new SWAS_OPEN20200601.CreateFirewallRulesRequestFirewallRules({
            ruleProtocol: rule.RuleProtocol,
            sourceCidrIp: cidrBlock,
            remark: this.config.tag,
            port: rule.Port
        });
        await this.client.createFirewallRulesWithOptions(new SWAS_OPEN20200601.CreateFirewallRulesRequest({
            instanceId: this.config.InstanceId,
            regionId: this.config.RegionId,
            firewallRules: [
                firewallRules
            ],
        }), runtime);
    }
}

module.exports = AliyunSwasProvider;