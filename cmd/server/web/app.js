// ── state ─────────────────────────────────────────────────────────────────────
let projects   = [];
let activeProj = null;
let files      = {};     // project -> [{name, size, mod_time}]
let contents   = {};     // "project/file" -> string
let activeFile = null;
let dirty      = false;

// ── api key ───────────────────────────────────────────────────────────────────
let apiKey = localStorage.getItem('envault_api_key') || '';
let _keyResolve = null;

function openKeyModal(msg = '') {
  return new Promise(resolve => {
    _keyResolve = resolve;
    document.getElementById('key-msg').textContent   = msg;
    document.getElementById('key-input').value       = '';
    document.getElementById('key-modal').classList.remove('hidden');
    document.getElementById('key-input').focus();
  });
}
function closeKeyModal(key) {
  document.getElementById('key-modal').classList.add('hidden');
  if (_keyResolve) { _keyResolve(key); _keyResolve = null; }
}

document.getElementById('key-confirm').addEventListener('click', () => {
  const key = document.getElementById('key-input').value.trim();
  if (!key) return;
  apiKey = key;
  localStorage.setItem('envault_api_key', key);
  closeKeyModal(key);
  loadProjects();
});
document.getElementById('key-input').addEventListener('keydown', e => {
  if (e.key === 'Enter') document.getElementById('key-confirm').click();
});
document.getElementById('key-toggle').addEventListener('click', () => {
  const el = document.getElementById('key-input');
  el.type = el.type === 'password' ? 'text' : 'password';
});

// ── api ───────────────────────────────────────────────────────────────────────
const headers = () => ({ 'X-API-Key': apiKey, 'Content-Type': 'text/plain' });

async function serverError(res) {
  try { const b = await res.clone().json(); if (b.error) return b.error; } catch (_) {}
  return res.statusText || String(res.status);
}

async function apiFetch(path, opts = {}) {
  try {
    const res = await fetch('/api' + path, { headers: headers(), ...opts });
    if (res.status === 401) {
      await openKeyModal('Incorrect or missing API key. Please try again.');
      return new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 });
    }
    return res;
  } catch (err) {
    showStatus(`Network error: ${err.message}`, true);
    throw err;
  }
}

async function loadProjects() {
  let res;
  try { res = await apiFetch('/projects'); } catch (_) { return; }
  if (!res.ok) { showStatus(`Failed to load projects: ${await serverError(res)}`, true); return; }
  projects = (await res.json()).projects || [];
  renderSidebar();
}

async function loadFiles(project) {
  let res;
  try { res = await apiFetch(`/projects/${enc(project)}/files`); } catch (_) { files[project] = []; return; }
  files[project] = res.ok ? (await res.json()).files || [] : [];
}

async function loadContent(project, file) {
  const k = `${project}/${file}`;
  if (contents[k] !== undefined) return contents[k];
  let res;
  try { res = await apiFetch(`/projects/${enc(project)}/files/${enc(file)}`); } catch (_) { return ''; }
  return res.ok ? (contents[k] = await res.text()) : '';
}

async function saveFile(project, file, content) {
  let res;
  try { res = await apiFetch(`/projects/${enc(project)}/files/${enc(file)}`, { method: 'PUT', body: content }); } catch (_) { return; }
  if (!res.ok) { showStatus(`Save failed: ${await serverError(res)}`, true); return; }
  contents[`${project}/${file}`] = content;
  await loadFiles(project);
  renderToolbar();
  showStatus('Saved');
}

async function deleteFile(project, file) {
  let res;
  try { res = await apiFetch(`/projects/${enc(project)}/files/${enc(file)}`, { method: 'DELETE' }); } catch (_) { return; }
  if (!res.ok) { showStatus(`Delete failed: ${await serverError(res)}`, true); return; }
  delete contents[`${project}/${file}`];
  await loadFiles(project);
  activeFile = null;
  renderToolbar();
  showStatus('Deleted');
}

async function deleteProject(project) {
  let res;
  try { res = await apiFetch(`/projects/${enc(project)}`, { method: 'DELETE' }); } catch (_) { return; }
  if (!res.ok) { showStatus(`Delete failed: ${await serverError(res)}`, true); return; }
  projects = projects.filter(p => p !== project);
  delete files[project];
  if (activeProj === project) { activeProj = null; activeFile = null; }
  renderSidebar();
  renderToolbar();
  showStatus('Project deleted');
}

// ── render ────────────────────────────────────────────────────────────────────
function renderSidebar() {
  const list = document.getElementById('project-list');
  list.innerHTML = '';
  for (const p of projects) {
    const el = document.createElement('div');
    el.className = 'proj' + (p === activeProj ? ' active' : '');
    el.innerHTML = `<span class="name">${esc(p)}</span><button class="x" title="Delete">✕</button>`;
    el.querySelector('.name').onclick = () => selectProject(p);
    el.querySelector('.x').onclick = async e => {
      e.stopPropagation();
      if (confirm(`Delete project "${p}"?`)) await deleteProject(p);
    };
    list.appendChild(el);
  }
}

async function selectProject(p) {
  if (dirty && !confirm('Unsaved changes. Discard?')) return;
  dirty = false;
  activeProj = p;
  activeFile = null;
  if (!files[p]) await loadFiles(p);
  renderSidebar();
  renderToolbar();
}

function renderToolbar() {
  const bar = document.getElementById('toolbar');
  const editor = document.getElementById('editor');

  if (!activeProj) {
    bar.innerHTML = `<span style="color:var(--muted)">Select a project</span>`;
    editor.value = '';
    editor.disabled = true;
    return;
  }

  const tabs = (files[activeProj] || []).map(f => `
    <button class="tab ${f.name === activeFile ? 'active' : ''}" data-file="${esc(f.name)}">
      ${esc(f.name)}<span class="x" data-del="${esc(f.name)}">✕</span>
    </button>`).join('');

  bar.innerHTML = `
    <span class="proj-name">${esc(activeProj)}</span>
    ${tabs}
    <button class="add-tab" id="add-file">+ file</button>
    <div class="spacer"></div>
    <button class="btn btn-primary" id="save-btn" ${activeFile ? '' : 'disabled'}>Save</button>
    <button class="btn btn-danger"  id="del-btn"  ${activeFile ? '' : 'disabled'}>Delete</button>
  `;

  bar.querySelectorAll('.tab').forEach(btn => {
    const name = btn.dataset.file;
    btn.addEventListener('click', () => selectFile(name));
    btn.querySelector('.x').addEventListener('click', async e => {
      e.stopPropagation();
      if (confirm(`Delete ${name}?`)) await deleteFile(activeProj, name);
    });
  });

  document.getElementById('add-file').addEventListener('click', openFileModal);
  document.getElementById('save-btn').addEventListener('click', async () => {
    if (activeProj && activeFile) {
      await saveFile(activeProj, activeFile, editor.value);
      dirty = false;
    }
  });
  document.getElementById('del-btn').addEventListener('click', async () => {
    if (activeProj && activeFile && confirm(`Delete ${activeFile}?`))
      await deleteFile(activeProj, activeFile);
  });

  editor.disabled = !activeFile;
}

async function selectFile(name) {
  if (dirty && !confirm('Unsaved changes. Discard?')) return;
  dirty = false;
  activeFile = name;
  renderToolbar();
  const content = await loadContent(activeProj, name);
  const editor = document.getElementById('editor');
  editor.value   = content;
  editor.disabled = false;
  editor.focus();
}

// ── file modal ────────────────────────────────────────────────────────────────
function openFileModal() {
  document.getElementById('file-name').value = '.env';
  document.getElementById('file-modal').classList.remove('hidden');
  document.getElementById('file-name').focus();
  document.getElementById('file-name').select();
}
function closeFileModal() {
  document.getElementById('file-modal').classList.add('hidden');
}

document.getElementById('file-cancel').addEventListener('click', closeFileModal);
document.getElementById('file-modal').addEventListener('click', e => { if (e.target === e.currentTarget) closeFileModal(); });
document.getElementById('file-confirm').addEventListener('click', async () => {
  const name = document.getElementById('file-name').value.trim();
  if (!name || !activeProj) { closeFileModal(); return; }
  closeFileModal();
  await saveFile(activeProj, name, '');
  await selectFile(name);
});
document.getElementById('file-name').addEventListener('keydown', e => {
  if (e.key === 'Enter')  document.getElementById('file-confirm').click();
  if (e.key === 'Escape') closeFileModal();
});

// ── new project ───────────────────────────────────────────────────────────────
document.getElementById('new-project').addEventListener('keydown', async e => {
  if (e.key !== 'Enter') return;
  const name = e.target.value.trim();
  if (!name) return;
  e.target.value = '';
  if (!projects.includes(name)) { projects.push(name); }
  renderSidebar();
  selectProject(name);
});

// ── editor dirty tracking + keyboard save ────────────────────────────────────
document.getElementById('editor').addEventListener('input', () => { dirty = true; });
document.addEventListener('keydown', e => {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault();
    document.getElementById('save-btn')?.click();
  }
});

// ── status ────────────────────────────────────────────────────────────────────
let _statusTimer;
function showStatus(msg, isErr = false) {
  const el = document.getElementById('status');
  el.textContent = msg;
  el.className = 'show ' + (isErr ? 'err' : 'ok');
  clearTimeout(_statusTimer);
  _statusTimer = setTimeout(() => { el.className = ''; }, 2500);
}

// ── utils ─────────────────────────────────────────────────────────────────────
const enc = s => encodeURIComponent(s);
const esc = s => s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');

// ── init ──────────────────────────────────────────────────────────────────────
if (apiKey) { loadProjects(); } else { openKeyModal('Enter your API key to connect.'); }
