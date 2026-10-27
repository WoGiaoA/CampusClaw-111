'use strict';
const app = document.querySelector('#app');
let user;
app.className = ['/login', '/register', '/profile'].includes(location.pathname) ? 'auth-page' : 'materials-page';

async function api(path, options = {}) {
  let response;
  try { response = await fetch(path, { credentials: 'same-origin', ...options }); }
  catch { throw new Error('网络连接中断，无法确认操作结果。请恢复连接后查看列表或登录状态。'); }
  let body;
  try { body = await response.json(); }
  catch { throw new Error(`服务暂不可用（HTTP ${response.status}），请稍后重试。`); }
  if (response.status === 401 && path !== '/api/login') {
    location.replace('/login'); throw new Error('请先登录');
  }
  if (response.status === 403 && body?.code === 'password_change_required') {
    location.replace('/profile'); throw new Error(body.error);
  }
  if (!response.ok) throw new Error(body?.error || '请求失败');
  return body;
}
function el(tag, text, className) {
  const node = document.createElement(tag);
  if (text !== undefined) node.textContent = text;
  if (className) node.className = className;
  return node;
}
function setStatus(node, message, tone = 'error') {
  node.textContent = message; node.dataset.tone = tone; node.hidden = !message;
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
function renderIdentity(errorTarget) {
  const identity = el('div', undefined, 'identity');
  const name = el('span', user.username, 'identity-name'); name.title = user.username;
  const role = el('span', user.role === 'teacher' ? '教师' : '学生', 'role-label');
  const profile = el('a', '个人信息', 'nav-link'); profile.href = '/profile';
  const logout = el('button', '退出', 'text-button nav-logout');
  logout.onclick = async () => {
    setBusy(logout, true, '退出中…');
    try { await api('/api/logout', { method: 'POST', headers: { 'X-CSRF-Token': user.csrf_token } }); location.replace('/login'); }
    catch (error) { setStatus(document.querySelector(errorTarget), error.message); }
    finally { setBusy(logout, false); }
  };
  identity.append(name, role, profile, logout);
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
      form.elements.password.value = '';
      location.assign(identity.must_change_password ? '/profile' : '/materials');
    } catch (error) {
      setStatus(status, error.message);
      if (error.message === '账号或密码错误') {
        form.elements.username.setAttribute('aria-invalid', 'true'); form.elements.password.setAttribute('aria-invalid', 'true');
      }
    } finally { setBusy(button, false); }
  });
}
async function showText(id) {
  try {
    const material = await api(`/api/materials/${id}`);
    const dialog = el('dialog', undefined, 'material-dialog'); dialog.setAttribute('aria-labelledby', 'dialog-title');
    const heading = el('div', undefined, 'dialog-heading');
    const title = el('h2', material.title); title.id = 'dialog-title';
    const close = el('button', '关闭', 'secondary'); close.type = 'button'; close.addEventListener('click', () => dialog.close());
    heading.append(title, close);
    dialog.append(heading, el('p', `${material.original_filename} · ${formatSize(material.byte_size)}`, 'dialog-meta'), el('pre', material.body_text));
    dialog.addEventListener('close', () => dialog.remove(), { once: true });
    document.body.append(dialog); dialog.showModal();
  } catch (error) { setStatus(document.querySelector('#status'), error.message); }
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
      const view = el('button', '查看正文', 'text-button'); view.onclick = () => showText(m.id);
      const download = el('a', '下载原文件', 'button secondary'); download.href = `/api/materials/${m.id}/file`;
      actions.append(view, download); row.append(icon, info, actions); list.append(row);
    }
  } finally { list.setAttribute('aria-busy', 'false'); }
}
async function materialsPage() {
  user = await api('/api/me');
  if (user.must_change_password) { location.replace('/profile'); return; }
  app.innerHTML = '<section class="heading"><div><div class="title-line"><span class="badge" id="class-name"></span><h1>本班教学材料</h1></div><p class="muted" id="welcome"></p></div></section><p class="feedback workspace-feedback" id="status" role="status" aria-live="polite" hidden></p><div class="workspace" id="workspace"><section class="card material-panel" aria-labelledby="materials-title"><div class="list-header"><div class="list-title"><h2 id="materials-title">教学材料</h2><span class="count" id="count"></span></div><button class="text-button" id="refresh-materials">刷新列表</button></div><div id="material-list"><p class="list-loading">正在加载材料…</p></div></section></div>';
  renderIdentity('#status');
  document.querySelector('#class-name').textContent = user.class_name;
  document.querySelector('#welcome').textContent = user.role === 'teacher' ? '教师空间 · 上传讲义，为本班积累学习资源。' : '学生空间 · 查看正文或下载原文件，本班材料只读。';
  document.querySelector('#refresh-materials').onclick = async event => {
    const button = event.currentTarget; setBusy(button, true, '刷新中…');
    try { await refresh(); setStatus(document.querySelector('#status'), '材料列表已更新。', 'success'); }
    catch (error) { setStatus(document.querySelector('#status'), error.message); }
    finally { setBusy(button, false); }
  };
  const workspace = document.querySelector('#workspace');
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
  try { await refresh(); }
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
  app.innerHTML = '<section class="card auth-single profile-card" aria-labelledby="profile-title"><div class="auth-heading"><p class="eyebrow">账号与安全</p><h1 id="profile-title">个人信息</h1><p id="profile-identity" class="profile-identity"></p></div><p class="feedback" data-tone="info" id="password-notice"></p><form id="password-form"><label id="current-password-label">当前密码<input name="current_password" type="password" autocomplete="current-password"></label><label>新密码<input name="new_password" type="password" minlength="8" maxlength="72" autocomplete="new-password" required></label><label>确认新密码<input name="confirm_password" type="password" autocomplete="new-password" required></label><p class="form-help">8–72 字节，包含字母和数字，不能使用初始密码或当前密码。</p><button class="primary full-width">修改密码并重新登录</button></form><p class="feedback" id="password-status" role="alert" hidden></p><div class="auth-footer"><a id="profile-back" href="/materials">返回材料</a><button class="text-button" id="profile-logout">退出登录</button></div></section>';
  renderIdentity('#password-status');
  document.querySelector('#profile-identity').textContent = `${user.username} · ${user.role === 'teacher' ? '教师' : '学生'} · ${user.class_name}`;
  document.querySelector('#password-notice').textContent = user.must_change_password ? '请先修改初始密码，完成后才能访问教学材料。' : '角色和班级由管理员管理；修改密码将退出所有已登录设备。';
  const form = document.querySelector('#password-form'); form.elements.current_password.required = !user.must_change_password;
  document.querySelector('#current-password-label').hidden = user.must_change_password; document.querySelector('#profile-back').hidden = user.must_change_password;
  form.addEventListener('input', () => clearFieldErrors(form));
  form.onsubmit = async event => {
    event.preventDefault(); const button = form.querySelector('button'); setBusy(button, true, '正在保存…'); clearFieldErrors(form); setStatus(document.querySelector('#password-status'), '');
    try {
      await api('/api/password', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': user.csrf_token }, body: JSON.stringify({ current_password: form.elements.current_password.value, new_password: form.elements.new_password.value, confirm_password: form.elements.confirm_password.value }) });
      form.reset(); location.replace('/login?password_changed=1');
    } catch (error) {
      setStatus(document.querySelector('#password-status'), error.message);
      const field = error.message === '当前密码错误' ? form.elements.current_password : form.elements.new_password;
      field.setAttribute('aria-invalid', 'true');
    } finally { setBusy(button, false); }
  };
  document.querySelector('#profile-logout').onclick = async event => {
    const button = event.currentTarget; setBusy(button, true, '退出中…');
    try { await api('/api/logout', { method: 'POST', headers: { 'X-CSRF-Token': user.csrf_token } }); location.replace('/login'); }
    catch (error) { setStatus(document.querySelector('#password-status'), error.message); }
    finally { setBusy(button, false); }
  };
}
if (location.pathname === '/login') loginPage();
else (location.pathname === '/register' ? registerPage() : location.pathname === '/profile' ? profilePage() : materialsPage()).catch(error => { app.textContent = error.message; });