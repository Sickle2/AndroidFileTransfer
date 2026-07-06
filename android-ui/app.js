let currentPath = '';

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function showError(msg) {
  const list = document.getElementById('file-list');
  const empty = document.getElementById('empty-tip');
  list.innerHTML = '';
  empty.textContent = msg;
  empty.classList.remove('hidden');
}

async function loadFiles(path) {
  currentPath = path;
  updateBreadcrumb(path);
  try {
    const res = await fetch('/api/files?path=' + encodeURIComponent(path));
    if (!res.ok) {
      const errText = await res.text();
      showError(`加载失败: ${res.status} ${errText}`);
      return;
    }
    const files = await res.json();
    renderFiles(files || []);
  } catch (e) {
    showError(`加载失败: ${e.message}`);
  }
}

function updateBreadcrumb(path) {
  const parts = path.split('/').filter(Boolean);
  const crumb = document.getElementById('breadcrumb');
  crumb.innerHTML = '<span data-path="">根目录</span>';
  let built = '';
  parts.forEach(part => {
    built += '/' + part;
    const p = built;
    crumb.innerHTML += ` / <span data-path="${escapeHtml(p)}">${escapeHtml(part)}</span>`;
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
        <div class="file-name"></div>
        <div class="file-meta">${escapeHtml(size)}</div>
      </div>
      ${f.isDir ? '' : `<span class="download-btn">⬇️</span>`}
    `;
    div.querySelector('.file-name').textContent = f.name;
    if (!f.isDir) {
      div.querySelector('.download-btn').dataset.path = f.path;
    }
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

  let successCount = 0;
  let failCount = 0;

  for (let i = 0; i < files.length; i++) {
    const f = files[i];
    const pct = Math.round(((i) / files.length) * 100);
    fill.style.width = pct + '%';
    text.textContent = `上传中 ${i+1}/${files.length}: ${f.name}`;

    const fd = new FormData();
    fd.append('file', f);
    try {
      const res = await fetch('/api/upload?path=' + encodeURIComponent(currentPath), { method: 'POST', body: fd });
      if (res.ok) {
        successCount++;
      } else {
        failCount++;
      }
    } catch (e) {
      failCount++;
    }
  }
  fill.style.width = '100%';
  text.textContent = failCount === 0
    ? `上传完成 ${successCount} 成功`
    : `上传完成 ${successCount} 成功 ${failCount} 失败`;
  setTimeout(() => bar.classList.add('hidden'), 2000);
  this.value = '';
  loadFiles(currentPath);
});

// Start
loadFiles('');
