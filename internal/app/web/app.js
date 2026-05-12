const TOKEN_KEY = 'gateway_token';

const authView = document.querySelector('#authView');
const dashboardView = document.querySelector('#dashboardView');
const authForm = document.querySelector('#authForm');
const tokenInput = document.querySelector('#tokenInput');
const authButton = document.querySelector('#authButton');
const logoutButton = document.querySelector('#logoutButton');
const targetIPInput = document.querySelector('#targetIPInput');
const tempButton = document.querySelector('#tempButton');
const permButton = document.querySelector('#permButton');
const currentIP = document.querySelector('#currentIP');
const temporaryList = document.querySelector('#temporaryList');
const permanentList = document.querySelector('#permanentList');
const toast = document.querySelector('#toast');

let token = localStorage.getItem(TOKEN_KEY) || '';

function showAuth(message = '') {
  dashboardView.classList.add('hidden');
  authView.classList.remove('hidden');
  if (message) showToast(message);
}

function showDashboard() {
  authView.classList.add('hidden');
  dashboardView.classList.remove('hidden');
}

function setBusy(button, busy, label) {
  button.disabled = busy;
  if (busy) {
    button.dataset.label = button.textContent;
    button.textContent = label;
  } else if (button.dataset.label) {
    button.textContent = button.dataset.label;
    delete button.dataset.label;
  }
}

function showToast(message) {
  toast.textContent = message;
  toast.classList.remove('hidden');
  window.clearTimeout(showToast.timer);
  showToast.timer = window.setTimeout(() => toast.classList.add('hidden'), 2600);
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
      ...(options.headers || {}),
    },
  });

  const payload = await response.json().catch(() => ({
    result: null,
    error: { message: '网关返回了无法解析的响应' },
  }));

  if (response.status === 401) {
    localStorage.removeItem(TOKEN_KEY);
    token = '';
    showAuth(payload.error?.message || '凭证已失效，请重新输入');
    throw new Error('unauthorized');
  }

  if (!response.ok || payload.error) {
    throw new Error(payload.error?.message || '网关请求失败');
  }

  return payload.result;
}

function formatTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(date);
}

function renderInfo(info) {
  currentIP.textContent = info.current_ip;
  renderTemporary(info.temporary_ips || []);
  renderPermanent(info.permanent_ips || []);
}

function renderTemporary(items) {
  temporaryList.innerHTML = '';
  if (items.length === 0) {
    temporaryList.innerHTML = '<p class="empty">暂无临时授权</p>';
    return;
  }

  const now = Date.now();
  for (const item of items) {
    const expired = new Date(item.expires_at).getTime() <= now;
    const row = document.createElement('div');
    row.className = `row${expired ? ' expired' : ''}`;
    row.innerHTML = `
      <div class="ip"></div>
      <div class="meta">添加时间：${formatTime(item.added_at)}</div>
      <div class="meta">到期时间：${formatTime(item.expires_at)}${expired ? ' · 已过期/即将清理' : ''}</div>
      <button class="delete-button" type="button">删除</button>
    `;
    row.querySelector('.ip').textContent = item.ip;
    row.querySelector('.delete-button').addEventListener('click', () => deleteIP(item.ip));
    temporaryList.append(row);
  }
}

function renderPermanent(items) {
  permanentList.innerHTML = '';
  if (items.length === 0) {
    permanentList.innerHTML = '<p class="empty">暂无永久授权</p>';
    return;
  }

  for (const item of items) {
    const row = document.createElement('div');
    row.className = 'row';
    row.innerHTML = `
      <div class="ip"></div>
      <div class="meta">添加时间：${formatTime(item.added_at)}</div>
      <button class="delete-button" type="button">删除</button>
    `;
    row.querySelector('.ip').textContent = item.ip;
    row.querySelector('.delete-button').addEventListener('click', () => deleteIP(item.ip));
    permanentList.append(row);
  }
}

async function refreshInfo() {
  const info = await api('/api/info');
  renderInfo(info);
  showDashboard();
}

async function addCurrentIP(type, button) {
  const ip = targetIPInput.value.trim();
  setBusy(button, true, '授权中...');
  try {
    await api('/api/add', {
      method: 'POST',
      body: JSON.stringify(ip ? { type, ip } : { type }),
    });
    showToast('授权成功');
    await refreshInfo();
  } catch (error) {
    if (error.message !== 'unauthorized') showToast(error.message || '授权失败');
  } finally {
    setBusy(button, false);
  }
}

async function deleteIP(ip) {
  try {
    await api('/api/delete', {
      method: 'POST',
      body: JSON.stringify({ ip }),
    });
    showToast('删除成功');
    await refreshInfo();
  } catch (error) {
    if (error.message !== 'unauthorized') showToast(error.message || '删除失败');
  }
}

authForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  const nextToken = tokenInput.value.trim();
  if (!nextToken) return;

  token = nextToken;
  setBusy(authButton, true, '验证中...');
  try {
    await refreshInfo();
    localStorage.setItem(TOKEN_KEY, token);
    tokenInput.value = '';
  } catch (error) {
    localStorage.removeItem(TOKEN_KEY);
    if (error.message !== 'unauthorized') showToast(error.message || '验证失败');
  } finally {
    setBusy(authButton, false);
  }
});

logoutButton.addEventListener('click', () => {
  localStorage.removeItem(TOKEN_KEY);
  token = '';
  showAuth();
});

tempButton.addEventListener('click', () => addCurrentIP('temp', tempButton));
permButton.addEventListener('click', () => addCurrentIP('perm', permButton));

if (token) {
  refreshInfo().catch((error) => {
    if (error.message !== 'unauthorized') showAuth('网关失去连接，请检查网络');
  });
} else {
  showAuth();
}
