const TencentCloudProvider = require('../providers/TencentCloudProvider');
const AliyunProvider = require('../providers/AliyunProvider');

class CloudProviderFactory {
    static createProviders(config) {
        const providers = [];

        if (config.tencentcloud.secretId && config.tencentcloud.secretKey) {
            providers.push(new TencentCloudProvider(config.tencentcloud));
        }

        if (config.aliyun.accessKeyId && config.aliyun.accessKeySecret) {
            providers.push(new AliyunProvider(config.aliyun));
        }

        return providers;
    }
}

module.exports = CloudProviderFactory;