const AliyunProvider = require('./AliyunProvider');
const Ecs20140526 = require('@alicloud/ecs20140526');
const OpenApi = require('@alicloud/openapi-client');

class AliyunEcsProvider extends AliyunProvider {
    constructor(config) {
        super(config);
        this.client = new Ecs20140526.default(new OpenApi.Config({
            accessKeyId: config.accessKeyId,
            accessKeySecret: config.accessKeySecret,
            regionId: config.RegionId
        }));
    }

    // 获取现有规则
    async describeSecurityGroupAttribute(securityGroupId, runtime) {
        let resp =  await this.client.describeSecurityGroupAttribute(new Ecs20140526.DescribeSecurityGroupAttributeRequest({
            securityGroupId: securityGroupId,
            direction: 'ingress',
            regionId: this.config.RegionId
        }), runtime);
        return resp.body.permissions.permission;
    }

    // 删除旧规则
    async revokeSecurityGroup(securityGroupId, rule, runtime) {
        await this.client.revokeSecurityGroup(new Ecs20140526.RevokeSecurityGroupRequest({
            securityGroupId: securityGroupId,
            regionId: this.config.RegionId,
            policy: rule.policy,
            ipProtocol: rule.ipProtocol,
            portRange: rule.portRange,
            sourceCidrIp: rule.sourceCidrIp,
            description: this.config.tag
        }), runtime);
    }

    // 增加新规则
    async authorizeSecurityGroup(securityGroupId, rule, cidrBlock, runtime) {
        if (rule.IpProtocol === 'ALL' || rule.IpProtocol === 'GRE' || rule.IpProtocol === 'ICMP') {
            rule.PortRange = '-1/-1';
        }
        if (rule.Priority == null || rule.Priority === '') {
            rule.Priority = 1;
        }
        await this.client.authorizeSecurityGroup(new Ecs20140526.AuthorizeSecurityGroupRequest({
            securityGroupId: securityGroupId,
            regionId: this.config.RegionId,
            policy: rule.Policy, // 授权策略
            ipProtocol: rule.IpProtocol, // 协议
            portRange: rule.PortRange, // 端口范围
            sourceCidrIp: cidrBlock, // 授权IP
            description: this.config.tag, // 描述
            riority: rule.Priority // 优先级
        }), runtime);
    }
}

module.exports = AliyunEcsProvider;