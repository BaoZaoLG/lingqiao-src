let currentAuditPage = 1;
let selectedCards = new Set();
let _extendCode = '';
let _cards = [];
let _autoRefresh = true;
let _lastSessionCount = 0;
let _notifPermission = false;

// ── XSS Protection ──────────────────────────────────────────────
function esc(s) {
  if (s == null) return '';
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');
}

// ── Toast ───────────────────────────────────────────────────────
function showToast(msg, type) {
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.className = 'toast ' + (type || 'success') + ' show';
  setTimeout(() => t.className = 'toast', 3500);
}

// ── Notification ────────────────────────────────────────────────
function requestNotifPermission() {
  if ('Notification' in window && Notification.permission === 'default') {
    Notification.requestPermission().then(p => { _notifPermission = (p === 'granted'); });
  } else if ('Notification' in window && Notification.permission === 'granted') {
    _notifPermission = true;
  }
}
function showBrowserNotif(title, body) {
  if (!_notifPermission) return;
  try { new Notification(title, { body, icon: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">&#x1F512;</text></svg>' }); } catch(e) {}
}
requestNotifPermission();

// ── Clipboard ───────────────────────────────────────────────────
function copyToClipboard(text, label) {
  navigator.clipboard.writeText(text).then(() => {
    showToast((label || '已复制') + ': ' + text);
  }).catch(() => showToast(text));
}

// ── Keyboard Shortcuts ──────────────────────────────────────────
document.addEventListener('keydown', e => {
  // Ctrl+K / Cmd+K → focus search
  if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
    e.preventDefault();
    const searchInput = document.querySelector('.page.active input[type="text"]');
    if (searchInput) { searchInput.focus(); searchInput.select(); }
  }
  // Escape → close modal
  if (e.key === 'Escape') {
    document.querySelectorAll('.modal-overlay.show').forEach(m => m.classList.remove('show'));
  }
});

function goLogin() {
  document.getElementById('loginScreen').style.display = 'flex';
  document.getElementById('app').classList.remove('show');
}

async function api(path, body) {
  const opts = { headers: {} };
  if (body) { opts.method = 'POST'; opts.headers['Content-Type'] = 'application/json'; opts.body = JSON.stringify(body); }
  const res = await fetch(path, opts);
  const text = await res.text();
  try { const data = JSON.parse(text); if (data.status === 'error') throw new Error(data.message); return data; }
  catch (e) { if (e instanceof SyntaxError) throw new Error('服务器响应异常 (HTTP ' + res.status + ')'); throw e; }
}

async function adminApi(path, body) {
  const opts = { credentials: 'include', headers: {} };
  if (body) { opts.method = 'POST'; opts.headers['Content-Type'] = 'application/json'; opts.body = JSON.stringify(body); }
  const res = await fetch(path, opts);
  if (res.status === 401) { goLogin(); throw new Error('登录已过期，请重新登录'); }
  if (res.status === 429) { throw new Error('请求过于频繁，请稍后再试'); }
  const text = await res.text();
  try { const data = JSON.parse(text); if (data.status === 'error') throw new Error(data.message); return data; }
  catch (e) { if (e instanceof SyntaxError) throw new Error('服务器响应异常 (HTTP ' + res.status + ')'); throw e; }
}

async function doLogin() {
  const pw = document.getElementById('loginPassword').value;
  if (!pw) return;
  try {
    await api('/admin/api/login', { password: pw });
    document.getElementById('loginScreen').style.display = 'none';
    document.getElementById('app').classList.add('show');
    loadDashboard();
    loadAnnouncement();
  } catch (e) {
    const el = document.getElementById('loginError');
    el.textContent = e.message === 'wrong password' ? '密码错误' : e.message;
    el.style.display = 'block';
  }
}

async function checkAuth() {
  try {
    await adminApi('/admin/api/dashboard');
    document.getElementById('loginScreen').style.display = 'none';
    document.getElementById('app').classList.add('show');
    loadDashboard();
    loadAnnouncement();
  } catch (e) { /* not logged in */ }
}
checkAuth();

function switchPage(name, el) {
  document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
  if (el) el.classList.add('active');
  document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
  document.getElementById('page-' + name).classList.add('active');
  const loaders = {
    dashboard: loadDashboard, cards: loadCards, sessions: loadSessions,
    machines: loadMachines, blacklist: loadBlacklist, announcement: loadAnnouncement,
    version: loadVersionPush, password: function(){}, audit: () => loadAudit(1), agents: loadAgents, invites: loadInvites, settings: function(){}
  };
  if (loaders[name]) loaders[name]();
}
function switchToPage(name) {
  const idx = { dashboard: 0, cards: 1, sessions: 2, machines: 3, blacklist: 4, announcement: 5, version: 6, password: 7, audit: 8, agents: 9, settings: 10 }[name];
  switchPage(name, document.querySelectorAll('.nav-item')[idx]);
}
function toggleSidebar() {
  const sb = document.querySelector('.sidebar');
  const ov = document.getElementById('sidebarOverlay');
  sb.classList.toggle('open');
  ov.classList.toggle('show');
}
// Close sidebar when clicking a nav item on mobile
document.querySelectorAll('.nav-item').forEach(item => {
  item.addEventListener('click', () => {
    if (window.innerWidth <= 768) toggleSidebar();
  });
});
function closeModal(id) { document.getElementById(id).classList.remove('show'); }

// ========== Dashboard ==========
async function loadDashboard() {
  try {
    const data = await adminApi('/admin/api/dashboard');
    const stats = await adminApi('/admin/api/stats').catch(() => ({}));
    document.getElementById('dashboardStats').innerHTML = `
      <div class="stat-card"><div class="label">总卡密数</div><div class="value accent">${data.total_cards}</div></div>
      <div class="stat-card"><div class="label">有效卡密</div><div class="value success">${data.active_cards}</div></div>
      <div class="stat-card"><div class="label">已过期</div><div class="value danger">${data.expired_cards}</div></div>
      <div class="stat-card"><div class="label">在线会话</div><div class="value warning">${data.active_sessions}</div></div>
      <div class="stat-card"><div class="label">总请求数</div><div class="value" style="color:var(--text-2)">${(stats.request_count || 0).toLocaleString()}</div></div>
      <div class="stat-card"><div class="label">运行时长</div><div class="value" style="color:var(--text-2)">${formatUptime(stats.uptime_seconds || 0)}</div></div>
    `;
  } catch (e) { showToast(e.message, 'error'); }
}
function formatUptime(s) {
  if (s < 60) return s + '秒'; if (s < 3600) return Math.floor(s / 60) + '分钟';
  if (s < 86400) return Math.floor(s / 3600) + '小时'; return Math.floor(s / 86400) + '天';
}

// ========== Cards ==========
async function loadCards() {
  try {
    const status = document.getElementById('cardStatusFilter')?.value || '';
    const search = document.getElementById('cardSearch')?.value || '';
    let url = '/admin/api/cards';
    const params = [];
    if (status) params.push('status=' + status);
    if (search) params.push('search=' + encodeURIComponent(search));
    if (params.length) url += '?' + params.join('&');
    const data = await adminApi(url);
    _cards = data.cards || [];
    renderCards();
  } catch (e) { showToast(e.message, 'error'); }
}
function renderCards() {
  const q = (document.getElementById('cardSearch')?.value || '').toUpperCase();
  const cards = _cards.filter(c => {
    if (q && !c.code.includes(q) && !(c.note || '').toUpperCase().includes(q)) return false;
    return true;
  });
  const tbody = document.getElementById('cardsTable');
  if (cards.length === 0) {
    tbody.innerHTML = '<tr><td colspan="10"><div class="empty-state"><div class="icon">&#x1F4ED;</div>暂无匹配卡密</div></td></tr>';
    return;
  }
  tbody.innerHTML = cards.map(c => {
    const isActive = c.status === 'active' && new Date(c.expires_at) > new Date();
    const sc = isActive ? 'badge-active' : c.status === 'disabled' ? 'badge-disabled' : 'badge-expired';
    const st = isActive ? '有效' : c.status === 'disabled' ? '已禁用' : '已过期';
    const bound = c.machine_id ? `<span class="mono" title="${esc(c.machine_id)}">${esc(c.machine_id.substring(0, 14))}...</span>` : '<span style="color:var(--text-3)">未绑定</span>';
    const codeEsc = esc(c.code);
    return `<tr>
      <td><input type="checkbox" value="${codeEsc}" ${selectedCards.has(c.code) ? 'checked' : ''} onchange="toggleCard('${codeEsc}')"></td>
      <td><span class="mono" style="cursor:pointer" title="点击复制" onclick="copyToClipboard('${codeEsc}','卡密')">${codeEsc}</span></td>
      <td><span class="badge ${sc}">${st}</span></td>
      <td>${c.max_sessions || 1}</td>
      <td>${new Date(c.created_at).toLocaleString('zh-CN')}</td>
      <td>${c.activated_at ? new Date(c.activated_at).toLocaleString('zh-CN') : '<span style="color:var(--text-3)">未激活</span>'}</td>
      <td>${new Date(c.expires_at).toLocaleString('zh-CN')}</td>
      <td>${bound}</td>
      <td>${esc(c.note) || '-'}</td>
      <td style="white-space:nowrap">
        <button class="btn btn-sm btn-ghost" onclick="showEditCard('${codeEsc}')">编辑</button>
        <button class="btn btn-sm btn-ghost" onclick="showExtendModal('${codeEsc}')">延期</button>
        ${isActive
          ? `<button class="btn btn-sm btn-danger" onclick="disableCard('${codeEsc}')">禁用</button>`
          : `<button class="btn btn-sm btn-success" onclick="enableCard('${codeEsc}')">启用</button>`}
        <button class="btn btn-sm btn-ghost" onclick="unbindCard('${codeEsc}')">解绑</button>
      </td></tr>`;
  }).join('');
  updateBulkBar();
  document.getElementById('selectAll').checked = false;
}
function toggleSelectAll() {
  const check = document.getElementById('selectAll').checked;
  const cbs = document.querySelectorAll('#cardsTable input[type=checkbox]');
  cbs.forEach(cb => { cb.checked = check; if (check) selectedCards.add(cb.value); else selectedCards.delete(cb.value); });
  updateBulkBar();
}
function toggleCard(code) { if (selectedCards.has(code)) selectedCards.delete(code); else selectedCards.add(code); updateBulkBar(); }
function updateBulkBar() {
  const bar = document.getElementById('bulkBar');
  const count = selectedCards.size;
  if (count > 0) { bar.classList.add('show'); document.getElementById('bulkCount').textContent = '已选 ' + count + ' 项'; }
  else bar.classList.remove('show');
}
async function bulkAction(action) {
  if (selectedCards.size === 0) return;
  const labels = { disable: '批量禁用', enable: '批量启用', expire: '批量过期', unbind: '批量解绑' };
  if (!confirm('确认' + labels[action] + ' ' + selectedCards.size + ' 张卡密？')) return;
  try {
    const data = await adminApi('/admin/api/cards/bulk', { codes: [...selectedCards], action });
    showToast('已' + labels[action] + ' ' + data.affected + ' 张卡密');
    selectedCards.clear(); loadCards();
  } catch (e) { showToast(e.message, 'error'); }
}
async function bulkExtend() {
  if (selectedCards.size === 0) return;
  _extendCode = '';
  document.getElementById('extendCardInfo').textContent = '批量延期 ' + selectedCards.size + ' 张卡密';
  document.getElementById('extendHours').value = '720';
  document.getElementById('extendModal').classList.add('show');
}
function showEditCard(code) {
  const card = _cards.find(c => c.code === code);
  if (!card) return;
  document.getElementById('editCardCode').textContent = code;
  document.getElementById('editCardNote').value = card.note || '';
  document.getElementById('editCardMaxSessions').value = card.max_sessions || 1;
  document.getElementById('editCardModal').classList.add('show');
}
async function saveCardDetails() {
  const code = document.getElementById('editCardCode').textContent;
  const note = document.getElementById('editCardNote').value;
  const ms = parseInt(document.getElementById('editCardMaxSessions').value) || 1;
  try {
    await adminApi('/admin/api/card/update-details', { code, note, max_sessions: ms });
    showToast('卡密信息已更新'); closeModal('editCardModal'); loadCards();
  } catch (e) { showToast(e.message, 'error'); }
}
function showExtendModal(code) {
  _extendCode = code;
  document.getElementById('extendCardInfo').textContent = '延期卡密: ' + code;
  document.getElementById('extendHours').value = '720';
  document.getElementById('extendModal').classList.add('show');
}
async function confirmExtend() {
  const hours = parseInt(document.getElementById('extendHours').value) || 720;
  try {
    if (_extendCode) {
      await adminApi('/admin/api/card/update', { code: _extendCode, action: 'extend', extend_hours: hours });
      showToast('已延期 ' + hours + ' 小时');
    } else {
      const data = await adminApi('/admin/api/cards/bulk', { codes: [...selectedCards], action: 'extend', extend_hours: hours });
      showToast('已延期 ' + data.affected + ' 张卡密 (' + hours + '小时)');
      selectedCards.clear();
    }
    closeModal('extendModal'); loadCards();
  } catch (e) { showToast(e.message, 'error'); }
}
async function disableCard(code) {
  if (!confirm('确认禁用 ' + code + '？')) return;
  try { await adminApi('/admin/api/card/update', { code, action: 'disable' }); showToast('已禁用'); loadCards(); } catch (e) { showToast(e.message, 'error'); }
}
async function enableCard(code) {
  try { await adminApi('/admin/api/card/update', { code, action: 'enable' }); showToast('已启用'); loadCards(); } catch (e) { showToast(e.message, 'error'); }
}
async function unbindCard(code) {
  if (!confirm('确认解绑 ' + code + '？')) return;
  try { await adminApi('/admin/api/card/update', { code, action: 'unbind' }); showToast('已解绑'); loadCards(); } catch (e) { showToast(e.message, 'error'); }
}
async function exportCards() {
  try {
    const res = await fetch('/admin/api/cards/export', { credentials: 'include' });
    const blob = await res.blob(); const a = document.createElement('a');
    a.href = URL.createObjectURL(blob); a.download = 'cards_export.csv'; a.click();
    showToast('导出成功');
  } catch (e) { showToast('导出失败: ' + e.message, 'error'); }
}
function showGenerateModal() { document.getElementById('generateModal').classList.add('show'); }
function showBatchGenerateModal() { document.getElementById('batchGenerateModal').classList.add('show'); }
function showImportModal() { document.getElementById('importModal').classList.add('show'); }
async function generateCard() {
  const d = parseInt(document.getElementById('genDuration').value) || 720;
  const ms = parseInt(document.getElementById('genMaxSessions').value) || 1;
  const note = document.getElementById('genNote').value;
  try {
    const data = await adminApi('/admin/api/card/generate', { duration_hours: d, max_sessions: ms, note });
    navigator.clipboard.writeText(data.card.code).then(() => showToast('卡密已复制: ' + data.card.code)).catch(() => showToast('卡密: ' + data.card.code));
    closeModal('generateModal'); loadCards();
  } catch (e) { showToast(e.message, 'error'); }
}
async function batchGenerateCards() {
  const count = parseInt(document.getElementById('batchCount').value) || 10;
  const d = parseInt(document.getElementById('batchDuration').value) || 720;
  const ms = parseInt(document.getElementById('batchMaxSessions').value) || 1;
  const note = document.getElementById('batchNote').value;
  try {
    const data = await adminApi('/admin/api/card/batch-generate', { count, duration_hours: d, max_sessions: ms, note });
    const codes = data.cards.map(c => c.code).join('\n');
    navigator.clipboard.writeText(codes).then(() => showToast('已生成 ' + data.count + ' 张卡密，已全部复制到剪贴板')).catch(() => showToast('已生成 ' + data.count + ' 张卡密'));
    closeModal('batchGenerateModal'); loadCards();
  } catch (e) { showToast(e.message, 'error'); }
}
async function importCards() {
  const csv = document.getElementById('importCSV').value.trim();
  if (!csv) { showToast('请输入卡密数据', 'warning'); return; }
  const d = parseInt(document.getElementById('importDuration').value) || 720;
  const ms = parseInt(document.getElementById('importMaxSessions').value) || 1;
  try {
    const data = await adminApi('/admin/api/cards/import', { csv, duration: d, max_sessions: ms });
    showToast('已导入 ' + data.imported + ' 张卡密');
    closeModal('importModal'); loadCards();
  } catch (e) { showToast(e.message, 'error'); }
}

// ========== Sessions ==========
async function loadSessions() {
  try {
    const data = await adminApi('/admin/api/sessions');
    const sessions = data.sessions || [];
    const tbody = document.getElementById('sessionsTable');
    if (sessions.length === 0) {
      tbody.innerHTML = '<tr><td colspan="8"><div class="empty-state"><div class="icon">&#x1F310;</div>暂无在线会话</div></td></tr>';
      return;
    }
    const currentCount = sessions.length;
    if (_lastSessionCount > 0 && currentCount > _lastSessionCount) {
      showBrowserNotif('新连接', '新增 ' + (currentCount - _lastSessionCount) + ' 个在线会话');
    }
    _lastSessionCount = currentCount;

    tbody.innerHTML = sessions.map(s => {
      const tok = esc(s.token);
      const card = esc(s.card_code);
      const mid = esc((s.machine_id || '').substring(0, 14));
      return `<tr>
      <td><span class="mono" style="cursor:pointer" title="点击复制" onclick="copyToClipboard('${tok}','Token')">${tok.substring(0, 16)}...</span></td>
      <td><span class="mono" style="cursor:pointer" title="点击复制" onclick="copyToClipboard('${card}','卡密')">${card}</span></td>
      <td><span class="mono">${mid}...</span></td>
      <td>${s.client_version ? '<span class="badge badge-active">v' + esc(s.client_version) + '</span>' : '<span style="color:var(--text-3)">未知</span>'}</td>
      <td>${esc(s.remote_addr)}</td>
      <td>${new Date(s.last_seen).toLocaleString('zh-CN')}</td>
      <td>${new Date(s.expires_at).toLocaleString('zh-CN')}</td>
      <td><button class="btn btn-sm btn-danger" onclick="forceLogout('${tok}')">强制下线</button></td></tr>`;
    }).join('');
  } catch (e) { showToast(e.message, 'error'); }
}
async function forceLogout(st) {
  if (!confirm('确认强制下线？')) return;
  try { await adminApi('/admin/api/force-logout', { session_token: st }); showToast('已下线'); loadSessions(); } catch (e) { showToast(e.message, 'error'); }
}

// ========== Machines ==========
async function loadMachines() {
  try {
    const data = await adminApi('/admin/api/machines');
    const machines = data.machines || [];
    const div = document.getElementById('machinesList');
    if (machines.length === 0) {
      div.innerHTML = '<div class="empty-state"><div class="icon">&#x1F4BB;</div>暂无绑定机器</div>';
      return;
    }
    div.innerHTML = machines.map(m => {
      const mid = esc(m.machine_id);
      return `
      <div class="machine-card" onclick="showMachineDetail('${mid}')">
        <span style="font-size:26px">&#x1F4BB;</span>
        <div style="flex:1">
          <div style="font-weight:600;color:var(--text);font-size:13px"><span class="mono">${esc(m.machine_id.substring(0, 20))}...</span></div>
          <div style="color:var(--text-2);font-size:12px;margin-top:3px">绑定卡密: ${m.card_count} 张${m.last_seen ? ' &middot; 最后活跃: ' + new Date(m.last_seen).toLocaleString('zh-CN') : ''}${m.is_blacklisted ? ' &middot; <span style="color:var(--danger)">已封禁</span>' : ''}</div>
        </div>
        <span style="color:var(--text-3)">&#x25B6;</span>
      </div>`;
    }).join('');
  } catch (e) { showToast(e.message, 'error'); }
}
async function showMachineDetail(mid) {
  try {
    const data = await adminApi('/admin/api/machine/cards?id=' + encodeURIComponent(mid));
    const cards = data.cards || [];
    document.getElementById('machineModalTitle').innerHTML = '机器 <span class="mono">' + mid.substring(0, 24) + '...</span> 的卡密列表';
    let html = '<div class="table-wrap"><table><thead><tr><th>卡密</th><th>状态</th><th>到期时间</th><th>备注</th></tr></thead><tbody>';
    if (cards.length === 0) html += '<tr><td colspan="4"><div class="empty-state">无绑定的卡密</div></td></tr>';
    else html += cards.map(c => `<tr><td><span class="mono">${c.code}</span></td><td>${c.status}</td><td>${new Date(c.expires_at).toLocaleString('zh-CN')}</td><td>${c.note || '-'}</td></tr>`).join('');
    html += '</tbody></table></div>';
    document.getElementById('machineDetailContent').innerHTML = html;
    document.getElementById('machineModal').classList.add('show');
  } catch (e) { showToast(e.message, 'error'); }
}

// ========== Blacklist ==========
function showBlacklistModal() { document.getElementById('blacklistModal').classList.add('show'); }
async function addBlacklist() {
  const typ = document.getElementById('blType').value, val = document.getElementById('blValue').value, reason = document.getElementById('blReason').value;
  if (!val) { showToast('请输入要封禁的值', 'warning'); return; }
  try {
    await adminApi('/admin/api/blacklist', { action: 'add', type: typ, value: val, reason });
    showToast('已封禁'); closeModal('blacklistModal'); loadBlacklist();
  } catch (e) { showToast(e.message, 'error'); }
}
async function removeBlacklist(val) {
  if (!confirm('确认移出黑名单？')) return;
  try { await adminApi('/admin/api/blacklist', { action: 'remove', value: val }); showToast('已移除'); loadBlacklist(); } catch (e) { showToast(e.message, 'error'); }
}
async function loadBlacklist() {
  try {
    const data = await adminApi('/admin/api/blacklist');
    const entries = data.entries || [];
    const tbody = document.getElementById('blacklistTable');
    if (entries.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5"><div class="empty-state"><div class="icon">&#x1F6E1;</div>黑名单为空</div></td></tr>';
      return;
    }
    tbody.innerHTML = entries.map(e => {
      const val = esc(e.value);
      return `<tr><td>${e.type === 'machine' ? '机器' : e.type === 'ip' ? 'IP' : '卡密'}</td>
      <td><span class="mono">${val}</span></td><td>${esc(e.reason) || '-'}</td>
      <td>${new Date(e.created_at).toLocaleString('zh-CN')}</td>
      <td><button class="btn btn-sm btn-success" onclick="removeBlacklist('${val}')">移除</button></td></tr>`;
    }).join('');
  } catch (e) { showToast(e.message, 'error'); }
}

// ========== Announcement ==========
let _announcement = null;
async function loadAnnouncement() {
  try {
    const data = await adminApi('/admin/api/announcement');
    _announcement = data.announcement;
    const a = _announcement;
    if (a && a.content) {
      document.getElementById('announceContent').value = a.content;
      document.getElementById('announcePreview').innerHTML = a.content;
    } else {
      document.getElementById('announceContent').value = '';
      document.getElementById('announcePreview').innerHTML = '<span class="empty">当前无公告</span>';
    }
  } catch (e) { /* ignore */ }
}
async function saveAnnouncementContent() {
  const content = document.getElementById('announceContent').value.trim();
  const a = _announcement || {};
  try {
    await adminApi('/admin/api/announcement', {
      content,
      latest_version: a.latest_version || '',
      min_version: a.min_version || '',
      force_update: !!a.force_update
    });
    if (content) { showToast('公告已发布'); document.getElementById('announcePreview').innerHTML = content; }
    else { showToast('公告已清除'); document.getElementById('announcePreview').innerHTML = '<span class="empty">当前无公告</span>'; }
    _announcement = { ..._announcement, content };
  } catch (e) { showToast(e.message, 'error'); }
}
async function clearAnnouncementContent() {
  document.getElementById('announceContent').value = '';
  const a = _announcement || {};
  try {
    await adminApi('/admin/api/announcement', {
      content: '',
      latest_version: a.latest_version || '',
      min_version: a.min_version || '',
      force_update: !!a.force_update
    });
    showToast('公告已清除');
    document.getElementById('announcePreview').innerHTML = '<span class="empty">当前无公告</span>';
    _announcement = { ..._announcement, content: '' };
  } catch (e) { showToast(e.message, 'error'); }
}

// ========== Version Push ==========
async function loadVersionPush() {
  try {
    const data = await adminApi('/admin/api/announcement');
    _announcement = data.announcement;
    const a = _announcement;
    document.getElementById('latestVersion').value = (a && a.latest_version) || '';
    document.getElementById('minVersion').value = (a && a.min_version) || '';
    document.getElementById('forceUpdate').checked = !!(a && a.force_update);
  } catch (e) { /* ignore */ }
  loadUpdateInfo();
}
async function saveVersionPush() {
  const content = (_announcement && _announcement.content) || '';
  const latestVersion = document.getElementById('latestVersion').value.trim();
  const minVersion = document.getElementById('minVersion').value.trim();
  const forceUpdate = document.getElementById('forceUpdate').checked;
  try {
    await adminApi('/admin/api/announcement', { content, latest_version: latestVersion, min_version: minVersion, force_update: forceUpdate });
    showToast('版本推送设置已保存');
    _announcement = { content, latest_version: latestVersion, min_version: minVersion, force_update: forceUpdate };
  } catch (e) { showToast(e.message, 'error'); }
}
async function clearVersionPush() {
  document.getElementById('latestVersion').value = '';
  document.getElementById('minVersion').value = '';
  document.getElementById('forceUpdate').checked = false;
  const content = (_announcement && _announcement.content) || '';
  try {
    await adminApi('/admin/api/announcement', { content, latest_version: '', min_version: '', force_update: false });
    showToast('版本推送已清除');
    _announcement = { ..._announcement, latest_version: '', min_version: '', force_update: false };
  } catch (e) { showToast(e.message, 'error'); }
}

// ========== Audit ==========
async function loadAudit(page) {
  try {
    currentAuditPage = page || 1;
    const action = document.getElementById('auditActionFilter')?.value || '';
    let url = '/admin/api/audit?page=' + currentAuditPage + '&per_page=30';
    if (action) url += '&action=' + action;
    const data = await adminApi(url);
    const entries = data.entries || [], total = data.total || 0;
    const totalPages = Math.max(Math.ceil(total / (data.per_page || 30)), 1);
    document.getElementById('auditSummary').textContent = '共 ' + total + ' 条记录，第 ' + data.page + '/' + totalPages + ' 页';
    const div = document.getElementById('auditLog');
    if (entries.length === 0) {
      div.innerHTML = '<div class="empty-state"><div class="icon">&#x1F4CB;</div>暂无日志</div>';
      document.getElementById('auditPagination').innerHTML = '';
      return;
    }
    const am = {
      card_generated: '生成卡密', card_activated: '激活卡密', card_status_changed: '状态变更',
      card_extended: '延期', card_unbound: '解绑', session_deactivated: '注销会话',
      blacklist_added: '添加黑名单', blacklist_removed: '移除黑名单', cards_batch_generated: '批量生成',
      card_details_updated: '编辑卡密', bulk_disable: '批量禁用', bulk_enable: '批量启用',
      bulk_expire: '批量过期', bulk_extend: '批量延期', bulk_unbind: '批量解绑',
      agent_card_generated: '代理生成卡密', agent_batch_generated: '代理批量生成',
      agent_created: '代理注册', agent_deleted: '代理删除', agent_status_changed: '代理状态变更',
      agent_password_changed: '代理改密', admin_reset_agent_password: '管理员重置代理密码'
    };
    div.innerHTML = entries.map(e => `<div class="audit-entry">
      <span class="audit-time">${new Date(e.time).toLocaleString('zh-CN')}</span>
      <span class="audit-action">${esc(am[e.action] || e.action)}</span>
      <span class="audit-detail">${e.agent_id ? '<span style="color:var(--accent)">代理: ' + esc(e.agent_id) + '</span> ' : ''}${e.card ? '卡密: ' + esc(e.card) : ''}${e.machine ? ' 机器: ' + esc((e.machine || '').substring(0, 14)) + '...' : ''}${e.detail ? ' ' + esc(e.detail) : ''}${e.addr ? ' IP: ' + esc(e.addr) : ''}</span></div>`).join('');
    const pg = document.getElementById('auditPagination');
    pg.innerHTML = '<button ' + (page > 1 ? '' : 'disabled') + ' onclick="loadAudit(' + (page - 1) + ')">&#x25C0; 上一页</button>' +
      '<span>第 ' + page + '/' + totalPages + ' 页</span>' +
      '<button ' + (page < totalPages ? '' : 'disabled') + ' onclick="loadAudit(' + (page + 1) + ')">下一页 &#x25B6;</button>';
  } catch (e) { showToast(e.message, 'error'); }
}

// ========== Settings ==========
async function loadUpdateInfo() {
  try {
    const data = await adminApi('/admin/api/update/info');
    const el = document.getElementById('currentUpdateInfo');
    if (data.update) {
      const u = data.update;
      const sizeMB = (u.file_size / 1048576).toFixed(1);
      const time = new Date(u.uploaded_at).toLocaleString('zh-CN');
      el.innerHTML = `当前版本: <b>v${u.version}</b> &middot; ${u.filename} (${sizeMB} MB) &middot; 上传于 ${time}`;
    } else {
      el.innerHTML = '<span style="color:var(--text-3)">暂无上传的更新包</span>';
    }
  } catch (e) { /* ignore */ }
}

async function uploadUpdate() {
  const version = document.getElementById('updateVersion').value.trim();
  const fileInput = document.getElementById('updateFile');
  if (!version) { showToast('请输入版本号', 'warning'); return; }
  if (!fileInput.files || !fileInput.files[0]) { showToast('请选择 .exe 文件', 'warning'); return; }

  const file = fileInput.files[0];
  if (!file.name.toLowerCase().endsWith('.exe')) { showToast('只支持 .exe 文件', 'warning'); return; }

  const formData = new FormData();
  formData.append('version', version);
  formData.append('file', file);

  const progressEl = document.getElementById('uploadProgress');
  progressEl.style.display = 'block';
  progressEl.textContent = '正在上传...';
  progressEl.style.color = 'var(--accent)';

  try {
    const res = await fetch('/admin/api/update/upload', {
      method: 'POST',
      credentials: 'include',
      body: formData
    });
    if (res.status === 401) { goLogin(); throw new Error('登录已过期'); }
    const data = await res.json();
    if (data.status === 'error') throw new Error(data.message);

    const sizeMB = (data.file_size / 1048576).toFixed(1);
    progressEl.textContent = `上传成功! v${data.version} (${sizeMB} MB)`;
    progressEl.style.color = 'var(--success)';
    showToast('更新包已上传，客户端将自动检测到新版本');
    document.getElementById('updateVersion').value = '';
    fileInput.value = '';
    loadUpdateInfo();
  } catch (e) {
    progressEl.textContent = '上传失败: ' + e.message;
    progressEl.style.color = 'var(--danger)';
    showToast(e.message, 'error');
  }
}

async function changePassword() {
  const oldPw = document.getElementById('oldPassword').value;
  const newPw = document.getElementById('newPassword').value;
  const confirmPw = document.getElementById('confirmPassword').value;
  if (!oldPw || !newPw) { showToast('请填写完整', 'warning'); return; }
  if (newPw !== confirmPw) { showToast('两次输入的新密码不一致', 'warning'); return; }
  if (newPw.length < 6) { showToast('新密码长度不能少于6位', 'warning'); return; }
  try {
    await adminApi('/admin/api/password', { old_password: oldPw, new_password: newPw });
    showToast('密码修改成功，请重新登录');
    document.getElementById('oldPassword').value = '';
    document.getElementById('newPassword').value = '';
    document.getElementById('confirmPassword').value = '';
    setTimeout(() => goLogin(), 1500);
  } catch (e) { showToast(e.message, 'error'); }
}

// ========== Auto-refresh & Extensions ==========
function toggleAutoRefresh() {
  _autoRefresh = document.getElementById('autoRefreshToggle').checked;
  showToast(_autoRefresh ? '自动刷新已开启' : '自动刷新已关闭', _autoRefresh ? 'success' : 'warning');
}

async function exportCardsJSON() {
  try {
    const data = await adminApi('/admin/api/cards');
    const cards = data.cards || [];
    const json = JSON.stringify(cards, null, 2);
    const blob = new Blob([json], { type: 'application/json' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'cards_export_' + new Date().toISOString().slice(0,10) + '.json';
    a.click();
    showToast('已导出 ' + cards.length + ' 张卡密 (JSON)');
  } catch (e) { showToast('导出失败: ' + e.message, 'error'); }
}

// ========== Agents ==========
let _agents = [];
let _resetAgentID = '';
async function loadAgents() {
  try {
    const data = await adminApi('/admin/api/agents');
    _agents = data.agents || [];
    renderAgents();
  } catch (e) { showToast(e.message, 'error'); }
}
function renderAgents() {
  const q = (document.getElementById('agentSearch')?.value || '').toLowerCase();
  const agents = _agents.filter(a => !q || a.username.toLowerCase().includes(q) || a.id.toLowerCase().includes(q));
  const tbody = document.getElementById('agentsTable');
  if (agents.length === 0) {
    tbody.innerHTML = '<tr><td colspan="6"><div class="empty-state"><div class="icon">&#x1F465;</div>暂无代理账号</div></td></tr>';
    return;
  }
  tbody.innerHTML = agents.map(a => {
    const aid = esc(a.id);
    const auser = esc(a.username);
    const statusBadge = a.disabled
      ? '<span class="badge badge-disabled">已禁用</span>'
      : '<span class="badge badge-active">正常</span>';
    return `<tr>
      <td><span class="mono">${aid}</span></td>
      <td><strong>${auser}</strong></td>
      <td><button class="btn btn-sm btn-ghost" onclick="viewAgentCards('${aid}','${auser}')">查看卡密</button></td>
      <td>${new Date(a.created_at).toLocaleString('zh-CN')}</td>
      <td>${statusBadge}</td>
      <td style="white-space:nowrap">
        ${a.disabled
          ? `<button class="btn btn-sm btn-success" onclick="toggleAgent('${aid}',false)">启用</button>`
          : `<button class="btn btn-sm btn-danger" onclick="toggleAgent('${aid}',true)">禁用</button>`}
        <button class="btn btn-sm btn-ghost" onclick="showResetAgentPw('${aid}','${auser}')">重置密码</button>
        <button class="btn btn-sm btn-danger" onclick="deleteAgent('${aid}')">删除</button>
      </td>
    </tr>`;
  }).join('');
}
async function toggleAgent(id, disable) {
  const action = disable ? 'disable' : 'enable';
  const label = disable ? '禁用' : '启用';
  if (!confirm('确认' + label + '此代理？')) return;
  try { await adminApi('/admin/api/agent/update', { agent_id: id, action }); showToast('已' + label); loadAgents(); } catch (e) { showToast(e.message, 'error'); }
}
async function deleteAgent(id) {
  if (!confirm('确认删除此代理？此操作不可撤销，该代理的所有卡密将保留但不再关联代理。')) return;
  try { await adminApi('/admin/api/agent/update', { agent_id: id, action: 'delete' }); showToast('已删除'); loadAgents(); } catch (e) { showToast(e.message, 'error'); }
}
function showResetAgentPw(id, username) {
  _resetAgentID = id;
  document.getElementById('resetAgentInfo').textContent = '重置代理 ' + username + ' 的密码';
  document.getElementById('resetAgentPassword').value = '';
  document.getElementById('resetAgentPwModal').classList.add('show');
}
async function confirmResetAgentPw() {
  const pw = document.getElementById('resetAgentPassword').value;
  if (!pw || pw.length < 6) { showToast('密码长度不能少于6位', 'warning'); return; }
  try {
    await adminApi('/admin/api/agent/update', { agent_id: _resetAgentID, action: 'reset_password', password: pw });
    showToast('密码已重置'); closeModal('resetAgentPwModal');
  } catch (e) { showToast(e.message, 'error'); }
}
async function viewAgentCards(id, username) {
  try {
    const data = await adminApi('/admin/api/agent/cards?agent_id=' + encodeURIComponent(id));
    const cards = data.cards || [];
    document.getElementById('agentCardsTitle').innerHTML = '代理 <span style="color:var(--accent)">' + esc(username) + '</span> 的卡密';
    let html = '<div class="table-wrap"><table><thead><tr><th>卡密</th><th>状态</th><th>到期时间</th><th>绑定机器</th><th>备注</th></tr></thead><tbody>';
    if (cards.length === 0) html += '<tr><td colspan="5"><div class="empty-state">该代理暂无卡密</div></td></tr>';
    else html += cards.map(c => {
      const isActive = c.status === 'active' && new Date(c.expires_at) > new Date();
      const sc = isActive ? 'badge-active' : c.status === 'disabled' ? 'badge-disabled' : 'badge-expired';
      const st = isActive ? '有效' : c.status === 'disabled' ? '已禁用' : '已过期';
      return `<tr><td><span class="mono" style="cursor:pointer" onclick="copyToClipboard('${esc(c.code)}','卡密')">${esc(c.code)}</span></td><td><span class="badge ${sc}">${st}</span></td><td>${new Date(c.expires_at).toLocaleString('zh-CN')}</td><td>${c.machine_id ? '<span class="mono">' + esc(c.machine_id.substring(0,12)) + '...</span>' : '-'}</td><td>${esc(c.note) || '-'}</td></tr>`;
    }).join('');
    html += '</tbody></table></div>';
    document.getElementById('agentCardsContent').innerHTML = html;
    document.getElementById('agentCardsModal').classList.add('show');
  } catch (e) { showToast(e.message, 'error'); }
}
setInterval(() => {
  if (_autoRefresh && document.getElementById('page-dashboard').classList.contains('active')) loadDashboard();
}, 15000);
setInterval(async () => {
  try {
    await fetch('/api/health');
    document.getElementById('serverDot').style.background = 'var(--success)';
    document.getElementById('serverStatusText').textContent = '服务器运行中';
  } catch (e) {
    document.getElementById('serverDot').style.background = 'var(--danger)';
    document.getElementById('serverStatusText').textContent = '服务器离线';
  }
}, 30000);

let _invites = [];
async function loadInvites() {
  try { const data = await adminApi('/admin/api/invites'); _invites = data.invites || []; renderInvites(); }
  catch (e) { showToast(e.message, 'error'); }
}
function renderInvites() {
  const tbody = document.getElementById('invitesTable');
  if (!_invites.length) { tbody.innerHTML='<tr><td colspan="5">暂无邀请码</td></tr>'; return; }
  tbody.innerHTML = _invites.map(c => {
    const code = esc(c.code);
    const usedUp = c.max_uses > 0 && c.use_count >= c.max_uses;
    const status = usedUp ? '<span class="badge badge-disabled">已用完</span>' : '<span class="badge badge-active">可用</span>';
    const limit = c.max_uses === 0 ? '不限' : c.use_count + '/' + c.max_uses;
    return '<tr><td><span class="mono">' + code + '</span></td><td>' + status + '</td><td>' + limit + '</td><td>' + new Date(c.created_at).toLocaleString('zh-CN') + '</td><td><button class="btn btn-sm btn-ghost" onclick="copyInvite(\'' + code + '\')">复制</button> <button class="btn btn-sm btn-danger" onclick="deleteInvite(\'' + code + '\')">删除</button></td></tr>';
  }).join('');
}
function copyInvite(code) { navigator.clipboard.writeText(code).then(() => showToast('已复制: ' + code)).catch(() => showToast(code)); }
async function createInvites() {
  const count = parseInt(document.getElementById('inviteCount').value) || 5;
  const maxUses = parseInt(document.getElementById('inviteMaxUses').value) || 1;
  try {
    const data = await adminApi('/admin/api/invite/create', { count, max_uses: maxUses });
    const codes = data.invites.map(c => c.code).join('\n');
    navigator.clipboard.writeText(codes).then(() => showToast('已生成 ' + data.count + ' 个邀请码，已复制')).catch(() => showToast('已生成 ' + data.count + ' 个'));
    loadInvites();
  } catch (e) { showToast(e.message, 'error'); }
}
async function deleteInvite(code) {
  if (!confirm('确定删除 ' + code + '?')) return;
  try { await adminApi('/admin/api/invite/delete', { code }); showToast('已删除'); loadInvites(); }
  catch (e) { showToast(e.message, 'error'); }
}
