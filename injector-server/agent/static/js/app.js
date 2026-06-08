let _agentID = localStorage.getItem('agentID') || '';
let _agentUser = localStorage.getItem('agentUser') || '';
let _cards = [];

// ── XSS Protection ──────────────────────────────────────────────
function esc(s) {
  if (s == null) return '';
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');
}

// ── Clipboard ───────────────────────────────────────────────────
function copyToClipboard(text, label) {
  navigator.clipboard.writeText(text).then(() => {
    showToast((label || '已复制') + ': ' + text);
  }).catch(() => showToast(text));
}

// ── Toast ───────────────────────────────────────────────────────
function showToast(msg, type) {
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.className = 'toast ' + (type || 'success') + ' show';
  setTimeout(() => t.className = 'toast', 3500);
}

// ── Keyboard Shortcuts ──────────────────────────────────────────
document.addEventListener('keydown', e => {
  if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
    e.preventDefault();
    const searchInput = document.querySelector('.page.active input[type="text"]');
    if (searchInput) { searchInput.focus(); searchInput.select(); }
  }
  if (e.key === 'Escape') {
    document.querySelectorAll('.modal-overlay.show').forEach(m => m.classList.remove('show'));
  }
});
function showAuthTab(tab) {
  document.querySelectorAll('.auth-tab').forEach(t => t.classList.remove('active'));
  if (tab === 'login') { document.querySelectorAll('.auth-tab')[0].classList.add('active'); document.getElementById('loginBox').style.display='block'; document.getElementById('registerBox').style.display='none'; }
  else { document.querySelectorAll('.auth-tab')[1].classList.add('active'); document.getElementById('loginBox').style.display='none'; document.getElementById('registerBox').style.display='block'; }
}
function goAuth() {
  _agentID=''; _agentUser='';
  localStorage.removeItem('agentID'); localStorage.removeItem('agentUser');
  document.getElementById('authScreen').style.display='flex';
  document.getElementById('app').classList.remove('show');
}
async function api(path, body) {
  const opts = { headers: {} };
  if (body) { opts.method='POST'; opts.headers['Content-Type']='application/json'; opts.body=JSON.stringify(body); }
  const res = await fetch(path, opts);
  const text = await res.text();
  try { const data = JSON.parse(text); if (data.status==='error') throw new Error(data.message); return data; }
  catch (e) { if (e instanceof SyntaxError) throw new Error('服务器响应异常'); throw e; }
}
async function agentApi(path, body) {
  const opts = { credentials: 'include', headers: {} };
  if (body) { opts.method='POST'; opts.headers['Content-Type']='application/json'; opts.body=JSON.stringify(body); }
  const res = await fetch(path, opts);
  if (res.status===401) { goAuth(); throw new Error('登录已过期，请重新登录'); }
  const text = await res.text();
  try { const data = JSON.parse(text); if (data.status==='error') throw new Error(data.message); return data; }
  catch (e) { if (e instanceof SyntaxError) throw new Error('服务器响应异常'); throw e; }
}
async function doLogin() {
  const user = document.getElementById('loginUsername').value.trim();
  const pw = document.getElementById('loginPassword').value;
  if (!user || !pw) return;
  try {
    const data = await api('/api/login', { username: user, password: pw });
    _agentID = data.agent_id; _agentUser = data.username;
    localStorage.setItem('agentID', _agentID); localStorage.setItem('agentUser', _agentUser);
    document.getElementById('authScreen').style.display='none';
    document.getElementById('app').classList.add('show');
    document.getElementById('agentNameDisplay').textContent = '@' + _agentUser;
    loadDashboard();
  } catch (e) { const el = document.getElementById('loginError'); el.textContent = e.message; el.style.display = 'block'; }
}
async function doRegister() {
  const user = document.getElementById('regUsername').value.trim();
  const pw = document.getElementById('regPassword').value;
  const pw2 = document.getElementById('regPassword2').value;
  const errEl = document.getElementById('registerError');
  if (!user || !pw) return;
  if (pw !== pw2) { errEl.textContent='两次输入的密码不一致'; errEl.style.display='block'; return; }
  try {
    const inviteCode = document.getElementById('regInviteCode').value.trim(); if (!inviteCode) { errEl.textContent='请输入邀请码'; errEl.style.display='block'; return; } await api('/api/register', { username: user, password: pw, invite_code: inviteCode });
    showToast('注册成功，请登录'); showAuthTab('login');
    document.getElementById('loginUsername').value = user;
    errEl.style.display='none';
  } catch (e) { errEl.textContent = e.message; errEl.style.display = 'block'; }
}
async function checkAuth() {
  if (!_agentUser) return;
  try {
    await agentApi('/api/dashboard');
    document.getElementById('authScreen').style.display='none';
    document.getElementById('app').classList.add('show');
    document.getElementById('agentNameDisplay').textContent = '@' + _agentUser;
    loadDashboard();
  } catch (e) {}
}
checkAuth();
function switchPage(name, el) {
  document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
  if (el) el.classList.add('active');
  document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
  document.getElementById('page-'+name).classList.add('active');
  if (name==='dashboard') loadDashboard();
  if (name==='cards') loadCards();
  if (window.innerWidth<=768) toggleSidebar();
}
function toggleSidebar() { document.querySelector('.sidebar').classList.toggle('open'); document.getElementById('sidebarOverlay').classList.toggle('show'); }
function closeModal(id) { document.getElementById(id).classList.remove('show'); }

async function loadDashboard() {
  try {
    const data = await agentApi('/api/dashboard');
    document.getElementById('dashboardStats').innerHTML = `
      <div class="stat-card"><div class="label">我的卡密</div><div class="value accent">${data.total_cards}</div></div>
      <div class="stat-card"><div class="label">有效卡密</div><div class="value success">${data.active_cards}</div></div>
      <div class="stat-card"><div class="label">已过期</div><div class="value danger">${data.expired_cards}</div></div>`;
  } catch (e) { showToast(e.message, 'error'); }
}
async function loadCards() {
  try { const data = await agentApi('/api/cards'); _cards = data.cards || []; renderCards(); }
  catch (e) { showToast(e.message, 'error'); }
}
function renderCards() {
  const q = (document.getElementById('cardSearch')?.value||'').toUpperCase();
  const sf = document.getElementById('cardStatusFilter')?.value||'';
  const cards = _cards.filter(c => {
    if (sf && c.status !== sf) return false;
    if (q && !c.code.includes(q) && !(c.note||'').toUpperCase().includes(q)) return false;
    return true;
  });
  const tbody = document.getElementById('cardsTable');
  if (!cards.length) { tbody.innerHTML='<tr><td colspan="6"><div class="empty-state"><div class="icon">&#x1F4ED;</div>暂无卡密，点击上方按钮生成</div></td></tr>'; return; }
  tbody.innerHTML = cards.map(c => {
    const isActive = c.status==='active' && new Date(c.expires_at)>new Date();
    const sc = isActive?'badge-active':c.status==='disabled'?'badge-disabled':'badge-expired';
    const st = isActive?'有效':c.status==='disabled'?'已禁用':'已过期';
    const code = esc(c.code);
    const bound = c.machine_id?`<span class="mono" title="${esc(c.machine_id)}">${esc(c.machine_id.substring(0,12))}...</span>`:'<span style="color:var(--text-3)">未绑定</span>';
    return `<tr><td><span class="mono" style="cursor:pointer" title="点击复制" onclick="copyToClipboard('${code}','卡密')">${code}</span></td><td><span class="badge ${sc}">${st}</span></td><td>${c.max_sessions||1}</td><td>${new Date(c.expires_at).toLocaleString('zh-CN')}</td><td>${bound}</td><td>${esc(c.note)||'-'}</td></tr>`;
  }).join('');
}
function showGenerateModal() { document.getElementById('generateModal').classList.add('show'); }
function showBatchGenerateModal() { document.getElementById('batchGenerateModal').classList.add('show'); }
async function generateCard() {
  const d=parseInt(document.getElementById('genDuration').value)||720;
  const ms=parseInt(document.getElementById('genMaxSessions').value)||1;
  const note=document.getElementById('genNote').value;
  try {
    const data = await agentApi('/api/card/generate', {duration_hours:d, max_sessions:ms, note});
    navigator.clipboard.writeText(data.card.code).then(()=>showToast('卡密已复制: '+data.card.code)).catch(()=>showToast('卡密: '+data.card.code));
    closeModal('generateModal'); loadCards();
  } catch (e) { showToast(e.message, 'error'); }
}
async function batchGenerateCards() {
  const count=parseInt(document.getElementById('batchCount').value)||10;
  const d=parseInt(document.getElementById('batchDuration').value)||720;
  const ms=parseInt(document.getElementById('batchMaxSessions').value)||1;
  const note=document.getElementById('batchNote').value;
  try {
    const data = await agentApi('/api/card/batch-generate', {count, duration_hours:d, max_sessions:ms, note});
    const codes = data.cards.map(c=>c.code).join('\n');
    navigator.clipboard.writeText(codes).then(()=>showToast('已生成 '+data.count+' 张卡密，已复制到剪贴板')).catch(()=>showToast('已生成 '+data.count+' 张卡密'));
    closeModal('batchGenerateModal'); loadCards();
  } catch (e) { showToast(e.message, 'error'); }
}
async function changePassword() {
  const oldPw=document.getElementById('oldPassword').value;
  const newPw=document.getElementById('newPassword').value;
  const confirmPw=document.getElementById('confirmPassword').value;
  if (!oldPw||!newPw) { showToast('请填写完整','warning'); return; }
  if (newPw!==confirmPw) { showToast('两次输入的新密码不一致','warning'); return; }
  if (newPw.length<6) { showToast('新密码长度不能少于6位','warning'); return; }
  try {
    await agentApi('/api/password', {old_password:oldPw, new_password:newPw});
    showToast('密码修改成功，请重新登录');
    document.getElementById('oldPassword').value='';
    document.getElementById('newPassword').value='';
    document.getElementById('confirmPassword').value='';
    setTimeout(()=>goAuth(), 1500);
  } catch (e) { showToast(e.message, 'error'); }
}
setInterval(()=>{ if (document.getElementById('page-dashboard').classList.contains('active')) loadDashboard(); }, 30000);
setInterval(async()=>{ try { await fetch('/api/health'); document.getElementById('serverDot').style.background='var(--success)'; document.getElementById('serverStatusText').textContent='服务器运行中'; } catch(e) { document.getElementById('serverDot').style.background='var(--danger)'; document.getElementById('serverStatusText').textContent='服务器离线'; } }, 30000);
