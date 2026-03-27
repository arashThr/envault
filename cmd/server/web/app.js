// ── state ─────────────────────────────────────────────────────────────────────
let projects   = [];
let activeProj = null;
let files      = {};   // project -> [{name,size,mod_time}]
let contents   = {};   // "project/file" -> string
let activeFile = null;
let dirty      = false;

// ── api key modal ─────────────────────────────────────────────────────────────
let apiKey = localStorage.getItem('envault_api_key') || '';
let _apiKeyResolve = null; // pending promise resolver when modal is open

function openApiKeyModal(message = '') {
  return new Promise((resolve) => {
    _apiKeyResolve = resolve;
    const overlay = document.getElementById('api-key-modal');
    const input   = document.getElementById('api-key-input');
    const msg     = document.getElementById('api-key-modal-msg');
    msg.textContent = message;
    input.value = '';
    overlay.classList.remove('hidden');
    input.focus();
  });
}

function closeApiKeyModal(key) {
  document.getElementById('api-key-modal').classList.add('hidden');
  if (_apiKeyResolve) { _apiKeyResolve(key); _apiKeyResolve = null; }
}

document.getElementById('api-key-confirm').addEventListener('click', () => {
  const key = document.getElementById('api-key-input').value.trim();
  if (!key) return;
  apiKey = key;
  localStorage.setItem('envault_api_key', key);
  closeApiKeyModal(key);
  loadProjects();
});

document.getElementById('api-key-input').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') document.getElementById('api-key-confirm').click();
});

document.getElementById('api-key-toggle').addEventListener('click', () => {
  const input = document.getElementById('api-key-input');
  input.type = input.type === 'password' ? 'text' : 'password';
});

// ── api ───────────────────────────────────────────────────────────────────────
const headers = () => ({ 'X-API-Key': apiKey, 'Content-Type': 'text/plain' });

async function responseError(res) {
  try {
    const body = await res.clone().json();
    if (body.error) return body.error;
  } catch (_) {}
  return res.statusText || String(res.status);
}

async function apiFetch(path, opts = {}) {
  try {
    const res = await fetch('/api' + path, { headers: headers(), ...opts });
    if (res.status === 401) {
      console.warn('apiFetch 401', path);
      await openApiKeyModal('Incorrect or missing API key. Please try again.');
      return new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 });
    }
    return res;
  } catch (err) {
    showStatus(`Network error: ${err.message}`, true);
    console.error('apiFetch network error', path, err);
    throw err;
  }
}

async function loadProjects() {
  let res;
  try { res = await apiFetch('/projects'); } catch (_) { return; }
  if (!res.ok) {
    const err = await responseError(res);
    showStatus(`Failed to load projects: ${res.status} — ${err}`, true);
    console.error('loadProjects failed', res.status, err);
    return;
  }
  const data = await res.json();
  projects = data.projects || [];
  renderProjects();
}

async function loadFiles(project) {
  let res;
  try { res = await apiFetch(`/projects/${encodeURIComponent(project)}/files`); } catch (_) { files[project] = []; return; }
  if (!res.ok) {
    console.warn('loadFiles failed', project, res.status, await responseError(res));
    files[project] = [];
    return;
  }
  const data = await res.json();
  files[project] = data.files || [];
}

async function loadFileContent(project, file) {
  const key = `${project}/${file}`;
  if (contents[key] !== undefined) return contents[key];
  let res;
  try { res = await apiFetch(`/projects/${encodeURIComponent(project)}/files/${encodeURIComponent(file)}`); } catch (_) { return ''; }
  if (!res.ok) {
    console.warn('loadFileContent failed', project, file, res.status, await responseError(res));
    return '';
  }
  const text = await res.text();
  contents[key] = text;
  return text;
}

async function saveFile(project, file, content) {
  let res;
  try {
    res = await apiFetch(
      `/projects/${encodeURIComponent(project)}/files/${encodeURIComponent(file)}`,
      { method: 'PUT', body: content }
    );
  } catch (_) { return; }
  if (!res.ok) {
    showStatus(`Save failed: ${res.status} — ${await responseError(res)}`, true);
    return;
  }
  contents[`${project}/${file}`] = content;
  await loadFiles(project);
  renderFileTabs();
  showStatus('Saved');
}

async function deleteFile(project, file) {
  let res;
  try { res = await apiFetch(`/projects/${encodeURIComponent(project)}/files/${encodeURIComponent(file)}`, { method: 'DELETE' }); } catch (_) { return; }
  if (!res.ok) {
    showStatus(`Delete failed: ${res.status} — ${await responseError(res)}`, true);
    return;
  }
  delete contents[`${project}/${file}`];
  await loadFiles(project);
  activeFile = null;
  renderFileTabs();
  renderEditor();
  showStatus('Deleted');
}

async function deleteProject(project) {
  let res;
  try { res = await apiFetch(`/projects/${encodeURIComponent(project)}`, { method: 'DELETE' }); } catch (_) { return; }
  if (!res.ok) {
    showStatus(`Delete project failed: ${res.status} — ${await responseError(res)}`, true);
    return;
  }
  projects = projects.filter(p => p !== project);
  delete files[project];
  if (activeProj === project) { activeProj = null; activeFile = null; }
  renderProjects();
  renderProjectView();
  showStatus('Project deleted');
}

// ── render ────────────────────────────────────────────────────────────────────
function renderProjects() {
  const list = document.getElementById('project-list');
  list.innerHTML = '';
  if (projects.length === 0) {
    list.innerHTML = '<div style="padding:10px 12px;color:var(--muted);font-size:12px;">No projects yet</div>';
    return;
  }
  for (const p of projects) {
    const item = document.createElement('div');
    item.className = 'project-item' + (p === activeProj ? ' active' : '');
    item.innerHTML = `
      <span class="icon">📁</span>
      <span class="name">${esc(p)}</span>
      <button class="del-btn" title="Delete project" data-project="${esc(p)}">✕</button>
    `;
    item.querySelector('.name').addEventListener('click', () => selectProject(p));
    item.querySelector('.icon').addEventListener('click', () => selectProject(p));
    item.querySelector('.del-btn').addEventListener('click', async (e) => {
      e.stopPropagation();
      if (confirm(`Delete project "${p}" and all its files?`)) await deleteProject(p);
    });
    list.appendChild(item);
  }
}

async function selectProject(p) {
  if (dirty && activeFile) {
    if (!confirm('You have unsaved changes. Discard them?')) return;
    dirty = false;
  }
  activeProj = p;
  activeFile = null;
  if (!files[p]) await loadFiles(p);
  renderProjects();
  renderProjectView();
}

function renderProjectView() {
  const noProj   = document.getElementById('no-project');
  const projView = document.getElementById('project-view');
  if (!activeProj) {
    noProj.style.display   = 'flex';
    projView.style.display = 'none';
    return;
  }
  noProj.style.display   = 'none';
  projView.style.display = 'flex';
  document.getElementById('proj-label').textContent = activeProj;
  renderFileTabs();
  renderEditor();
}

function renderFileTabs() {
  const tabs     = document.getElementById('file-tabs');
  tabs.innerHTML = '';
  for (const f of (files[activeProj] || [])) {
    const btn = document.createElement('button');
    btn.className = 'file-tab' + (f.name === activeFile ? ' active' : '');
    btn.innerHTML = `${esc(f.name)} <span class="close" data-file="${esc(f.name)}">✕</span>`;
    btn.addEventListener('click', () => selectFile(f.name));
    btn.querySelector('.close').addEventListener('click', async (e) => {
      e.stopPropagation();
      if (confirm(`Delete ${f.name}?`)) await deleteFile(activeProj, f.name);
    });
    tabs.appendChild(btn);
  }
}

async function selectFile(name) {
  if (dirty && activeFile && activeFile !== name) {
    if (!confirm('You have unsaved changes. Discard them?')) return;
    dirty = false;
  }
  activeFile = name;
  const content = await loadFileContent(activeProj, name);
  renderFileTabs();
  renderEditor(content);
}

function renderEditor(content) {
  const noFile  = document.getElementById('no-file');
  const editor  = document.getElementById('editor');
  const saveBtn = document.getElementById('save-btn');
  const delBtn  = document.getElementById('delete-file-btn');

  if (!activeFile) {
    noFile.style.display  = 'flex';
    editor.style.display  = 'none';
    saveBtn.style.display = 'none';
    delBtn.style.display  = 'none';
    return;
  }
  noFile.style.display  = 'none';
  editor.style.display  = 'block';
  saveBtn.style.display = '';
  delBtn.style.display  = '';
  if (content !== undefined) {
    editor.value = content;
    dirty = false;
  }
}

// ── interactions ──────────────────────────────────────────────────────────────
document.getElementById('editor').addEventListener('input', () => { dirty = true; });

document.getElementById('save-btn').addEventListener('click', async () => {
  if (!activeProj || !activeFile) return;
  await saveFile(activeProj, activeFile, document.getElementById('editor').value);
  dirty = false;
});

document.getElementById('delete-file-btn').addEventListener('click', async () => {
  if (!activeProj || !activeFile) return;
  if (confirm(`Delete ${activeFile}?`)) await deleteFile(activeProj, activeFile);
});

document.getElementById('add-file-btn').addEventListener('click', () => openModal());

document.getElementById('new-project-input').addEventListener('keydown', async (e) => {
  if (e.key !== 'Enter') return;
  const name = e.target.value.trim();
  if (!name) return;
  e.target.value = '';
  if (projects.includes(name)) { selectProject(name); return; }
  projects.push(name);
  renderProjects();
  selectProject(name);
});

// ── file modal ────────────────────────────────────────────────────────────────
function openModal() {
  const overlay = document.getElementById('modal');
  const input   = document.getElementById('modal-input');
  overlay.classList.remove('hidden');
  input.value = '.env';
  input.focus();
  input.select();
}

function closeModal() {
  document.getElementById('modal').classList.add('hidden');
}

document.getElementById('modal-cancel').addEventListener('click', closeModal);
document.getElementById('modal').addEventListener('click', (e) => {
  if (e.target === e.currentTarget) closeModal();
});

document.getElementById('modal-confirm').addEventListener('click', async () => {
  const name = document.getElementById('modal-input').value.trim();
  if (!name || !activeProj) { closeModal(); return; }
  closeModal();
  await saveFile(activeProj, name, '');
  await loadFiles(activeProj);
  renderFileTabs();
  await selectFile(name);
});

document.getElementById('modal-input').addEventListener('keydown', (e) => {
  if (e.key === 'Enter')  document.getElementById('modal-confirm').click();
  if (e.key === 'Escape') closeModal();
});

// keyboard save
document.addEventListener('keydown', (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault();
    document.getElementById('save-btn').click();
  }
});

// ── status ────────────────────────────────────────────────────────────────────
let statusTimer;
function showStatus(msg, isErr = false) {
  const el = document.getElementById('status-msg');
  el.textContent = msg;
  el.className = 'show ' + (isErr ? 'err' : 'ok');
  clearTimeout(statusTimer);
  statusTimer = setTimeout(() => { el.className = ''; }, 2500);
}

// ── utils ─────────────────────────────────────────────────────────────────────
function esc(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// ── init ──────────────────────────────────────────────────────────────────────
if (apiKey) {
  loadProjects();
} else {
  openApiKeyModal('Enter your API key to connect to the server.');
}
