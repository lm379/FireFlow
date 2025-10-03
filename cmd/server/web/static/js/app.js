// 全局变量
let currentEditId = null;
let currentEditType = null;

// 云服务商中文映射
const providerNames = {
    'TencentCloud': '腾讯云',
    'Aliyun': '阿里云（暂未支持）',
    'AWS': '亚马逊云（暂未支持）',
    'HuaweiCloud': '华为云（暂未支持）',
};

// 获取云服务商中文名称
function getProviderDisplayName(provider) {
    return providerNames[provider] || provider;
}

// 标签页切换
function switchTab(tabName, targetElement = null) {
    // 隐藏所有标签页
    document.querySelectorAll('.tab-pane').forEach(tab => {
        tab.classList.remove('active');
    });
    document.querySelectorAll('.nav-tab').forEach(tab => {
        tab.classList.remove('active');
    });

    // 显示选中的标签页
    document.getElementById(`${tabName}-tab`).classList.add('active');
    
    // 激活对应的导航标签
    if (targetElement) {
        targetElement.classList.add('active');
    } else {
        // 如果没有传递目标元素，查找对应的标签按钮
        const navButtons = document.querySelectorAll('.nav-tab');
        navButtons.forEach(button => {
            if (button.textContent.includes('云服务配置') && tabName === 'cloud-config') {
                button.classList.add('active');
            } else if (button.textContent.includes('防火墙规则') && tabName === 'rules') {
                button.classList.add('active');
            } else if (button.textContent.includes('系统设置') && tabName === 'system') {
                button.classList.add('active');
            }
        });
    }

    // 加载对应数据
    switch(tabName) {
        case 'rules':
            fetchRules();
            break;
        case 'cloud-config':
            fetchCloudConfigs();
            break;
        case 'system':
            fetchSystemConfig();
            break;
    }
}

// 显示消息提示
function showMessage(message, type = 'success') {
    // 创建消息容器
    let messageContainer = document.getElementById('message-container');
    if (!messageContainer) {
        messageContainer = document.createElement('div');
        messageContainer.id = 'message-container';
        messageContainer.className = 'message-container';
        document.body.appendChild(messageContainer);
    }

    // 创建消息元素
    const messageElement = document.createElement('div');
    const messageClass = type === 'success' ? 'message-success' : 'message-error';
    messageElement.className = `message ${messageClass}`;
    
    // 添加图标和文本
    const icon = type === 'success' ? '✓' : '✗';
    messageElement.innerHTML = `
        <span class="message-icon">${icon}</span>
        <span class="message-text">${message}</span>
        <span class="message-close" onclick="closeMessage(this)">×</span>
    `;

    // 添加到容器中
    messageContainer.appendChild(messageElement);

    // 触发动画
    setTimeout(() => {
        messageElement.classList.add('message-show');
    }, 10);

    // 3秒后自动删除
    setTimeout(() => {
        closeMessage(messageElement);
    }, 3000);
}

// 关闭消息提示
function closeMessage(element) {
    const messageElement = element.tagName === 'SPAN' ? element.parentElement : element;
    messageElement.classList.add('message-hide');
    
    setTimeout(() => {
        if (messageElement.parentElement) {
            messageElement.parentElement.removeChild(messageElement);
        }
    }, 300);
}

// 设置加载状态
function setLoading(element, loading = true) {
    if (loading) {
        element.classList.add('loading');
    } else {
        element.classList.remove('loading');
    }
}

// API请求封装
async function apiRequest(url, options = {}) {
    try {
        const response = await fetch(url, {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        });
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        return await response.json();
    } catch (error) {
        console.error('API请求错误:', error);
        showMessage(`请求失败: ${error.message}`, 'error');
        throw error;
    }
}

// ============= 防火墙规则管理 =============

async function fetchRules() {
    try {
        const rules = await apiRequest('/api/v1/rules/');
        const tableBody = document.querySelector('#rulesTable tbody');
        tableBody.innerHTML = '';
        
        (rules || []).forEach(rule => {
            const statusBadge = rule.enabled ? 
                '<span class="status-badge status-enabled">启用</span>' : 
                '<span class="status-badge status-disabled">禁用</span>';
            
            const row = `
                <tr>
                    <td>
                        <div class="row-container">
                            <select class="action-select" data-rule-id="${rule.ID}">
                                <option value="execute">执行</option>
                                <option value="edit">编辑</option>
                                <option value="delete">删除</option>
                            </select>
                            <button class="btn-confirm" onclick="confirmRuleAction(${rule.ID})">确定</button>
                        </div>
                    </td>
                    <td>${rule.remark || ''}</td>
                    <td>${getProviderDisplayName(rule.provider)}</td>
                    <td>${rule.instance_id || ''}</td>
                    <td>${rule.port || ''}</td>
                    <td>${rule.protocol || 'TCP'}</td>
                    <td>${rule.last_ip || '未设置'}</td>
                    <td>${statusBadge}</td>
                    <td>${rule.UpdatedAt ? new Date(rule.UpdatedAt).toLocaleString() : ''}</td>
                </tr>
            `;
            tableBody.insertAdjacentHTML('beforeend', row);
        });
        
        // 加载完规则后，加载云服务配置选项
        await loadCloudConfigOptions();
    } catch (error) {
        console.error('获取规则失败:', error);
    }
}

// 确认执行规则操作
function confirmRuleAction(ruleId) {
    const selectElement = document.querySelector(`select[data-rule-id="${ruleId}"]`);
    const action = selectElement.value;
    
    // 执行对应操作
    switch(action) {
        case 'execute':
            executeRule(ruleId);
            break;
        case 'edit':
            editRule(ruleId);
            break;
        case 'delete':
            deleteRule(ruleId);
            break;
    }
    
    // 操作完成后重置为默认值（执行）
    setTimeout(() => {
        selectElement.value = 'execute';
    }, 100);
}

async function addRule(event) {
    event.preventDefault();
    const form = event.target;
    setLoading(form);
    
    try {
        const cloudConfigId = document.getElementById('cloudConfigId').value;
        if (!cloudConfigId) {
            throw new Error('请选择云服务配置');
        }
        
        const remark = document.getElementById('remark').value.trim();
        if (!remark) {
            throw new Error('备注为必填项');
        }
        
        const protocol = document.getElementById('protocol').value;
        let port = document.getElementById('port').value;
        
        // 如果协议是ICMP或ALL，强制端口为ALL
        if (protocol === 'ICMP' || protocol === 'ALL') {
            port = 'ALL';
        }
        
        const rule = {
            remark: remark,
            cloud_config_id: parseInt(cloudConfigId),
            port: port,
            protocol: protocol,
            enabled: document.getElementById('enabled').value === 'true',
        };

        // 检查是否为编辑模式
        const editId = form.dataset.editId;
        const isEdit = form.dataset.currentAction === 'edit' && editId;
        
        let response;
        if (isEdit) {
            // 编辑模式：获取完整的规则数据并更新
            const rules = await apiRequest('/api/v1/rules/');
            const existingRule = rules.find(r => r.ID == editId);
            
            if (existingRule) {
                // 合并现有数据和更新数据，保留云服务配置相关字段
                const completeRule = {
                    ...existingRule,
                    ...rule,
                    ID: parseInt(editId), // 确保ID正确
                };
                
                response = await apiRequest(`/api/v1/rules/${editId}`, {
                    method: 'PUT',
                    body: JSON.stringify(completeRule)
                });
            } else {
                throw new Error('规则不存在');
            }
            
            showMessage('规则更新成功！');
            
            // 退出编辑模式
            cancelEditRule();
        } else {
            // 新增模式：添加规则
            response = await apiRequest('/api/v1/rules/', {
                method: 'POST',
                body: JSON.stringify(rule)
            });
            
            form.reset();
            // 重置后恢复端口输入框状态
            document.getElementById('port').disabled = false;
            document.getElementById('port').style.backgroundColor = '';
            document.getElementById('port').style.cursor = '';
            
            showMessage('规则添加成功！');
        }
        
        // 添加小延时后刷新数据，确保后端数据同步
        setTimeout(() => {
            fetchRules();
        }, 500);
    } catch (error) {
        showMessage(error.message || (form.dataset.currentAction === 'edit' ? '更新规则失败' : '添加规则失败'), 'error');
    } finally {
        setLoading(form, false);
    }
}

async function deleteRule(id) {
    if (!confirm('确定要删除这条规则吗？')) return;
    
    try {
        await apiRequest(`/api/v1/rules/${id}`, { method: 'DELETE' });
        fetchRules();
        showMessage('规则删除成功！');
    } catch (error) {
        showMessage('删除规则失败', 'error');
    }
}

async function executeRule(id) {
    if (!confirm('确定要立即执行这条规则吗？')) return;
    
    try {
        await apiRequest(`/api/v1/rules/${id}/execute`, { method: 'POST' });
        fetchRules();
        showMessage('规则执行成功！');
    } catch (error) {
        showMessage('规则执行失败', 'error');
    }
}

async function editRule(id) {
    try {
        // 获取规则详情
        const rules = await apiRequest('/api/v1/rules/');
        const rule = rules.find(r => r.ID === id);
        
        if (!rule) {
            showMessage('规则不存在', 'error');
            return;
        }
        
        // 确保云服务配置选项已加载
        await loadCloudConfigOptions();
        
        // 填充表单
        document.getElementById('remark').value = rule.remark || '';
        document.getElementById('cloudConfigId').value = rule.cloud_config_id || '';
        document.getElementById('port').value = rule.port || '';
        document.getElementById('protocol').value = rule.protocol || 'TCP';
        document.getElementById('enabled').value = rule.enabled ? 'true' : 'false';
        
        // 更新表单状态为编辑模式
        const form = document.getElementById('addRuleForm');
        const submitButton = form.querySelector('button[type="submit"]');
        
        // 保存原始表单状态以便取消编辑
        if (!form.dataset.originalAction) {
            form.dataset.originalAction = 'add';
        }
        
        // 设置编辑模式
        form.dataset.editId = id;
        form.dataset.currentAction = 'edit';
        submitButton.textContent = '更新规则';
        
        // 添加取消编辑按钮
        let cancelButton = form.querySelector('.cancel-edit-btn');
        if (!cancelButton) {
            cancelButton = document.createElement('button');
            cancelButton.type = 'button';
            cancelButton.className = 'btn btn-secondary cancel-edit-btn';
            cancelButton.textContent = '取消编辑';
            cancelButton.onclick = cancelEditRule;
            submitButton.parentNode.insertBefore(cancelButton, submitButton.nextSibling);
        }
        
        // 滚动到表单区域
        form.scrollIntoView({ behavior: 'smooth', block: 'start' });

        showMessage('规则加载成功');

    } catch (error) {
        console.error('获取规则详情失败:', error);
        showMessage('获取规则详情失败', 'error');
    }
}

function cancelEditRule() {
    const form = document.getElementById('addRuleForm');
    const submitButton = form.querySelector('button[type="submit"]');
    const cancelButton = form.querySelector('.cancel-edit-btn');
    
    // 清空表单
    form.reset();
    
    // 恢复新增模式
    delete form.dataset.editId;
    form.dataset.currentAction = 'add';
    submitButton.textContent = '添加规则';
    
    // 移除取消按钮
    if (cancelButton) {
        cancelButton.remove();
    }
    
    // showMessage('已取消编辑，表单恢复为新增模式');
}

// ============= 云服务配置管理 =============

async function fetchCloudConfigs() {
    try {
        const configs = await apiRequest('/api/v1/cloud-configs/');
        const tableBody = document.querySelector('#cloudConfigTable tbody');
        tableBody.innerHTML = '';
        
        (configs || []).forEach(config => {
            const statusBadge = config.is_enabled ? 
                '<span class="status-badge status-enabled">启用</span>' : 
                '<span class="status-badge status-disabled">禁用</span>';
                
            const defaultBadge = config.is_default ? 
                '<span class="status-badge status-enabled">是</span>' : 
                '<span class="status-badge status-disabled">否</span>';
            
            const row = `
                <tr>
                    <td>
                        <div class="row-container">
                            <select class="action-select" data-config-id="${config.ID}">
                                <option value="test">测试</option>
                                <option value="edit">编辑</option>
                                <option value="delete">删除</option>
                            </select>
                            <button class="btn-confirm" onclick="confirmCloudConfigAction(${config.ID})">确定</button>
                        </div>
                    </td>
                    <td>${getProviderDisplayName(config.provider)}</td>
                    <td>${config.region || ''}</td>
                    <td>${config.instance_id || '未设置'}</td>
                    <td>${config.secret_id ? config.secret_id.substr(0, 8) + '***' : ''}</td>
                    <td>${config.description || ''}</td>
                    <td>${defaultBadge}</td>
                    <td>${statusBadge}</td>
                    <td>${config.CreatedAt ? new Date(config.CreatedAt).toLocaleString() : ''}</td>
                </tr>
            `;
            tableBody.insertAdjacentHTML('beforeend', row);
        });
    } catch (error) {
        console.error('获取云服务配置失败:', error);
    }
}

// 确认执行云服务配置操作
function confirmCloudConfigAction(configId) {
    const selectElement = document.querySelector(`select[data-config-id="${configId}"]`);
    const action = selectElement.value;
    
    // 执行对应操作
    switch(action) {
        case 'test':
            testCloudConfig(configId);
            break;
        case 'edit':
            editCloudConfig(configId);
            break;
        case 'delete':
            deleteCloudConfig(configId);
            break;
    }
    
    // 操作完成后重置为默认值（测试）
    setTimeout(() => {
        selectElement.value = 'test';
    }, 100);
}

// 加载云服务配置选项到规则表单中
async function loadCloudConfigOptions() {
    try {
        const configs = await apiRequest('/api/v1/cloud-configs/');
        const select = document.getElementById('cloudConfigId');
        
        // 清空现有选项，保留默认选项
        select.innerHTML = '<option value="">请选择已配置的云服务</option>';
        
        // 只显示启用的配置
        const enabledConfigs = (configs || []).filter(config => config.is_enabled);
        
        if (enabledConfigs.length === 0) {
            select.innerHTML = '<option value="">请先在云服务配置中添加配置</option>';
            return;
        }
        
        enabledConfigs.forEach(config => {
            const option = document.createElement('option');
            option.value = config.ID;
            option.textContent = `${getProviderDisplayName(config.provider)} - ${config.description || config.instance_id} (${config.region})`;
            select.appendChild(option);
        });
    } catch (error) {
        console.error('加载云服务配置选项失败:', error);
    }
}

async function addCloudConfig(event) {
    event.preventDefault();
    const form = event.target;
    setLoading(form);
    
    try {
        const config = {
            provider: document.getElementById('cloud-provider').value,
            secret_id: document.getElementById('secret-id').value,
            secret_key: document.getElementById('secret-key').value,
            region: document.getElementById('cloud-region').value,
            instance_id: document.getElementById('instance-id').value,
            description: document.getElementById('cloud-description').value,
            is_default: document.getElementById('is-default').value === 'true',
            is_enabled: document.getElementById('cloud-enabled').value === 'true',
        };

        // 检查是否为编辑模式
        const editId = form.dataset.editId;
        const isEdit = form.dataset.currentAction === 'edit' && editId;
        
        if (isEdit) {
            // 编辑模式：更新配置
            // 如果密码为空，则不更新密码字段
            if (!config.secret_key.trim()) {
                delete config.secret_key;
            }
            
            await apiRequest(`/api/v1/cloud-configs/${editId}`, {
                method: 'PUT',
                body: JSON.stringify(config)
            });
            
            showMessage('云服务配置更新成功！');
            
            // 退出编辑模式
            cancelEditCloudConfig();
        } else {
            // 新增模式：添加配置
            await apiRequest('/api/v1/cloud-configs/', {
                method: 'POST',
                body: JSON.stringify(config)
            });
            
            form.reset();
            showMessage('云服务配置添加成功！');
        }
        
        fetchCloudConfigs();
    } catch (error) {
        showMessage(error.message || (form.dataset.currentAction === 'edit' ? '更新云服务配置失败' : '添加云服务配置失败'), 'error');
    } finally {
        setLoading(form, false);
    }
}

async function deleteCloudConfig(id) {
    if (!confirm('确定要删除这个云服务配置吗？')) return;
    
    try {
        await apiRequest(`/api/v1/cloud-configs/${id}`, { method: 'DELETE' });
        fetchCloudConfigs();
        showMessage('云服务配置删除成功！');
    } catch (error) {
        showMessage('删除云服务配置失败', 'error');
    }
}

async function testCloudConfig(id) {
    try {
        const result = await apiRequest(`/api/v1/cloud-configs/${id}/test`, { method: 'POST' });
        
        let message = result.message;
        if (result.success && result.instance_exists && result.instance_ip) {
            message += `\n实例IP地址: ${result.instance_ip}`;
        }
        
        showMessage(message, result.success ? 'success' : 'error');
    } catch (error) {
        showMessage('连接测试失败', 'error');
    }
}

async function editCloudConfig(id) {
    try {
        // 获取配置详情
        const configs = await apiRequest('/api/v1/cloud-configs/');
        const config = configs.find(c => c.ID === id);
        
        if (!config) {
            showMessage('配置不存在', 'error');
            return;
        }
        
        // 填充云服务配置表单
        document.getElementById('cloud-provider').value = config.provider || '';
        document.getElementById('cloud-region').value = config.region || '';
        document.getElementById('instance-id').value = config.instance_id || '';
        document.getElementById('secret-id').value = config.secret_id || '';
        document.getElementById('secret-key').value = config.secret_key || '';
        document.getElementById('cloud-description').value = config.description || '';
        document.getElementById('is-default').value = config.is_default ? 'true' : 'false';
        document.getElementById('cloud-enabled').value = config.is_enabled ? 'true' : 'false';
        
        // 切换到云服务配置标签
        switchTab('cloud-config');
        
        // 更新表单状态为编辑模式
        const form = document.getElementById('addCloudConfigForm');
        const submitButton = form.querySelector('button[type="submit"]');
        
        // 保存原始表单状态
        if (!form.dataset.originalAction) {
            form.dataset.originalAction = 'add';
        }
        
        // 设置编辑模式
        form.dataset.editId = id;
        form.dataset.currentAction = 'edit';
        submitButton.textContent = '更新配置';
        
        // 添加取消编辑按钮
        let cancelButton = form.querySelector('.cancel-edit-btn');
        if (!cancelButton) {
            cancelButton = document.createElement('button');
            cancelButton.type = 'button';
            cancelButton.className = 'btn btn-secondary cancel-edit-btn';
            cancelButton.textContent = '取消编辑';
            cancelButton.onclick = cancelEditCloudConfig;
            submitButton.parentNode.insertBefore(cancelButton, submitButton.nextSibling);
        }
        
        // 滚动到表单区域
        form.scrollIntoView({ behavior: 'smooth', block: 'start' });
        
        showMessage('加载配置成功');
        
    } catch (error) {
        console.error('获取配置详情失败:', error);
        showMessage('获取配置详情失败', 'error');
    }
}

function cancelEditCloudConfig() {
    const form = document.getElementById('addCloudConfigForm');
    const submitButton = form.querySelector('button[type="submit"]');
    const cancelButton = form.querySelector('.cancel-edit-btn');
    
    // 清空表单
    form.reset();
    
    // 恢复新增模式
    delete form.dataset.editId;
    form.dataset.currentAction = 'add';
    submitButton.textContent = '保存配置';
    
    // 移除取消按钮
    if (cancelButton) {
        cancelButton.remove();
    }
    
    // showMessage('已取消编辑，表单恢复为新增模式');
}

// ============= 系统设置管理 =============

async function fetchSystemConfig() {
    try {
        const config = await apiRequest('/api/v1/system-config/');
        
        // IP获取设置
        if (config.ip_fetch_url) {
            document.getElementById('ip-fetch-url').value = config.ip_fetch_url;
        }
        
        // 定时任务设置
        if (config.ip_check_interval) {
            document.getElementById('ip-check-interval').value = config.ip_check_interval;
            document.getElementById('currentInterval').textContent = config.ip_check_interval + '分钟';
        }
        
        if (config.cron_enabled !== undefined) {
            document.getElementById('cron-enabled').value = config.cron_enabled;
            const statusEl = document.getElementById('currentStatus');
            statusEl.textContent = config.cron_enabled === 'true' ? '启用' : '禁用';
            statusEl.className = config.cron_enabled === 'true' ? 'status-badge status-enabled' : 'status-badge status-disabled';
        }
        
        // 计算下次检查时间
        if (config.ip_check_interval && config.cron_enabled === 'true') {
            const nextCheck = new Date();
            nextCheck.setMinutes(nextCheck.getMinutes() + parseInt(config.ip_check_interval));
            document.getElementById('nextCheck').textContent = nextCheck.toLocaleString();
        } else {
            document.getElementById('nextCheck').textContent = '已禁用';
        }
        
        // 获取当前IP
        fetchCurrentIP();
    } catch (error) {
        console.error('获取系统配置失败:', error);
    }
}

async function saveSystemConfig(event) {
    event.preventDefault();
    const form = event.target;
    setLoading(form);
    
    try {
        const config = {
            ip_fetch_url: document.getElementById('ip-fetch-url').value,
            ip_check_interval: parseInt(document.getElementById('ip-check-interval').value),
            cron_enabled: document.getElementById('cron-enabled').value,
        };

        await apiRequest('/api/v1/system-config/', {
            method: 'PUT',
            body: JSON.stringify(config)
        });
        
        fetchSystemConfig(); // 刷新显示
        showMessage('系统设置保存成功！');
    } catch (error) {
        showMessage('保存系统设置失败', 'error');
    } finally {
        setLoading(form, false);
    }
}

// ============= 模态框管理 =============

function openModal(title, content) {
    document.getElementById('modal-title').textContent = title;
    document.getElementById('modal-body').innerHTML = content;
    document.getElementById('editModal').style.display = 'block';
}

function closeModal() {
    document.getElementById('editModal').style.display = 'none';
    currentEditId = null;
    currentEditType = null;
}

// 立即获取并同步IP
async function syncIPNow() {
    const button = event.target;
    const originalText = button.textContent;
    
    try {
        button.textContent = '同步中...';
        button.disabled = true;
        
        const response = await fetch('/api/v1/sync-ip/', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });
        
        if (!response.ok) {
            throw new Error('同步失败');
        }
        
        const result = await response.json();
        
        if (result.success) {
            document.getElementById('currentIP').textContent = result.current_ip;
            showMessage(`IP同步成功！当前IP: ${result.current_ip}，已更新 ${result.updated_rules} 条规则`);
        } else {
            throw new Error(result.message || '同步失败');
        }
        
    } catch (error) {
        console.error('IP同步失败:', error);
        showMessage('IP同步失败: ' + error.message, 'error');
    } finally {
        button.textContent = originalText;
        button.disabled = false;
    }
}

// 获取当前IP显示
async function fetchCurrentIP() {
    try {
        const response = await fetch('/api/v1/current-ip/');
        if (response.ok) {
            const result = await response.json();
            document.getElementById('currentIP').textContent = result.current_ip || '未知';
        }
    } catch (error) {
        console.error('获取当前IP失败:', error);
        document.getElementById('currentIP').textContent = '获取失败';
    }
}

// 点击模态框外部关闭
window.onclick = function(event) {
    const modal = document.getElementById('editModal');
    if (event.target === modal) {
        closeModal();
    }
}

// ============= 事件绑定 =============

document.addEventListener('DOMContentLoaded', function() {
    // 绑定表单事件
    document.getElementById('addRuleForm').addEventListener('submit', addRule);
    document.getElementById('addCloudConfigForm').addEventListener('submit', addCloudConfig);
    document.getElementById('systemConfigForm').addEventListener('submit', saveSystemConfig);
    
    // 协议选择变化时的处理逻辑
    document.getElementById('protocol').addEventListener('change', function() {
        const protocolSelect = this;
        const portInput = document.getElementById('port');
        
        if (protocolSelect.value === 'ICMP' || protocolSelect.value === 'ALL') {
            // 当协议为ICMP或ALL时，端口自动设为ALL并禁用输入
            portInput.value = 'ALL';
            portInput.disabled = true;
            portInput.style.backgroundColor = '#f5f5f5';
            portInput.style.cursor = 'not-allowed';
        } else {
            // 其他协议时，启用端口输入
            if (portInput.value === 'ALL' && portInput.disabled) {
                portInput.value = ''; // 清空之前的ALL值
            }
            portInput.disabled = false;
            portInput.style.backgroundColor = '';
            portInput.style.cursor = '';
        }
    });
    
    // 初始加载防火墙规则
    fetchRules();
});