class CloudProvider {
    async updateFirewallRules(currentIp) {
        throw new Error('方法未实现');
    }
}

module.exports = CloudProvider;