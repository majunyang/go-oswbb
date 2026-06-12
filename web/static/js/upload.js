// Upload page logic

document.addEventListener('DOMContentLoaded', function() {
    const uploadZone = document.getElementById('uploadZone');
    const fileInput = document.getElementById('fileInput');
    const uploadProgress = document.getElementById('uploadProgress');
    const progressBar = document.getElementById('progressBar');
    const uploadStatus = document.getElementById('uploadStatus');
    const uploadResult = document.getElementById('uploadResult');

    // Drag and drop
    ['dragenter', 'dragover'].forEach(event => {
        uploadZone.addEventListener(event, function(e) {
            e.preventDefault();
            uploadZone.classList.add('dragover');
        });
    });

    ['dragleave', 'drop'].forEach(event => {
        uploadZone.addEventListener(event, function(e) {
            e.preventDefault();
            uploadZone.classList.remove('dragover');
        });
    });

    uploadZone.addEventListener('drop', function(e) {
        const files = e.dataTransfer.files;
        if (files.length > 0) handleUpload(files[0]);
    });

    fileInput.addEventListener('change', function() {
        if (this.files.length > 0) handleUpload(this.files[0]);
    });

    // Load statistics on page load
    loadStatistics();
});

function handleUpload(file) {
    const uploadZone = document.getElementById('uploadZone');
    const uploadProgress = document.getElementById('uploadProgress');
    const progressBar = document.getElementById('progressBar');
    const uploadStatus = document.getElementById('uploadStatus');
    const uploadResult = document.getElementById('uploadResult');

    // Validate file type
    const name = file.name.toLowerCase();
    if (!name.endsWith('.tar') && !name.endsWith('.tar.gz') && !name.endsWith('.tgz')) {
        alert('只支持 .tar, .tar.gz, .tgz 格式的文件');
        return;
    }

    uploadZone.style.display = 'none';
    uploadProgress.style.display = 'block';
    uploadResult.style.display = 'none';

    const formData = new FormData();
    formData.append('archive', file);

    const xhr = new XMLHttpRequest();

    xhr.upload.addEventListener('progress', function(e) {
        if (e.lengthComputable) {
            const percent = Math.round((e.loaded / e.total) * 100);
            progressBar.style.width = percent + '%';
            progressBar.textContent = percent + '%';
            uploadStatus.textContent = '上传中... ' + formatFileSize(e.loaded) + ' / ' + formatFileSize(e.total);
        }
    });

    xhr.addEventListener('load', function() {
        if (xhr.status === 200) {
            const response = JSON.parse(xhr.responseText);
            if (response.success) {
                progressBar.style.width = '100%';
                progressBar.textContent = '100%';
                progressBar.classList.remove('progress-bar-animated');
                progressBar.classList.add('bg-success');
                uploadStatus.textContent = '处理完成!';

                showUploadResult(response.data);
                loadStatistics();
                loadSidebarNav();
            } else {
                showUploadError(response.message);
            }
        } else {
            showUploadError('上传失败: HTTP ' + xhr.status);
        }
    });

    xhr.addEventListener('error', function() {
        showUploadError('网络错误');
    });

    xhr.open('POST', '/api/upload/archive');
    xhr.send(formData);
}

function showUploadResult(data) {
    const uploadResult = document.getElementById('uploadResult');
    let html = '<div class="alert alert-success mt-3">';
    html += '<h6><i class="fas fa-check-circle"></i> 上传并导入成功</h6>';
    html += '<p class="mb-1"><strong>文件:</strong> ' + escapeHtml(data.originalFile) + '</p>';
    html += '<p class="mb-1"><strong>大小:</strong> ' + formatFileSize(data.fileSize) + '</p>';

    if (data.importSummary) {
        html += '<hr><p class="mb-1"><strong>导入摘要:</strong></p>';
        html += '<ul class="mb-0">';
        for (const [name, info] of Object.entries(data.importSummary)) {
            html += '<li>' + escapeHtml(name) + ': ' + info.files + ' 个文件, ' + formatCount(info.lines) + ' 行</li>';
        }
        html += '</ul>';
    }

    html += '</div>';
    html += '<button class="btn btn-primary btn-sm" onclick="resetUpload()"><i class="fas fa-upload"></i> 继续上传</button>';
    uploadResult.innerHTML = html;
    uploadResult.style.display = 'block';
}

function showUploadError(message) {
    const uploadResult = document.getElementById('uploadResult');
    uploadResult.innerHTML = '<div class="alert alert-danger mt-3"><i class="fas fa-exclamation-circle"></i> ' + escapeHtml(message) + '</div>';
    uploadResult.style.display = 'block';

    const progressBar = document.getElementById('progressBar');
    progressBar.classList.remove('progress-bar-animated');
    progressBar.classList.add('bg-danger');
}

function resetUpload() {
    document.getElementById('uploadZone').style.display = 'block';
    document.getElementById('uploadProgress').style.display = 'none';
    document.getElementById('uploadResult').style.display = 'none';
    document.getElementById('fileInput').value = '';

    const progressBar = document.getElementById('progressBar');
    progressBar.style.width = '0%';
    progressBar.textContent = '0%';
    progressBar.classList.add('progress-bar-animated');
    progressBar.classList.remove('bg-success', 'bg-danger');
}

function loadStatistics() {
    fetch('/api/statistics')
        .then(r => r.json())
        .then(data => {
            if (data.success && data.data.hasData) {
                renderStats(data.data.tables);
                renderDataTable(data.data.tables);
            }
        })
        .catch(() => {});
}

function renderStats(tables) {
    const body = document.getElementById('statsBody');
    let totalRecords = 0;
    let html = '';

    for (const [name, count] of Object.entries(tables)) {
        totalRecords += count;
        const icon = getTableIcon(name);
        const displayName = getTableDisplayName(name);

        html += '<div class="stat-item">';
        html += '  <div class="d-flex align-items-center gap-2">';
        html += '    <div class="stat-icon" style="background:linear-gradient(135deg,#667eea,#764ba2)"><i class="' + icon + '"></i></div>';
        html += '    <span>' + escapeHtml(displayName) + '</span>';
        html += '  </div>';
        html += '  <span class="stat-count">' + formatCount(count) + '</span>';
        html += '</div>';
    }

    body.innerHTML = html;
}

function renderDataTable(tables) {
    const section = document.getElementById('dataTables');
    const tbody = document.getElementById('dataTableBody');

    if (Object.keys(tables).length === 0) {
        section.style.display = 'none';
        return;
    }

    section.style.display = 'block';
    let html = '';

    for (const [name, count] of Object.entries(tables)) {
        const icon = getTableIcon(name);
        const displayName = getTableDisplayName(name);
        const tableType = getTableType(name);

        html += '<tr>';
        html += '<td><i class="' + icon + ' me-2"></i>' + escapeHtml(displayName) + '</td>';
        html += '<td><code>' + escapeHtml(name) + '</code></td>';
        html += '<td>' + formatCount(count) + '</td>';
        html += '<td>';

        if (tableType) {
            html += '<a href="/viewer?table=' + encodeURIComponent(name) + '" class="btn btn-sm btn-outline-primary me-1"><i class="fas fa-chart-line"></i> 图表</a>';
        }
        html += '<a href="/viewer?table=' + encodeURIComponent(name) + '" class="btn btn-sm btn-outline-secondary"><i class="fas fa-table"></i> 数据</a>';
        html += '</td>';
        html += '</tr>';
    }

    tbody.innerHTML = html;
}
