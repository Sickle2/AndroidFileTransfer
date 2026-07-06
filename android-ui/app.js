let currentPath = '/';

async function loadFiles(path) {
  currentPath = path;
  updateBreadcrumb(path);
  const res = await fetch('/api/files?path=' + encodeURIComponent(path));
  const files = await res.json();
  renderFiles(files || []);
}

function updateBreadcrumb(path) {
  const parts = path.split('/').filter(Boolean);
  const crumb = document.getElementById('breadcrumb');
  crumb.innerHTML = '<span data-path="/">根目录</span>';
  let built = '';
  parts.forEach(part => {
    built += '/' + part;
    const p = built;
    crumb.innerHTML += ` / <span data-path="${p}">${part}</span>`;
  });
  crumb.querySelectorAll('span').forEach(s => {
    s.addEventListener('click', () => loadFiles(s.dataset.path));
  });
}

function renderFiles(files) {
  const list = document.getElementById('file-list');
  const empty = document.getElementById('empty-tip');
  list.innerHTML = '';
  if (files.length === 0) {
    empty.classList.remove('hidden');
    return;
  }
  empty.classList.add('hidden');
  files.forEach(f => {
    const div = document.createElement('div');
    div.className = 'file-item';
    const icon = f.isDir ? '📁' : getFileIcon(f.name);
    const size = f.isDir ? '' : formatSize(f.size);
    div.innerHTML = `
      <span class="file-icon">${icon}</span>
      <div class="file-info">
        <div class="file-name">${f.name}</div>
        <div class="file-meta">${size}</div>
      </div>
      ${f.isDir ? '' : `<span class="download-btn" data-path="${f.path}">⬇️</span>`}
    `;
    if (f.isDir) {
      div.addEventListener('click', () => loadFiles(f.path));
    } else {
      div.querySelector('.download-btn').addEventListener('click', e => {
        e.stopPropagation();
        window.location.href = '/api/download?path=' + encodeURIComponent(f.path);
      });
    }
    list.appendChild(div);
  });
}

function getFileIcon(name) {
  const ext = name.split('.').pop().toLowerCase();
  const icons = { jpg:'🖼️', jpeg:'🖼️', png:'🖼️', gif:'🖼️', mp4:'🎬', mov:'🎬',
    mp3:'🎵', wav:'🎵', pdf:'📄', zip:'📦', txt:'📝', doc:'📝', docx:'📝' };
  return icons[ext] || '📄';
}

function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / 1024 / 1024).toFixed(1) + ' MB';
}

// Upload
document.getElementById('upload-input').addEventListener('change', async function() {
  const files = Array.from(this.files);
  if (files.length === 0) return;
  const bar = document.getElementById('progress-bar');
  const fill = document.getElementById('progress-fill');
  const text = document.getElementById('progress-text');
  bar.classList.remove('hidden');

  for (let i = 0; i < files.length; i++) {
    const f = files[i];
    const pct = Math.round(((i) / files.length) * 100);
    fill.style.width = pct + '%';
    text.textContent = `上传中 ${i+1}/${files.length}: ${f.name}`;

    const fd = new FormData();
    fd.append('file', f);
    await fetch('/api/upload?path=' + encodeURIComponent(currentPath), { method: 'POST', body: fd });
  }
  fill.style.width = '100%';
  text.textContent = '上传完成';
  setTimeout(() => bar.classList.add('hidden'), 2000);
  this.value = '';
  loadFiles(currentPath);
});

// Start
loadFiles('/Users');
