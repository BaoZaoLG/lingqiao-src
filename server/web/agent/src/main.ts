import '../../shared/src/styles.css';
import { ApiClient, ApiError, JsonRecord } from '../../shared/src/api';
import { AppShell } from '../../shared/src/shell';
import { escapeHtml, formatDate, on, qs, qsa } from '../../shared/src/dom';
import { ToastHost } from '../../shared/src/toast';

interface Card extends JsonRecord {
  code: string;
  status: string;
  max_sessions?: number;
  expires_at: string;
  machine_id?: string;
  note?: string;
}

const toast = new ToastHost();
const api = new ApiClient({ onUnauthorized: showAuth });
const shell = new AppShell({
  dashboard: loadDashboard,
  cards: loadCards,
  password: () => undefined
});
let registerMode = false;
let batchMode = false;
let cards: Card[] = [];

boot();

function boot(): void {
  shell.bind();
  bindChrome();
  bindAuth();
  bindCards();
  bindPassword();
  void checkAuth();
}

function bindChrome(): void {
  on(qs<HTMLElement>('#sidebarToggle'), 'click', () => {
    qs<HTMLElement>('.sidebar').classList.toggle('open');
    qs<HTMLElement>('#sidebarOverlay').classList.toggle('show');
  });
  on(qs<HTMLElement>('#sidebarOverlay'), 'click', () => {
    qs<HTMLElement>('.sidebar').classList.remove('open');
    qs<HTMLElement>('#sidebarOverlay').classList.remove('show');
  });
  qsa<HTMLElement>('[data-close-modal]').forEach((button) => {
    on(button, 'click', () => qs<HTMLElement>(`#${button.dataset.closeModal}`).classList.remove('show'));
  });
  on(qs<HTMLElement>('#logoutButton'), 'click', showAuth);
}

function bindAuth(): void {
  on(qs<HTMLElement>('#authModeToggle'), 'click', toggleAuthMode);
  on(qs<HTMLElement>('#authSubmit'), 'click', () => void submitAuth());
}

function bindCards(): void {
  on(qs<HTMLElement>('#refreshCardsButton'), 'click', () => void loadCards());
  on(qs<HTMLElement>('#generateCardButton'), 'click', () => showGenerate(false));
  on(qs<HTMLElement>('#batchGenerateButton'), 'click', () => showGenerate(true));
  on(qs<HTMLElement>('#confirmGenerate'), 'click', () => void generateCards());
  qs<HTMLInputElement>('#cardSearch').addEventListener('input', renderCards);
  qs<HTMLSelectElement>('#cardStatusFilter').addEventListener('change', renderCards);
}

function bindPassword(): void {
  on(qs<HTMLElement>('#changePasswordButton'), 'click', () => void changePassword());
}

function toggleAuthMode(): void {
  registerMode = !registerMode;
  qs<HTMLElement>('#authTitle').textContent = registerMode ? '注册代理账号' : '代理登录';
  qs<HTMLElement>('#authSubtitle').textContent = registerMode ? '使用邀请码注册代理账号' : '使用代理账号进入自助面板';
  qs<HTMLInputElement>('#passwordConfirm').hidden = !registerMode;
  qs<HTMLInputElement>('#inviteCode').hidden = !registerMode;
  qs<HTMLElement>('#authSubmit').textContent = registerMode ? '注册' : '登录';
  qs<HTMLElement>('#authModeToggle').textContent = registerMode ? '返回登录' : '注册代理账号';
}

async function submitAuth(): Promise<void> {
  const username = qs<HTMLInputElement>('#username').value.trim();
  const password = qs<HTMLInputElement>('#password').value;
  try {
    if (registerMode) {
      const passwordConfirm = qs<HTMLInputElement>('#passwordConfirm').value;
      if (password !== passwordConfirm) throw new Error('两次输入的密码不一致');
      await api.post('/api/register', { username, password, invite_code: qs<HTMLInputElement>('#inviteCode').value.trim() });
      toast.show('注册成功，请登录');
      toggleAuthMode();
      return;
    }
    const response = await api.post<JsonRecord>('/api/login', { username, password });
    localStorage.setItem('agentUser', String(response.username ?? username));
    localStorage.setItem('agentID', String(response.agent_id ?? ''));
    showApp(String(response.username ?? username));
  } catch (error) {
    qs<HTMLElement>('#authError').textContent = messageOf(error);
  }
}

async function checkAuth(): Promise<void> {
  const username = localStorage.getItem('agentUser');
  if (!username) {
    showAuth();
    return;
  }
  try {
    await api.get('/api/dashboard');
    showApp(username);
  } catch {
    showAuth();
  }
}

function showApp(username: string): void {
  qs<HTMLElement>('#agentNameDisplay').textContent = `@${username}`;
  shell.showApp('#authScreen', '#app');
  void loadDashboard();
}

function showAuth(): void {
  localStorage.removeItem('agentUser');
  localStorage.removeItem('agentID');
  shell.showAuth('#authScreen', '#app');
}

async function loadDashboard(): Promise<void> {
  const data = await api.get<JsonRecord>('/api/dashboard');
  qs<HTMLElement>('#dashboardStats').innerHTML = [
    stat('我的卡密', data.total_cards),
    stat('有效卡密', data.active_cards, 'success'),
    stat('已过期', data.expired_cards, 'danger')
  ].join('');
}

async function loadCards(): Promise<void> {
  const response = await api.get<{ cards?: Card[] }>('/api/cards');
  cards = response.cards ?? [];
  renderCards();
}

function renderCards(): void {
  const query = qs<HTMLInputElement>('#cardSearch').value.toUpperCase();
  const status = qs<HTMLSelectElement>('#cardStatusFilter').value;
  const list = cards.filter((card) => {
    if (status && card.status !== status) return false;
    return !query || card.code.includes(query) || String(card.note ?? '').toUpperCase().includes(query);
  });
  qs<HTMLElement>('#cardsTable').innerHTML = list.length ? list.map(renderCardRow).join('') : '<tr><td colspan="6"><div class="empty-state">暂无卡密</div></td></tr>';
}

function renderCardRow(card: Card): string {
  const active = card.status === 'active' && new Date(card.expires_at) > new Date();
  const status = active ? 'active' : card.status;
  const label = active ? '有效' : status === 'disabled' ? '已禁用' : '已过期';
  return `<tr><td><span class="mono">${escapeHtml(card.code)}</span></td><td><span class="badge badge-${escapeHtml(status)}">${label}</span></td><td>${card.max_sessions ?? 1}</td><td>${formatDate(card.expires_at)}</td><td>${escapeHtml(card.machine_id ?? '-')}</td><td>${escapeHtml(card.note ?? '-')}</td></tr>`;
}

function showGenerate(batch: boolean): void {
  batchMode = batch;
  qs<HTMLElement>('#generateTitle').textContent = batch ? '批量生成卡密' : '生成卡密';
  qs<HTMLInputElement>('#genCount').value = batch ? '10' : '1';
  qs<HTMLInputElement>('#genCount').hidden = !batch;
  qs<HTMLElement>('#generateModal').classList.add('show');
}

async function generateCards(): Promise<void> {
  const payload = {
    count: Number(qs<HTMLInputElement>('#genCount').value || 1),
    duration_hours: Number(qs<HTMLInputElement>('#genDuration').value || 720),
    max_sessions: Number(qs<HTMLInputElement>('#genMaxSessions').value || 1),
    note: qs<HTMLInputElement>('#genNote').value
  };
  const path = batchMode ? '/api/card/batch-generate' : '/api/card/generate';
  await api.post(path, payload);
  qs<HTMLElement>('#generateModal').classList.remove('show');
  toast.show(batchMode ? '批量卡密已生成' : '卡密已生成');
  await loadCards();
}

async function changePassword(): Promise<void> {
  const oldPassword = qs<HTMLInputElement>('#oldPassword').value;
  const newPassword = qs<HTMLInputElement>('#newPassword').value;
  const confirmPassword = qs<HTMLInputElement>('#confirmPassword').value;
  if (newPassword !== confirmPassword) {
    toast.show('两次输入的新密码不一致', 'warning');
    return;
  }
  await api.post('/api/password', { old_password: oldPassword, new_password: newPassword });
  toast.show('密码已修改，请重新登录');
  showAuth();
}

function stat(label: string, value: unknown, tone = 'accent'): string {
  return `<div class="stat-card"><div class="label">${label}</div><div class="value ${tone}">${escapeHtml(value)}</div></div>`;
}

function messageOf(error: unknown): string {
  return error instanceof ApiError || error instanceof Error ? error.message : '操作失败';
}
