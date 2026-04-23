<template>
  <div class="settings-page">
    <div class="settings-layout">
      <div class="settings-menu">
        <div
          v-for="item in menuItems"
          :key="item.key"
          class="menu-item"
          :class="{ active: activeTab === item.key }"
          @click="activeTab = item.key"
        >
          <el-icon><component :is="item.icon" /></el-icon>
          <span>{{ item.label }}</span>
        </div>
      </div>

      <div class="settings-content">
        <!-- 服务器管理 -->
        <div v-if="activeTab === 'servers'">
          <div class="content-header">
            <h3>服务器管理</h3>
            <el-button type="primary" size="small" @click="showAddDialog = true">
              <el-icon><Plus /></el-icon>
              添加服务器
            </el-button>
          </div>
          <el-table :data="servers" v-loading="loading" style="width:100%">
            <el-table-column prop="name" label="名称" width="140" />
            <el-table-column prop="host" label="主机" width="160" />
            <el-table-column prop="port" label="端口" width="80" />
            <el-table-column prop="username" label="用户名" width="100" />
            <el-table-column label="连接方式" width="100">
              <template #default="{ row }">
                <span style="font-size:11px;font-weight:600">{{ ({ ssh: 'SSH', agent: 'Agent', plugin: '插件', api: 'API' } as Record<string, string>)[row.connectMethod] || 'SSH' }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="group" label="分组" width="100" />
            <el-table-column label="操作" width="160">
              <template #default="{ row }">
                <el-button type="primary" text size="small" @click="editServer(row)">编辑</el-button>
                <el-popconfirm title="确定删除此服务器?" @confirm="deleteServer(row.id)">
                  <template #reference>
                    <el-button type="danger" text size="small">删除</el-button>
                  </template>
                </el-popconfirm>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <!-- 告警规则 -->
        <div v-if="activeTab === 'rules'">
          <div class="content-header">
            <h3>告警规则</h3>
          </div>
          <el-table :data="rules" v-loading="rulesLoading" style="width:100%">
            <el-table-column prop="description" label="规则描述" width="200" />
            <el-table-column prop="metric" label="指标类型" width="100" />
            <el-table-column label="警告阈值" width="120">
              <template #default="{ row }">
                <span class="font-num">{{ row.warningThreshold }}%</span>
              </template>
            </el-table-column>
            <el-table-column label="危险阈值" width="120">
              <template #default="{ row }">
                <span class="font-num">{{ row.criticalThreshold }}%</span>
              </template>
            </el-table-column>
            <el-table-column label="启用" width="80">
              <template #default="{ row }">
                <el-switch v-model="row.enabled" @change="updateRule(row)" size="small" />
              </template>
            </el-table-column>
          </el-table>
        </div>

        <!-- 消息推送 -->
        <div v-if="activeTab === 'notify'">
          <div class="content-header">
            <h3>企业微信推送</h3>
          </div>
          <div class="notify-form">
            <el-form :model="notifyConfig" label-position="top" style="max-width:520px" v-loading="notifyLoading">
              <el-form-item label="启用推送">
                <el-switch v-model="notifyConfig.enabled" />
              </el-form-item>
              <el-form-item label="Webhook URL" required>
                <el-input v-model="notifyConfig.webhookUrl" placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx" />
                <div style="font-size:11px;color:var(--t3);margin-top:4px">企业微信群机器人 Webhook 地址</div>
              </el-form-item>
              <el-form-item label="推送条件">
                <div style="display:flex;flex-direction:column;gap:8px">
                  <el-checkbox v-model="notifyConfig.notifyWarning">警告级别告警</el-checkbox>
                  <el-checkbox v-model="notifyConfig.notifyCritical">危险级别告警</el-checkbox>
                  <el-checkbox v-model="notifyConfig.notifyOffline">服务器离线告警</el-checkbox>
                </div>
              </el-form-item>
              <el-form-item label="冷却时间（分钟）">
                <el-input-number v-model="notifyConfig.cooldownMin" :min="1" :max="1440" style="width:200px" />
                <div style="font-size:11px;color:var(--t3);margin-top:4px">同类告警在此时间内不会重复推送</div>
              </el-form-item>
              <el-form-item>
                <div style="display:flex;gap:10px">
                  <el-button type="primary" @click="saveNotifyConfig" :loading="notifySaving">保存配置</el-button>
                  <el-button @click="testNotify" :loading="notifyTesting">发送测试消息</el-button>
                </div>
              </el-form-item>
            </el-form>
          </div>
        </div>

        <!-- 修改密码 -->
        <div v-if="activeTab === 'password'">
          <div class="content-header">
            <h3>修改密码</h3>
          </div>
          <div class="password-form">
            <el-form :model="pwdForm" label-position="top" style="max-width:400px">
              <el-form-item label="当前密码" required>
                <el-input v-model="pwdForm.oldPassword" type="password" show-password placeholder="输入当前密码" />
              </el-form-item>
              <el-form-item label="新密码" required>
                <el-input v-model="pwdForm.newPassword" type="password" show-password placeholder="至少 6 位" />
              </el-form-item>
              <el-form-item label="确认新密码" required>
                <el-input v-model="pwdForm.confirmPassword" type="password" show-password placeholder="再次输入新密码" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="changePassword" :loading="pwdLoading">修改密码</el-button>
              </el-form-item>
            </el-form>
          </div>
        </div>

        <!-- IP 黑名单 -->
        <div v-if="activeTab === 'blacklist'">
          <div class="content-header">
            <h3>IP 黑名单</h3>
            <el-button type="danger" size="small" @click="showBlacklistDialog = true">添加封禁</el-button>
          </div>
          <el-table :data="blacklist" v-loading="blLoading" style="width:100%">
            <el-table-column prop="ip" label="IP 地址" width="160" />
            <el-table-column prop="reason" label="封禁原因" min-width="200" />
            <el-table-column label="类型" width="80">
              <template #default="{ row }">
                <el-tag :type="row.autoBan ? 'warning' : 'danger'" size="small">{{ row.autoBan ? '自动' : '手动' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="过期时间" width="180">
              <template #default="{ row }">
                <span v-if="row.expiresAt">{{ formatTime(row.expiresAt) }}</span>
                <el-tag v-else type="danger" size="small">永久</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="100">
              <template #default="{ row }">
                <el-popconfirm title="确定解除封禁?" @confirm="removeBlacklist(row.id)">
                  <template #reference>
                    <el-button type="success" text size="small">解封</el-button>
                  </template>
                </el-popconfirm>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <!-- 安全日志 -->
        <div v-if="activeTab === 'logs'">
          <div class="content-header">
            <h3>安全日志</h3>
            <div style="display:flex;gap:8px">
              <el-button :type="logTab === 'security' ? 'primary' : 'default'" size="small" @click="logTab = 'security'; fetchSecurityLogs()">操作日志</el-button>
              <el-button :type="logTab === 'login' ? 'primary' : 'default'" size="small" @click="logTab = 'login'; fetchLoginAttempts()">登录记录</el-button>
            </div>
          </div>
          <el-table v-if="logTab === 'security'" :data="securityLogs" v-loading="logLoading" style="width:100%">
            <el-table-column label="时间" width="180">
              <template #default="{ row }"><span class="font-num">{{ formatTime(row.createdAt) }}</span></template>
            </el-table-column>
            <el-table-column prop="action" label="动作" width="150">
              <template #default="{ row }">
                <el-tag :type="actionTagType(row.action)" size="small">{{ row.action }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="ip" label="IP" width="140" />
            <el-table-column prop="username" label="用户" width="100" />
            <el-table-column prop="detail" label="详情" min-width="200" />
          </el-table>
          <el-table v-if="logTab === 'login'" :data="loginAttempts" v-loading="logLoading" style="width:100%">
            <el-table-column label="时间" width="180">
              <template #default="{ row }"><span class="font-num">{{ formatTime(row.createdAt) }}</span></template>
            </el-table-column>
            <el-table-column prop="ip" label="IP" width="140" />
            <el-table-column prop="username" label="用户名" width="120" />
            <el-table-column label="结果" width="80">
              <template #default="{ row }">
                <el-tag :type="row.success ? 'success' : 'danger'" size="small">{{ row.success ? '成功' : '失败' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="userAgent" label="User-Agent" min-width="200" show-overflow-tooltip />
          </el-table>
        </div>

        <!-- Agent 更新 -->
        <div v-if="activeTab === 'agent-update'">
          <div class="content-header">
            <h3>Agent 远程更新</h3>
          </div>

          <!-- Linux Agent -->
          <div class="agent-update-section">
            <div class="platform-title">🐧 Linux Agent</div>
            <div class="info-grid" style="margin-bottom: 16px">
              <div class="info-item">
                <span class="info-label">二进制状态</span>
                <span class="info-value">{{ linuxBinInfo.exists ? `已上传 (${(linuxBinInfo.size / 1024 / 1024).toFixed(2)} MB)` : '未上传' }}</span>
              </div>
              <div class="info-item" v-if="linuxBinInfo.exists">
                <span class="info-label">上传时间</span>
                <span class="info-value">{{ new Date(linuxBinInfo.modified).toLocaleString('zh-CN') }}</span>
              </div>
            </div>
            <div class="agent-update-actions">
              <div class="action-card">
                <h4>① 上传 Linux Agent</h4>
                <p>选择编译好的 agent-linux 二进制文件</p>
                <input ref="linuxFileInput" type="file" style="display:none" @change="handleUpload($event, 'linux')" />
                <el-button type="primary" :loading="linuxUploading" @click="($refs.linuxFileInput as HTMLInputElement)?.click()">
                  选择文件并上传
                </el-button>
              </div>
              <div class="action-card">
                <h4>② 推送到 Linux Agent</h4>
                <p>向所有在线 Linux Agent 推送更新</p>
                <el-button type="warning" :loading="linuxPushing" :disabled="!linuxBinInfo.exists" @click="pushUpdate('linux')">
                  一键推送更新
                </el-button>
                <span v-if="linuxPushResult" class="push-result" :class="{ success: linuxPushResult.includes('成功') }">{{ linuxPushResult }}</span>
              </div>
            </div>
          </div>

          <!-- Windows Agent -->
          <div class="agent-update-section" style="margin-top: 20px">
            <div class="platform-title">🪟 Windows Agent</div>
            <div class="info-grid" style="margin-bottom: 16px">
              <div class="info-item">
                <span class="info-label">二进制状态</span>
                <span class="info-value">{{ winBinInfo.exists ? `已上传 (${(winBinInfo.size / 1024 / 1024).toFixed(2)} MB)` : '未上传' }}</span>
              </div>
              <div class="info-item" v-if="winBinInfo.exists">
                <span class="info-label">上传时间</span>
                <span class="info-value">{{ new Date(winBinInfo.modified).toLocaleString('zh-CN') }}</span>
              </div>
            </div>
            <div class="agent-update-actions">
              <div class="action-card">
                <h4>① 上传 Windows Agent</h4>
                <p>选择编译好的 agent-windows.exe 文件</p>
                <input ref="winFileInput" type="file" style="display:none" @change="handleUpload($event, 'windows')" />
                <el-button type="primary" :loading="winUploading" @click="($refs.winFileInput as HTMLInputElement)?.click()">
                  选择文件并上传
                </el-button>
              </div>
              <div class="action-card">
                <h4>② 推送到 Windows Agent</h4>
                <p>向所有在线 Windows Agent 推送更新</p>
                <el-button type="warning" :loading="winPushing" :disabled="!winBinInfo.exists" @click="pushUpdate('windows')">
                  一键推送更新
                </el-button>
                <span v-if="winPushResult" class="push-result" :class="{ success: winPushResult.includes('成功') }">{{ winPushResult }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 系统信息 -->
        <div v-if="activeTab === 'system'">
          <div class="content-header">
            <h3>系统信息</h3>
          </div>
          <div class="info-grid">
            <div class="info-item">
              <span class="info-label">版本</span>
              <span class="info-value">v1.0.0</span>
            </div>
            <div class="info-item">
              <span class="info-label">采集间隔</span>
              <span class="info-value">10 秒</span>
            </div>
            <div class="info-item">
              <span class="info-label">SSH 超时</span>
              <span class="info-value">5 秒</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 添加黑名单对话框 -->
    <el-dialog v-model="showBlacklistDialog" title="添加 IP 封禁" width="420px">
      <el-form :model="blForm" label-position="top">
        <el-form-item label="IP 地址" required>
          <el-input v-model="blForm.ip" placeholder="如 1.2.3.4" />
        </el-form-item>
        <el-form-item label="封禁原因">
          <el-input v-model="blForm.reason" placeholder="可选" />
        </el-form-item>
        <el-form-item label="封禁时长">
          <el-select v-model="blForm.duration" style="width:100%">
            <el-option :value="0" label="永久封禁" />
            <el-option :value="1" label="1 小时" />
            <el-option :value="6" label="6 小时" />
            <el-option :value="24" label="24 小时" />
            <el-option :value="72" label="3 天" />
            <el-option :value="168" label="7 天" />
            <el-option :value="720" label="30 天" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showBlacklistDialog = false">取消</el-button>
        <el-button type="danger" @click="addBlacklist">封禁</el-button>
      </template>
    </el-dialog>

    <!-- 添加/编辑服务器对话框 -->
    <el-dialog
      v-model="showAddDialog"
      :title="editingServer ? '编辑服务器' : '添加服务器'"
      width="720px"
      top="5vh"
      class="server-dialog"
      @close="resetForm"
    >
      <el-form :model="serverForm" label-position="top" size="default">
        <div class="form-section">
          <div class="section-title"><span class="dot" />基本信息</div>
          <div class="form-grid">
            <el-form-item label="名称" required>
              <el-input v-model="serverForm.name" placeholder="如：Web-主站" />
            </el-form-item>
            <el-form-item label="分组">
              <el-input v-model="serverForm.group" placeholder="如：生产" />
            </el-form-item>
            <el-form-item label="主机地址" required>
              <el-input v-model="serverForm.host" placeholder="IP 或域名" />
            </el-form-item>
            <el-form-item label="端口">
              <el-input-number v-model="serverForm.port" :min="1" :max="65535" style="width:100%" />
            </el-form-item>
            <el-form-item label="连接方式">
              <el-select v-model="serverForm.connectMethod" style="width:100%">
                <el-option value="ssh" label="SSH" />
                <el-option value="agent" label="Agent" />
                <el-option value="plugin" label="插件" />
                <el-option value="api" label="API" />
              </el-select>
            </el-form-item>
            <el-form-item label="用户名" required v-if="serverForm.connectMethod === 'ssh'">
              <el-input v-model="serverForm.username" placeholder="root" />
            </el-form-item>
            <el-form-item label="密码" v-if="serverForm.connectMethod === 'ssh'" class="form-span-2">
              <el-input v-model="serverForm.authValue" type="password" show-password placeholder="SSH 密码" />
            </el-form-item>
          </div>
        </div>

        <!-- Agent/插件 安装说明 -->
        <div v-if="(serverForm.connectMethod === 'agent' || serverForm.connectMethod === 'plugin') && (editingServer?.agentToken || savedAgentToken)" class="agent-info">
          <div class="section-title">
            <span class="dot cyan" />Agent 安装说明
          </div>
          <div class="agent-token-row">
            <span class="agent-label">Agent Token</span>
            <div class="cmd-box">
              <code class="agent-token">{{ editingServer?.agentToken || savedAgentToken }}</code>
              <button type="button" class="copy-btn" @click="copyText(editingServer?.agentToken || savedAgentToken)">复制</button>
            </div>
          </div>

          <div class="install-tabs">
            <button
              v-for="tab in installTabs"
              :key="tab.key"
              type="button"
              class="install-tab"
              :class="{ active: activeInstallTab === tab.key }"
              @click="activeInstallTab = tab.key"
            >
              <span class="os-icon" :class="tab.os" />{{ tab.label }}
            </button>
          </div>

          <!-- Linux -->
          <div v-show="activeInstallTab === 'linux'" class="install-panel">
            <div class="agent-field">
              <span class="agent-label">一键安装（带 Token）</span>
              <div class="cmd-box">
                <code class="agent-cmd">curl -fsSL '{{ serverOrigin }}/api/agent/install.sh?key={{ installKey }}&token={{ editingServer?.agentToken || savedAgentToken }}' | bash</code>
                <button type="button" class="copy-btn" @click="copyText(`curl -fsSL '${serverOrigin}/api/agent/install.sh?key=${installKey}&token=${editingServer?.agentToken || savedAgentToken}' | bash`)">复制</button>
              </div>
            </div>
            <div class="agent-field">
              <span class="agent-label">一键安装（免 Token）</span>
              <div class="cmd-box">
                <code class="agent-cmd">curl -fsSL '{{ serverOrigin }}/api/agent/install.sh?key={{ installKey }}' | bash</code>
                <button type="button" class="copy-btn" @click="copyText(`curl -fsSL '${serverOrigin}/api/agent/install.sh?key=${installKey}' | bash`)">复制</button>
              </div>
            </div>
          </div>

          <!-- Windows -->
          <div v-show="activeInstallTab === 'windows'" class="install-panel">
            <div class="agent-field">
              <span class="agent-label">一键安装（带 Token）</span>
              <div class="cmd-box">
                <code class="agent-cmd">irm '{{ serverOrigin }}/api/agent/install.ps1?key={{ installKey }}&token={{ editingServer?.agentToken || savedAgentToken }}' | iex</code>
                <button type="button" class="copy-btn" @click="copyText(`irm '${serverOrigin}/api/agent/install.ps1?key=${installKey}&token=${editingServer?.agentToken || savedAgentToken}' | iex`)">复制</button>
              </div>
            </div>
            <div class="agent-field">
              <span class="agent-label">一键安装（免 Token）</span>
              <div class="cmd-box">
                <code class="agent-cmd">irm '{{ serverOrigin }}/api/agent/install.ps1?key={{ installKey }}' | iex</code>
                <button type="button" class="copy-btn" @click="copyText(`irm '${serverOrigin}/api/agent/install.ps1?key=${installKey}' | iex`)">复制</button>
              </div>
            </div>
          </div>

          <div class="agent-tip">
            <el-icon><InfoFilled /></el-icon>
            <span>免 Token 模式下 Agent 首次启动会自动注册到服务端，无需手动创建服务器。需先在"Agent 更新"页上传对应平台的二进制文件。</span>
          </div>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="testConnection" :loading="testing">测试连接</el-button>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" @click="saveServer" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { serverApi, alertRuleApi, authApi, securityApi, notifyApi, agentUpdateApi, systemApi } from '@/api'
import { ElMessage } from 'element-plus'
import { InfoFilled } from '@element-plus/icons-vue'
import type { ServerInfo } from '@/api'

const installTabs = [
  { key: 'linux', label: 'Linux', os: 'linux' },
  { key: 'windows', label: 'Windows', os: 'windows' },
]
const activeInstallTab = ref('linux')

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制')
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
    ElMessage.success('已复制')
  }
}

const activeTab = ref('servers')
const menuItems = [
  { key: 'servers', label: '服务器管理', icon: 'Monitor' },
  { key: 'rules', label: '告警规则', icon: 'Warning' },
  { key: 'notify', label: '消息推送', icon: 'Bell' },
  { key: 'agent-update', label: 'Agent 更新', icon: 'Upload' },
  { key: 'password', label: '修改密码', icon: 'Lock' },
  { key: 'blacklist', label: 'IP 黑名单', icon: 'CircleClose' },
  { key: 'logs', label: '安全日志', icon: 'Document' },
  { key: 'system', label: '系统信息', icon: 'InfoFilled' },
]

const loading = ref(false)
const servers = ref<ServerInfo[]>([])
const rulesLoading = ref(false)
const rules = ref<any[]>([])
const showAddDialog = ref(false)
const editingServer = ref<any>(null)
const testing = ref(false)
const saving = ref(false)
const savedAgentToken = ref('')
const serverOrigin = window.location.origin
const installKey = ref('')

async function fetchInstallKey() {
  try {
    const res: any = await systemApi.getConfig()
    if (res.success && res.data?.installKey) {
      installKey.value = res.data.installKey
    }
  } catch {}
}

// Agent 更新 (Linux + Windows)
const linuxFileInput = ref<HTMLInputElement>()
const winFileInput = ref<HTMLInputElement>()
const linuxBinInfo = reactive({ exists: false, size: 0, modified: '' })
const winBinInfo = reactive({ exists: false, size: 0, modified: '' })
const linuxUploading = ref(false)
const winUploading = ref(false)
const linuxPushing = ref(false)
const winPushing = ref(false)
const linuxPushResult = ref('')
const winPushResult = ref('')

async function fetchAgentBinInfo() {
  try {
    const res: any = await agentUpdateApi.info()
    if (res.success) {
      if (res.data.linux) Object.assign(linuxBinInfo, res.data.linux)
      if (res.data.windows) Object.assign(winBinInfo, res.data.windows)
    }
  } catch {}
}

async function handleUpload(e: Event, platform: string) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const uploading = platform === 'linux' ? linuxUploading : winUploading
  const inputRef = platform === 'linux' ? linuxFileInput : winFileInput
  uploading.value = true
  try {
    const res: any = await agentUpdateApi.upload(file, platform)
    if (res.success) {
      ElMessage.success(`${platform} Agent 上传成功 (${(res.data.size / 1024 / 1024).toFixed(2)} MB)`)
      fetchAgentBinInfo()
    } else {
      ElMessage.error(res.error || '上传失败')
    }
  } catch (err: any) {
    ElMessage.error('上传失败: ' + (err.message || err))
  } finally {
    uploading.value = false
    if (inputRef.value) inputRef.value.value = ''
  }
}

async function pushUpdate(platform: string) {
  const pushing = platform === 'linux' ? linuxPushing : winPushing
  const pushResult = platform === 'linux' ? linuxPushResult : winPushResult
  pushing.value = true
  pushResult.value = ''
  try {
    const res: any = await agentUpdateApi.pushUpdate(platform)
    if (res.success) {
      pushResult.value = `推送成功，已发送到 ${res.data.sent} 个 Agent`
      ElMessage.success(pushResult.value)
    } else {
      pushResult.value = res.error || '推送失败'
      ElMessage.error(pushResult.value)
    }
  } catch (err: any) {
    pushResult.value = '推送失败: ' + (err.message || err)
    ElMessage.error(pushResult.value)
  } finally {
    pushing.value = false
  }
}

const serverForm = reactive({
  name: '',
  host: '',
  port: 22,
  username: 'root',
  authValue: '',
  connectMethod: 'ssh',
  group: '',
})

async function fetchServers() {
  loading.value = true
  try {
    const res: any = await serverApi.list()
    if (res.success) servers.value = res.data || []
  } finally {
    loading.value = false
  }
}

async function fetchRules() {
  rulesLoading.value = true
  try {
    const res: any = await alertRuleApi.list()
    if (res.success) rules.value = res.data || []
  } finally {
    rulesLoading.value = false
  }
}

function editServer(row: any) {
  editingServer.value = row
  serverForm.name = row.name
  serverForm.host = row.host
  serverForm.port = row.port
  serverForm.username = row.username
  serverForm.connectMethod = row.connectMethod || 'ssh'
  serverForm.group = row.group
  serverForm.authValue = ''
  showAddDialog.value = true
}

function resetForm() {
  editingServer.value = null
  serverForm.name = ''
  serverForm.host = ''
  serverForm.port = 22
  serverForm.connectMethod = 'ssh'
  serverForm.username = 'root'
  serverForm.authValue = ''
  serverForm.group = ''
  savedAgentToken.value = ''
}

async function saveServer() {
  if (!serverForm.name || !serverForm.host) {
    ElMessage.warning('请填写必填项')
    return
  }
  if (serverForm.connectMethod === 'ssh' && !editingServer.value && !serverForm.authValue) {
    ElMessage.warning('SSH 连接需要填写密码')
    return
  }
  saving.value = true
  try {
    if (editingServer.value) {
      await serverApi.update(editingServer.value.id, serverForm)
      ElMessage.success('更新成功')
    } else {
      const res: any = await serverApi.create(serverForm)
      ElMessage.success('创建成功')
      const isAgent = serverForm.connectMethod === 'agent' || serverForm.connectMethod === 'plugin'
      if (isAgent && res?.data?.agentToken) {
        savedAgentToken.value = res.data.agentToken
      }
    }
    showAddDialog.value = false
    fetchServers()
  } catch (e: any) {
    console.error('保存失败:', e)
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

async function deleteServer(id: string) {
  await serverApi.remove(id)
  ElMessage.success('已删除')
  fetchServers()
}

async function testConnection() {
  testing.value = true
  try {
    const res: any = await serverApi.testNew(serverForm)
    if (res.success && res.data.connected) {
      ElMessage.success(`连接成功，延迟 ${res.data.latency}ms`)
    } else {
      ElMessage.error('连接失败')
    }
  } catch {
    ElMessage.error('连接测试失败')
  } finally {
    testing.value = false
  }
}

async function updateRule(rule: any) {
  await alertRuleApi.update(rule.id, {
    warningThreshold: rule.warningThreshold,
    criticalThreshold: rule.criticalThreshold,
    enabled: rule.enabled,
  })
  ElMessage.success('规则已更新')
}

// ── 消息推送 ──
const notifyConfig = reactive({
  enabled: false,
  webhookUrl: '',
  notifyWarning: true,
  notifyCritical: true,
  notifyOffline: true,
  cooldownMin: 10,
})
const notifyLoading = ref(false)
const notifySaving = ref(false)
const notifyTesting = ref(false)

async function fetchNotifyConfig() {
  notifyLoading.value = true
  try {
    const res: any = await notifyApi.getConfig()
    if (res.success && res.data) {
      Object.assign(notifyConfig, res.data)
    }
  } finally {
    notifyLoading.value = false
  }
}

async function saveNotifyConfig() {
  notifySaving.value = true
  try {
    const res: any = await notifyApi.updateConfig(notifyConfig)
    if (res.success) {
      ElMessage.success('配置已保存')
    } else {
      ElMessage.error(res.error || '保存失败')
    }
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || '保存失败')
  } finally {
    notifySaving.value = false
  }
}

async function testNotify() {
  if (!notifyConfig.webhookUrl) {
    ElMessage.warning('请先填写 Webhook URL')
    return
  }
  notifyTesting.value = true
  try {
    const res: any = await notifyApi.test()
    if (res.success) {
      ElMessage.success('测试消息已发送，请检查企业微信')
    } else {
      ElMessage.error(res.error || '发送失败')
    }
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || '发送失败')
  } finally {
    notifyTesting.value = false
  }
}

// ── 修改密码 ──
const pwdForm = reactive({ oldPassword: '', newPassword: '', confirmPassword: '' })
const pwdLoading = ref(false)

async function changePassword() {
  if (!pwdForm.oldPassword || !pwdForm.newPassword) {
    ElMessage.warning('请填写密码')
    return
  }
  if (pwdForm.newPassword.length < 6) {
    ElMessage.warning('新密码至少 6 位')
    return
  }
  if (pwdForm.newPassword !== pwdForm.confirmPassword) {
    ElMessage.warning('两次密码不一致')
    return
  }
  pwdLoading.value = true
  try {
    const res: any = await authApi.changePassword(pwdForm.oldPassword, pwdForm.newPassword)
    if (res.success) {
      ElMessage.success('密码修改成功，请重新登录')
      pwdForm.oldPassword = ''
      pwdForm.newPassword = ''
      pwdForm.confirmPassword = ''
      localStorage.removeItem('token')
      setTimeout(() => location.reload(), 1000)
    } else {
      ElMessage.error(res.error || '修改失败')
    }
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || '修改失败')
  } finally {
    pwdLoading.value = false
  }
}

// ── 黑名单 ──
const blacklist = ref<any[]>([])
const blLoading = ref(false)
const showBlacklistDialog = ref(false)
const blForm = reactive({ ip: '', reason: '', duration: 0 })

async function fetchBlacklist() {
  blLoading.value = true
  try {
    const res: any = await securityApi.getBlacklist()
    if (res.success) blacklist.value = res.data || []
  } finally {
    blLoading.value = false
  }
}

async function addBlacklist() {
  if (!blForm.ip) {
    ElMessage.warning('请输入 IP 地址')
    return
  }
  try {
    const res: any = await securityApi.addBlacklist({ ip: blForm.ip, reason: blForm.reason, duration: blForm.duration })
    if (res.success) {
      ElMessage.success('已封禁')
      showBlacklistDialog.value = false
      blForm.ip = ''
      blForm.reason = ''
      blForm.duration = 0
      fetchBlacklist()
    } else {
      ElMessage.error(res.error || '操作失败')
    }
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || '操作失败')
  }
}

async function removeBlacklist(id: number) {
  await securityApi.removeBlacklist(id)
  ElMessage.success('已解封')
  fetchBlacklist()
}

// ── 安全日志 ──
const logTab = ref('security')
const securityLogs = ref<any[]>([])
const loginAttempts = ref<any[]>([])
const logLoading = ref(false)

async function fetchSecurityLogs() {
  logLoading.value = true
  try {
    const res: any = await securityApi.getLogs()
    if (res.success) securityLogs.value = res.data || []
  } finally {
    logLoading.value = false
  }
}

async function fetchLoginAttempts() {
  logLoading.value = true
  try {
    const res: any = await securityApi.getLoginAttempts()
    if (res.success) loginAttempts.value = res.data || []
  } finally {
    logLoading.value = false
  }
}

function formatTime(t: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN', { hour12: false })
}

function actionTagType(action: string): string {
  if (action.includes('fail') || action.includes('ban') || action.includes('locked')) return 'danger'
  if (action.includes('success') || action.includes('changed')) return 'success'
  if (action.includes('add') || action.includes('remove')) return 'warning'
  return 'info'
}

onMounted(() => {
  fetchServers()
  fetchRules()
  fetchNotifyConfig()
  fetchBlacklist()
  fetchSecurityLogs()
  fetchAgentBinInfo()
  fetchInstallKey()
})
</script>

<style scoped lang="scss">
.settings-page {
  padding: 16px 20px;
  max-width: 1200px;
  margin: 0 auto;
}

.settings-layout {
  display: flex;
  gap: 14px;
}

.settings-menu {
  width: 170px;
  flex-shrink: 0;
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 6px;
}

.menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  color: var(--t2);
  transition: color 0.2s, background 0.2s;

  &:hover { background: rgba(255,255,255,0.03); color: var(--t1); }
  &.active { background: rgba(45, 124, 246, 0.1); color: var(--c-blue); }
}

.settings-content {
  flex: 1;
  min-width: 0;
}

.content-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;

  h3 { font-size: 14px; font-weight: 600; }
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.info-item {
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-label {
  font-size: 11px;
  color: var(--t3);
}

.info-value {
  font-size: 13px;
  font-weight: 500;
  color: var(--t1);
}

/* ===== 服务器编辑弹窗 ===== */
.form-section {
  margin-bottom: 18px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--t1);
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);

  .dot {
    width: 3px;
    height: 14px;
    background: linear-gradient(180deg, #3b82f6, #8b5cf6);
    border-radius: 2px;

    &.cyan { background: linear-gradient(180deg, #06b6d4, #22d3ee); }
  }
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px 16px;

  .form-span-2 { grid-column: 1 / -1; }

  :deep(.el-form-item) { margin-bottom: 0; }
  :deep(.el-form-item__label) {
    font-size: 12px;
    font-weight: 500;
    padding-bottom: 4px;
    line-height: 1.4;
  }
}

.agent-info {
  margin-top: 18px;
  padding: 16px;
  background: linear-gradient(135deg, rgba(6,182,212,0.04), rgba(139,92,246,0.04));
  border: 1px solid rgba(6,182,212,0.15);
  border-radius: 10px;
}

.agent-token-row {
  margin-bottom: 16px;
}

.agent-field {
  margin-bottom: 12px;

  &:last-child { margin-bottom: 0; }
}

.agent-label {
  display: block;
  font-size: 11px;
  font-weight: 600;
  color: var(--t2);
  margin-bottom: 6px;
  letter-spacing: 0.3px;
}

.cmd-box {
  position: relative;
  display: flex;
  align-items: stretch;
  background: rgba(0,0,0,0.22);
  border: 1px solid rgba(255,255,255,0.05);
  border-radius: 6px;
  overflow: hidden;
  transition: border-color .15s;

  &:hover {
    border-color: rgba(6,182,212,0.3);
  }
}

.agent-token,
.agent-cmd {
  flex: 1;
  display: block;
  font-size: 12px;
  font-family: 'JetBrains Mono', 'SF Mono', 'Courier New', monospace;
  color: #67e8f9;
  padding: 9px 12px;
  word-break: break-all;
  user-select: all;
  line-height: 1.5;
}

.copy-btn {
  flex-shrink: 0;
  min-width: 48px;
  padding: 0 12px;
  background: rgba(6,182,212,0.12);
  border: none;
  border-left: 1px solid rgba(255,255,255,0.06);
  color: #67e8f9;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: background .15s;

  &:hover { background: rgba(6,182,212,0.25); }
  &:active { background: rgba(6,182,212,0.35); }
}

.install-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 12px;
  padding: 3px;
  background: rgba(0,0,0,0.15);
  border-radius: 7px;
  width: fit-content;
}

.install-tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  background: transparent;
  border: none;
  border-radius: 5px;
  font-size: 12px;
  font-weight: 500;
  color: var(--t3);
  cursor: pointer;
  transition: all .15s;

  &:hover { color: var(--t1); }
  &.active {
    background: rgba(6,182,212,0.18);
    color: #67e8f9;
    box-shadow: 0 1px 3px rgba(0,0,0,0.2);
  }
}

.os-icon {
  width: 14px;
  height: 14px;
  background-repeat: no-repeat;
  background-position: center;
  background-size: contain;
  opacity: 0.8;

  &.linux {
    background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='%2367e8f9'><path d='M12.504 2C9.748 2 7.508 4.24 7.508 6.996c0 .67.132 1.31.371 1.892-.78.55-1.381 1.402-1.631 2.363-.48 1.838.371 3.727 1.971 4.488-.03.389.03 1.04.39 1.602.24.429.66.79 1.17 1.04-.03.66.19 1.312.63 1.842.48.55 1.199.89 2.04.97.179.23.419.45.77.6.689.35 1.711.35 2.4 0 .35-.15.59-.37.77-.6.84-.08 1.56-.42 2.04-.97.44-.53.66-1.181.63-1.842.51-.25.93-.61 1.17-1.04.36-.561.42-1.212.39-1.601 1.6-.761 2.45-2.65 1.97-4.488-.25-.961-.851-1.813-1.631-2.363.24-.582.371-1.221.371-1.892C17.5 4.24 15.26 2 12.504 2zm-.051 3c.829 0 1.5.672 1.5 1.5 0 .83-.67 1.5-1.5 1.5-.83 0-1.5-.67-1.5-1.5 0-.828.67-1.5 1.5-1.5zm-3.949 1.752c.44 0 .8.36.8.8 0 .44-.36.8-.8.8-.44 0-.8-.36-.8-.8 0-.44.36-.8.8-.8zm8 0c.44 0 .8.36.8.8 0 .44-.36.8-.8.8-.44 0-.8-.36-.8-.8 0-.44.36-.8.8-.8zM12 14c1.75 0 3 .75 3 1.5S13.75 17 12 17s-3-.75-3-1.5.75-1.5 3-1.5z'/></svg>");
  }
  &.windows {
    background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='%2367e8f9'><path d='M3 5.5L10.5 4.5v7H3v-6zm0 7h7.5v7L3 18.5v-6zm8.5-8.2L21 3v8.5h-9.5V4.3zm0 8.2H21V21l-9.5-1.3V12.5z'/></svg>");
  }
}

.install-panel {
  padding: 4px 0;
}

.agent-tip {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin-top: 14px;
  padding: 10px 12px;
  background: rgba(245, 158, 11, 0.08);
  border: 1px solid rgba(245, 158, 11, 0.2);
  border-radius: 6px;
  font-size: 11.5px;
  line-height: 1.5;
  color: #fbbf24;

  .el-icon { font-size: 14px; flex-shrink: 0; margin-top: 1px; }
}

:deep(.server-dialog) {
  .el-dialog__body {
    max-height: 75vh;
    overflow-y: auto;
    padding: 16px 24px;
  }
  .el-dialog__header {
    padding: 16px 24px;
    margin: 0;
    border-bottom: 1px solid var(--border);
  }
  .el-dialog__footer {
    padding: 12px 24px;
    border-top: 1px solid var(--border);
  }
}

/* ===== Agent 更新 ===== */
.platform-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--t1);
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid rgba(255,255,255,0.06);
}
.agent-update-actions {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}
.action-card {
  background: rgba(255,255,255,0.04);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 10px;
  padding: 20px;
  h4 { margin: 0 0 8px; font-size: 15px; color: #e8edf5; font-weight: 600; }
  p { margin: 0 0 16px; font-size: 13px; color: #94a3b8; line-height: 1.5; }
}
.push-result {
  display: inline-block;
  margin-left: 12px;
  font-size: 13px;
  color: #f59e0b;
  &.success { color: #10b981; }
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .settings-page { padding: 10px 12px; }
  .settings-layout { flex-direction: column; gap: 10px; }
  .settings-menu {
    width: 100%;
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    padding: 4px;
  }
  .menu-item { padding: 6px 10px; font-size: 11px; }
  .info-grid { grid-template-columns: repeat(2, 1fr); gap: 6px; }
  .info-item { padding: 10px 12px; }
  .content-header { margin-bottom: 10px; h3 { font-size: 13px; } }
}

@media (max-width: 480px) {
  .settings-page { padding: 8px; }
  .info-grid { grid-template-columns: 1fr; }
}
</style>

<style lang="scss">
/* Settings Light Theme */
html.light .settings-menu {
  background: rgba(255,255,255,0.95);
  box-shadow: 0 1px 6px rgba(0,0,0,0.08);
  border-color: rgba(0,0,0,0.1);
}
html.light .menu-item {
  &:hover { background: rgba(0,0,0,0.05); }
  &.active { background: rgba(37,99,235,0.1); color: #1d4ed8; }
}
html.light .info-item {
  background: rgba(255,255,255,0.95);
  box-shadow: 0 1px 6px rgba(0,0,0,0.08);
  border-color: rgba(0,0,0,0.1);
}
html.light .agent-info {
  background: linear-gradient(135deg, rgba(6,182,212,0.04), rgba(139,92,246,0.04));
  border-color: rgba(6,182,212,0.25);
}
html.light .cmd-box {
  background: #f1f5f9;
  border-color: rgba(0,0,0,0.08);
  &:hover { border-color: rgba(6,182,212,0.4); }
}
html.light .agent-token,
html.light .agent-cmd {
  color: #0891b2;
}
html.light .copy-btn {
  background: rgba(6,182,212,0.1);
  border-left-color: rgba(0,0,0,0.06);
  color: #0891b2;
  &:hover { background: rgba(6,182,212,0.2); }
}
html.light .install-tabs {
  background: rgba(0,0,0,0.05);
}
html.light .install-tab.active {
  background: #fff;
  color: #0891b2;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
}
html.light .agent-tip {
  background: rgba(245, 158, 11, 0.08);
  color: #b45309;
}
html.light .section-title {
  border-bottom-color: rgba(0,0,0,0.08);
}
html.light .platform-title {
  border-bottom-color: rgba(0,0,0,0.08);
}
html.light .action-card {
  background: rgba(0,0,0,0.02);
  border-color: rgba(0,0,0,0.1);
  h4 { color: #1e293b; }
  p { color: #64748b; }
}
</style>
