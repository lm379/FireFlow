# 数据库中的云配置关联

`firewall_rules.cloud_config_id` 引用 `cloud_provider_configs.id`，不能为空或为 0，并建立索引和外键。仍被规则引用的云配置不能删除。

规则表仅保存关联 ID 和规则自身的数据：端口、协议、启用状态、备注、上次更新 IP 及时间戳。厂商、实例 ID、项目 ID 和凭据统一从云配置读取。查询接口仍返回 `provider`、`instance_id`、`project_id`，这些字段由当前关联配置生成；创建和更新规则必须提交 `cloud_config_id`。

服务启动时自动在事务中迁移旧表，移除重复的 `provider`、`instance_id`、`project_id` 列，并保留原有规则 ID、时间戳、状态和自增序列。已有非零关联 ID 优先使用；缺少关联 ID 时，按厂商、实例 ID 和非空项目 ID 查找唯一云配置。禁用的配置也可保留关联，但不会执行规则。

如果某条旧规则没有匹配的云配置、匹配多个配置，或引用不存在的配置，启动会报告规则 ID，迁移整体回滚。修复该规则的 `cloud_config_id` 后重新启动即可。
