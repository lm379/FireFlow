const CloudProvider = require('../interfaces/CloudProvider');
const Ecs20140526 = require('@alicloud/ecs20140526');
const OpenApi = require('@alicloud/openapi-client');
const Util = require('@alicloud/tea-util');
const Tea = require('@alicloud/tea-typescript');

class AliyunProvider extends CloudProvider {
    constructor(config) {
        super();
        this.config = config;
        this.client = new Ecs20140526.default(new OpenApi.Config({
            accessKeyId: config.accessKeyId,
            accessKeySecret: config.accessKeySecret,
            regionId: config.ecs[0].RegionId
        }));
    }

    async updateFirewallRules(currentIp) {
        const cidrBlock = `${currentIp}/32`;
        // console.log(`更新阿里云安全组规则，当前IP: ${currentIp}`);

        const securityGroupId = this.config.ecs[0].SecurityGroupId;
        const runtime = new Util.RuntimeOptions({});

        // 获取现有规则
        const result = await this.client.describeSecurityGroupAttribute(new Ecs20140526.DescribeSecurityGroupAttributeRequest({
            securityGroupId: securityGroupId,
            direction: 'ingress',
            regionId: this.config.ecs[0].RegionId
        }), runtime);

        const existingRules = result.body.permissions.permission;
        const targetRules = existingRules.filter(rule =>
            rule.description === 'DynamicIPRule'
        );

        if (targetRules.some(rule => rule.sourceCidrIp === cidrBlock)) {
            console.log('阿里云: IP未变动，安全组规则无需更新');
            return;
        }

        // 删除旧规则
        for (const rule of targetRules) {
            await this.client.revokeSecurityGroup(new Ecs20140526.RevokeSecurityGroupRequest({
                securityGroupId: securityGroupId,
                regionId: this.config.ecs[0].RegionId,
                policy: rule.policy,
                ipProtocol: rule.ipProtocol,
                portRange: rule.portRange,
                sourceCidrIp: rule.sourceCidrIp,
                description: 'DynamicIPRule'
            }));
        }

        // 添加新规则
        for (const rule of this.config.ecs[0].Permissions) {
            await this.client.authorizeSecurityGroup(new Ecs20140526.AuthorizeSecurityGroupRequest({
                securityGroupId: securityGroupId,
                regionId: this.config.ecs[0].RegionId,
                policy: rule.Policy,
                ipProtocol: rule.IpProtocol,
                portRange: rule.PortRange,
                sourceCidrIp: cidrBlock,
                description: 'DynamicIPRule'
            }));
        }

        console.log('阿里云安全组规则更新成功');
    }
}

module.exports = AliyunProvider;