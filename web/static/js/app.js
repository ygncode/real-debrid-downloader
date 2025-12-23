// Modal Management
function openAddModal() {
    document.getElementById('add-modal').classList.add('active');
    document.getElementById('magnet-input').focus();
}

function closeAddModal(event) {
    if (event && event.target !== event.currentTarget) return;
    document.getElementById('add-modal').classList.remove('active');
    // Reset forms
    document.getElementById('magnet-form').reset();
    document.getElementById('file-form').reset();
    document.getElementById('selected-file-name').textContent = '';
    document.querySelectorAll('.btn-submit').forEach(btn => btn.classList.remove('loading'));
}

function openFileSelection(downloadId) {
    const modal = document.getElementById('file-select-modal');
    const content = document.getElementById('file-select-content');

    modal.classList.add('active');
    content.innerHTML = '<div class="empty-state"><div class="spinner"></div><p>Loading files...</p></div>';

    fetch(`/api/downloads/${downloadId}/files`)
        .then(response => response.text())
        .then(html => {
            content.innerHTML = html;
            updateSelectedCount();
        })
        .catch(error => {
            content.innerHTML = `<div class="error-state"><p>Error loading files: ${error.message}</p></div>`;
        });
}

function closeFileSelectModal(event) {
    if (event && event.target !== event.currentTarget) return;
    document.getElementById('file-select-modal').classList.remove('active');
}

// Tab Switching
function switchTab(tabName) {
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.tab === tabName);
    });
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.toggle('active', content.id === `tab-${tabName}`);
    });
}

// Form Submissions
async function submitMagnet(event) {
    event.preventDefault();
    const form = event.target;
    const btn = form.querySelector('.btn-submit');
    const magnetInput = document.getElementById('magnet-input');
    const downloadSubs = document.getElementById('magnet-subs')?.checked ?? true;

    btn.classList.add('loading');
    btn.disabled = true;

    try {
        const response = await fetch('/api/torrents/magnet', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ magnet: magnetInput.value, download_subs: downloadSubs })
        });

        const data = await response.json();

        if (!response.ok) {
            throw new Error(data.error || 'Failed to add magnet');
        }

        closeAddModal();
        refreshDownloads();
    } catch (error) {
        alert('Error: ' + error.message);
    } finally {
        btn.classList.remove('loading');
        btn.disabled = false;
    }
}

async function submitTorrentFile(event) {
    event.preventDefault();
    const form = event.target;
    const btn = form.querySelector('.btn-submit');
    const fileInput = document.getElementById('torrent-file');
    const downloadSubs = document.getElementById('file-subs')?.checked ?? true;

    if (!fileInput.files.length) {
        alert('Please select a torrent file');
        return;
    }

    btn.classList.add('loading');
    btn.disabled = true;

    const formData = new FormData();
    formData.append('torrent', fileInput.files[0]);
    formData.append('download_subs', downloadSubs ? 'true' : 'false');

    try {
        const response = await fetch('/api/torrents/file', {
            method: 'POST',
            body: formData
        });

        const data = await response.json();

        if (!response.ok) {
            throw new Error(data.error || 'Failed to add torrent');
        }

        closeAddModal();
        refreshDownloads();
    } catch (error) {
        alert('Error: ' + error.message);
    } finally {
        btn.classList.remove('loading');
        btn.disabled = false;
    }
}

async function submitFileSelection(event, downloadId) {
    event.preventDefault();
    const form = event.target;
    const btn = form.querySelector('.btn-submit');

    const checkboxes = form.querySelectorAll('input[name="files"]:checked');
    if (checkboxes.length === 0) {
        alert('Please select at least one file');
        return;
    }

    const fileIds = Array.from(checkboxes).map(cb => cb.value).join(',');

    btn.classList.add('loading');
    btn.disabled = true;

    try {
        const response = await fetch(`/api/downloads/${downloadId}/select`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ file_ids: fileIds })
        });

        const data = await response.json();

        if (!response.ok) {
            throw new Error(data.error || 'Failed to select files');
        }

        closeFileSelectModal();
        refreshDownloads();
    } catch (error) {
        alert('Error: ' + error.message);
    } finally {
        btn.classList.remove('loading');
        btn.disabled = false;
    }
}

async function deleteDownload(id) {
    if (!confirm('Are you sure you want to remove this download?')) {
        return;
    }

    try {
        const response = await fetch(`/api/downloads/${id}`, {
            method: 'DELETE'
        });

        if (!response.ok) {
            const data = await response.json();
            throw new Error(data.error || 'Failed to delete download');
        }

        const item = document.getElementById(`download-${id}`);
        if (item) {
            item.style.opacity = '0';
            item.style.transform = 'translateX(20px)';
            setTimeout(() => item.remove(), 300);
        }
    } catch (error) {
        alert('Error: ' + error.message);
    }
}

// File Selection Helpers
function toggleAllFiles(checkbox) {
    const fileCheckboxes = document.querySelectorAll('.file-checkbox');
    fileCheckboxes.forEach(cb => cb.checked = checkbox.checked);
    updateSelectedCount();
}

function updateSelectedCount() {
    const checkboxes = document.querySelectorAll('.file-checkbox');
    const checked = document.querySelectorAll('.file-checkbox:checked');
    const countEl = document.querySelector('.selected-count');
    if (countEl) {
        countEl.textContent = `${checked.length} of ${checkboxes.length} files selected`;
    }
}

// Refresh downloads list
function refreshDownloads() {
    fetch('/api/downloads')
        .then(response => response.text())
        .then(html => {
            document.getElementById('downloads-list').innerHTML = html;
        });
}

// Delete file from collection
async function deleteFile(path) {
    const fileName = path.split('/').pop();
    if (!confirm(`Are you sure you want to delete "${fileName}"?\n\nThis action cannot be undone.`)) {
        return;
    }

    try {
        const response = await fetch('/api/movies', {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: path })
        });

        if (!response.ok) {
            const data = await response.json();
            throw new Error(data.error || 'Failed to delete file');
        }

        // Refresh the movies list
        refreshMovies();
    } catch (error) {
        alert('Error: ' + error.message);
    }
}

// SSE Event Handling
document.addEventListener('DOMContentLoaded', function() {
    // File input change handler
    const fileInput = document.getElementById('torrent-file');
    if (fileInput) {
        fileInput.addEventListener('change', function() {
            const fileName = this.files[0]?.name || '';
            document.getElementById('selected-file-name').textContent = fileName;
        });
    }

    // File drop zone
    const dropZone = document.getElementById('file-drop');
    if (dropZone) {
        dropZone.addEventListener('dragover', (e) => {
            e.preventDefault();
            dropZone.classList.add('dragover');
        });

        dropZone.addEventListener('dragleave', () => {
            dropZone.classList.remove('dragover');
        });

        dropZone.addEventListener('drop', () => {
            dropZone.classList.remove('dragover');
        });
    }

    // Close modals on escape
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') {
            closeAddModal();
            closeFileSelectModal();
        }
    });

    // Delegate click for file checkboxes
    document.addEventListener('change', function(e) {
        if (e.target.classList.contains('file-checkbox')) {
            updateSelectedCount();

            // Update select all checkbox
            const allCheckboxes = document.querySelectorAll('.file-checkbox');
            const allChecked = document.querySelectorAll('.file-checkbox:checked');
            const selectAll = document.getElementById('select-all-files');
            if (selectAll) {
                selectAll.checked = allCheckboxes.length === allChecked.length;
                selectAll.indeterminate = allChecked.length > 0 && allChecked.length < allCheckboxes.length;
            }
        }
    });
});

// SSE Updates
function setupSSE() {
    const evtSource = new EventSource('/api/downloads/stream');

    evtSource.addEventListener('download', function(event) {
        const download = JSON.parse(event.data);
        updateDownloadItem(download);
    });

    evtSource.addEventListener('refresh-movies', function(event) {
        // Refresh the movies collection when a download completes
        refreshMovies();
    });

    evtSource.onerror = function() {
        console.log('SSE connection error, will retry...');
    };
}

// Refresh movies list
function refreshMovies() {
    fetch('/api/movies')
        .then(response => response.text())
        .then(html => {
            document.getElementById('movies-list').innerHTML = html;
            // Update the count in the header
            const countEl = document.querySelector('.movies-panel .panel-count');
            if (countEl) {
                const movieItems = document.querySelectorAll('#movies-list .movie-item');
                countEl.textContent = `${movieItems.length} titles`;
            }
        });
}

function updateDownloadItem(download) {
    const existingItem = document.getElementById(`download-${download.id}`);

    if (existingItem) {
        // Update status
        existingItem.dataset.status = download.status;

        // Update status text
        const statusText = existingItem.querySelector('.download-status-text');
        if (statusText) {
            statusText.textContent = getStatusText(download);
        }

        // Update progress bar
        const progressFill = existingItem.querySelector('.progress-fill');
        if (progressFill) {
            progressFill.style.width = `${download.progress}%`;
        }

        // Update name if changed
        const nameEl = existingItem.querySelector('.download-name');
        if (nameEl && download.name) {
            nameEl.textContent = download.name;
        }

        // Refresh the full list if status changed significantly
        if (download.status === 'awaiting_selection' ||
            download.status === 'subtitles' ||
            download.status === 'complete' ||
            download.status === 'error') {
            refreshDownloads();
        }
    } else {
        // New download, refresh list
        refreshDownloads();
    }
}

function getStatusText(download) {
    switch (download.status) {
        case 'pending':
            return 'Processing magnet...';
        case 'awaiting_selection':
            return 'Select files to download';
        case 'processing':
            return `Downloading on Real-Debrid (${download.progress.toFixed(1)}%)`;
        case 'downloading':
            return `Downloading to disk (${download.progress.toFixed(1)}%)`;
        case 'subtitles':
            return download.subtitle_status || 'Downloading subtitles...';
        case 'complete':
            let text = 'Complete';
            if (download.subtitle_status) {
                text += ` · Subs: ${download.subtitle_status}`;
            }
            return text;
        case 'error':
            return download.error_message || 'Error';
        default:
            return download.status;
    }
}

// Initialize SSE on page load
document.addEventListener('DOMContentLoaded', setupSSE);
