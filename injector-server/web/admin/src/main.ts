import '../../shared/src/styles.css';
import { ApiClient, ApiError, JsonRecord } from '../../shared/src/api';
import { AppShell } from '../../shared/src/shell';
import { escapeHtml, formatDate, formatUptime, on, qs, qsa } from '../../shared/src/dom';
import { ToastHost } from '../../shared/src/toast';

interface Card extends JsonRecord {
  code: string;
  status: string;
  max_sessions?: number;
  created_at?: string;
  activated_at?: string;
  expires_at: string;
  machine_id?: string;
  note?: string;
}

interface SessionRecord extends JsonRecord {
  token: string;
  card_code: string;
  machine_id: string;
  client_version?: string;
  remote_addr?: string;
  last_seen: string;
  expires_at: string;
}

interface CardImportReport extends JsonRecord {
  total_rows?: number;
  valid_rows?: number;
  imported?: number;
  duplicates?: number;
  invalid?: number;
  skipped?: number;
  items?: JsonRecord[];
}

const toast = new ToastHost();
const api = new ApiClient({ onUnauthorized: showLogin });
const selectedCards = new Set<string>();
let cards: Card[] = [];
let agents: JsonRecord[] = [];
let invites: JsonRecord[] = [];
let batchMode = false;
let extendCode = '';
let resetAgentID = '';
let auditPage = 1;
let announcement: JsonRecord | null = null;
let announcementHistory: JsonRecord[] = [];
let cardMachineFilter = '';
let scripts: JsonRecord[] = [];
let selectedScriptID = '';
let releaseItems: JsonRecord[] = [];
let payloadItems: JsonRecord[] = [];

const shell = new AppShell({
  dashboard: loadDashboard,
  cards: loadCards,
  sessions: loadSessions,
  machines: loadMachines,
  blacklist: loadBlacklist,
  announcement: loadAnnouncement,
  updates: loadVersionPush,
  scripts: loadScriptModule,
  payloads: loadPayloads,
  password: noop,
  audit: () => loadAudit(1),
  agents: loadAgents,
  invites: loadInvites,
  settings: loadHealth
});

boot();

function boot(): void {
  shell.bind();
  bindChrome();
  bindAuth();
  bindCards();
  bindSessions();
  bindMachines();
  bindBlacklist();
  bindAnnouncement();
  bindUpdates();
  bindScripts();
  bindPayloads();
  bindPassword();
  bindAgents();
  bindInvites();
  bindAudit();
  void checkAuth();
  window.setInterval(updateServerStatus, 30000);
}

function noop(): void {}

function bindChrome(): void {
  on(qs<HTMLElement>('#sidebarToggle'), 'click', () => {
    qs<HTMLElement>('.sidebar').classList.toggle('open');
    qs<HTMLElement>('#sidebarOverlay').classList.toggle('show');
  });
  on(qs<HTMLElement>('#sidebarOverlay'), 'click', closeSidebar);
  qsa<HTMLElement>('[data-close-modal]').forEach((button) => {
    on(button, 'click', () => closeModal(String(button.dataset.closeModal)));
  });
  qsa<HTMLElement>('[data-page-shortcut]').forEach((button) => {
    on(button, 'click', () => activatePage(String(button.dataset.pageShortcut)));
  });
}

function bindAuth(): void {
  on(qs<HTMLElement>('#loginButton'), 'click', () => void login());
  qs<HTMLInputElement>('#loginPassword').addEventListener('keydown', (event) => {
    if (event.key === 'Enter') void login();
  });
  on(qs<HTMLElement>('#logoutButton'), 'click', showLogin);
}

function bindCards(): void {
  on(qs<HTMLElement>('#refreshCardsButton'), 'click', () => void loadCards());
  on(qs<HTMLElement>('#generateCardButton'), 'click', () => showGenerate(false));
  on(qs<HTMLElement>('#batchGenerateButton'), 'click', () => showGenerate(true));
  on(qs<HTMLElement>('#confirmGenerate'), 'click', () => void generateCard());
  on(qs<HTMLElement>('#exportCsvButton'), 'click', () => void exportCardsCSV());
  on(qs<HTMLElement>('#exportJsonButton'), 'click', () => void exportCardsJSON());
  on(qs<HTMLElement>('#importCardsButton'), 'click', showImportModal);
  on(qs<HTMLElement>('#previewImportButton'), 'click', () => void previewImportCards());
  on(qs<HTMLElement>('#confirmImportButton'), 'click', () => void importCards());
  on(qs<HTMLElement>('#saveCardDetailsButton'), 'click', () => void saveCardDetails());
  on(qs<HTMLElement>('#confirmExtendButton'), 'click', () => void confirmExtend());
  qs<HTMLInputElement>('#cardSearch').addEventListener('input', renderCards);
  qs<HTMLSelectElement>('#cardStatusFilter').addEventListener('change', () => void loadCards());
  qsa<HTMLInputElement | HTMLSelectElement>('#cardAgentFilter, #cardBoundFilter, #cardExpiresAfter, #cardExpiresBefore, #cardMaxSessionsFilter').forEach((input) => {
    input.addEventListener('change', () => void loadCards());
  });
  on(qs<HTMLElement>('#clearCardFiltersButton'), 'click', () => {
    cardMachineFilter = '';
    qs<HTMLInputElement>('#cardAgentFilter').value = '';
    qs<HTMLSelectElement>('#cardBoundFilter').value = '';
    qs<HTMLInputElement>('#cardExpiresAfter').value = '';
    qs<HTMLInputElement>('#cardExpiresBefore').value = '';
    qs<HTMLInputElement>('#cardMaxSessionsFilter').value = '';
    qs<HTMLInputElement>('#cardSearch').value = '';
    void loadCards();
  });
  qs<HTMLInputElement>('#selectAllCards').addEventListener('change', (event) => {
    const checked = (event.target as HTMLInputElement).checked;
    filteredCards().forEach((card) => checked ? selectedCards.add(card.code) : selectedCards.delete(card.code));
    renderCards();
  });
  qsa<HTMLElement>('[data-bulk-action]').forEach((button) => {
    on(button, 'click', () => void bulkAction(String(button.dataset.bulkAction)));
  });
}

function bindSessions(): void {
  on(qs<HTMLElement>('#refreshSessionsButton'), 'click', () => void loadSessions());
}

function bindMachines(): void {
  on(qs<HTMLElement>('#refreshMachinesButton'), 'click', () => void loadMachines());
}

function bindBlacklist(): void {
  on(qs<HTMLElement>('#showBlacklistButton'), 'click', () => showModal('blacklistModal'));
  on(qs<HTMLElement>('#refreshBlacklistButton'), 'click', () => void loadBlacklist());
  on(qs<HTMLElement>('#addBlacklistButton'), 'click', () => void addBlacklist());
}

function bindAnnouncement(): void {
  on(qs<HTMLElement>('#saveAnnouncementButton'), 'click', () => void saveAnnouncementDraft());
  on(qs<HTMLElement>('#publishAnnouncementButton'), 'click', () => void publishAnnouncementNow());
  on(qs<HTMLElement>('#clearAnnouncementButton'), 'click', () => void clearAnnouncement());
  qs<HTMLTextAreaElement>('#announceContent').addEventListener('input', renderAnnouncementPreview);
}

function bindUpdates(): void {
  on(qs<HTMLElement>('#createReleaseButton'), 'click', () => void createRelease());
  on(qs<HTMLElement>('#uploadReleasePackageButton'), 'click', () => void uploadReleasePackage());
  on(qs<HTMLElement>('#refreshReleasesButton'), 'click', () => void loadVersionPush());
}

function bindScripts(): void {
  on(qs<HTMLElement>('#saveScriptButton'), 'click', () => void saveScriptModule());
  on(qs<HTMLElement>('#publishScriptButton'), 'click', () => void publishScriptModule());
  on(qs<HTMLElement>('#reloadScriptButton'), 'click', () => void loadScriptModule());
}

function bindPayloads(): void {
  on(qs<HTMLElement>('#refreshPayloadsButton'), 'click', () => void loadPayloads());
}

function bindPassword(): void {
  on(qs<HTMLElement>('#changePasswordButton'), 'click', () => void changePassword());
}

function bindAgents(): void {
  on(qs<HTMLElement>('#refreshAgentsButton'), 'click', () => void loadAgents());
  on(qs<HTMLElement>('#exportAgentsCsvButton'), 'click', () => void exportAgentsCSV());
  on(qs<HTMLElement>('#confirmResetAgentPwButton'), 'click', () => void confirmResetAgentPassword());
  qs<HTMLInputElement>('#agentSearch').addEventListener('input', renderAgents);
}

function bindInvites(): void {
  on(qs<HTMLElement>('#createInviteButton'), 'click', () => void createInvites());
  on(qs<HTMLElement>('#refreshInvitesButton'), 'click', () => void loadInvites());
}

function bindAudit(): void {
  qs<HTMLSelectElement>('#auditActionFilter').addEventListener('change', () => void loadAudit(1));
  qs<HTMLInputElement>('#auditQuery').addEventListener('keydown', (event) => {
    if (event.key === 'Enter') void loadAudit(1);
  });
  qsa<HTMLInputElement>('#auditFrom, #auditTo').forEach((input) => input.addEventListener('change', () => void loadAudit(1)));
  on(qs<HTMLElement>('#exportAuditCsvButton'), 'click', () => void exportAuditCSV());
  on(qs<HTMLElement>('#clearAuditFiltersButton'), 'click', () => {
    qs<HTMLSelectElement>('#auditActionFilter').value = '';
    qs<HTMLInputElement>('#auditQuery').value = '';
    qs<HTMLInputElement>('#auditFrom').value = '';
    qs<HTMLInputElement>('#auditTo').value = '';
    void loadAudit(1);
  });
}

async function login(): Promise<void> {
  const password = qs<HTMLInputElement>('#loginPassword').value;
  if (!password) return;
  try {
    await api.post('/admin/api/login', { password });
    shell.showApp('#loginScreen', '#app');
    shell.restoreLastPage();
  } catch (error) {
    qs<HTMLElement>('#loginError').textContent = messageOf(error);
    qs<HTMLElement>('#loginError').style.display = 'block';
  }
}

async function checkAuth(): Promise<void> {
  try {
    await api.get('/admin/api/dashboard');
    shell.showApp('#loginScreen', '#app');
    shell.restoreLastPage();
  } catch {
    showLogin();
  }
}

function showLogin(): void {
  shell.showAuth('#loginScreen', '#app');
}

async function loadDashboard(): Promise<void> {
  const data = await api.get<JsonRecord>('/admin/api/dashboard');
  const stats: JsonRecord = await api.get<JsonRecord>('/admin/api/stats').catch(() => ({}));
  qs<HTMLElement>('#dashboardStats').innerHTML = [
    stat('总卡密数', data.total_cards),
    stat('有效卡密', data.active_cards, 'success'),
    stat('已过期', data.expired_cards, 'danger'),
    stat('在线会话', data.active_sessions, 'warning'),
    stat('总请求数', stats.request_count ?? 0),
    stat('运行时长', formatUptime(Number(stats.uptime_seconds ?? 0)))
  ].join('');
  await loadOpsOverview();
}

async function loadOpsOverview(): Promise<void> {
  const data = await api.get<JsonRecord>('/admin/api/ops/overview');
  qs<HTMLElement>('#opsTodayStats').innerHTML = [
    stat('今日激活', data.today_activations ?? 0, 'success'),
    stat('新机器', data.today_new_machines ?? 0, 'accent'),
    stat('在线会话', data.active_sessions ?? 0, 'warning'),
    stat('风险机器', (data.risky_machines as unknown[] | undefined)?.length ?? 0, 'danger')
  ].join('');
  qs<HTMLElement>('#expiringCardsList').innerHTML = renderExpiringCards(data.expiring_cards as JsonRecord[] | undefined);
  qs<HTMLElement>('#riskyMachinesList').innerHTML = renderRiskyMachines(data.risky_machines as JsonRecord[] | undefined);
  qs<HTMLElement>('#agentLeaderboardList').innerHTML = renderAgentLeaderboard(data.agent_leaderboard as JsonRecord[] | undefined);
  qs<HTMLElement>('#recentAuditList').innerHTML = renderRecentAudit(data.recent_audit as JsonRecord[] | undefined);
  bindDashboardActions();
}

async function loadCards(): Promise<void> {
  const params = new URLSearchParams();
  const status = qs<HTMLSelectElement>('#cardStatusFilter').value;
  const search = qs<HTMLInputElement>('#cardSearch').value;
  if (status) params.set('status', status);
  if (search) params.set('search', search);
  if (cardMachineFilter) params.set('machine', cardMachineFilter);
  setParam(params, 'agent_id', qs<HTMLInputElement>('#cardAgentFilter').value.trim());
  setParam(params, 'bound', qs<HTMLSelectElement>('#cardBoundFilter').value);
  setParam(params, 'expires_after', dateToRFC3339(qs<HTMLInputElement>('#cardExpiresAfter').value));
  setParam(params, 'expires_before', dateToRFC3339(qs<HTMLInputElement>('#cardExpiresBefore').value, true));
  setParam(params, 'max_sessions', qs<HTMLInputElement>('#cardMaxSessionsFilter').value.trim());
  const response = await api.get<{ cards?: Card[] }>(`/admin/api/cards${params.size ? `?${params}` : ''}`);
  cards = response.cards ?? [];
  renderCards();
}

function renderCards(): void {
  const list = filteredCards();
  qs<HTMLElement>('#bulkCount').textContent = `已选 ${selectedCards.size} 项`;
  qs<HTMLElement>('#cardsTable').innerHTML = list.length ? list.map(renderCardRow).join('') : emptyRow(10, '暂无匹配卡密');
  qsa<HTMLInputElement>('[data-card-check]').forEach((checkbox) => {
    checkbox.checked = selectedCards.has(checkbox.value);
    checkbox.addEventListener('change', () => {
      checkbox.checked ? selectedCards.add(checkbox.value) : selectedCards.delete(checkbox.value);
      renderCards();
    });
  });
  qsa<HTMLElement>('[data-card-action]').forEach((button) => {
    on(button, 'click', () => void handleCardAction(String(button.dataset.cardCode), String(button.dataset.cardAction)));
  });
}

function filteredCards(): Card[] {
  const query = qs<HTMLInputElement>('#cardSearch').value.toUpperCase();
  return cards.filter((card) => !query || card.code.includes(query) || String(card.note ?? '').toUpperCase().includes(query));
}

function renderCardRow(card: Card): string {
  const status = card.status === 'active' && new Date(card.expires_at) > new Date() ? 'active' : card.status;
  const label = status === 'active' ? '有效' : status === 'disabled' ? '已禁用' : '已过期';
  return `<tr>
    <td><input type="checkbox" data-card-check value="${escapeHtml(card.code)}"></td>
    <td><span class="mono">${escapeHtml(card.code)}</span></td>
    <td><span class="badge badge-${escapeHtml(status)}">${label}</span></td>
    <td>${card.max_sessions ?? 1}</td>
    <td>${formatDate(card.created_at ?? '')}</td>
    <td>${card.activated_at ? formatDate(card.activated_at) : '-'}</td>
    <td>${formatDate(card.expires_at)}</td>
    <td>${card.machine_id ? `<span class="mono">${escapeHtml(card.machine_id).slice(0, 14)}...</span>` : '-'}</td>
    <td>${escapeHtml(card.note ?? '-')}</td>
    <td>
      <button class="btn btn-sm btn-ghost" data-card-code="${escapeHtml(card.code)}" data-card-action="edit">编辑</button>
      <button class="btn btn-sm btn-ghost" data-card-code="${escapeHtml(card.code)}" data-card-action="extend">延期</button>
      <button class="btn btn-sm btn-ghost" data-card-code="${escapeHtml(card.code)}" data-card-action="${status === 'disabled' ? 'enable' : 'disable'}">${status === 'disabled' ? '启用' : '禁用'}</button>
      <button class="btn btn-sm btn-ghost" data-card-code="${escapeHtml(card.code)}" data-card-action="unbind">解绑</button>
    </td>
  </tr>`;
}

function showGenerate(batch: boolean): void {
  batchMode = batch;
  qs<HTMLElement>('#generateTitle').textContent = batch ? '批量生成卡密' : '生成卡密';
  qs<HTMLElement>('#genCountLabel').hidden = !batch;
  qs<HTMLInputElement>('#genCount').hidden = !batch;
  qs<HTMLInputElement>('#genCount').value = batch ? '10' : '1';
  showModal('generateModal');
}

async function generateCard(): Promise<void> {
  try {
    const payload = {
      count: Number(qs<HTMLInputElement>('#genCount').value || 1),
      duration_hours: Number(qs<HTMLInputElement>('#genDuration').value || 720),
      max_sessions: Number(qs<HTMLInputElement>('#genMaxSessions').value || 1),
      note: qs<HTMLInputElement>('#genNote').value
    };
    const response = await api.post<JsonRecord>(batchMode ? '/admin/api/card/batch-generate' : '/admin/api/card/generate', payload);
    closeModal('generateModal');
    const generated = batchMode ? (response.cards as Card[] | undefined)?.map((card) => card.code).join('\n') : (response.card as Card | undefined)?.code;
    if (generated) void navigator.clipboard?.writeText(generated).catch(() => undefined);
    toast.show(batchMode ? `已生成 ${response.count ?? 0} 张卡密` : '卡密已生成');
    await loadCards();
  } catch (error) {
    toast.show(messageOf(error), 'error');
  }
}

async function handleCardAction(code: string, action: string): Promise<void> {
  if (action === 'edit') return showEditCard(code);
  if (action === 'extend') return showExtendModal(code);
  if ((action === 'disable' || action === 'unbind') && !confirm(`确认${action === 'disable' ? '禁用' : '解绑'} ${code}？`)) return;
  await api.post('/admin/api/card/update', { code, action });
  toast.show('操作已完成');
  await loadCards();
}

function showEditCard(code: string): void {
  const card = cards.find((item) => item.code === code);
  if (!card) return;
  qs<HTMLElement>('#editCardCode').textContent = code;
  qs<HTMLInputElement>('#editCardNote').value = String(card.note ?? '');
  qs<HTMLInputElement>('#editCardMaxSessions').value = String(card.max_sessions ?? 1);
  showModal('editCardModal');
}

async function saveCardDetails(): Promise<void> {
  const code = qs<HTMLElement>('#editCardCode').textContent ?? '';
  await api.post('/admin/api/card/update-details', {
    code,
    note: qs<HTMLInputElement>('#editCardNote').value,
    max_sessions: Number(qs<HTMLInputElement>('#editCardMaxSessions').value || 1)
  });
  closeModal('editCardModal');
  toast.show('卡密信息已更新');
  await loadCards();
}

function showExtendModal(code: string): void {
  extendCode = code;
  qs<HTMLElement>('#extendCardInfo').textContent = code ? `延期卡密：${code}` : `批量延期 ${selectedCards.size} 张卡密`;
  qs<HTMLInputElement>('#extendHours').value = '720';
  showModal('extendModal');
}

async function confirmExtend(): Promise<void> {
  const extend_hours = Number(qs<HTMLInputElement>('#extendHours').value || 720);
  if (extendCode) {
    await api.post('/admin/api/card/update', { code: extendCode, action: 'extend', extend_hours });
  } else {
    const codes = Array.from(selectedCards);
    const preview = await api.post<JsonRecord>('/admin/api/cards/bulk', { codes, action: 'extend', extend_hours, dry_run: true });
    if (!confirmBulkPreview('extend', preview.result as JsonRecord | undefined)) return;
    await api.post('/admin/api/cards/bulk', { codes, action: 'extend', extend_hours });
    selectedCards.clear();
  }
  closeModal('extendModal');
  toast.show('延期已完成');
  await loadCards();
}

async function bulkAction(action: string): Promise<void> {
  if (!selectedCards.size) return toast.show('请先选择卡密', 'warning');
  if (action === 'extend') return showExtendModal('');
  const codes = Array.from(selectedCards);
  const preview = await api.post<JsonRecord>('/admin/api/cards/bulk', { codes, action, dry_run: true });
  if (!confirmBulkPreview(action, preview.result as JsonRecord | undefined)) return;
  const response = await api.post<JsonRecord>('/admin/api/cards/bulk', { codes, action });
  const result = response.result as JsonRecord | undefined;
  toast.show(`批量完成：更新 ${result?.updated ?? response.affected ?? 0} 项，跳过 ${result?.skipped ?? 0} 项`);
  selectedCards.clear();
  await loadCards();
}

function confirmBulkPreview(action: string, result?: JsonRecord): boolean {
  const updated = Number(result?.updated ?? 0);
  const skipped = Number(result?.skipped ?? 0);
  const failed = Number(result?.failed ?? 0);
  const label = bulkActionLabel(action);
  return confirm(`批量${label}预览：预计影响 ${updated} 张，跳过 ${skipped} 张，失败 ${failed} 张。\n确认执行？`);
}

function bulkActionLabel(action: string): string {
  const labels: Record<string, string> = {
    disable: '禁用',
    enable: '启用',
    expire: '过期',
    extend: '延期',
    unbind: '解绑'
  };
  return labels[action] ?? action;
}

async function exportCardsCSV(): Promise<void> {
  const response = await fetch('/admin/api/cards/export', { credentials: 'include' });
  if (!response.ok) throw new ApiError(`导出失败 (HTTP ${response.status})`, response.status);
  downloadBlob(await response.blob(), 'cards_export.csv');
  toast.show('CSV 已导出');
}

async function exportCardsJSON(): Promise<void> {
  const response = await api.get<{ cards?: Card[] }>('/admin/api/cards');
  downloadBlob(new Blob([JSON.stringify(response.cards ?? [], null, 2)], { type: 'application/json' }), `cards_export_${new Date().toISOString().slice(0, 10)}.json`);
  toast.show('JSON 已导出');
}

function showImportModal(): void {
  qs<HTMLElement>('#importReport').innerHTML = '';
  showModal('importModal');
}

function importPayload(dryRun: boolean): JsonRecord | null {
  const csv = qs<HTMLTextAreaElement>('#importCSV').value.trim();
  if (!csv) {
    toast.show('请输入卡密数据', 'warning');
    return null;
  }
  return {
    csv,
    duration: Number(qs<HTMLInputElement>('#importDuration').value || 720),
    max_sessions: Number(qs<HTMLInputElement>('#importMaxSessions').value || 1),
    dry_run: dryRun
  };
}

async function previewImportCards(): Promise<void> {
  const payload = importPayload(true);
  if (!payload) return;
  const response = await api.post<{ report?: CardImportReport }>('/admin/api/cards/import', payload);
  renderImportReport(response.report);
  toast.show('导入预览已生成');
}

async function importCards(): Promise<void> {
  const payload = importPayload(false);
  if (!payload) return;
  const response = await api.post<{ imported?: number; report?: CardImportReport }>('/admin/api/cards/import', payload);
  renderImportReport(response.report);
  toast.show(`已导入 ${response.imported ?? 0} 张卡密`);
  await loadCards();
}

function renderImportReport(report?: CardImportReport): void {
  if (!report) {
    qs<HTMLElement>('#importReport').innerHTML = '';
    return;
  }
  const items = (report.items ?? []).slice(0, 30);
  const summary = [
    stat('数据行', report.total_rows ?? 0),
    stat('可导入', report.valid_rows ?? 0, 'success'),
    stat('已导入', report.imported ?? 0, 'accent'),
    stat('重复', report.duplicates ?? 0, Number(report.duplicates ?? 0) ? 'warning' : 'success'),
    stat('无效', report.invalid ?? 0, Number(report.invalid ?? 0) ? 'danger' : 'success'),
    stat('跳过', report.skipped ?? 0, 'warning')
  ].join('');
  const rows = items.map((item) => [
    escapeHtml(item.row ?? '-'),
    `<span class="mono">${escapeHtml(item.code ?? '-')}</span>`,
    escapeHtml(importStatusLabel(String(item.status ?? ''))),
    escapeHtml(item.message ?? item.note ?? '-')
  ]);
  qs<HTMLElement>('#importReport').innerHTML = `<div class="release-metrics">${summary}</div>${table(['行', '卡密', '状态', '说明'], rows, '暂无明细')}`;
}

function importStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    valid: '可导入',
    imported: '已导入',
    duplicate: '重复',
    invalid: '无效',
    skipped: '跳过'
  };
  return labels[status] ?? status;
}

async function loadSessions(): Promise<void> {
  const response = await api.get<{ sessions?: SessionRecord[]; total?: number }>('/admin/api/sessions');
  const sessions = response.sessions ?? [];
  qs<HTMLElement>('#sessionsSummary').textContent = `当前 ${response.total ?? sessions.length} 个活跃会话`;
  qs<HTMLElement>('#sessionsTable').innerHTML = sessions.length ? sessions.map((s) => `<tr>
    <td><span class="mono">${escapeHtml(s.token).slice(0, 16)}...</span></td>
    <td><span class="mono">${escapeHtml(s.card_code)}</span></td>
    <td><span class="mono">${escapeHtml(s.machine_id).slice(0, 14)}...</span></td>
    <td>${s.client_version ? `<span class="badge badge-active">v${escapeHtml(s.client_version)}</span>` : '-'}</td>
    <td>${escapeHtml(s.remote_addr ?? '-')}</td>
    <td>${formatDate(s.last_seen)}</td>
    <td>${formatDate(s.expires_at)}</td>
    <td><button class="btn btn-sm btn-danger" data-force-logout="${escapeHtml(s.token)}">强制下线</button></td>
  </tr>`).join('') : emptyRow(8, '暂无在线会话');
  qsa<HTMLElement>('[data-force-logout]').forEach((button) => on(button, 'click', () => void forceLogout(String(button.dataset.forceLogout))));
}

async function forceLogout(token: string): Promise<void> {
  if (!confirm('确认强制下线？')) return;
  await api.post('/admin/api/force-logout', { session_token: token });
  toast.show('已下线');
  await loadSessions();
}

async function loadMachines(): Promise<void> {
  const response = await api.get<{ machines?: JsonRecord[] }>('/admin/api/machines');
  const machines = response.machines ?? [];
  qs<HTMLElement>('#machinesList').innerHTML = machines.length ? machines.map((machine) => {
    const id = String(machine.machine_id ?? '');
    return `<button class="list-item machine-card" data-machine-id="${escapeHtml(id)}"><strong class="mono">${escapeHtml(id).slice(0, 28)}...</strong><span>绑定 ${machine.card_count ?? 0} 张卡密</span><em>${machine.is_blacklisted ? '已封禁' : formatDate(String(machine.last_seen ?? ''))}</em></button>`;
  }).join('') : emptyBlock('暂无绑定机器');
  qsa<HTMLElement>('[data-machine-id]').forEach((button) => on(button, 'click', () => void showMachineDetail(String(button.dataset.machineId))));
}

async function showMachineDetail(machineID: string): Promise<void> {
  const response = await api.get<{ cards?: Card[] }>(`/admin/api/machine/cards?id=${encodeURIComponent(machineID)}`);
  qs<HTMLElement>('#machineModalTitle').innerHTML = `机器 <span class="mono">${escapeHtml(machineID).slice(0, 24)}...</span> 的卡密`;
  qs<HTMLElement>('#machineDetailContent').innerHTML = table(['卡密', '状态', '到期', '备注'], (response.cards ?? []).map((card) => [
    `<span class="mono">${escapeHtml(card.code)}</span>`, escapeHtml(card.status), formatDate(card.expires_at), escapeHtml(card.note ?? '-')
  ]), '无绑定卡密');
  showModal('machineModal');
}

async function loadBlacklist(): Promise<void> {
  const response = await api.get<{ entries?: JsonRecord[] }>('/admin/api/blacklist');
  const entries = response.entries ?? [];
  qs<HTMLElement>('#blacklistTable').innerHTML = entries.length ? entries.map((entry) => {
    const value = String(entry.value ?? '');
    return `<tr><td>${escapeHtml(typeLabel(String(entry.type)))}</td><td><span class="mono">${escapeHtml(value)}</span></td><td>${escapeHtml(entry.reason ?? '-')}</td><td>${formatDate(String(entry.created_at ?? ''))}</td><td><button class="btn btn-sm btn-ghost" data-remove-blacklist="${escapeHtml(value)}">移除</button></td></tr>`;
  }).join('') : emptyRow(5, '黑名单为空');
  qsa<HTMLElement>('[data-remove-blacklist]').forEach((button) => on(button, 'click', () => void removeBlacklist(String(button.dataset.removeBlacklist))));
}

async function addBlacklist(): Promise<void> {
  const value = qs<HTMLInputElement>('#blValue').value.trim();
  if (!value) return toast.show('请输入封禁值', 'warning');
  await api.post('/admin/api/blacklist', {
    action: 'add',
    type: qs<HTMLSelectElement>('#blType').value,
    value,
    reason: qs<HTMLInputElement>('#blReason').value
  });
  closeModal('blacklistModal');
  toast.show('已加入黑名单');
  await loadBlacklist();
}

async function removeBlacklist(value: string): Promise<void> {
  if (!confirm('确认移出黑名单？')) return;
  await api.post('/admin/api/blacklist', { action: 'remove', value });
  toast.show('已移除');
  await loadBlacklist();
}

async function loadAnnouncement(): Promise<void> {
  const response = await api.get<{ announcement?: JsonRecord | null; announcements?: JsonRecord[] }>('/admin/api/announcement');
  announcement = response.announcement ?? null;
  announcementHistory = response.announcements ?? [];
  qs<HTMLTextAreaElement>('#announceContent').value = String(announcement?.content ?? '');
  renderAnnouncementPreview();
  renderAnnouncementList();
}

function renderAnnouncementPreview(): void {
  const content = qs<HTMLTextAreaElement>('#announceContent').value.trim();
  qs<HTMLElement>('#announcePreview').innerHTML = content ? escapeHtml(content).replace(/\n/g, '<br>') : '<span class="empty-state">当前无公告</span>';
}

async function saveAnnouncementDraft(): Promise<void> {
  const content = qs<HTMLTextAreaElement>('#announceContent').value.trim();
  await saveAnnouncementPayload(content, String(announcement?.latest_version ?? ''), String(announcement?.min_version ?? ''), Boolean(announcement?.force_update), 'save');
  toast.show('公告草稿已保存');
  await loadAnnouncement();
}

async function publishAnnouncementNow(): Promise<void> {
  const content = qs<HTMLTextAreaElement>('#announceContent').value.trim();
  await saveAnnouncementPayload(content, String(announcement?.latest_version ?? ''), String(announcement?.min_version ?? ''), Boolean(announcement?.force_update), '');
  toast.show(content ? '公告已发布' : '公告已清除');
  await loadAnnouncement();
}

async function clearAnnouncement(): Promise<void> {
  qs<HTMLTextAreaElement>('#announceContent').value = '';
  await publishAnnouncementNow();
}

function renderAnnouncementList(): void {
  qs<HTMLElement>('#announcementList').innerHTML = announcementHistory.length ? announcementHistory.map((item) => {
    const id = String(item.id ?? '');
    const active = id && id === String(announcement?.id ?? '');
    const status = active ? '当前发布' : String(item.status ?? 'draft') === 'draft' ? '草稿' : '历史版本';
    return `<div class="list-item ${active ? 'success-item' : ''}">
      <strong>${escapeHtml(status)}</strong>
      <span>${formatDate(String(item.updated_at ?? item.created_at ?? ''))} · ${formatBytes(Number(String(item.content ?? '').length))}</span>
      <em>${escapeHtml(String(item.content ?? '无内容')).slice(0, 60)}</em>
      <button class="btn btn-sm btn-ghost" data-load-announcement="${escapeHtml(id)}">编辑</button>
      <button class="btn btn-sm btn-success" data-publish-announcement="${escapeHtml(id)}" ${active ? 'disabled' : ''}>启用</button>
      <button class="btn btn-sm btn-danger" data-delete-announcement="${escapeHtml(id)}" ${active ? 'disabled' : ''}>删除</button>
    </div>`;
  }).join('') : emptyBlock('暂无公告版本');
  qsa<HTMLElement>('[data-load-announcement]').forEach((button) => on(button, 'click', () => loadAnnouncementDraft(String(button.dataset.loadAnnouncement))));
  qsa<HTMLElement>('[data-publish-announcement]').forEach((button) => on(button, 'click', () => void publishAnnouncementRevision(String(button.dataset.publishAnnouncement))));
  qsa<HTMLElement>('[data-delete-announcement]').forEach((button) => on(button, 'click', () => void deleteAnnouncementRevision(String(button.dataset.deleteAnnouncement))));
}

function loadAnnouncementDraft(id: string): void {
  const item = announcementHistory.find((entry) => String(entry.id ?? '') === id);
  if (!item) return;
  qs<HTMLTextAreaElement>('#announceContent').value = String(item.content ?? '');
  renderAnnouncementPreview();
}

async function publishAnnouncementRevision(id: string): Promise<void> {
  await api.post('/admin/api/announcement', { action: 'publish', id });
  toast.show('公告版本已启用');
  await loadAnnouncement();
}

async function deleteAnnouncementRevision(id: string): Promise<void> {
  if (!confirm('确认删除这个公告版本？当前发布公告不会被允许删除。')) return;
  await api.post('/admin/api/announcement', { action: 'delete', id });
  toast.show('公告版本已删除');
  await loadAnnouncement();
}

async function loadVersionPush(): Promise<void> {
  const response = await api.get<JsonRecord>('/admin/api/releases');
  releaseItems = (response.releases as JsonRecord[] | undefined) ?? [];
  qs<HTMLElement>('#releasePublicKey').innerHTML = response.public_key
    ? `Manifest 公钥：<span class="mono">${escapeHtml(response.public_key)}</span>`
    : 'Manifest 公钥尚未生成';
  renderReleaseList();
  renderReleasePackageTargets();
}

async function saveAnnouncementPayload(content: string, latest_version: string, min_version: string, force_update: boolean, action: string): Promise<void> {
  await api.post('/admin/api/announcement', { content, latest_version, min_version, force_update, action });
}

async function createRelease(): Promise<void> {
  const version = qs<HTMLInputElement>('#releaseVersion').value.trim();
  const channel = qs<HTMLSelectElement>('#releaseChannel').value;
  const rollout_percent = Number(qs<HTMLInputElement>('#releaseRollout').value || 100);
  const min_version = qs<HTMLInputElement>('#releaseMinVersion').value.trim();
  const force_update = qs<HTMLInputElement>('#releaseForceUpdate').checked;
  const notes = qs<HTMLTextAreaElement>('#releaseNotes').value.trim();
  if (!version) return toast.show('请输入版本号', 'warning');
  await api.post('/admin/api/releases', {
    version,
    channel,
    rollout_percent,
    min_version,
    force_update,
    notes,
    targeting: {
      allow_cards: splitTargetList(qs<HTMLTextAreaElement>('#releaseAllowCards').value),
      deny_cards: splitTargetList(qs<HTMLTextAreaElement>('#releaseDenyCards').value),
      allow_machines: splitTargetList(qs<HTMLTextAreaElement>('#releaseAllowMachines').value),
      deny_machines: splitTargetList(qs<HTMLTextAreaElement>('#releaseDenyMachines').value),
      allow_agents: splitTargetList(qs<HTMLTextAreaElement>('#releaseAllowAgents').value),
      deny_agents: splitTargetList(qs<HTMLTextAreaElement>('#releaseDenyAgents').value)
    }
  });
  toast.show('发布草稿已创建');
  qs<HTMLInputElement>('#releaseVersion').value = '';
  qs<HTMLTextAreaElement>('#releaseNotes').value = '';
  await loadVersionPush();
}

async function uploadReleasePackage(): Promise<void> {
  const releaseID = qs<HTMLSelectElement>('#releasePackageTarget').value;
  const kind = qs<HTMLSelectElement>('#releasePackageKind').value;
  const file = qs<HTMLInputElement>('#releasePackageFile').files?.[0];
  const progress = qs<HTMLElement>('#releaseUploadProgress');
  if (!releaseID) return toast.show('请选择发布版本', 'warning');
  if (!file) return toast.show('请选择安装包文件', 'warning');
  const lower = file.name.toLowerCase();
  if (!lower.endsWith('.exe') && !lower.endsWith('.msi')) return toast.show('只支持 .exe 或 .msi', 'warning');
  const form = new FormData();
  form.append('kind', kind);
  form.append('file', file);
  progress.style.display = 'block';
  progress.textContent = '正在上传安装包...';
  const response = await fetch(`/admin/api/releases/${encodeURIComponent(releaseID)}/packages`, { method: 'POST', credentials: 'include', body: form });
  const payload = await response.json() as JsonRecord;
  if (payload.status === 'error' || !response.ok) throw new ApiError(String(payload.message ?? '上传失败'), response.status);
  progress.textContent = '安装包已上传并写入发布记录';
  qs<HTMLInputElement>('#releasePackageFile').value = '';
  toast.show('安装包已上传');
  await loadVersionPush();
}

function splitTargetList(value: string): string[] {
  return value.split(/[\s,，]+/).map((item) => item.trim()).filter(Boolean);
}

async function loadScriptModule(): Promise<void> {
  const response = await api.get<JsonRecord>('/admin/api/script');
  const script = response.script as JsonRecord | null | undefined;
  scripts = (response.scripts as JsonRecord[] | undefined) ?? [];
  const hasScript = Boolean(response.version || script);
  const version = String(response.version ?? script?.version ?? '');
  const sha = String(response.sha256 ?? script?.sha256 ?? '');
  const content = String(response.content ?? script?.content ?? '');
  const size = Number(response.size ?? script?.size ?? 0);
  const updatedAt = String(response.updated_at ?? script?.updated_at ?? '');
  selectedScriptID = String(response.active_id ?? script?.id ?? '');
  qs<HTMLInputElement>('#scriptVersion').value = version;
  qs<HTMLInputElement>('#scriptSha').value = sha;
  qs<HTMLInputElement>('#scriptNote').value = String(script?.note ?? '');
  qs<HTMLTextAreaElement>('#scriptContent').value = content;
  qs<HTMLElement>('#scriptCurrentInfo').innerHTML = hasScript
    ? `当前脚本：<b>v${escapeHtml(version)}</b> · ${formatBytes(size)} · ${formatDate(updatedAt)}<br><span class="mono">${escapeHtml(sha)}</span>`
    : '暂无发布脚本，客户端会使用 DLL 内嵌降级脚本';
  renderScriptList();
}

async function saveScriptModule(): Promise<void> {
  await saveScriptModuleWithAction('save');
}

async function publishScriptModule(): Promise<void> {
  await saveScriptModuleWithAction('');
}

async function saveScriptModuleWithAction(action: string): Promise<void> {
  const version = qs<HTMLInputElement>('#scriptVersion').value.trim();
  const note = qs<HTMLInputElement>('#scriptNote').value.trim();
  const content = qs<HTMLTextAreaElement>('#scriptContent').value.trim();
  if (!content) return toast.show('脚本内容不能为空', 'warning');
  const response = await api.post<JsonRecord>('/admin/api/script', { version, content, note, action });
  qs<HTMLInputElement>('#scriptVersion').value = String(response.version ?? version);
  qs<HTMLInputElement>('#scriptSha').value = String(response.sha256 ?? '');
  qs<HTMLElement>('#scriptCurrentInfo').innerHTML = `当前脚本：<b>v${escapeHtml(response.version ?? version)}</b> · ${formatBytes(Number(response.size ?? 0))} · ${formatDate(String(response.updated_at ?? ''))}<br><span class="mono">${escapeHtml(response.sha256 ?? '')}</span>`;
  toast.show(action === 'save' ? '脚本已保存，可在列表中启用' : '脚本模块已发布');
  await loadScriptModule();
}

function renderScriptList(): void {
  qs<HTMLElement>('#scriptList').innerHTML = scripts.length ? scripts.map((script) => {
    const id = String(script.id ?? '');
    const active = Boolean(script.active);
    return `<div class="list-item ${active ? 'success-item' : ''}">
      <strong>${escapeHtml(script.name ?? script.version ?? id)}</strong>
      <span>v${escapeHtml(script.version ?? '-')} · ${formatBytes(Number(script.size ?? 0))}</span>
      <em>${active ? '当前启用' : formatDate(String(script.updated_at ?? ''))}</em>
      <button class="btn btn-sm btn-ghost" data-load-script="${escapeHtml(id)}">编辑</button>
      <button class="btn btn-sm btn-success" data-activate-script="${escapeHtml(id)}" ${active ? 'disabled' : ''}>启用</button>
      <button class="btn btn-sm btn-danger" data-delete-script="${escapeHtml(id)}" ${active ? 'disabled' : ''}>删除</button>
    </div>`;
  }).join('') : emptyBlock('暂无脚本版本');
  qsa<HTMLElement>('[data-load-script]').forEach((button) => on(button, 'click', () => void loadScriptDetail(String(button.dataset.loadScript))));
  qsa<HTMLElement>('[data-activate-script]').forEach((button) => on(button, 'click', () => void activateScript(String(button.dataset.activateScript))));
  qsa<HTMLElement>('[data-delete-script]').forEach((button) => on(button, 'click', () => void deleteScript(String(button.dataset.deleteScript))));
}

async function loadScriptDetail(id: string): Promise<void> {
  const response = await api.get<JsonRecord>(`/admin/api/script?id=${encodeURIComponent(id)}`);
  const script = response.script as JsonRecord | undefined;
  if (!script) return;
  selectedScriptID = String(script.id ?? id);
  qs<HTMLInputElement>('#scriptVersion').value = String(script.version ?? '');
  qs<HTMLInputElement>('#scriptSha').value = String(script.sha256 ?? '');
  qs<HTMLInputElement>('#scriptNote').value = String(script.note ?? '');
  qs<HTMLTextAreaElement>('#scriptContent').value = String(script.content ?? '');
}

async function activateScript(id: string): Promise<void> {
  await api.post('/admin/api/script', { action: 'activate', id });
  toast.show('脚本已启用');
  await loadScriptModule();
}

async function deleteScript(id: string): Promise<void> {
  if (!confirm('确认删除这个脚本版本？当前启用脚本不会被允许删除。')) return;
  await api.post('/admin/api/script', { action: 'delete', id });
  if (selectedScriptID === id) selectedScriptID = '';
  toast.show('脚本已删除');
  await loadScriptModule();
}

function renderReleaseList(): void {
  qs<HTMLElement>('#releasesList').innerHTML = releaseItems.length ? releaseItems.map((item) => {
    const release = (item.release as JsonRecord | undefined) ?? {};
    const packages = (item.packages as JsonRecord[] | undefined) ?? [];
    const metrics = (item.metrics as JsonRecord | undefined) ?? {};
    const id = String(release.id ?? '');
    const status = String(release.status ?? 'draft');
    const version = String(release.version ?? '');
    const channel = String(release.channel ?? 'stable');
    const packageText = packages.length
      ? packages.map((pkg) => `${escapeHtml(pkg.filename ?? '-')} · ${formatBytes(Number(pkg.file_size ?? 0))}`).join('<br>')
      : '未上传安装包';
    return `<div class="release-card ${statusToneClass(status)}">
      <div>
        <strong>v${escapeHtml(version)} · ${escapeHtml(channelLabel(channel))}</strong>
        <span class="badge ${releaseStatusClass(status)}">${escapeHtml(releaseStatusLabel(status))}</span>
        <p>${escapeHtml(release.notes ?? '无更新说明')}</p>
        <p class="mono">${escapeHtml(id)}</p>
      </div>
      <div class="release-meta">
        <span>灰度 ${escapeHtml(release.rollout_percent ?? 0)}%</span>
        <span>最低版本 ${escapeHtml(release.min_version ?? '-')}</span>
        <span>${Boolean(release.force_update) ? '强制更新' : '非强制'}</span>
        <span>${packageText}</span>
      </div>
      <div class="release-metrics">
        ${stat('触达', metrics.offered ?? 0)}
        ${stat('成功', metrics.install_success ?? 0, 'success')}
        ${stat('失败', Number(metrics.download_failed ?? 0) + Number(metrics.install_failed ?? 0), 'danger')}
      </div>
      <div class="release-actions">
        <button class="btn btn-sm btn-success" data-release-action="publish" data-release-id="${escapeHtml(id)}">发布</button>
        <button class="btn btn-sm btn-ghost" data-release-action="pause" data-release-id="${escapeHtml(id)}">暂停</button>
        <button class="btn btn-sm btn-danger" data-release-action="rollback" data-release-id="${escapeHtml(id)}">回滚</button>
        <button class="btn btn-sm btn-ghost" data-release-events="${escapeHtml(id)}">事件</button>
      </div>
    </div>`;
  }).join('') : emptyBlock('暂无 release，先创建一个发布草稿');
  qsa<HTMLElement>('[data-release-action]').forEach((button) => on(button, 'click', () => void runReleaseAction(String(button.dataset.releaseId), String(button.dataset.releaseAction))));
  qsa<HTMLElement>('[data-release-events]').forEach((button) => on(button, 'click', () => void loadReleaseEvents(String(button.dataset.releaseEvents))));
}

function renderReleasePackageTargets(): void {
  const select = qs<HTMLSelectElement>('#releasePackageTarget');
  const options = releaseItems.map((item) => {
    const release = (item.release as JsonRecord | undefined) ?? {};
    const id = String(release.id ?? '');
    return `<option value="${escapeHtml(id)}">v${escapeHtml(release.version ?? '')} · ${escapeHtml(release.channel ?? 'stable')} · ${escapeHtml(id)}</option>`;
  }).join('');
  select.innerHTML = options || '<option value="">暂无 release</option>';
}

async function runReleaseAction(id: string, action: string): Promise<void> {
  if (!id) return;
  let body: JsonRecord | undefined;
  if (action === 'rollback') {
    const target = prompt('回滚目标 release ID，可留空仅标记该版本回滚') ?? '';
    body = { target_release_id: target.trim() };
  }
  await api.post(`/admin/api/releases/${encodeURIComponent(id)}/${encodeURIComponent(action)}`, body);
  toast.show(action === 'publish' ? '发布已启用' : action === 'pause' ? '发布已暂停' : '发布已回滚');
  await loadVersionPush();
}

async function loadReleaseEvents(id: string): Promise<void> {
  const response = await api.get<JsonRecord>(`/admin/api/releases/${encodeURIComponent(id)}/events`);
  const events = (response.events as JsonRecord[] | undefined) ?? [];
  const metrics = (response.metrics as JsonRecord | undefined) ?? {};
  qs<HTMLElement>('#releaseEventsPanel').innerHTML = `
    <div class="stats-grid compact">
      ${stat('触达', metrics.offered ?? 0)}
      ${stat('下载失败', metrics.download_failed ?? 0, 'danger')}
      ${stat('安装成功', metrics.install_success ?? 0, 'success')}
      ${stat('回滚', metrics.rollback ?? 0, 'warning')}
    </div>
    ${table(['时间', '事件', '机器', '错误'], events.map((event) => [
      formatDate(String(event.created_at ?? '')),
      escapeHtml(event.type ?? ''),
      `<span class="mono">${escapeHtml(event.machine_id ?? '-')}</span>`,
      escapeHtml(event.error_code ?? '-')
    ]), '暂无更新事件')}
  `;
}

function releaseStatusLabel(status: string): string {
  return ({ draft: '草稿', published: '已发布', paused: '已暂停', rolled_back: '已回滚' } as Record<string, string>)[status] ?? status;
}

function releaseStatusClass(status: string): string {
  if (status === 'published') return 'badge-active';
  if (status === 'paused') return 'badge-expired';
  if (status === 'rolled_back') return 'badge-disabled';
  return '';
}

function statusToneClass(status: string): string {
  if (status === 'published') return 'success-item';
  if (status === 'rolled_back') return 'danger-item';
  if (status === 'paused') return 'warning-item';
  return '';
}

function channelLabel(channel: string): string {
  return ({ stable: 'Stable', beta: 'Beta', canary: 'Canary' } as Record<string, string>)[channel] ?? channel;
}

async function loadPayloads(): Promise<void> {
  const response = await api.get<JsonRecord>('/admin/api/payloads');
  payloadItems = (response.payloads as JsonRecord[] | undefined) ?? [];
  const active = response.active as JsonRecord | null | undefined;
  qs<HTMLElement>('#payloadCurrentInfo').innerHTML = active?.payload_id
    ? `当前启用载荷：<b class="mono">${escapeHtml(active.payload_id)}</b> · ${formatBytes(Number(active.total_size ?? 0))} · ${formatDate(String(active.created_at ?? ''))}<br><span class="mono">${escapeHtml(active.exe_hash ?? '')}</span>`
    : '暂无启用载荷，客户端只能按显式 payload_id 获取载荷';
  renderPayloads();
}

function renderPayloads(): void {
  qs<HTMLElement>('#payloadsTable').innerHTML = payloadItems.length ? payloadItems.map((payload) => {
    const id = String(payload.payload_id ?? '');
    const active = Boolean(payload.active);
    return `<tr>
      <td><span class="mono">${escapeHtml(id)}</span></td>
      <td><span class="badge badge-${active ? 'active' : 'disabled'}">${active ? '当前启用' : '待启用'}</span></td>
      <td>${formatBytes(Number(payload.total_size ?? 0))}</td>
      <td>${escapeHtml(payload.chunk_count ?? 0)} × ${formatBytes(Number(payload.chunk_size ?? 0))}</td>
      <td><span class="mono">${escapeHtml(payload.exe_hash ?? '-').slice(0, 20)}...</span></td>
      <td>${formatDate(String(payload.created_at ?? ''))}</td>
      <td><button class="btn btn-sm btn-success" data-payload-action="activate" data-payload-id="${escapeHtml(id)}" ${active ? 'disabled' : ''}>启用</button> <button class="btn btn-sm btn-danger" data-payload-action="delete" data-payload-id="${escapeHtml(id)}" ${active ? 'disabled' : ''}>删除</button></td>
    </tr>`;
  }).join('') : emptyRow(7, '暂无载荷记录');
  qsa<HTMLElement>('[data-payload-action]').forEach((button) => on(button, 'click', () => void managePayload(String(button.dataset.payloadId), String(button.dataset.payloadAction))));
}

async function managePayload(id: string, action: string): Promise<void> {
  if (!id) return;
  if (action === 'delete' && !confirm('确认删除这个载荷？当前启用载荷不会被允许删除。')) return;
  await api.post('/admin/api/payloads/manage', { action, payload_id: id });
  toast.show(action === 'activate' ? '载荷已启用' : '载荷已删除');
  await loadPayloads();
}

async function changePassword(): Promise<void> {
  const old_password = qs<HTMLInputElement>('#oldPassword').value;
  const new_password = qs<HTMLInputElement>('#newPassword').value;
  const confirm = qs<HTMLInputElement>('#confirmPassword').value;
  if (!old_password || !new_password) return toast.show('请填写完整', 'warning');
  if (new_password !== confirm) return toast.show('两次输入的新密码不一致', 'warning');
  if (new_password.length < 6) return toast.show('新密码至少 6 位', 'warning');
  await api.post('/admin/api/password', { old_password, new_password });
  toast.show('密码已修改，请重新登录');
  setTimeout(showLogin, 1200);
}

async function loadAudit(page = 1): Promise<void> {
  auditPage = page;
  const params = auditParams(page);
  const response = await api.get<{ entries?: JsonRecord[]; total?: number; page?: number; per_page?: number }>(`/admin/api/audit?${params}`);
  const entries = response.entries ?? [];
  const perPage = Number(response.per_page ?? 30);
  const totalPages = Math.max(1, Math.ceil(Number(response.total ?? 0) / perPage));
  qs<HTMLElement>('#auditSummary').textContent = `共 ${response.total ?? 0} 条记录，第 ${response.page ?? page}/${totalPages} 页`;
  qs<HTMLElement>('#auditLog').innerHTML = entries.length ? entries.map((entry) => `<div class="audit-entry"><span class="audit-time">${formatDate(String(entry.time ?? ''))}</span><span class="audit-action">${escapeHtml(actionLabel(String(entry.action ?? '')))}</span><span class="audit-detail">${escapeHtml(entry.detail ?? '')} ${entry.card ? `卡密:${escapeHtml(entry.card)}` : ''} ${entry.machine ? `机器:${escapeHtml(String(entry.machine)).slice(0, 14)}...` : ''}</span></div>`).join('') : emptyBlock('暂无日志');
  qs<HTMLElement>('#auditPagination').innerHTML = `<button class="btn btn-sm btn-ghost" id="auditPrev" ${auditPage <= 1 ? 'disabled' : ''}>上一页</button><span>第 ${auditPage}/${totalPages} 页</span><button class="btn btn-sm btn-ghost" id="auditNext" ${auditPage >= totalPages ? 'disabled' : ''}>下一页</button>`;
  on(qs<HTMLElement>('#auditPrev'), 'click', () => void loadAudit(auditPage - 1));
  on(qs<HTMLElement>('#auditNext'), 'click', () => void loadAudit(auditPage + 1));
}

async function exportAuditCSV(): Promise<void> {
  const params = auditParams(1);
  params.set('export', 'csv');
  params.delete('page');
  params.delete('per_page');
  const response = await fetch(`/admin/api/audit?${params}`, { credentials: 'include' });
  if (!response.ok) throw new ApiError(`导出失败 (HTTP ${response.status})`, response.status);
  downloadBlob(await response.blob(), `audit_export_${new Date().toISOString().slice(0, 10)}.csv`);
  toast.show('审计 CSV 已导出');
}

function auditParams(page: number): URLSearchParams {
  const action = qs<HTMLSelectElement>('#auditActionFilter').value;
  const params = new URLSearchParams({ page: String(page), per_page: '30' });
  if (action) params.set('action', action);
  setParam(params, 'q', qs<HTMLInputElement>('#auditQuery').value.trim());
  setParam(params, 'from', dateToRFC3339(qs<HTMLInputElement>('#auditFrom').value));
  setParam(params, 'to', dateToRFC3339(qs<HTMLInputElement>('#auditTo').value, true));
  return params;
}

async function loadAgents(): Promise<void> {
  const response = await api.get<{ agents?: JsonRecord[] }>('/admin/api/agents');
  agents = response.agents ?? [];
  renderAgents();
}

function renderAgents(): void {
  const query = qs<HTMLInputElement>('#agentSearch').value.toLowerCase();
  const filtered = agents.filter((agent) => !query || String(agent.username ?? '').toLowerCase().includes(query) || String(agent.id ?? '').toLowerCase().includes(query));
  qs<HTMLElement>('#agentsTable').innerHTML = filtered.length ? filtered.map((agent) => {
    const id = String(agent.id ?? '');
    const user = String(agent.username ?? '');
    const disabled = Boolean(agent.disabled);
    return `<tr><td><span class="mono">${escapeHtml(id)}</span></td><td><strong>${escapeHtml(user)}</strong></td><td><button class="btn btn-sm btn-ghost" data-agent-cards="${escapeHtml(id)}" data-agent-user="${escapeHtml(user)}">总 ${escapeHtml(agent.total_cards ?? 0)}</button></td><td>${escapeHtml(agent.active_cards ?? 0)}</td><td>${escapeHtml(agent.expired_cards ?? 0)}</td><td>${escapeHtml(agent.bound_machines ?? 0)}</td><td>${formatDate(String(agent.last_card_created_at ?? ''))}</td><td>${formatDate(String(agent.created_at ?? ''))}</td><td><span class="badge badge-${disabled ? 'disabled' : 'active'}">${disabled ? '已禁用' : '正常'}</span></td><td><button class="btn btn-sm btn-ghost" data-agent-toggle="${escapeHtml(id)}" data-agent-disable="${disabled ? 'false' : 'true'}">${disabled ? '启用' : '禁用'}</button> <button class="btn btn-sm btn-ghost" data-agent-reset="${escapeHtml(id)}" data-agent-user="${escapeHtml(user)}">重置密码</button> <button class="btn btn-sm btn-danger" data-agent-delete="${escapeHtml(id)}">删除</button></td></tr>`;
  }).join('') : emptyRow(10, '暂无代理账号');
  qsa<HTMLElement>('[data-agent-toggle]').forEach((button) => on(button, 'click', () => void toggleAgent(String(button.dataset.agentToggle), button.dataset.agentDisable === 'true')));
  qsa<HTMLElement>('[data-agent-reset]').forEach((button) => on(button, 'click', () => showResetAgentPassword(String(button.dataset.agentReset), String(button.dataset.agentUser))));
  qsa<HTMLElement>('[data-agent-delete]').forEach((button) => on(button, 'click', () => void deleteAgent(String(button.dataset.agentDelete))));
  qsa<HTMLElement>('[data-agent-cards]').forEach((button) => on(button, 'click', () => void viewAgentCards(String(button.dataset.agentCards), String(button.dataset.agentUser))));
}

async function exportAgentsCSV(): Promise<void> {
  const response = await fetch('/admin/api/agents?export=csv', { credentials: 'include' });
  if (!response.ok) throw new ApiError(`导出失败 (HTTP ${response.status})`, response.status);
  downloadBlob(await response.blob(), `agents_export_${new Date().toISOString().slice(0, 10)}.csv`);
  toast.show('代理 CSV 已导出');
}

async function toggleAgent(id: string, disable: boolean): Promise<void> {
  await api.post('/admin/api/agent/update', { agent_id: id, action: disable ? 'disable' : 'enable' });
  toast.show(disable ? '代理已禁用' : '代理已启用');
  await loadAgents();
}

async function deleteAgent(id: string): Promise<void> {
  if (!confirm('确认删除此代理？')) return;
  await api.post('/admin/api/agent/update', { agent_id: id, action: 'delete' });
  toast.show('代理已删除');
  await loadAgents();
}

function showResetAgentPassword(id: string, username: string): void {
  resetAgentID = id;
  qs<HTMLElement>('#resetAgentInfo').textContent = `重置代理 ${username} 的密码`;
  qs<HTMLInputElement>('#resetAgentPassword').value = '';
  showModal('resetAgentPwModal');
}

async function confirmResetAgentPassword(): Promise<void> {
  const password = qs<HTMLInputElement>('#resetAgentPassword').value;
  if (password.length < 6) return toast.show('密码至少 6 位', 'warning');
  await api.post('/admin/api/agent/update', { agent_id: resetAgentID, action: 'reset_password', password });
  closeModal('resetAgentPwModal');
  toast.show('代理密码已重置');
}

async function viewAgentCards(id: string, username: string): Promise<void> {
  const response = await api.get<{ cards?: Card[] }>(`/admin/api/agent/cards?agent_id=${encodeURIComponent(id)}`);
  qs<HTMLElement>('#agentCardsTitle').innerHTML = `代理 <span class="accent">${escapeHtml(username)}</span> 的卡密`;
  qs<HTMLElement>('#agentCardsContent').innerHTML = table(['卡密', '状态', '到期', '机器', '备注'], (response.cards ?? []).map((card) => [
    `<span class="mono">${escapeHtml(card.code)}</span>`, escapeHtml(card.status), formatDate(card.expires_at), card.machine_id ? `<span class="mono">${escapeHtml(card.machine_id).slice(0, 12)}...</span>` : '-', escapeHtml(card.note ?? '-')
  ]), '该代理暂无卡密');
  showModal('agentCardsModal');
}

async function loadInvites(): Promise<void> {
  const response = await api.get<{ invites?: JsonRecord[] }>('/admin/api/invites');
  invites = response.invites ?? [];
  renderInvites();
}

function renderInvites(): void {
  qs<HTMLElement>('#invitesTable').innerHTML = invites.length ? invites.map((invite) => {
    const code = String(invite.code ?? '');
    const maxUses = Number(invite.max_uses ?? 0);
    const useCount = Number(invite.use_count ?? 0);
    const usedUp = maxUses > 0 && useCount >= maxUses;
    return `<tr><td><span class="mono">${escapeHtml(code)}</span></td><td><span class="badge badge-${usedUp ? 'disabled' : 'active'}">${usedUp ? '已用完' : '可用'}</span></td><td>${maxUses === 0 ? `${useCount}/不限` : `${useCount}/${maxUses}`}</td><td>${formatDate(String(invite.created_at ?? ''))}</td><td><button class="btn btn-sm btn-ghost" data-copy-invite="${escapeHtml(code)}">复制</button> <button class="btn btn-sm btn-danger" data-delete-invite="${escapeHtml(code)}">删除</button></td></tr>`;
  }).join('') : emptyRow(5, '暂无邀请码');
  qsa<HTMLElement>('[data-copy-invite]').forEach((button) => on(button, 'click', () => copyText(String(button.dataset.copyInvite), '邀请码已复制')));
  qsa<HTMLElement>('[data-delete-invite]').forEach((button) => on(button, 'click', () => void deleteInvite(String(button.dataset.deleteInvite))));
}

async function createInvites(): Promise<void> {
  const count = Number(qs<HTMLSelectElement>('#inviteCount').value || 5);
  const max_uses = Number(qs<HTMLSelectElement>('#inviteMaxUses').value || 1);
  const response = await api.post<{ invites?: JsonRecord[]; count?: number }>('/admin/api/invite/create', { count, max_uses });
  const codes = (response.invites ?? []).map((invite) => invite.code).join('\n');
  if (codes) copyText(codes, `已生成 ${response.count ?? 0} 个邀请码并复制`);
  await loadInvites();
}

async function deleteInvite(code: string): Promise<void> {
  if (!confirm(`确认删除 ${code}？`)) return;
  await api.post('/admin/api/invite/delete', { code });
  toast.show('邀请码已删除');
  await loadInvites();
}

async function loadHealth(): Promise<void> {
  const response = await api.get<JsonRecord>('/api/health');
  qs<HTMLElement>('#healthInfo').innerHTML = `<pre>${escapeHtml(JSON.stringify(response, null, 2))}</pre>`;
  const overview = await api.get<{ modules?: JsonRecord[] }>('/admin/api/modules/overview').catch(() => ({ modules: [] }));
  const modules = overview.modules ?? [];
  qs<HTMLElement>('#resourceInfo').innerHTML = modules.length
    ? `<div class="list">${modules.map(renderModuleOverview).join('')}</div>`
    : emptyBlock('暂无模块状态');
}

function renderModuleOverview(item: JsonRecord): string {
  const ready = item.status === 'ready';
  const updatedAt = item.updated_at ? ` · ${formatDate(String(item.updated_at))}` : '';
  return `<div class="list-item">
    <strong>${escapeHtml(item.name ?? item.key ?? '-')}</strong>
    <span><span class="badge badge-${ready ? 'active' : 'disabled'}">${ready ? '就绪' : '待处理'}</span>${updatedAt}</span>
    <em>${escapeHtml(item.summary ?? '')}</em>
  </div>`;
}

async function updateServerStatus(): Promise<void> {
  try {
    await api.get('/api/health');
    qs<HTMLElement>('#serverDot').style.background = 'var(--success)';
    qs<HTMLElement>('#serverStatusText').textContent = '服务器运行中';
  } catch {
    qs<HTMLElement>('#serverDot').style.background = 'var(--danger)';
    qs<HTMLElement>('#serverStatusText').textContent = '服务器离线';
  }
}

function stat(label: string, value: unknown, tone = 'accent'): string {
  return `<div class="stat-card"><div class="label">${label}</div><div class="value ${tone}">${escapeHtml(value)}</div></div>`;
}

function table(headers: string[], rows: string[][], emptyText: string): string {
  return `<div class="table-wrap"><table><thead><tr>${headers.map((header) => `<th>${escapeHtml(header)}</th>`).join('')}</tr></thead><tbody>${rows.length ? rows.map((row) => `<tr>${row.map((cell) => `<td>${cell}</td>`).join('')}</tr>`).join('') : emptyRow(headers.length, emptyText)}</tbody></table></div>`;
}

function emptyRow(colspan: number, text: string): string {
  return `<tr><td colspan="${colspan}"><div class="empty-state">${text}</div></td></tr>`;
}

function emptyBlock(text: string): string {
  return `<div class="empty-state">${text}</div>`;
}

function messageOf(error: unknown): string {
  return error instanceof ApiError || error instanceof Error ? error.message : '操作失败';
}

function showModal(id: string): void {
  qs<HTMLElement>(`#${id}`).classList.add('show');
}

function closeModal(id: string): void {
  qs<HTMLElement>(`#${id}`).classList.remove('show');
}

function closeSidebar(): void {
  qs<HTMLElement>('.sidebar').classList.remove('open');
  qs<HTMLElement>('#sidebarOverlay').classList.remove('show');
}

function activatePage(name: string): void {
  const button = document.querySelector<HTMLElement>(`[data-page-target="${name}"]`);
  button?.click();
}

function downloadBlob(blob: Blob, filename: string): void {
  const link = document.createElement('a');
  link.href = URL.createObjectURL(blob);
  link.download = filename;
  link.click();
  URL.revokeObjectURL(link.href);
}

function copyText(text: string, success: string): void {
  void navigator.clipboard?.writeText(text).then(() => toast.show(success)).catch(() => toast.show(text));
}

function typeLabel(type: string): string {
  return type === 'machine' ? '机器' : type === 'ip' ? 'IP' : '卡密';
}

function actionLabel(action: string): string {
  const labels: Record<string, string> = {
    card_generated: '生成卡密',
    card_activated: '激活卡密',
    card_status_changed: '状态变更',
    card_extended: '延期',
    card_unbound: '解绑',
    cards_import_completed: '导入卡密',
    session_deactivated: '注销会话',
    blacklist_added: '添加黑名单',
    blacklist_removed: '移除黑名单',
    agent_created: '代理注册',
    agent_deleted: '代理删除',
    agent_status_changed: '代理状态变更',
    agent_password_changed: '代理改密',
    agent_password_updated: '代理改密',
    invite_created: '生成邀请码',
    invite_deleted: '删除邀请码',
    announcement_saved: '保存公告',
    announcement_published: '发布公告',
    announcement_deleted: '删除公告',
    release_created: '创建发布',
    release_published: '发布客户端',
    release_paused: '暂停发布',
    release_rolled_back: '回滚发布',
    release_package_uploaded: '上传安装包',
    release_status_changed: '发布状态变更',
    script_saved: '保存脚本',
    script_published: '发布脚本',
    script_activated: '启用脚本',
    script_deleted: '删除脚本',
    payload_uploaded: '上传载荷',
    payload_activated: '启用载荷',
    payload_deleted: '删除载荷'
  };
  return labels[action] ?? action;
}

function formatBytes(bytes: number): string {
  if (!bytes) return '0 B';
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function renderExpiringCards(items: JsonRecord[] = []): string {
  if (!items.length) return emptyBlock('7 天内暂无即将过期卡密');
  return items.map((card) => {
    const code = String(card.code ?? '');
    return `<button class="list-item warning-item" data-open-card="${escapeHtml(code)}"><strong class="mono">${escapeHtml(code)}</strong><span>${formatDate(String(card.expires_at ?? ''))}</span><em>${escapeHtml(card.note ?? '即将过期')}</em></button>`;
  }).join('');
}

function renderRiskyMachines(items: JsonRecord[] = []): string {
  if (!items.length) return emptyBlock('暂无高风险机器');
  return items.map((machine) => {
    const id = String(machine.machine_id ?? '');
    const blacklisted = Boolean(machine.is_blacklisted);
    return `<button class="list-item danger-item" data-risk-machine="${escapeHtml(id)}"><strong class="mono">${escapeHtml(id).slice(0, 20)}...</strong><span>${machine.card_count ?? 0} 张卡密</span><em>${blacklisted ? '已封禁' : '多卡绑定'}</em></button>`;
  }).join('');
}

function renderAgentLeaderboard(items: JsonRecord[] = []): string {
  if (!items.length) return emptyBlock('暂无代理出卡数据');
  return items.map((agent, index) => `<div class="list-item"><strong>${index + 1}. ${escapeHtml(agent.username ?? agent.agent_id ?? '-')}</strong><span>总 ${agent.total_cards ?? 0}</span><em>有效 ${agent.active_cards ?? 0}</em></div>`).join('');
}

function renderRecentAudit(items: JsonRecord[] = []): string {
  if (!items.length) return emptyBlock('暂无近期操作');
  return items.map((entry) => `<div class="audit-entry"><span class="audit-time">${formatDate(String(entry.time ?? ''))}</span><span class="audit-action">${escapeHtml(actionLabel(String(entry.action ?? '')))}</span><span class="audit-detail">${escapeHtml(entry.detail ?? '')} ${entry.card ? `卡密:${escapeHtml(entry.card)}` : ''} ${entry.machine ? `机器:${escapeHtml(String(entry.machine)).slice(0, 14)}...` : ''}</span></div>`).join('');
}

function bindDashboardActions(): void {
  qsa<HTMLElement>('[data-open-card]').forEach((button) => {
    on(button, 'click', () => {
      activatePage('cards');
      qs<HTMLInputElement>('#cardSearch').value = String(button.dataset.openCard ?? '');
      void loadCards();
    });
  });
  qsa<HTMLElement>('[data-risk-machine]').forEach((button) => {
    on(button, 'click', () => {
      cardMachineFilter = String(button.dataset.riskMachine ?? '');
      activatePage('cards');
      void loadCards();
    });
  });
}

function setParam(params: URLSearchParams, key: string, value: string): void {
  if (value) params.set(key, value);
}

function dateToRFC3339(value: string, endOfDay = false): string {
  if (!value) return '';
  return `${value}T${endOfDay ? '23:59:59' : '00:00:00'}+08:00`;
}
