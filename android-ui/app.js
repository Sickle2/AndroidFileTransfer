// The server exposes a virtual filesystem. Paths look like "/", "/shared/<id>"
// or "/browse/<rel>" — they never contain real Mac paths. Because the
// "/shared/<id>" segments are opaque, we track navigation as a stack of
// {name, path} crumbs so the breadcrumb can show friendly names.
let navStack = [{ name: '共享内容', path: '/' }];

function currentPath() {
  return navStack[navStack.length - 1].path;
}

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

// loadFiles fetches the given virtual path. The navStack must already reflect
// the target location before calling (see navigateInto / navigateToIndex).
async function loadFiles(path) {
  updateBreadcrumb();
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

function navigateInto(file) {
  navStack.push({ name: file.name, path: file.path });
  loadFiles(file.path);
}

function navigateToIndex(index) {
  navStack = navStack.slice(0, index + 1);
  loadFiles(currentPath());
}

// navigateUp goes back one level. No-op at the root.
function navigateUp() {
  if (navStack.length <= 1) return;
  navStack.pop();
  loadFiles(currentPath());
}

function updateBreadcrumb() {
  const crumb = document.getElementById('breadcrumb');
  crumb.innerHTML = '';
  navStack.forEach((entry, i) => {
    if (i > 0) crumb.appendChild(document.createTextNode(' / '));
    const span = document.createElement('span');
    span.textContent = entry.name;
    span.addEventListener('click', () => navigateToIndex(i));
    crumb.appendChild(span);
  });
  // Show the back button only when there is a parent to go to.
  const backBtn = document.getElementById('back-btn');
  backBtn.classList.toggle('hidden', navStack.length <= 1);
}

function renderFiles(files) {
  const list = document.getElementById('file-list');
  const empty = document.getElementById('empty-tip');
  list.innerHTML = '';
  if (files.length === 0) {
    empty.textContent = navStack.length === 1
      ? 'Mac 端还没有共享任何文件'
      : '此目录为空';
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
    if (f.isDir) {
      div.addEventListener('click', () => navigateInto(f));
    } else {
      const btn = div.querySelector('.download-btn');
      btn.dataset.path = f.path;
      btn.addEventListener('click', e => {
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

// Upload. The server ignores the client-supplied path and always stores files
// in the Mac's configured receiving directory, so we don't send a path.
document.getElementById('upload-input').addEventListener('change', async function() {
  const files = Array.from(this.files);
  if (files.length === 0) return;
  const bar = document.getElementById('progress-bar');
  const fill = document.getElementById('progress-fill');
  const text = document.getElementById('progress-text');
  bar.classList.remove('hidden');

  let successCount = 0;
  let failCount = 0;
  let lastError = '';

  for (let i = 0; i < files.length; i++) {
    const f = files[i];
    const pct = Math.round(((i) / files.length) * 100);
    fill.style.width = pct + '%';
    text.textContent = `上传中 ${i+1}/${files.length}: ${f.name}`;

    const fd = new FormData();
    fd.append('file', f);
    try {
      const res = await fetch('/api/upload', { method: 'POST', body: fd });
      if (res.ok) {
        successCount++;
      } else {
        failCount++;
        lastError = await res.text();
      }
    } catch (e) {
      failCount++;
      lastError = e.message;
    }
  }
  fill.style.width = '100%';
  if (failCount === 0) {
    text.textContent = `已上传 ${successCount} 个文件到 Mac 接收目录`;
  } else {
    text.textContent = `完成 ${successCount} 成功 ${failCount} 失败：${lastError}`;
  }
  setTimeout(() => bar.classList.add('hidden'), 3000);
  this.value = '';
  loadFiles(currentPath());
});

// Back button.
document.getElementById('back-btn').addEventListener('click', navigateUp);

// Start at the virtual root.
loadFiles('/');
