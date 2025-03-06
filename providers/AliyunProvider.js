const CloudProvider = require('../interfaces/CloudProvider');
const Util = require('@alicloud/tea-util');

class AliyunProvider extends CloudProvider {
    constructor(config) {
        super();
        this.config = config;
        this.client = null;
    }

    async updateFirewallRules(currentIp) {
        const cidrBlock = `${currentIp}/32`;

        const securityGroupId = this.config.SecurityGroupId;
        const runtime = new Util.RuntimeOptions({});

        // 获取现有规则
        const existingRules = await this.describeSecurityGroupAttribute(securityGroupId, runtime);
        const targetRules = existingRules.filter(rule =>
            rule.description === this.config.tag || rule.remark === this.config.tag
        );

        if (targetRules.some(rule => rule.sourceCidrIp === cidrBlock)) {
            console.log('阿里云: IP未变动，安全组规则无需更新');
            return;
        }

        // 删除旧规则
        for (const rule of targetRules) {
            await this.revokeSecurityGroup(securityGroupId, rule, runtime);
        }

        // 添加新规则
        for (const rule of this.config.Permissions) {
            await this.authorizeSecurityGroup(securityGroupId, rule, cidrBlock, runtime);
        }

        console.log('阿里云安全组规则更新成功');
    }

    // 由子类实现的方法
    async describeSecurityGroupAttribute(securityGroupId, runtime) {
        throw new Error('子类必须实现 describeSecurityGroupAttribute 方法');
    }

    async revokeSecurityGroup(securityGroupId, rule, runtime) {
        throw new Error('子类必须实现 revokeSecurityGroup 方法');
    }

    async authorizeSecurityGroup(securityGroupId, rule, cidrBlock, runtime) {
        throw new Error('子类必须实现 authorizeSecurityGroup 方法');
    }
}
module.exports = AliyunProvider;