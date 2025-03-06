const TencentCloudProvider = require('../providers/TencentCloudProvider');
// const AliyunProvider = require('../providers/AliyunProvider');
const AliyunSwasProvider = require('../providers/AliyunSwasProvider');
const AliyunEcsProvider = require('../providers/AliyunEcsProvider');

class CloudProviderFactory {
    static createProviders(config) {
        const providers = [];

        if (config.tencentcloud.secretId && config.tencentcloud.secretKey) {
            providers.push(new TencentCloudProvider(config.tencentcloud));
        }

        if (config.aliyun.accessKeyId && config.aliyun.accessKeySecret) {
            if (config.aliyun.ecs_enable) {
                for (const ecsConfig of config.aliyun.ecs) {
                    ecsConfig.tag = config.tag;
                    ecsConfig.accessKeyId = config.aliyun.accessKeyId;
                    ecsConfig.accessKeySecret = config.aliyun.accessKeySecret;
                    providers.push(new AliyunEcsProvider(ecsConfig));
                }
            }
            if (config.aliyun.swas_enable) {
                for (const swasConfig of config.aliyun.swas) {
                    swasConfig.tag = config.tag;
                    swasConfig.accessKeyId = config.aliyun.accessKeyId;
                    swasConfig.accessKeySecret = config.aliyun.accessKeySecret;
                    providers.push(new AliyunSwasProvider(swasConfig));
                }
            }
        }

        return providers;
    }
}

module.exports = CloudProviderFactory;