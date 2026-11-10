'use strict';
const app = document.querySelector('#app');
let user;
app.className = ['/login', '/register', '/profile'].includes(location.pathname) ? 'auth-page' : 'materials-page';

function storedToken() { try { return sessionStorage.getItem('campusclaw_token'); } catch { return null; } }
function clearToken() { try { sessionStorage.removeItem('campusclaw_token'); } catch {} }
function authHeaders(headers = {}) { const token = storedToken(); return token ? { ...headers, Authorization: `Bearer ${token}` } : headers; }
async function api(path, options = {}) {
  let response;
  try { response = await fetch(path, { credentials: 'same-origin', ...options, headers: authHeaders(options.headers) }); }
  catch { throw new Error('网络连接中断，无法确认操作结果。请恢复连接后查看列表或登录状态。'); }
  if (options.signal?.aborted) throw new Error("请求已取消");
  let body;
  try { body = await response.json(); }
  catch { throw new Error(`服务暂不可用（HTTP ${response.status}），请稍后重试。`); }
  if (options.signal?.aborted) throw new Error('请求已取消');
  if (response.status === 401 && path !== '/api/login') {
    clearToken(); location.replace('/login'); throw new Error('请先登录');
  }
  if (response.status === 403 && body?.code === 'password_change_required') {
    location.replace('/profile'); throw new Error(body.error);
  }
  if (!response.ok) {
    const error = new Error(body?.error || '请求失败');
    error.code = body?.code; error.status = response.status;
    throw error;
  }
  return body;
}
function el(tag, text, className) {
  const node = document.createElement(tag);
  if (text !== undefined) node.textContent = text;
  if (className) node.className = className;
  return node;
}
let toastTimer;
function showToast(message) {
  let toast = document.querySelector('#global-toast');
  if (!toast) {
    toast = el('div', undefined, 'global-toast'); toast.id = 'global-toast';
    toast.setAttribute('role', 'status'); toast.setAttribute('aria-live', 'polite');
    document.body.append(toast);
  }
  clearTimeout(toastTimer);
  toast.replaceChildren(el('span', '✓', 'toast-check'), el('span', message));
  toast.classList.remove('toast-visible');
  void toast.offsetWidth;
  toast.classList.add('toast-visible');
  toastTimer = setTimeout(() => toast.classList.remove('toast-visible'), 2000);
}
function setStatus(node, message, tone = 'error') {
  if (!node) return;
  node.textContent = message; node.dataset.tone = tone; node.hidden = !message;
  node.classList.toggle('feedback-success', Boolean(message) && tone === 'success');
  if (message && tone === 'success') showToast(message);
}
function uiDialog(title, description, confirmLabel, work, inputLabel) {
  const dialog = el('dialog', undefined, 'material-dialog management-dialog');
  const heading = el('h2', title); heading.id = 'management-dialog-title';
  dialog.setAttribute('aria-labelledby', heading.id);
  const form = el('form');
  form.append(heading, el('p', description, 'muted'));
  let input;
  if (inputLabel) {
    const label = el('label', inputLabel); input = el('input'); input.required = true; input.maxLength = 100;
    label.append(input); form.append(label);
  }
  const error = el('p', '', 'feedback'); error.setAttribute('role', 'alert'); error.hidden = true;
  const actions = el('div', undefined, 'actions');
  const cancel = el('button', '取消', 'secondary'); cancel.type = 'button';
  const confirm = el('button', confirmLabel, confirmLabel.includes('删除') ? 'danger-button' : 'primary');
  actions.append(cancel, confirm); form.append(error, actions); dialog.append(form);
  const focusBack = document.activeElement;
  let resolve;
  const result = new Promise(done => { resolve = done; });
  cancel.onclick = () => dialog.close();
  dialog.addEventListener('cancel', event => { if (confirm.disabled) event.preventDefault(); });
  dialog.addEventListener('close', () => { dialog.remove(); if (focusBack?.isConnected) focusBack.focus(); resolve(false); }, { once: true });
  form.onsubmit = async event => {
    event.preventDefault(); if (confirm.disabled) return;
    setBusy(confirm, true, '正在处理…'); cancel.disabled = true; setStatus(error, '');
    try { await work(input?.value.trim()); resolve(true); dialog.close(); }
    catch (err) { setStatus(error, err.message); }
    finally { setBusy(confirm, false); cancel.disabled = false; }
  };
  document.body.append(dialog); dialog.showModal();
  (input || cancel).focus();
  return result;
}
function confirmDiscard() {
  return uiDialog('放弃未保存的修改？', '当前修改尚未保存，离开后将丢失。', '放弃修改', async () => {});
}

function setBusy(button, busy, label) {
  if (!button.dataset.idleLabel) button.dataset.idleLabel = button.textContent;
  button.disabled = busy; button.setAttribute('aria-busy', String(busy));
  button.textContent = busy ? label : button.dataset.idleLabel;
}
function clearFieldErrors(form) {
  form.querySelectorAll('[aria-invalid]').forEach(field => field.removeAttribute('aria-invalid'));
}
function formatSize(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
function formatTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '时间未知';
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(date);
}
function nicknameKey() {
  return 'campusclaw.nickname:' + (user.id ?? user.username);
}
function legacyNickname() {
  try { return localStorage.getItem(nicknameKey()) || ''; }
  catch { return ''; }
}
function mountNicknameEditor() {
  const nickname = el('p', '', 'profile-identity');
  const trigger = el('button', '修改昵称', 'profile-outline');
  trigger.type = 'button';
  const dialog = el('dialog', undefined, 'material-dialog password-dialog');
  dialog.setAttribute('aria-labelledby', 'change-nickname-title');
  dialog.innerHTML = '<div class="dialog-heading"><h2 id="change-nickname-title">修改昵称</h2><button type="button" class="secondary">关闭</button></div><form><label>昵称<input name="nickname" autocomplete="nickname" autofocus aria-describedby="nickname-help"></label><p class="form-help" id="nickname-help">最多 24 个字符。留空后保存可清除昵称，保存在当前部署的账号中。</p><p class="feedback" role="alert" hidden></p><button class="primary full-width">保存昵称</button></form>';
  const form = dialog.querySelector('form');
  const field = form.elements.nickname;
  const error = form.querySelector('.feedback');
  const update = () => { nickname.textContent = '昵称：' + (user.nickname || '未设置'); };
  update();
  document.querySelector('#profile-identity').after(nickname);
  document.querySelector('#open-password').before(trigger);
  document.querySelector('.profile-actions').after(dialog);
  trigger.onclick = () => {
    field.value = user.nickname ?? legacyNickname();
    setStatus(error, '');
    dialog.showModal();
  };

  dialog.addEventListener('close', () => trigger.focus());
  const submit = form.querySelector('button');
  const close = dialog.querySelector('.dialog-heading button');
  close.onclick = () => { if (!submit.disabled) dialog.close(); };
  dialog.addEventListener('cancel', event => { if (submit.disabled) event.preventDefault(); });
  form.onsubmit = async event => {
    event.preventDefault();
    if (submit.disabled) return;
    const value = field.value.trim();
    if ([...value].length > 24) {
      setStatus(error, '昵称最多 24 个字符。');
      return;
    }
    setStatus(error, '');
    setBusy(submit, true, '正在保存…');
    try {
      const saved = await api('/api/me', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': user.csrf_token },
        body: JSON.stringify({ nickname: value })
      });
      user.nickname = saved.nickname;
      try { localStorage.removeItem(nicknameKey()); } catch {}
      update();
      renderIdentity('#profile-status');
      setStatus(document.querySelector('#profile-status'), saved.nickname ? '昵称已保存。' : '已清除昵称。', 'success');
      dialog.close();
    } catch (failure) {
      setStatus(error, failure.message);
    } finally {
      setBusy(submit, false);
    }
  };
}

function renderIdentity(errorTarget) {
  const identity = el('div', undefined, 'identity');
  const group = el('span', undefined, 'identity-group');
  const role = el('span', user.role === 'teacher' ? '教师' : '学生', 'role-label');
  const profile = el('a', '个人信息', 'nav-link'); profile.href = '/profile';
  const logout = el('button', '退出', 'text-button nav-logout');
  logout.onclick = async () => {
    setBusy(logout, true, '退出中…');
    try { await api('/api/logout', { method: 'POST', headers: { 'X-CSRF-Token': user.csrf_token } }); clearToken(); location.replace('/login'); }
    catch (error) { setStatus(document.querySelector(errorTarget), error.message); }
    finally { setBusy(logout, false); }
  };
  group.append(role, el('span', user.class_name, 'identity-class'));
  const nickname = user.nickname;
  if (nickname) {
    const name = el('span', nickname, 'identity-name');
    name.title = nickname;
    group.prepend(name);
  }
  identity.append(group, profile, logout);
  document.querySelector('#identity').replaceChildren(identity);
}
function loginPage() {
  app.replaceChildren(document.querySelector('#login-template').content.cloneNode(true));
  const form = document.querySelector('#login-form'); const status = document.querySelector('#login-error');
  form.addEventListener('input', () => { clearFieldErrors(form); setStatus(status, ''); });
  api('/api/account-options').then(options => {
    const note = document.querySelector('#account-note');
    if (options.demo_registration) {
      const link = el('a', '注册账号'); link.href = '/register';
      note.replaceChildren(link, el('span', '本地演示初始密码为 123456，首次登录须修改。', 'account-hint'));
    } else note.textContent = '当前环境不开放注册，请联系管理员。';
  }).catch(() => { document.querySelector('#account-note').textContent = '注册信息暂不可用'; });
  form.addEventListener('submit', async event => {
    event.preventDefault(); const button = form.querySelector('button');
    clearFieldErrors(form); setStatus(status, ''); setBusy(button, true, '正在登录…');
    try {
      const identity = await api('/api/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username: form.elements.username.value, password: form.elements.password.value }) });
      if (identity.access_token) sessionStorage.setItem('campusclaw_token', identity.access_token);
      form.elements.password.value = '';
      location.assign(identity.must_change_password ? '/profile' : '/workspace');
    } catch (error) {
      setStatus(status, error.message);
      if (error.message === '账号或密码错误') {
        form.elements.username.setAttribute('aria-invalid', 'true'); form.elements.password.setAttribute('aria-invalid', 'true');
      }
    } finally { setBusy(button, false); }
  });
}
async function showText(id, chunkID) {
  if (!/^\d+$/.test(String(id)) || (chunkID && !/^\d+$/.test(String(chunkID)))) {
    setStatus(document.querySelector('#status'), '资料来源地址无效。');
    return;
  }
  try {
    const material = await api(`/api/materials/${id}`);
    const dialog = el('dialog', undefined, 'material-dialog'); dialog.setAttribute('aria-labelledby', 'dialog-title');
    const heading = el('div', undefined, 'dialog-heading');
    const title = el('h2', material.title); title.id = 'dialog-title';
    const close = el('button', '关闭', 'secondary'); close.type = 'button'; close.addEventListener('click', () => dialog.close());
    heading.append(title, close);
    dialog.append(heading, el('p', `${material.original_filename} · ${formatSize(material.byte_size)}`, 'dialog-meta'));
    const indexNote = el('p', `索引：${material.index_status || '未知'} · ${material.chunk_count || 0} 个切片${material.index_error ? ' · ' + material.index_error : ''}`);
    dialog.append(indexNote);
    const reload = el('button', '刷新索引状态', 'secondary');
    reload.onclick = async () => { try { const m = await api(`/api/materials/${id}`); indexNote.textContent = `索引：${m.index_status} · ${m.chunk_count} 个切片${m.index_error ? ' · ' + m.index_error : ''}`; } catch (e) { indexNote.textContent = e.message; } };
    dialog.append(reload);
    if (user.role === 'teacher') {
      const rebuild = el('button', '重建索引', 'secondary');
      rebuild.onclick = async () => { setBusy(rebuild, true, '登记中…'); try { await api(`/api/materials/${id}/reindex`, { method: 'POST', headers: { 'X-CSRF-Token': user.csrf_token } }); indexNote.textContent = '索引：pending，已登记。请稍后刷新状态；原文仍可使用。'; } catch (e) { indexNote.textContent = e.message; } finally { setBusy(rebuild, false); } };
      dialog.append(rebuild);
    }
    if (chunkID) { const chunk = await api(`/api/materials/${id}/chunks/${chunkID}`); dialog.append(el('h3', `引用切片 ${chunk.chunk_index} · 字符 [${chunk.char_start}, ${chunk.char_end})`), el('pre', chunk.chunk_text)); }
    dialog.append(el('h3', '材料全文'), el('pre', material.body_text));
    dialog.addEventListener('close', () => dialog.remove(), { once: true });
    document.body.append(dialog); dialog.showModal();
  } catch (error) {
    const status = document.querySelector('#status');
    if (status) setStatus(status, error.message);
    // Some chat views have a different status region. Always expose a stale
    // source failure where the user clicked instead of silently losing it.
    else {
      const dialog = el('dialog', undefined, 'material-dialog');
      const close = el('button', '关闭', 'secondary'); close.onclick = () => dialog.close();
      dialog.append(el('h2', error.code === 'material_deleted' ? '材料已删除' : '来源当前不可用'), el('p', error.message), close);
      dialog.addEventListener('close', () => dialog.remove(), { once: true });
      document.body.append(dialog); dialog.showModal();
    }
  }
}
function confirmMaterialDeletion(material, trigger, row) {
  const dialog = el('dialog', undefined, 'material-dialog delete-material-dialog');
  dialog.setAttribute('aria-labelledby', 'delete-material-title');
  const title = el('h2', '确认删除教学材料？'); title.id = 'delete-material-title';
  const detail = el('p', `确定删除「${material.title}」吗？`);
  const warning = el('p', '删除后，该材料将不再参与知识库检索，也无法继续作为 AI 应用的有效绑定资料。历史回答保留，引用来源会提示已删除。此操作不可撤销。', 'muted');
  const status = el('p', '', 'status'); status.hidden = true; status.setAttribute('role', 'status');
  const actions = el('div', undefined, 'actions');
  const cancel = el('button', '取消', 'secondary'); cancel.type = 'button'; cancel.autofocus = true;
  const confirm = el('button', '确认删除', 'danger-button'); confirm.type = 'button';
  let busy = false;
  cancel.onclick = () => dialog.close();
  dialog.addEventListener('cancel', event => { if (busy) event.preventDefault(); });
  dialog.addEventListener('close', () => { dialog.remove(); if (trigger.isConnected) trigger.focus(); else document.querySelector('#refresh-materials')?.focus(); }, { once: true });
  confirm.onclick = async () => {
    if (busy) return;
    busy = true; cancel.disabled = true; setBusy(confirm, true, '正在删除…');
    try {
      const result = await api(`/api/materials/${material.id}`, { method: 'DELETE', headers: { 'X-CSRF-Token': user.csrf_token } });
      row.remove();
      const count = document.querySelector('#count');
      count.textContent = `${document.querySelectorAll('#material-list .material').length} 份材料`;
      dialog.close();
      const message = result.status === 'deleted' ? '材料已删除。' : '材料已停止使用，关联资源正在清理。';
      const pageStatus = document.querySelector('#status');
      if (pageStatus) setStatus(pageStatus, message, 'success');
      try { await refresh(); }
      catch { if (pageStatus) setStatus(pageStatus, message + ' 列表刷新失败，请点击刷新列表。', 'info'); }
    } catch (error) { setStatus(status, error.message); }
    finally { busy = false; cancel.disabled = false; setBusy(confirm, false); }
  };
  actions.append(cancel, confirm); dialog.append(title, detail, warning, status, actions);
  document.body.append(dialog); dialog.showModal(); cancel.focus();
}
async function refresh() {
  const list = document.querySelector('#material-list'); list.setAttribute('aria-busy', 'true');
  try {
    const materials = await api('/api/materials'); list.replaceChildren();
    document.querySelector('#count').textContent = `${materials.length} 份材料`;
    if (!materials.length) {
      const empty = el('div', undefined, 'empty');
      empty.append(el('p', '本班暂时没有材料', 'empty-title'), el('p', user.role === 'teacher' ? '上传一份 TXT 或 Markdown 文件，与本班共享。' : '教师上传材料后，可在这里查看与下载。', 'muted'));
      list.append(empty);
    }
    for (const m of materials) {
      const row = el('article', undefined, 'material');
      const extension = m.original_filename.toLowerCase().endsWith('.md') ? 'MD' : 'TXT';
      const icon = el('span', extension, 'file-icon'); icon.setAttribute('aria-hidden', 'true');
      const info = el('div', undefined, 'material-info'); const title = el('h3', m.title); title.title = m.title;
      const metadata = [m.original_filename === m.title ? '' : m.original_filename, formatSize(m.byte_size), formatTime(m.created_at)].filter(Boolean).join(' · ');
      info.append(title, el('p', metadata, 'material-meta'));
      const actions = el('div', undefined, 'actions');
      const view = el('button', undefined, 'text-button material-action'); view.append(lineIcon('eye'), el('span', '查看')); view.onclick = () => showText(m.id);
      const download = el('a', undefined, 'material-action'); download.append(lineIcon('download'), el('span', '下载')); download.href = `/api/materials/${m.id}/file`;
      download.onclick = async event => { event.preventDefault(); try { const response = await fetch(download.href, { credentials: 'same-origin', headers: authHeaders() }); if (response.status === 401) { clearToken(); location.replace('/login'); return; } if (!response.ok) throw new Error('下载失败，请检查登录状态及权限'); const blob = await response.blob(); const url = URL.createObjectURL(blob); const link = el('a'); link.href = url; link.download = m.original_filename; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); } catch (e) { setStatus(document.querySelector('#status'), e.message); } };
      actions.append(view, download);
      if (user.role === 'teacher') {
        const remove = el('button', undefined, 'text-button material-action material-delete'); remove.append(lineIcon('trash'), el('span', '删除')); remove.type = 'button';
        remove.setAttribute('aria-label', `删除 ${m.title}`);
        remove.onclick = () => confirmMaterialDeletion(m, remove, row);
        actions.append(remove);
      }
      row.append(icon, info, actions); list.append(row);
    }
  } finally { list.setAttribute('aria-busy', 'false'); }
}
const iconPaths = {
  eye: 'M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12zm13 0a3 3 0 1 1-6 0 3 3 0 0 1 6 0z',
  download: 'M12 3v12m-4-4 4 4 4-4M4 15v4a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-4',
  trash: 'M4 7h16M9 7V4h6v3M6 7l1 14h10l1-14M10 10v7m4-7v7',
  home: 'M3 10l9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1z',
  bot: 'M12 3v3M8 11h.01M16 11h.01M9 15h6M6 6h12a3 3 0 0 1 3 3v8a3 3 0 0 1-3 3H6a3 3 0 0 1-3-3V9a3 3 0 0 1 3-3z',
  book: 'M12 6c-3-2-6-3-9-2v15c3-1 6 0 9 2 3-2 6-3 9-2V4c-3-1-6 0-9 2zm0 0v15',
  plug: 'M8 3v5m8-5v5M6 8h12v3a6 6 0 0 1-12 0zm6 9v4',
  clipboard: 'M9 5H5v16h14V5h-4M9 3h6v4H9zm0 9h6m-6 4h6',
  chart: 'M4 4v16h16M8 15v-3m4 3V8m4 7v-5',
  chat: 'M21 11.5a8.5 8.5 0 0 1-8.5 8.5 9 9 0 0 1-4-.9L3 21l1.8-5A8.5 8.5 0 1 1 21 11.5z',
  bookmark: 'M6 4h12v17l-6-4-6 4z',
  arrow: 'M5 12h14m-5-5 5 5-5 5',
  plus: 'M12 5v14M5 12h14',
  heart: 'M12 20 4.5 12.5A5 5 0 0 1 12 6a5 5 0 0 1 7.5 6.5z'
};
function lineIcon(key) {
  const namespace = 'http://www.w3.org/2000/svg';
  const icon = document.createElementNS(namespace, 'svg');
  icon.setAttribute('viewBox', '0 0 24 24');
  icon.setAttribute('class', 'line-icon');
  icon.setAttribute('aria-hidden', 'true');
  const path = document.createElementNS(namespace, 'path');
  path.setAttribute('d', iconPaths[key]);
  icon.append(path);
  return icon;
}
function stateTag(text, tone = 'neutral') {
  const tag = el('span', text, 'state-tag');
  tag.dataset.tone = tone;
  return tag;
}

const pages = {
  teacher: [
    ['/workspace', '工作台首页', 'home', '查看本班与常用任务。'],
    ['/applications', 'AI 应用管理', 'bot', '查看角色提示词、模板及能力绑定的配置入口。'],
    ['/knowledge', '知识库管理', 'book', '上传、检索和核对本班资料。'],
    ['/tools', 'MCP 工具管理', 'plug', '查看服务器登记、连接和工具发现入口。'],
    ['/assignments/manage', '作业管理', 'clipboard', '查看发布、提交与批改的规划。'],
    ['/audit', '审计与统计', 'chart', '查看操作记录与调用证据的规划。']
  ],
  student: [
    ['/workspace', '学习首页', 'home', '从常用入口开始本班学习。'],
    ['/assistant', 'AI 助手', 'chat', '向本班资料助手提问，核对回答出处。'],
    ['/handouts', '讲义与检索', 'book', '浏览讲义，检索并定位原文。'],
    ['/assignments', '我的作业', 'clipboard', '查看提交与批改反馈的规划。'],
    ['/mistakes', '我的错题', 'bookmark', '查看错题回顾与讲解的规划。'],
    ['/learning', '学情分析', 'chart', '查看学习记录与分析的规划。']
  ]
};

function pageHeading(title, description) {
  const heading = el('header', undefined, 'heading work-heading');
  heading.append(el('h1', title), el('p', description, 'muted'));
  return heading;
}
function pageLink(path, label, className) {
  const link = el('a', label, className);
  link.href = path;
  return link;
}
function showWorkspace(content, activePath) {
  app.className = activePath === '/assistant' ? 'workspace-page assistant-page' : 'workspace-page';
  const shell = el('div', undefined, 'work-shell');
  const nav = el('nav', undefined, 'work-nav');
  nav.setAttribute('aria-label', user.role === 'teacher' ? '教师工作台导航' : '学生工作台导航');
  nav.append(el('p', user.role === 'teacher' ? '教师工作台' : '学生工作台', 'nav-caption'));
  for (const [path, label, group] of pages[user.role]) {
    const link = pageLink(path, undefined, 'work-link');
    const icon = el('span', undefined, 'nav-mark');
    icon.append(lineIcon(group));
    icon.setAttribute('aria-hidden', 'true');
    link.append(icon, el('span', label));
    if (path === activePath) link.setAttribute('aria-current', 'page');
    nav.append(link);
  }
  shell.append(nav, content);
  app.replaceChildren(shell);
  renderIdentity('#status');
}
function taskLink(path, title, icon, description) {
  const link = pageLink(path, undefined, 'task-link');
  const copy = el('span', undefined, 'task-copy');
  copy.append(el('strong', title), el('span', description, 'muted'));
  link.append(lineIcon(icon), copy, lineIcon('arrow'));
  return link;
}
function homePage(content, route) {
  const teacher = user.role === 'teacher';
  const hero = el('section', undefined, 'home-hero');
  const heading = el('div', undefined, 'home-heading');
  const name = user.nickname ? '，' + user.nickname : '';
  hero.append(el('p', route[1] + ' · ' + user.class_name, 'eyebrow'), heading);
  heading.append(
    el('h1', teacher ? '欢迎回来' + name : '今天想学什么？'),
    el('p', teacher ? '管理本班资料，查看应用与工具配置。' : '用讲义找到依据，向课程资料助手提出问题。', 'muted')
  );
  if (!teacher) {
    const actions = el('div', undefined, 'home-actions');
    actions.append(pageLink('/assistant', '向 AI 助手提问', 'button primary'), pageLink('/handouts', '浏览讲义', 'button secondary'));
    hero.append(actions);
  }
  content.append(hero);
  if (teacher) {
    const overview = el('section', undefined, 'home-overview');
    overview.setAttribute('aria-label', '本班概况');
    const count = stateTag('读取中…', 'info');
    const appsCount = stateTag('读取中…', 'info');
    const toolsCount = stateTag('读取中…', 'info');
    count.setAttribute('role', 'status');
    for (const [path, label, icon, badge] of [
      ['/knowledge', '本班教学资料', 'book', count],
      ['/applications', 'AI 应用', 'bot', appsCount],
      ['/tools', 'MCP 工具', 'plug', toolsCount]
    ]) {
      const card = pageLink(path, undefined, 'card overview-card');
      card.append(lineIcon(icon), el('strong', label), badge);
      overview.append(card);
    }
    content.append(overview);
    api('/api/ai/apps').then(data => {
      if (!Array.isArray(data.items)) throw new Error('Invalid app list');
      appsCount.textContent = data.items.filter(item => item.enabled).length + ' 个已启用';
      appsCount.dataset.tone = 'success';
    }).catch(() => { appsCount.textContent = '应用读取失败'; appsCount.dataset.tone = 'warning'; });
    api('/api/mcp/servers').then(data => {
      if (!Array.isArray(data.items)) throw new Error('Invalid tool list');
      const count = data.items.filter(item => item.status === 'connected').length;
      toolsCount.textContent = data.configured === false ? '服务待配置' : count ? count + ' 个服务已连接' : '暂无已连接服务';
      toolsCount.dataset.tone = count ? 'success' : 'neutral';
    }).catch(() => { toolsCount.textContent = '工具读取失败'; toolsCount.dataset.tone = 'warning'; });
    api('/api/materials').then(materials => {
      if (!Array.isArray(materials)) throw new Error('Invalid material list');
      count.textContent = materials.length + ' 份';
      count.dataset.tone = 'info';
    }).catch(() => {
      count.textContent = '资料读取失败';
      count.dataset.tone = 'warning';
    });
  }
  const section = el('section', undefined, 'card home-panel');
  section.append(el('h2', teacher ? '快速开始' : '学习入口'));
  const links = teacher ? [
    ['/knowledge', '管理本班知识库', 'book', '上传材料、检查索引并核对检索结果。'],
    ['/applications', '配置 AI 应用', 'bot', '管理应用、提示词和能力绑定。'],
    ['/tools', '管理 MCP 工具', 'plug', '查看服务连接与工具发现入口。']
  ] : [
    ['/handouts', '查找课堂资料', 'book', '按关键词或语义检索，并打开来源片段。'],
    ['/assignments', '我的作业', 'clipboard', '作业与批改结果入口 · 暂未开放'],
    ['/mistakes', '我的错题', 'bookmark', '错题回顾入口 · 暂未开放']
  ];
  for (const entry of links) section.append(taskLink(...entry));
  content.append(section);
}
async function workspacePage() {
  user = await api('/api/me');
  if (user.must_change_password) { location.replace('/profile'); return; }
  const materialPath = user.role === 'teacher' ? '/knowledge' : '/handouts';
  const path = location.pathname === '/' ? '/workspace' : location.pathname === '/materials' ? materialPath : location.pathname;
  const route = pages[user.role].find(entry => entry[0] === path);
  const content = el('section', undefined, 'work-content');
  const status = el('p', '', 'feedback');
  status.id = 'status'; status.hidden = true; status.setAttribute('role', 'status');
  content.append(status);
  showWorkspace(content, path);
  if (!route) {
    content.append(pageHeading('页面不可访问', '当前账号无法访问此页面。'), pageLink('/workspace', '返回工作台', 'button secondary'));
  } else if (path === materialPath) {
    await materialsPage(content, route);
  } else if (path === '/workspace') {
    homePage(content, route);
  } else {
    renderModule(content, route);
  }
}

async function materialsPage(content, route) {
  content.append(pageHeading(route[1], user.role === 'teacher' ? '上传、管理和检索本班教学资料。' : '查看和下载本班讲义，按原文核对检索结果。'));
  const workspace = el('div', undefined, 'workspace');
  workspace.id = 'workspace';
  workspace.innerHTML = '<section class="card material-panel" aria-labelledby="materials-title"><div class="list-header"><div class="list-title"><h2 id="materials-title">教学材料</h2><span class="count" id="count"></span></div><button class="text-button" id="refresh-materials">刷新列表</button></div><div id="material-list"><p class="list-loading">正在加载材料…</p></div></section>';
  content.append(workspace);
  document.querySelector('#refresh-materials').onclick = async event => {
    const button = event.currentTarget; setBusy(button, true, '刷新中…');
    try { await refresh(); setStatus(document.querySelector('#status'), '材料列表已更新。', 'success'); }
    catch (error) { setStatus(document.querySelector('#status'), error.message); }
    finally { setBusy(button, false); }
  };
  if (user.role === 'teacher') {
    const upload = el('aside', undefined, 'card upload-card'); upload.setAttribute('aria-labelledby', 'upload-title');
    upload.innerHTML = '<div class="card-heading"><h2 id="upload-title">上传教学材料</h2><p class="muted">将讲义与学习资料分享给本班。</p></div><form id="upload-form"><label>材料标题 <span class="optional">可选</span><input name="title" maxlength="255" placeholder="默认使用文件名"></label><div class="file-field"><span class="field-label">教学文件</span><label class="file-picker" id="file-picker"><span class="picker-icon" aria-hidden="true">↑</span><span class="picker-title">选择文件</span><span class="picker-name" id="file-name">尚未选择文件</span><span class="picker-meta" id="file-meta">UTF-8 · TXT / MD · 最大 5 MiB</span><input name="file" type="file" accept=".txt,.md" required aria-label="选择文件" aria-describedby="file-meta"></label></div><button class="primary full-width">上传并入库</button></form><p class="upload-note">原文件与正文一并保留，仅本班成员可查看。</p>';
    workspace.append(upload);
    const form = document.querySelector('#upload-form'); const fileInput = form.elements.file;
    const resetSelection = () => {
      document.querySelector('#file-name').textContent = '尚未选择文件'; document.querySelector('#file-meta').textContent = 'UTF-8 · TXT / MD · 最大 5 MiB'; document.querySelector('#file-picker').classList.remove('has-file');
    };
    fileInput.addEventListener('change', () => {
      const file = fileInput.files[0]; if (!file) { resetSelection(); return; }
      document.querySelector('#file-name').textContent = file.name;
      document.querySelector('#file-meta').textContent = `${formatSize(file.size)} · 可重新选择文件`;
      document.querySelector('#file-picker').classList.add('has-file');
    });
    form.onsubmit = async event => {
      event.preventDefault(); const button = form.querySelector('button'); setBusy(button, true, '正在上传…'); form.setAttribute('aria-busy', 'true');
      try {
        await api('/api/materials/upload', { method: 'POST', headers: { 'X-CSRF-Token': user.csrf_token }, body: new FormData(form) });
        form.reset(); resetSelection();
        try { await refresh(); setStatus(document.querySelector('#status'), '上传成功：原文件已保存，知识库正文已入库。', 'success'); }
        catch { setStatus(document.querySelector('#status'), '上传已成功，但列表刷新失败。请点击“刷新列表”查看，无需重复上传。', 'warning'); }
      } catch (error) { setStatus(document.querySelector('#status'), error.message); }
      finally { setBusy(button, false); form.setAttribute('aria-busy', 'false'); }
    };
  } else { workspace.classList.add('student'); }
  const tools = el('div');
  if (location.pathname === '/materials') renderRetrieval(tools, 'legacy');
  else {
    renderRetrieval(tools, 'search');
    if (user.role === 'teacher') {
      const ask = el('details', undefined, 'card teacher-ask');
      ask.append(el('summary', '打开单轮资料问答'));
      renderRetrieval(ask, 'chat');
      ask.querySelectorAll('[id]').forEach(node => { node.id = 'teacher-' + node.id; });
      tools.append(ask);
    }
  }
  workspace.before(tools);
  try { await refresh(); const params = new URLSearchParams(location.search); if (params.get('material_id')) await showText(params.get('material_id'), params.get('chunk_id')); }
  catch (error) { document.querySelector('#material-list').textContent = '材料暂时无法加载，请点击刷新列表重试。'; setStatus(document.querySelector('#status'), error.message); }
}
async function registerPage() {
  const options = await api('/api/account-options'); if (!options.demo_registration) { app.textContent = '当前环境不开放注册'; return; }
  app.innerHTML = '<section class="card auth-single" aria-labelledby="register-title"><div class="auth-heading"><p class="eyebrow">加入班级空间</p><h1 id="register-title">注册学生账号</h1><p class="muted">选择所属班级，创建你的学习账号。</p></div><form id="register-form"><label>用户名<input name="username" required minlength="3" maxlength="32" pattern="[A-Za-z0-9_]+" autocomplete="username" placeholder="3–32 位字母、数字或下划线"></label><label>所属班级<select name="class_code" required></select></label><p class="form-help">仅本地课堂演示。初始密码 123456，首次登录必须修改。</p><button class="primary full-width">注册账号</button></form><p class="feedback" id="register-status" role="status" hidden></p><a class="auth-back" href="/login">返回登录</a></section>';
  const form = document.querySelector('#register-form');
  for (const c of options.classes) { const option = el('option', c.name); option.value = c.code; form.elements.class_code.append(option); }
  form.onsubmit = async event => {
    event.preventDefault(); const button = form.querySelector('button'); setBusy(button, true, '正在注册…');
    try {
      await api('/api/register', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username: form.elements.username.value, class_code: form.elements.class_code.value }) });
      form.hidden = true; setStatus(document.querySelector('#register-status'), '注册成功！初始密码：123456。请返回登录，首次登录后修改密码。', 'success');
    } catch (error) { setStatus(document.querySelector('#register-status'), error.message); }
    finally { setBusy(button, false); }
  };
}
async function profilePage() {
  user = await api('/api/me');
  app.innerHTML = '<section class="card auth-single profile-card" aria-labelledby="profile-title"><div class="auth-heading"><h1 id="profile-title">个人信息</h1><p id="profile-identity" class="profile-identity"></p></div><p class="feedback" data-tone="info" id="password-notice"></p><div class="profile-actions"><button type="button" class="primary" id="open-password" hidden>修改密码</button></div><form id="password-form"><label id="current-password-label">当前密码<input name="current_password" type="password" autocomplete="current-password"></label><label>新密码<input name="new_password" type="password" minlength="8" maxlength="72" autocomplete="new-password" required></label><label>确认新密码<input name="confirm_password" type="password" autocomplete="new-password" required></label><p class="form-help">8–72 字节，包含字母和数字，不能使用初始密码或当前密码。</p><button class="primary full-width">修改密码并重新登录</button></form><p class="feedback" id="password-status" role="alert" hidden></p><p class="feedback" id="profile-status" role="alert" hidden></p><div class="auth-footer"><a id="profile-back" href="/workspace">返回工作台</a><button class="text-button" id="profile-logout">退出登录</button></div></section>';
  renderIdentity('#profile-status');
  document.querySelector('#profile-identity').textContent = `${user.role === 'teacher' ? '教师' : '学生'} · ${user.class_name}`;
  document.querySelector('#password-notice').textContent = user.must_change_password ? '请先修改初始密码，完成后才能访问教学材料。' : '角色和班级由管理员管理；修改密码将退出所有已登录设备。';
  const form = document.querySelector('#password-form'); form.elements.current_password.required = !user.must_change_password;
  document.querySelector('#current-password-label').hidden = user.must_change_password; document.querySelector('#profile-back').hidden = user.must_change_password;
  if (!user.must_change_password) {
    const trigger = document.querySelector('#open-password');
    const dialog = el('dialog', undefined, 'material-dialog password-dialog');
    const heading = el('div', undefined, 'dialog-heading');
    const title = el('h2', '修改密码'); title.id = 'change-password-title';
    const close = el('button', '关闭', 'secondary'); close.type = 'button';
    dialog.setAttribute('aria-labelledby', title.id);
    const submitting = () => form.querySelector('button').disabled;
    close.onclick = () => { if (!submitting()) dialog.close(); };
    heading.append(title, close);
    const status = document.querySelector('#password-status');
    dialog.append(heading, form, status);
    trigger.after(dialog);
    trigger.hidden = false;
    mountNicknameEditor();
    trigger.onclick = () => dialog.showModal();
    dialog.addEventListener('cancel', event => { if (submitting()) event.preventDefault(); });
    dialog.addEventListener('close', () => {
      form.reset();
      clearFieldErrors(form);
      setStatus(status, '');
      trigger.focus();
    });
  }
  form.addEventListener('input', () => clearFieldErrors(form));
  form.onsubmit = async event => {
    event.preventDefault(); const button = form.querySelector('button'); setBusy(button, true, '正在保存…'); clearFieldErrors(form); setStatus(document.querySelector('#password-status'), '');
    try {
      await api('/api/password', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': user.csrf_token }, body: JSON.stringify({ current_password: form.elements.current_password.value, new_password: form.elements.new_password.value, confirm_password: form.elements.confirm_password.value }) });
      clearToken(); form.reset(); location.replace('/login?password_changed=1');
    } catch (error) {
      setStatus(document.querySelector('#password-status'), error.message);
      const field = error.message === '当前密码错误' ? form.elements.current_password : form.elements.new_password;
      field.setAttribute('aria-invalid', 'true');
    } finally { setBusy(button, false); }
  };
  document.querySelector('#profile-logout').onclick = async event => {
    const button = event.currentTarget; setBusy(button, true, '退出中…');
    try { await api('/api/logout', { method: 'POST', headers: { 'X-CSRF-Token': user.csrf_token } }); clearToken(); location.replace('/login'); }
    catch (error) { setStatus(document.querySelector('#profile-status'), error.message); }
    finally { setBusy(button, false); }
  };
}
const plannedModules = {
  '/assignments/manage': ['发布作业', '学生提交', 'AI 批改', '人工改分'],
  '/audit': ['操作记录', 'Skill 执行', 'MCP 调用', '验收证据'],
  '/assignments': ['查看作业', '提交作业', '批改结果'],
  '/mistakes': ['错题记录', '错误原因', 'AI 讲解'],
  '/learning': ['学习记录', '成绩趋势', '知识掌握']
};
function pendingPanel(title, items, description) {
  const panel = el('section', undefined, 'card pending-panel');
  const heading = el('div', undefined, 'panel-title');
  heading.append(el('h2', title), el('span', '暂未开放', 'state-tag'));
  panel.append(heading, el('p', description, 'muted'));
  if (items.length) {
    const list = el('ul', undefined, 'feature-list');
    for (const item of items) list.append(el('li', item));
    panel.append(list);
  }
  return panel;
}
function applicationsPage(content) {
  const layout = el('div', undefined, 'app-management');
  const catalog = el('section', undefined, 'card app-catalog'); catalog.setAttribute('aria-label', '应用列表');
  const detail = el('section', undefined, 'card app-config');
  const header = el('div', undefined, 'config-header');
  const title = el('h2', '应用配置'), badge = stateTag('加载中', 'info');
  header.append(title, badge);
  const tabs = el('div', undefined, 'config-tabs'); tabs.setAttribute('aria-label', '配置分区');
  const controls = el('div', undefined, 'config-controls');
  const status = el('p', '', 'feedback'); status.setAttribute('role', 'status'); status.hidden = true;
  detail.append(header, tabs, controls, status); layout.append(catalog, detail); content.append(layout);
  const sections = ['基本配置', '提示词模板', 'Skill 技能', '知识库绑定', 'MCP 工具'];
  let apps = [], templates = [], skills = [], materials = [], selected = 0, section = 0, busy = false;
  let dirtyChecks = [], refreshChecks = [];
  const dirty = () => dirtyChecks.some(check => check());
  async function leave(next) { if (busy) return; if (dirty() && !await confirmDiscard()) return; next(); }
  const tabButtons = sections.map((name, index) => {
    const button = el('button', name, 'config-tab');
    button.onclick = () => { if (section !== index) leave(() => { section = index; render(); }); };
    tabs.append(button); return button;
  });
  const warnUnload = event => { if (layout.isConnected && dirty()) { event.preventDefault(); event.returnValue = ''; } };
  window.addEventListener('beforeunload', warnUnload);
  document.addEventListener('click', async event => {
    const link = event.target.closest?.('a[href]');
    if (!layout.isConnected || !link || link.target || event.defaultPrevented || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || !dirty()) return;
    event.preventDefault(); if (await confirmDiscard()) { dirtyChecks = []; location.assign(link.href); }
  });
  function field(parent, label, input) {
    const wrapper = el('label');
    if (input.type === 'checkbox') {
      wrapper.className = input.getAttribute('role') === 'switch' ? 'choice-row switch-row' : 'choice-row';
      wrapper.append(input, el('span', label));
    } else { wrapper.append(el('span', label), input); }
    parent.append(wrapper); return wrapper;
  }
  function block(name, description, tag) {
    const group = el('section', undefined, 'config-section');
    const heading = el('div', undefined, 'section-heading'); heading.append(el('h3', name));
    if (tag) heading.append(stateTag(tag));
    group.append(heading, el('p', description, 'muted section-help')); controls.append(group); return group;
  }
  function button(parent, text, action, primary = false) {
    const node = el('button', text, primary ? 'primary' : 'secondary'); node.type = 'button'; node.disabled = busy;
    node.onclick = action; parent.append(node); return node;
  }
  async function action(work, redraw = true, success = '操作已保存') {
    if (busy) return false;
    busy = true;
    const nodes = [...layout.querySelectorAll('button, input, select, textarea')];
    const previous = nodes.map(node => node.disabled);
    nodes.forEach(node => { node.disabled = true; });
    setStatus(status, '正在保存…', 'info');
    let saved = false;
    try { await work(); saved = true; setStatus(status, success, 'success'); }
    catch (error) { setStatus(status, error.message); }
    finally {
      busy = false; nodes.forEach((node, i) => { node.disabled = previous[i]; });
      if (saved && redraw) render(); else refreshChecks.forEach(check => check());
    }
    return saved;
  }
  function save(parent, text, read, baseline, work) {
    let snapshot = JSON.stringify(baseline);
    const changed = () => JSON.stringify(read()) !== snapshot;
    dirtyChecks.push(changed);
    const actions = el('div', undefined, 'config-actions');
    const hint = el('span', '未修改', 'save-state'); hint.setAttribute('role', 'status');
    const node = button(actions, text, async () => {
      if (!changed() || busy) return;
      const value = read();
      // Each panel saves independently so other unsaved forms are kept.
      if (await action(() => work(value), false)) snapshot = JSON.stringify(value);
      update();
    }, true);
    function update() { node.disabled = busy || !changed(); hint.textContent = busy ? '保存中…' : changed() ? '有未保存的修改' : '未修改'; }
    refreshChecks.push(update);
    parent.addEventListener('input', update); parent.addEventListener('change', update);
    actions.prepend(hint); parent.append(actions); update(); return update;
  }
  const appSave = (app, payload) => aiMutation('/api/ai/apps/' + app.id, 'PATCH', payload).then(value => { apps[selected] = value; Object.assign(app, value); updateCatalog(); });
  function updateCatalog() {
    catalog.replaceChildren();
    const head = el('div', undefined, 'section-heading'); head.append(el('h2', '应用列表'));
    button(head, '+ 新建', () => leave(async () => {
      await uiDialog('新建 AI 应用', '新应用默认停用。配置提示词与能力后，再允许学生新建会话。', '创建应用', async name => {
        if (!name || [...name].length > 100) throw new Error('请输入 1–100 个字符的应用名称');
        const value = await aiMutation('/api/ai/apps', 'POST', { name });
        apps.push(value); selected = apps.length - 1; section = 0; render(); showToast('应用已创建');
      }, '应用名称');
    })); catalog.append(head);
    apps.forEach((item, index) => {
      const choice = button(catalog, '', () => { if (selected !== index) leave(() => { selected = index; section = 0; render(); }); });
      choice.className = 'app-choice'; choice.setAttribute('aria-pressed', String(selected === index));
      const copy = el('span', undefined, 'app-choice-copy'); copy.append(el('strong', item.name), el('span', item.role_prompt || '尚未设置角色提示词', 'muted app-description'));
      choice.append(lineIcon('bot'), copy, stateTag(item.enabled ? '已启用' : '已停用', item.enabled ? 'success' : 'neutral'));
    });
  }
  function bindingList(parent, items, ids, describe, search = false) {
    const selectedIDs = new Set(ids), count = stateTag('');
    const tools = el('div', undefined, 'binding-toolbar'); tools.append(count);
    let query;
    if (search) {
      query = el('input'); query.type = 'search'; query.placeholder = '搜索资料名称'; query.setAttribute('aria-label', '搜索资料名称'); tools.prepend(query);
    }
    const all = button(tools, '全选', () => {
      const visible = items.filter(matches), remove = visible.length && visible.every(item => selectedIDs.has(item.id));
      visible.forEach(item => remove ? selectedIDs.delete(item.id) : selectedIDs.add(item.id));
      draw(); parent.dispatchEvent(new Event('change', { bubbles: true }));
    });
    const list = el('div', undefined, 'binding-list'); parent.append(tools, list);
    function matches(item) { return !query || item.title.toLowerCase().includes(query.value.trim().toLowerCase()); }
    function draw() {
      count.textContent = selectedIDs.size + ' 项已选择'; list.replaceChildren();
      const visible = items.filter(matches); all.disabled = !visible.length || busy;
      all.textContent = search && query.value ? '全选筛选结果' : '全选';
      if (visible.length && visible.every(item => selectedIDs.has(item.id))) all.textContent = search && query.value ? '取消选择筛选结果' : '取消全选';
      if (!visible.length) list.append(el('p', items.length ? '没有匹配的资料' : '暂无可绑定项', 'muted'));
      for (const item of visible) {
        const check = el('input'); check.type = 'checkbox'; check.checked = selectedIDs.has(item.id);
        const row = field(list, item.title || item.name, check); row.classList.add('binding-row');
        if (describe) row.append(stateTag(describe(item, check.checked), item.enabled && check.checked ? 'success' : 'neutral'));
        check.onchange = () => { check.checked ? selectedIDs.add(item.id) : selectedIDs.delete(item.id); draw(); };
      }
    }
    if (query) query.oninput = draw;
    draw(); const read = () => [...selectedIDs].sort((a, b) => a - b); read.refresh = draw; return read;
  }
  function skillFile(parent, item) {
    const file = el('input'); file.type = 'file'; file.accept = '.txt,.md';
    const control = el('div', undefined, 'skill-file-control');
    const picker = el('label', undefined, 'skill-file-picker');
    const icon = el('span', '↑', 'skill-file-icon'); icon.setAttribute('aria-hidden', 'true');
    const copy = el('span', undefined, 'skill-file-copy');
    const title = el('span', item ? '选择替换文件' : '选择 Skill 文件', 'skill-file-title');
    const name = el('span', '尚未选择文件', 'skill-file-name'); name.setAttribute('aria-live', 'polite');
    const meta = el('span', 'UTF-8 · TXT / MD · 最大 128 KB', 'skill-file-meta');
    copy.append(title, name, meta);
    file.setAttribute('aria-label', item ? '选择替换 Skill 文件' : '选择新 Skill 文件');
    picker.append(icon, copy, file); control.append(picker); parent.append(control);
    const actions = el('div', undefined, 'skill-file-actions'); control.append(actions);
    const submit = button(actions, item ? '上传替换' : '上传 Skill', async () => {
      if (!file.files.length) { setStatus(status, '请选择 TXT 或 Markdown 文件'); return; }
      if (dirty() && !await confirmDiscard()) return;
      const data = new FormData(); data.append('file', file.files[0]);
      action(async () => {
        const saved = await aiMutation('/api/ai/skills' + (item ? '/' + item.id : ''), item ? 'PATCH' : 'POST', data);
        if (item) skills[skills.indexOf(item)] = saved; else skills.push(saved);
      }, true, item ? 'Skill 文件已替换' : 'Skill 已上传，请选择绑定');
    });
    submit.disabled = true;
    file.onchange = () => {
      const selectedFile = file.files[0];
      submit.disabled = !selectedFile;
      picker.classList.toggle('has-file', Boolean(selectedFile));
      title.textContent = selectedFile ? '更换所选文件' : item ? '选择替换文件' : '选择 Skill 文件';
      name.textContent = selectedFile ? selectedFile.name : '尚未选择文件';
      meta.textContent = selectedFile ? formatSize(selectedFile.size) + ' · UTF-8 · TXT / MD · 最大 128 KB' : 'UTF-8 · TXT / MD · 最大 128 KB';
    };
  }
  function render() {
    dirtyChecks = []; refreshChecks = []; updateCatalog();
    tabButtons.forEach((tab, index) => tab.setAttribute('aria-pressed', String(section === index)));
    const app = apps[selected]; controls.replaceChildren();
    if (!app) { badge.textContent = '暂无应用'; title.textContent = '创建第一个 AI 应用'; return; }
    title.textContent = app.name; badge.textContent = app.enabled ? '已启用' : '已停用'; badge.dataset.tone = app.enabled ? 'success' : 'neutral';
    if (section === 0) {
      const group = block('基本配置', '设置应用名称与学生使用状态。', '当前应用');
      const name = el('input'); name.value = app.name; field(group, '应用名称', name);
      const enabled = el('input'); enabled.type = 'checkbox'; enabled.setAttribute('role', 'switch'); enabled.checked = app.enabled;
      field(group, '启用应用', enabled);
      group.append(el('p', '停用后，学生端不再显示此应用；已有会话仅可查看历史，不能继续发送。', 'muted'));
      save(group, '保存配置', () => ({ name: name.value, enabled: enabled.checked }), { name: app.name, enabled: app.enabled }, async value => {
        await appSave(app, value); title.textContent = app.name; badge.textContent = app.enabled ? '已启用' : '已停用'; badge.dataset.tone = app.enabled ? 'success' : 'neutral';
      });
      const removal = el('div', undefined, 'application-delete-row');
      const copy = el('div'); copy.append(el('strong', '删除应用'), el('p', '从本班应用列表移除，历史会话与审计记录保留。', 'muted'));
      removal.append(copy);
      const remove = button(removal, '删除应用', () => leave(async () => {
        await uiDialog('删除 AI 应用？', '删除“' + app.name + '”后，学生无法再使用它，历史会话仅可查看。此操作无法在页面中撤销。', '确认删除', async () => {
          await aiMutation('/api/ai/apps/' + app.id, 'DELETE', {});
          apps = apps.filter(item => item.id !== app.id); selected = Math.min(selected, Math.max(0, apps.length - 1)); section = 0;
          setStatus(status, ''); render(); showToast('应用已删除');
        });
      })); remove.classList.add('application-delete-button'); group.append(removal);
    } else if (section === 1) {
      const group = block('应用提示词配置', '角色提示词与所选模板共同决定当前应用的回答规则。', '当前应用');
      const prompt = el('textarea'); prompt.rows = 5; prompt.value = app.role_prompt; field(group, '角色提示词', prompt);
      const select = el('select'); const none = el('option', '不绑定模板'); none.value = ''; select.append(none);
      templates.forEach(item => { const option = el('option', item.name); option.value = item.id; select.append(option); });
      select.value = app.template_id ?? ''; field(group, '选择模板', select);
      save(group, '保存应用提示词', () => ({ role_prompt: prompt.value, template_id: Number(select.value) || null }), { role_prompt: app.role_prompt, template_id: app.template_id ?? null }, value => appSave(app, value));
      const manager = block('提示词模板管理', '模板为本班共享。更新正文会影响所有绑定它的应用之后的回答，已完成的回答保持原样。', '班级共享');
      const editing = el('select'); const fresh = el('option', '新建模板'); fresh.value = ''; editing.append(fresh);
      templates.forEach(item => { const option = el('option', item.name); option.value = item.id; editing.append(option); }); editing.value = app.template_id ?? '';
      field(manager, '编辑模板', editing);
      const editor = el('div'); manager.append(editor);
      let templateDirty = () => false;
      function populate() {
        const item = templates.find(value => value.id === Number(editing.value));
        editor.replaceChildren();
        const name = el('input'); name.value = item?.name || ''; field(editor, '模板名称', name);
        const body = el('textarea'); body.rows = 5; body.value = item?.body || ''; field(editor, '模板正文', body);
        const initial = { name: name.value, body: body.value };
        templateDirty = () => name.value !== initial.name || body.value !== initial.body;
        // A local editor avoids resetting unsaved application prompts.
        const actions = el('div', undefined, 'config-actions'); const hint = el('span', '未修改', 'save-state'); actions.append(hint);
        const update = button(actions, '更新所选模板', async () => {
          if (!item || !templateDirty()) return;
          const value = { name: name.value, body: body.value };
          if (await action(async () => {
            Object.assign(item, await aiMutation('/api/ai/prompt-templates/' + item.id, 'PATCH', value));
            for (const picker of [select, editing]) { const option = [...picker.options].find(option => Number(option.value) === item.id); if (option) option.textContent = item.name; }
          }, false)) Object.assign(initial, value);
          sync();
        }, true);
        const create = button(actions, '另存为新模板', async () => {
          const value = { name: name.value, body: body.value };
          if (await action(async () => {
            const saved = await aiMutation('/api/ai/prompt-templates', 'POST', value); templates.push(saved);
            for (const picker of [select, editing]) { const option = el('option', saved.name); option.value = saved.id; picker.append(option); }
            editing.value = saved.id; prior = editing.value;
          }, false, '模板已创建；选择并保存后应用才会绑定')) populate();
        });
        function sync() { update.disabled = busy || !item || !templateDirty(); create.disabled = busy || !name.value.trim() || !body.value.trim(); hint.textContent = templateDirty() ? '有未保存的修改' : '未修改'; }
        for (const field of [name, body]) field.oninput = sync;
        editor.append(actions); sync();
      }
      dirtyChecks.push(() => templateDirty());
      let prior = editing.value;
      editing.onchange = async () => { const next = editing.value; editing.value = prior; if (templateDirty() && !await confirmDiscard()) return; editing.value = next; prior = next; populate(); };
      populate();
    } else if (section === 2) {
      const bindings = block('当前应用绑定的 Skill', '只有已启用且绑定到当前应用的技能，才可能被问题匹配。勾选表示绑定，不代表启用。', '当前应用');
      const bindingRead = bindingList(bindings, skills, app.skill_ids, (item, bound) => bound ? item.enabled ? '已绑定 · 可匹配' : '已绑定 · 已停用' : item.enabled ? '已启用 · 未绑定' : '已停用 · 未绑定');
      save(bindings, '保存 Skill 绑定', () => ({ skill_ids: bindingRead() }), { skill_ids: [...app.skill_ids].sort((a,b) => a-b) }, value => appSave(app, value));
      const manager = block('Skill 管理', '启用状态、适用条件和文件是本班共享配置，修改会影响所有绑定该技能的应用。', '班级共享');
      const upload = el('details', undefined, 'skill-upload'); upload.append(el('summary', '上传新 Skill')); skillFile(upload, null); manager.append(upload);
      for (const item of skills) {
        const card = el('section', undefined, 'skill-card'); const heading = el('div', undefined, 'section-heading'); heading.append(el('h4', item.name), stateTag(item.enabled ? '已启用' : '已停用', item.enabled ? 'success' : 'neutral'));
        card.append(heading, el('p', '适用条件：' + item.conditions, 'muted'));
        const group = el('details', undefined, 'skill-edit-details'); group.append(el('summary', '编辑配置'));
        const editor = el('div', undefined, 'skill-detail-panel'); group.append(editor);
        const condition = el('input'); condition.value = item.conditions; field(editor, '适用条件', condition);
        const enabled = el('input'); enabled.type = 'checkbox'; enabled.setAttribute('role', 'switch'); enabled.checked = item.enabled; field(editor, '启用 Skill（全班共享）', enabled);
        const refs = el('input'); refs.value = (item.tool_refs || []).join('、'); field(editor, '工具声明', refs);
        editor.append(el('p', item.procedure || '请上传包含规程的文件。', 'skill-procedure'));
        const read = () => ({ conditions: condition.value, enabled: enabled.checked, tool_refs: refs.value.split(/[、,]/).map(value => value.trim()).filter(Boolean) });
        save(editor, '保存 Skill 设置', read, read(), async value => {
          Object.assign(item, await aiMutation('/api/ai/skills/' + item.id, 'PATCH', value));
          heading.lastChild.replaceWith(stateTag(item.enabled ? '已启用' : '已停用', item.enabled ? 'success' : 'neutral'));
          card.querySelector('p').textContent = '适用条件：' + item.conditions;
          bindingRead.refresh();
        });
        const original = el('details'); original.append(el('summary', '查看上传原文'), el('pre', item.raw_text, 'skill-detail-panel citation-full'));
        const replace = el('details'); replace.append(el('summary', '替换文件')); skillFile(replace, item); card.append(group, original, replace); manager.append(card);
      }
    } else if (section === 3) {
      const group = block('绑定知识库资料', '只检索当前应用绑定的本班就绪资料。搜索只筛选列表，不会清除其他已选项。', '当前应用');
      const read = bindingList(group, materials, app.material_ids, null, true);
      save(group, '保存资料绑定', () => ({ material_ids: read() }), { material_ids: [...app.material_ids].sort((a,b) => a-b) }, value => appSave(app, value));
      group.append(pageLink('/knowledge', '打开知识库管理', 'button text-button'));
    } else {
      const group = block('应用工具授权', '应用授权与命中 Skill 的工具声明共同决定可用工具。', '当前应用');
      appToolGrants(group, app, status, check => dirtyChecks.push(check)); group.append(pageLink('/tools', '打开 MCP 工具管理', 'button text-button'));
    }
  }
  setStatus(status, '正在加载本班配置…', 'info');
  Promise.all(['/api/ai/apps', '/api/ai/prompt-templates', '/api/ai/skills', '/api/materials'].map(path => api(path)))
    .then(([a,p,s,m]) => { apps = a.items; templates = p.items; skills = s.items; materials = m; setStatus(status, ''); render(); })
    .catch(error => { setStatus(status, error.message); button(catalog, '重新加载', () => location.reload()); });
}

async function aiMutation(path, method, value) {
  const multipart = value instanceof FormData;
  return api(path, { method, headers: { 'X-CSRF-Token': user.csrf_token, ...(multipart ? {} : { 'Content-Type': 'application/json' }) }, body: multipart ? value : JSON.stringify(value) });
}
function mcpActionBar(parent, text, action) {
  const button = el('button', text, 'secondary'); button.type = 'button'; button.onclick = action; parent.append(button); return button;
}

function mcpInput(parent, name, value = '', type = 'text') {
  const label = el('label', name);
  const input = el('input');
  input.type = type;
  input.value = value;
  label.append(input);
  parent.append(label);
  return input;
}

function toolsPage(content) {
  const section = el('section', undefined, 'card config-controls');
  const status = el('p', '正在加载…', 'feedback');
  status.setAttribute('role', 'status');
  const controls = el('div');
  section.append(el('h2', '本班 MCP 服务'), status, controls);
  content.append(section, pageLink('/audit', '查看工具审计', 'button secondary'));
  async function act(button, work, message) {
    setBusy(button, true, '处理中…');
    try {
      await work();
      await load();
      setStatus(status, message, 'success');
    } catch (error) {
      await load();
      setStatus(status, error.message);
    } finally { if (button.isConnected) setBusy(button, false); }
  }
  async function load() {
    try {
      const data = await api('/api/mcp/servers');
      controls.replaceChildren();
      if (!data.configured) {
        setStatus(status, '服务端尚未配置 MCP 凭据加密密钥。', 'info');
        return;
      }
      const registration = el('div', undefined, 'config-empty');
      const slug = mcpInput(registration, '服务标识');
      const name = mcpInput(registration, '显示名称');
      const base = el('select');
      for (const url of data.allowed_base_urls) {
        const option = el('option', url);
        option.value = url;
        base.append(option);
      }
      const baseField = el('label', '受控 Base URL');
      baseField.append(base);
      registration.append(baseField);
      const token = mcpInput(registration, '连接令牌', '', 'password');
      token.autocomplete = 'new-password';
      const create = mcpActionBar(registration, '登记服务', () => {
        const secret = token.value;
        token.value = '';
        act(create, () => aiMutation('/api/mcp/servers', 'POST', {slug: slug.value, display_name: name.value, base_url: base.value, token: secret || null}), '已登记服务');
      });
      controls.append(registration);
      for (const server of data.items) {
        const card = el('article', undefined, 'config-empty');
        const path = '/api/mcp/servers/' + server.id;
        const labels = { connected: '已连接', connecting: '连接中', disconnected: '未连接' };
        card.append(el('h3', server.display_name), stateTag(labels[server.status], server.status === 'connected' ? 'success' : 'neutral'));
        card.append(el('p', server.slug + ' · ' + server.base_url + ' · 工具 ' + server.tool_count + ' 项'));
        card.append(el('p', server.has_credential ? '已配置令牌' : '未配置令牌', 'muted'));
        if (server.last_error_code) card.append(el('p', '上次连接状态：' + server.last_error_code, 'muted'));
        const connect = mcpActionBar(card, server.status === 'connected' ? '重新连接' : '连接并发现', () => act(connect, () => aiMutation(path + '/connect', 'POST', {}), '工具清单已更新'));
        const disconnect = mcpActionBar(card, '断开', () => act(disconnect, () => aiMutation(path + '/disconnect', 'POST', {}), '服务已断开'));
        const list = el('div');
        mcpActionBar(card, '查看工具清单', async () => {
          try {
            const value = await api(path + '/tools');
            list.replaceChildren();
            if (!value.items.length) list.append(el('p', '暂无可用工具。'));
            for (const tool of value.items) {
              const item = el('details');
              item.append(el('summary', tool.name), el('p', tool.description), el('pre', JSON.stringify(tool.inputSchema, null, 2), 'citation-full'));
              list.append(item);
            }
          } catch (error) { setStatus(status, error.message); }
        });
        const edit = el('details');
        edit.append(el('summary', '编辑服务'));
        const display = mcpInput(edit, '显示名称', server.display_name);
        const replacement = mcpInput(edit, '新令牌（留空保留）', '', 'password');
        replacement.autocomplete = 'new-password';
        const save = mcpActionBar(edit, '保存服务', () => {
          if (display.value === server.display_name && !replacement.value) return;
          const value = { display_name: display.value };
          if (replacement.value) value.token = replacement.value;
          replacement.value = '';
          act(save, () => aiMutation(path, 'PATCH', value), '服务已保存');
        });
        const syncSave = () => { save.disabled = display.value === server.display_name && !replacement.value; };
        display.addEventListener('input', syncSave); replacement.addEventListener('input', syncSave); syncSave();
        const clear = mcpActionBar(edit, '清除令牌', () => act(clear, () => aiMutation(path, 'PATCH', { token: null }), '令牌已清除'));
        clear.disabled = !server.has_credential;
        card.append(list, edit);
        controls.append(card);
      }
      setStatus(status, '');
    } catch (error) { setStatus(status, error.message); }
  }
  load();
}

async function appToolGrants(parent, app, status, registerDirty = () => {}) {
  const path = '/api/ai/apps/' + app.id + '/tool-grants';
  const wrap = el('div');
  parent.append(wrap);
  wrap.append(el('p', '正在读取工具授权…', 'muted'));
  try {
    const [grants, servers] = await Promise.all([api(path), api('/api/mcp/servers')]);
    const sources = await Promise.all(servers.items.filter(server => server.status === 'connected').map(async server => ({
      server, list: await api('/api/mcp/servers/' + server.id + '/tools')
    })));
    const available = new Map([['knowledge_search', '本轮资料检索']]);
    for (const { server, list } of sources) {
      for (const tool of list.items) available.set(tool.name + '@' + server.slug, tool.description);
    }
    if (!wrap.isConnected) return;
    const current = new Set(grants.items.map(item => item.ref));
    wrap.replaceChildren();
    for (const item of grants.items) {
      if (!item.available) wrap.append(el('p', item.ref + ' · 发现清单已失效，请核对后重新保存。', 'muted'));
    }
    const fields = [];
    for (const [ref, description] of available) {
      const label = el('label', undefined, 'choice-row');
      const check = el('input');
      check.type = 'checkbox';
      check.checked = current.has(ref);
      label.append(check, el('span', ref + ' · ' + description));
      wrap.append(label);
      fields.push([ref, check]);
    }
    let baseline = JSON.stringify([...current].sort());
    const read = () => fields.filter(([, check]) => check.checked).map(([ref]) => ref).sort();
    const changed = () => JSON.stringify(read()) !== baseline;
    registerDirty(changed);
    const hint = el('p', '未修改', 'save-state'); wrap.append(hint);
    const save = mcpActionBar(wrap, '保存工具授权', async () => {
      if (!changed()) return;
      setBusy(save, true, '正在保存…');
      try {
        await aiMutation(path, 'PUT', { refs: fields.filter(([, check]) => check.checked).map(([ref]) => ref) });
        setStatus(status, '工具授权已保存', 'success');
        baseline = JSON.stringify(read());
      } catch (error) { setStatus(status, error.message); }
      finally { if (save.isConnected) { setBusy(save, false); sync(); } }
    });
    function sync() { save.disabled = !changed(); hint.textContent = changed() ? '有未保存的修改' : '未修改'; }
    wrap.addEventListener('change', sync); sync();
  } catch (error) { wrap.replaceChildren(el('p', error.message, 'feedback')); }
}

function auditPage(content) {
  const section = el('section', undefined, 'card config-controls');
  const status = el('p', '正在加载…', 'feedback');
  status.setAttribute('role', 'status');
  const items = el('div');
  const more = mcpActionBar(section, '更多记录', () => load(cursor));
  more.hidden = true;
  section.prepend(el('h2', '本班工具调用记录'), status, items);
  content.append(section);
  let cursor;
  async function load(after) {
    more.disabled = true;
    try {
      const data = await api('/api/mcp/audit' + (after ? '?cursor=' + encodeURIComponent(after) : ''));
      if (!after) items.replaceChildren();
      for (const event of data.items) {
        const row = el('article', undefined, 'config-empty');
        row.append(el('strong', event.tool), stateTag(toolStatusLabel(event.status), event.status === 'success' ? 'success' : 'info'),
          el('p', formatTime(event.created_at) + ' · 用户 ' + event.user_id + ' · 会话 ' + event.conversation_id + ' · 第 ' + event.stage + ' 阶段'),
          el('p', event.summary + (event.reason_code ? ' · ' + event.reason_code : ''), 'muted'));
        items.append(row);
      }
      cursor = data.next_cursor;
      more.hidden = !cursor;
      setStatus(status, data.items.length ? '' : '暂无记录', 'info');
    } catch (error) { setStatus(status, error.message); }
    finally { more.disabled = false; }
  }
  load();
}

function toolStatusLabel(status) {
  return { running: '执行中', success: '成功', denied: '已拒绝', error: '失败', cancelled: '已取消' }[status] || status;
}

function renderToolProcess(details, events) {
  const latest = new Map(events.map(event => [event.attempt_id, event]));
  details.replaceChildren(el('summary', '工具执行过程 · ' + latest.size + ' 项'));
  for (const event of latest.values()) {
    details.append(el('p', (event.tool || '未授权工具') + ' · ' + toolStatusLabel(event.status) + ' · ' + event.summary, 'muted'));
  }
  details.hidden = !latest.size;
}

function renderModule(content, route) {
  const [path, title, , description] = route;
  content.append(pageHeading(title, description));
  if (path === '/applications') applicationsPage(content);
  else if (path === '/tools') toolsPage(content);
  else if (path === '/audit') auditPage(content);
  else if (path === '/assistant') assistantPage(content);
  else content.append(pendingPanel(title, plannedModules[path], '该模块将在后续教学业务接入后开放。'));
}
function assistantDisplayText(text) {
  if (/<\|FunctionCall(?:Begin|End)\|>|<\/?tool_call>|\[TOOL_CALLS\]/i.test(text || '')) {
    return '这条历史回答包含未执行的内部工具调用文本，已隐藏。请重新提问。';
  }
  return text;
}
function assistantPage(content) {
  const shell = el('section', undefined, 'card assistant-shell');
  const toolbar = el('div', undefined, 'assistant-toolbar');
  const title = el('h2', 'AI 助手');
  const badge = stateTag('流式会话', 'success');
  const headline = el('div', undefined, 'assistant-title');
  headline.append(lineIcon('bot'), title, badge);
  const picker = el('label', '选择 AI 应用', 'application-picker');
  const select = el('select', undefined, 'workspace-select');
  picker.append(select);
  toolbar.append(headline, picker);
  const layout = el('div', undefined, 'assistant-layout');
  const aside = el('aside', undefined, 'session-rail');
  aside.setAttribute('aria-label', '会话区域');
  const heading = el('div', undefined, 'session-heading');
  const create = el('button', '新对话', 'secondary reset-chat');
  heading.append(el('h3', '历史会话'), create);
  const sessions = el('div', undefined, 'session-list');
  aside.append(heading, sessions);
  const view = el('div', undefined, 'assistant-view');
  const live = el('div', undefined, 'assistant-live');
  const panel = el('section', undefined, 'card retrieval-panel chat-panel');
  const transcript = el('div', undefined, 'chat-transcript');
  transcript.setAttribute('aria-label', '会话消息');
  transcript.tabIndex = 0;
  const older = el('button', '加载更早消息', 'secondary');
  older.hidden = true;
  const messages = el('div');
  const status = el('p', '', 'feedback');
  status.setAttribute('role', 'status');
  transcript.append(older, messages, status);
  const form = el('form');
  form.id = 'ask-form';
  const label = el('label');
  const input = el('textarea');
  input.name = 'question';
  input.rows = 2;
  input.setAttribute('aria-label', '你的问题');
  input.placeholder = '你的问题';
  label.append(input);
  const actions = el('div', undefined, 'retrieval-actions');
  const send = el('button', '发送', 'primary');
  const cancel = el('button', '取消生成', 'secondary');
  cancel.id = 'cancel-answer';
  cancel.type = 'button';
  actions.append(el('span', '多轮会话 · 来源可核对', 'composer-meta'), send, cancel);
  form.append(label, actions);
  panel.append(transcript, form);
  live.append(panel);
  view.append(live);
  layout.append(aside, view);
  const appNotice = el('p', '', 'application-readonly-notice'); appNotice.hidden = true; appNotice.setAttribute('role', 'status');
  shell.append(toolbar, appNotice, layout);
  content.append(shell);

  let appChoices = [], conversationList = [], active = null, turns = [], nextTurn = null, nextConversation = null;
  let revision = 0, pending, loading = false;
  function appOptions() {
    select.replaceChildren();
    for (const app of appChoices) {
      const option = el('option', app.name);
      option.value = app.id;
      select.append(option);
    }
    if (active && !appChoices.some(app => app.id === active.app_id)) {
      const option = el('option', active.app_name + ' · 已停用');
      option.value = active.app_id;
      select.append(option);
    }
    if (active) select.value = active.app_id;
  }
  function updateButtons() {
    select.disabled = Boolean(active) || !appChoices.length;
    const unavailable = active?.app_enabled === false;
    send.disabled = Boolean(pending) || loading || unavailable || (!active && !appChoices.length);
    input.disabled = unavailable;
    appNotice.hidden = !unavailable;
    appNotice.textContent = unavailable ? '此应用已停用或删除，当前会话仅可查看历史。请新建对话选择其他已启用应用。' : '';
    create.disabled = loading || (!active && !appChoices.length);
    cancel.hidden = !pending;
    older.hidden = !nextTurn;
    title.textContent = active?.app_name || appChoices.find(item => item.id === Number(select.value))?.name || 'AI 助手';
  }
  function appendTurn(turn, parent) {
    const article = el('article', undefined, 'conversation-turn');
    for (const item of turn.messages) {
      if (item.role === 'assistant' && turn.tools?.length) {
        const process = el('details', undefined, 'chat-capabilities');
        renderToolProcess(process, turn.tools);
        article.append(process);
      }
      article.append(el('div', item.role === 'user' ? item.body : assistantDisplayText(item.body), item.role === 'user' ? 'chat-question' : 'rag-answer'));
      if (item.metadata?.skill) article.append(stateTag('技能 · ' + item.metadata.skill.name, 'info'));
      if (item.metadata?.citations?.length) {
        const sources = el('div');
        renderSources(sources, item.metadata.citations, true, true);
        article.append(sources);
      }
    }
    if (turn.status !== 'completed') {
      if (turn.tools?.length) {
        const process = el('details', undefined, 'chat-capabilities');
        renderToolProcess(process, turn.tools);
        article.append(process);
      }
      article.append(el('p', turn.status === 'running' ? '生成进行中' : '此轮回答未保存', 'muted'));
    }
    parent.append(article);
  }
  function renderMessages(scroll = true) {
    messages.replaceChildren();
    if (!turns.length) {
      const empty = el('div', undefined, 'chat-empty');
      empty.append(el('h3', '有什么可以帮你？'), el('p', '选择应用提问，也可以恢复已有会话。', 'muted'));
      const suggestions = el('div', undefined, 'suggestions');
      for (const question of ['期末项目如何提交？', '讲义中有哪些重点？']) {
        const suggestion = el('button', question, 'secondary');
        suggestion.onclick = () => { input.value = question; sizeInput(); input.focus(); };
        suggestions.append(suggestion);
      }
      empty.append(suggestions);
      messages.append(empty);
    }
    turns.forEach(turn => appendTurn(turn, messages));
    if (scroll) transcript.scrollTop = transcript.scrollHeight;
    updateButtons();
  }
  function renderSessions() {
    sessions.replaceChildren();
    if (!conversationList.length) sessions.append(el('p', '暂无会话', 'muted'));
    for (const item of conversationList) {
      const row = el('div', undefined, 'session-row');
      row.dataset.active = String(item.id === active?.id);
      const button = el('button', undefined, 'app-choice session-open');
      button.append(el('span', item.title, 'session-title'));
      button.title = item.title;
      button.setAttribute('aria-pressed', String(item.id === active?.id));
      button.onclick = () => open(item);
      const remove = el('button', '⋯', 'session-menu'); remove.type = 'button';
      remove.setAttribute('aria-label', '删除会话：' + item.title); remove.title = '删除会话';
      remove.onclick = () => uiDialog('删除此会话？', '会话“' + item.title + '”将从你的历史列表移除，无法在页面中恢复。', '删除会话', async () => {
        await aiMutation('/api/ai/conversations/' + item.id, 'DELETE', {});
        conversationList = conversationList.filter(value => value.id !== item.id);
        if (active?.id === item.id) newDraft(); else renderSessions();
        showToast('会话已删除');
        try { await loadSessions(); } catch { setStatus(status, '会话已删除，但列表刷新失败。请刷新页面查看。', 'warning'); }
      });
      row.append(button, remove); sessions.append(row);
    }
    if (nextConversation) {
      const more = el('button', '更多会话', 'secondary');
      more.onclick = () => loadSessions(nextConversation).catch(error => setStatus(status, error.message));
      sessions.append(more);
    }
  }
  async function loadSessions(cursor) {
    const data = await api('/api/ai/conversations' + (cursor ? '?cursor=' + encodeURIComponent(cursor) : ''));
    const previous = cursor ? conversationList : [];
    const unique = new Map([...previous, ...data.items].map(item => [item.id, item]));
    conversationList = [...unique.values()];
    nextConversation = data.next_cursor;
    renderSessions();
  }
  async function loadConversation(id, stamp, cursor) {
    const data = await api('/api/ai/conversations/' + id + (cursor ? '?cursor=' + encodeURIComponent(cursor) : ''));
    if (stamp !== revision) return;
    active = data.conversation;
    if (active.app_enabled === false) appChoices = appChoices.filter(item => item.id !== active.app_id);
    else if (active.app_enabled === true && !appChoices.some(item => item.id === active.app_id)) appChoices.push({ id: active.app_id, name: active.app_name });
    turns = cursor ? [...data.items, ...turns] : data.items;
    nextTurn = data.next_cursor;
    appOptions();
    renderSessions();
    renderMessages(!cursor);
  }
  function abandon() {
    revision++;
    pending?.abort();
    pending = undefined;
    loading = false;
  }
  async function open(item) {
    abandon();
    active = item;
    turns = [];
    nextTurn = null;
    appOptions();
    loading = true;
    renderMessages();
    const stamp = revision;
    setStatus(status, '正在读取会话…', 'info');
    try {
      await loadConversation(item.id, stamp);
      if (stamp === revision) setStatus(status, '');
    } catch (error) { if (stamp === revision) setStatus(status, error.message); }
    finally { if (stamp === revision) { loading = false; updateButtons(); } }
  }
  function newDraft() {
    abandon();
    active = null;
    turns = [];
    nextTurn = null;
    appOptions();
    renderSessions();
    renderMessages();
    input.value = '';
    sizeInput();
    setStatus(status, '');
    input.focus();
  }
  create.onclick = newDraft;
  select.onchange = updateButtons;
  older.onclick = () => {
    const button = older;
    button.disabled = true;
    loadConversation(active.id, revision, nextTurn).catch(error => setStatus(status, error.message)).finally(() => { button.disabled = false; });
  };
  function sizeInput() {
    input.style.height = 'auto';
    input.style.height = (input.scrollHeight + input.offsetHeight - input.clientHeight) + 'px';
  }
  input.addEventListener('input', sizeInput);
  input.addEventListener('keydown', event => {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
      event.preventDefault();
      if (!send.disabled) form.requestSubmit();
    }
  });
  window.addEventListener('pagehide', abandon, { once: true });
  cancel.onclick = () => pending?.abort();
  form.onsubmit = async event => {
    event.preventDefault();
    const question = input.value.trim();
    if (!question || [...question].length > 500) { setStatus(status, '请输入 1–500 字的问题'); return; }
    if (pending || loading || active?.app_enabled === false) return;
    const stamp = revision;
    const controller = new AbortController();
    pending = controller;
    input.value = '';
    sizeInput();
    updateButtons();
    let failed = false;
    let finalText = '';
    let sessionId = active?.id;
    let requestId;
    setStatus(status, '正在准备对话…', 'info');
    try {
      if (!sessionId) {
        const item = await aiMutation('/api/ai/conversations', 'POST', { app_id: Number(select.value) });
        if (stamp !== revision) return;
        active = item;
        sessionId = item.id;
        appOptions();
        updateButtons();
        if (controller.signal.aborted) throw new Error('已取消');
      }
      messages.querySelector('.chat-empty')?.remove();
      const article = el('article', undefined, 'conversation-turn');
      const answer = el('div', '', 'rag-answer');
      const process = el('details', undefined, 'chat-capabilities');
      process.hidden = true;
      const toolEvents = [];
      article.append(el('div', question, 'chat-question'), process, answer);
      messages.append(article);
      requestId = crypto.randomUUID();
      await readConversationStream(sessionId, question, requestId, controller.signal, (type, data) => {
        if (stamp !== revision || controller.signal.aborted) return;
        if (type === 'meta') {
          if (data.skill) article.insertBefore(stateTag('技能 · ' + data.skill.name, 'info'), answer);
          const info = { hits: '正在生成 · 已找到资料', no_hits: '正在生成 · 无匹配资料', unbound: '正在生成 · 应用未绑定资料' };
          setStatus(status, info[data.retrieval_status], 'info');
        }
        if (type === 'delta') {
          finalText += data.text;
          answer.textContent = finalText;
          transcript.scrollTop = transcript.scrollHeight;
        }
        if (type === 'tool') {
          toolEvents.push(data);
          renderToolProcess(process, toolEvents);
          setStatus(status, toolStatusLabel(data.status) + ' · ' + data.summary, 'info');
        }
        if (type === 'done') {
          const sources = el('div');
          renderSources(sources, data.citations, true, true);
          article.append(sources);
          setStatus(status, '回答已保存', 'success');
        }
      });
    } catch (error) {
      failed = true;
      if (stamp === revision) setStatus(status, controller.signal.aborted ? '已取消，正在核对保存结果。' : error.message);
    } finally {
      if (stamp === revision) {
        try {
          if (sessionId) await loadConversation(sessionId, stamp);
          if (stamp === revision && failed && finalText && !turns.some(turn => turn.client_request_id === requestId && turn.status === 'completed')) {
            messages.append(el('p', '未保存的部分回答', 'muted'), el('div', finalText, 'rag-answer'));
          }
          if (stamp === revision && controller.signal.aborted) setStatus(status, '已取消生成，以上历史以服务器保存结果为准。', 'info');
          if (stamp === revision) await loadSessions();
        } catch (error) {
          if (stamp === revision) setStatus(status, '无法确认保存结果。请重新打开会话。' + error.message);
        } finally {
          if (stamp === revision) { pending = undefined; updateButtons(); }
        }
      }
    }
  };
  async function refreshAppAvailability() {
    if (!shell.isConnected || document.hidden) return;
    try {
      const data = await api('/api/ai/apps');
      if (!shell.isConnected) return;
      appChoices = data.items;
      if (active) active.app_enabled = appChoices.some(item => item.id === active.app_id);
      const previousChoice = select.value; appOptions();
      if (!active && appChoices.some(item => String(item.id) === previousChoice)) select.value = previousChoice;
      updateButtons();
    } catch (error) { if (shell.isConnected) setStatus(status, error.message); }
  }
  window.addEventListener('focus', refreshAppAvailability);
  document.addEventListener('visibilitychange', refreshAppAvailability);
  renderMessages();
  setStatus(status, '正在加载应用和会话…', 'info');
  Promise.all([api('/api/ai/apps'), loadSessions()]).then(([data]) => {
    appChoices = data.items;
    appOptions();
    updateButtons();
    setStatus(status, appChoices.length ? '' : '暂无启用应用，可打开已有会话。', 'info');
  }).catch(error => setStatus(status, error.message));
}

async function readConversationStream(id, question, requestId, signal, onEvent) {
  const response = await fetch('/api/ai/conversations/' + id + '/messages', {
    method: 'POST', credentials: 'same-origin', signal,
    headers: authHeaders({ 'Content-Type': 'application/json', 'Accept': 'text/event-stream', 'X-CSRF-Token': user.csrf_token }),
    body: JSON.stringify({ question, client_request_id: requestId })
  });
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    if (response.status === 401) { clearToken(); location.replace('/login'); }
    if (response.status === 403 && error.code === 'password_change_required') location.replace('/profile');
    throw new Error(error.error || '会话请求失败（HTTP ' + response.status + '）');
  }
  if (!response.headers.get('content-type')?.startsWith('text/event-stream') || !response.body) {
    throw new Error('服务未返回有效的会话流');
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '', turnId;
  try {
    for (;;) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value, { stream: !done });
      for (let boundary; (boundary = /\r?\n\r?\n/.exec(buffer)); ) {
        const frame = buffer.slice(0, boundary.index);
        buffer = buffer.slice(boundary.index + boundary[0].length);
        const lines = frame.split(/\r?\n/);
        const type = lines.find(line => line.startsWith('event:'))?.slice(6).trim();
        if (!type) continue;
        const data = JSON.parse(lines.filter(line => line.startsWith('data:')).map(line => line.slice(5).replace(/^ /, '')).join('\n'));
        if (type === 'meta' && turnId === undefined) turnId = data.turn_id;
        if (!turnId || data.turn_id !== turnId) throw new Error('会话流事件顺序无效');
        if (type === 'error') throw new Error(data.message || '回答未完成');
        onEvent(type, data);
        if (type === 'done') return;
      }
      if (done) throw new Error('会话流已中断，正在核对保存结果。');
      if (buffer.length > 256 * 1024) throw new Error('会话流事件超限');
    }
  } finally { await reader.cancel().catch(() => {}); }
}

function renderSources(container, hits, numbered, compact = false) {
  const labels = [];
  for (const [i, hit] of hits.entries()) {
    const item = el('article', undefined, 'source-hit');
    const number = numbered ? '[' + (hit.number || i + 1) + '] ' : '';
    if (hit.source_status === 'material_deleted') { container.append(el('p', number + '材料已删除，原文与引用片段不可用', 'muted deleted-source')); continue; }
    if (!hit.material_id) { container.append(el('p', number + '来源当前不可用', 'muted')); continue; }
    const suffix = ' · 切片 ' + hit.chunk_index;
    const link = el('a', number + hit.material_title + suffix);
    link.href = '/materials?material_id=' + encodeURIComponent(hit.material_id) + (hit.chunk_id ? '&chunk_id=' + encodeURIComponent(hit.chunk_id) : '');
    link.onclick = event => { event.preventDefault(); showText(hit.material_id, hit.chunk_id); };
    const position = el('p', '原文字符 [' + hit.char_start + ', ' + hit.char_end + ')', 'muted');
    if (compact) {
      item.classList.add('compact-source');
      const name = el('span', hit.material_title, 'source-title');
      labels.push([name, hit]);
      const details = el('details', undefined, 'citation-details');
      const summary = el('summary');
      summary.append(el('span', '来源 ' + (hit.number || i + 1), 'source-chip'), name);
      link.textContent = '查看原文' + suffix;
      details.append(summary, position, el('p', hit.chunk_text || hit.excerpt, 'citation-full'), link);
      item.append(details);
    } else {
      item.append(link, position, el('p', hit.excerpt));
    }
    container.append(item);
  }
  return labels;
}
function renderRetrieval(target, kind) {
  const chat = kind === 'chat';
  const searchOnly = kind === 'search';
  const workspaceChat = chat && target.classList.contains('assistant-live');
  const section = el('section', undefined, 'card retrieval-panel' + (chat ? ' chat-panel' : searchOnly ? ' search-panel' : ''));
  section.innerHTML = (chat ? '<div class="chat-transcript" aria-label="当前问答">' : '<h2>' + (searchOnly ? '检索本班资料' : '知识库 AI 问答') + '</h2><p class="muted">使用本班已就绪资料，可以打开引用核对原文。</p>')
    + (chat ? '<div class="chat-empty"><span class="chat-symbol"></span><h3>有什么可以帮你？</h3><p class="muted">问问本班讲义，打开引用核对原文。</p><div class="suggestions"></div></div><div id="chat-question" class="chat-question" hidden></div>' : '')
    + '<div id="rag-answer" class="rag-answer" aria-live="polite"></div><div id="rag-sources"></div>'
    + '<p id="rag-status" class="feedback" role="status" hidden></p>'
    + (chat ? '</div>' : '')
    + '<form id="ask-form"><label>' + (workspaceChat ? '' : searchOnly ? '检索内容' : '你的问题')
    + '<textarea name="question" required maxlength="500" rows="2"'
    + (workspaceChat ? ' aria-label="你的问题" placeholder="你的问题"' : ' placeholder="例如：期末项目如何提交？"')
    + '></textarea></label>'
    + '<div class="retrieval-actions">'
    + (chat ? '<span class="composer-meta">单轮问答 · 来源可核对</span>' : '')
    + (!searchOnly ? '<button class="primary" type="submit">基于资料回答</button>' : '')
    + (!chat ? '<label class="search-mode-field"><span>检索模式</span><select class="workspace-select" name="mode"><option value="hybrid">混合检索</option><option value="keyword">关键字检索</option><option value="vector">向量检索</option></select></label><button id="search-only" class="' + (searchOnly ? 'primary' : 'secondary') + '" type="' + (searchOnly ? 'submit' : 'button') + '">仅检索资料</button>' : '')
    + (chat ? '<button id="cancel-answer" type="button" class="secondary" hidden>取消等待</button>' : '')
    + '</div></form>';
  target.append(section);
  const form = section.querySelector('form');
  const status = section.querySelector('#rag-status');
  const answer = section.querySelector('#rag-answer');
  const sources = section.querySelector('#rag-sources');
  const cancel = section.querySelector('#cancel-answer');
  const questionBubble = section.querySelector('#chat-question');
  const transcript = section.querySelector('.chat-transcript');
  const input = form.elements.question;
  const sizeInput = () => {
    if (!workspaceChat || !input.isConnected) return;
    input.style.height = 'auto';
    input.style.height = (input.scrollHeight + input.offsetHeight - input.clientHeight) + 'px';
  };
  if (workspaceChat) {
    input.addEventListener('input', sizeInput);
    window.addEventListener('resize', sizeInput);
    transcript.tabIndex = 0;
  }
  const buttons = [...form.querySelectorAll('button:not(#cancel-answer)')];
  let request;
  const updateBusy = busy => {
    buttons.forEach(button => setBusy(button, busy, '处理中…'));
    if (cancel) cancel.hidden = !busy;
  };
  if (chat) {
    section.querySelector('.chat-symbol').append(lineIcon('chat'));
    for (const question of ['期末项目如何提交？', '讲义中有哪些重点？']) {
      const suggestion = el('button', question, 'secondary');
      suggestion.type = 'button';
      suggestion.onclick = () => { input.value = question; sizeInput(); input.focus(); };
      section.querySelector('.suggestions').append(suggestion);
    }
    cancel.onclick = () => {
      if (!request) return;
      request.abort();
      request = undefined;
      updateBusy(false);
      setStatus(status, '已取消浏览器等待，服务器端可能仍在处理。', 'info');
    };
  }
  const run = async ask => {
    const question = form.elements.question.value.trim();
    if (request || !question || !form.reportValidity()) return;
    const current = new AbortController();
    request = current;
    if (workspaceChat) {
      input.value = '';
      sizeInput();
    }
    updateBusy(true);
    answer.textContent = '';
    sources.replaceChildren();
    if (questionBubble) {
      transcript.scrollTop = 0;
      section.querySelector('.chat-empty').hidden = true;
      questionBubble.textContent = question;
      questionBubble.hidden = false;
    }
    setStatus(status, ask ? '正在检索并等待单轮回答…' : '正在检索本班资料…', 'info');
    try {
      const data = await api(ask ? '/api/ask' : '/api/search', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(ask ? { question } : { q: question, mode: form.elements.mode.value, limit: 10 }),
        signal: current.signal
      });
      if (request !== current) return;
      answer.textContent = ask ? data.answer : '';
      const labels = renderSources(sources, ask ? data.citations : data.hits, ask, workspaceChat);
      if (labels.length) {
        api('/api/ai/apps').then(data => {
      if (!Array.isArray(data.items)) throw new Error('Invalid app list');
      appsCount.textContent = data.items.filter(item => item.enabled).length + ' 个已启用';
      appsCount.dataset.tone = 'success';
    }).catch(() => { appsCount.textContent = '应用读取失败'; appsCount.dataset.tone = 'warning'; });
    api('/api/mcp/servers').then(data => {
      if (!Array.isArray(data.items)) throw new Error('Invalid tool list');
      const count = data.items.filter(item => item.status === 'connected').length;
      toolsCount.textContent = data.configured === false ? '服务待配置' : count ? count + ' 个服务已连接' : '暂无已连接服务';
      toolsCount.dataset.tone = count ? 'success' : 'neutral';
    }).catch(() => { toolsCount.textContent = '工具读取失败'; toolsCount.dataset.tone = 'warning'; });
    api('/api/materials').then(materials => {
          if (!Array.isArray(materials)) return;
          const filenames = new Map(materials.map(material => [material.id, material.original_filename]));
          for (const [label, hit] of labels) {
            const filename = filenames.get(hit.material_id);
            if (filename) {
              label.title = hit.material_title;
              label.textContent = filename;
            }
          }
        }).catch(() => { /* Keep the material title when metadata is unavailable. */ });
      }
      const message = ask
        ? data.citations.length ? '回答已生成，请核对引用。' : '本次回答没有资料引用。'
        : data.message || (data.hits.length ? '找到 ' + data.hits.length + ' 条资料。' : '暂无匹配资料。');
      setStatus(status, message, (ask ? data.citations : data.hits).length ? 'success' : 'info');
    } catch (error) {
      if (request === current) setStatus(status, error.message);
    } finally {
      if (request === current) {
        request = undefined;
        updateBusy(false);
      }
    }
  };
  form.onsubmit = event => { event.preventDefault(); run(!searchOnly); };
  if (!chat && !searchOnly) section.querySelector('#search-only').onclick = () => run(false);
  if (chat) return () => {
    cancel.click();
    form.reset();
    sizeInput();
    transcript.scrollTop = 0;
    questionBubble.hidden = true;
    questionBubble.textContent = '';
    answer.textContent = '';
    sources.replaceChildren();
    setStatus(status, '');
    section.querySelector('.chat-empty').hidden = false;
    form.elements.question.focus();
  };
}

if (location.pathname === '/login') loginPage();
else (location.pathname === '/register' ? registerPage() : location.pathname === '/profile' ? profilePage() : workspacePage()).catch(error => {
  app.textContent = error.message;
});
