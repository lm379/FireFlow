// 全局变量
let currentEditId = null;
let currentEditType = null;

// 标签页切换
function switchTab(tabName) {
    // 隐藏所有标签页
    document.querySelectorAll('.tab-pane').forEach(tab => {
        tab.classList.remove('active');
    });
    document.querySelectorAll('.nav-tab').forEach(tab => {
        tab.classList.remove('active');
    });

    // 显示选中的标签页
    document.getElementById(`${tabName}-tab`).classList.add('active');
    event.target.classList.add('active');

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
    const alertClass = type === 'success' ? 'alert-success' : 'alert-error';
    const alertHtml = `<div class="alert ${alertClass}">${message}</div>`;
    
    // 在当前激活的tab中显示消息
    const activeTab = document.querySelector('.tab-pane.active');
    const existingAlert = activeTab.querySelector('.alert');
    if (existingAlert) {
        existingAlert.remove();
    }
    activeTab.insertAdjacentHTML('afterbegin', alertHtml);
    
    // 3秒后自动删除消息
    setTimeout(() => {
        const alert = activeTab.querySelector('.alert');
        if (alert) alert.remove();
    }, 3000);
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
                    <td>${rule.remark || ''}</td>
                    <td>${rule.provider || ''}</td>
                    <td>${rule.instanceId || ''}</td>
                    <td>${rule.port || ''}</td>
                    <td>${rule.protocol || 'TCP'}</td>
                    <td>${rule.lastIp || '未设置'}</td>
                    <td>${statusBadge}</td>
                    <td>${rule.updatedAt ? new Date(rule.updatedAt).toLocaleString() : ''}</td>
                    <td>
                        <button class="btn btn-small btn-secondary" onclick="editRule(${rule.ID})">编辑</button>
                        <button class="btn btn-small btn-danger" onclick="deleteRule(${rule.ID})">删除</button>
                        <button class="btn btn-small btn-success" onclick="executeRule(${rule.ID})">立即执行</button>
                    </td>
                </tr>
            `;
            tableBody.insertAdjacentHTML('beforeend', row);
        });
    } catch (error) {
        console.error('获取规则失败:', error);
    }
}

async function addRule(event) {
    event.preventDefault();
    const form = event.target;
    setLoading(form);
    
    try {
        const rule = {
            remark: document.getElementById('remark').value,
            provider: document.getElementById('provider').value,
            instanceId: document.getElementById('instanceId').value,
            port: document.getElementById('port').value,
            protocol: document.getElementById('protocol').value,
            enabled: document.getElementById('enabled').value === 'true',
        };

        await apiRequest('/api/v1/rules/', {
            method: 'POST',
            body: JSON.stringify(rule)
        });
        
        form.reset();
        fetchRules();
        showMessage('规则添加成功！');
    } catch (error) {
        showMessage('添加规则失败', 'error');
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

function editRule(id) {
    // TODO: 实现编辑规则功能
    showMessage('编辑功能开发中...');
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
                    <td>${config.provider || ''}</td>
                    <td>${config.region || ''}</td>
                    <td>${config.instance_id || '未设置'}</td>
                    <td>${config.secret_id ? config.secret_id.substr(0, 8) + '***' : ''}</td>
                    <td>${config.description || ''}</td>
                    <td>${defaultBadge}</td>
                    <td>${statusBadge}</td>
                    <td>${config.created_at ? new Date(config.created_at).toLocaleString() : ''}</td>
                    <td>
                        <button class="btn btn-small btn-secondary" onclick="editCloudConfig(${config.ID})">编辑</button>
                        <button class="btn btn-small btn-danger" onclick="deleteCloudConfig(${config.ID})">删除</button>
                        <button class="btn btn-small btn-success" onclick="testCloudConfig(${config.ID})">测试连接</button>
                    </td>
                </tr>
            `;
            tableBody.insertAdjacentHTML('beforeend', row);
        });
    } catch (error) {
        console.error('获取云服务配置失败:', error);
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

        await apiRequest('/api/v1/cloud-configs/', {
            method: 'POST',
            body: JSON.stringify(config)
        });
        
        form.reset();
        fetchCloudConfigs();
        showMessage('云服务配置添加成功！');
    } catch (error) {
        showMessage('添加云服务配置失败', 'error');
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

function editCloudConfig(id) {
    // TODO: 实现编辑云服务配置功能
    showMessage('编辑功能开发中...');
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
    
    // 初始加载防火墙规则
    fetchRules();
});