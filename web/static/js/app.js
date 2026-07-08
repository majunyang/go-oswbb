// Shared utility functions

function formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i];
}

function formatCount(count) {
    if (count >= 1000000) return (count / 1000000).toFixed(1) + 'M';
    if (count >= 1000) return (count / 1000).toFixed(1) + 'K';
    return count.toString();
}

function getTableIcon(table) {
    const icons = {
        'vmstat': 'fas fa-microchip',
        'iostat': 'fas fa-hdd',
        'ifconfig': 'fas fa-network-wired',
        'mpstat': 'fas fa-microchip',
        'top': 'fas fa-tasks',
        'meminfo': 'fas fa-memory',
        'netstat': 'fas fa-ethernet',
        'ps': 'fas fa-list'
    };
    for (const [key, icon] of Object.entries(icons)) {
        if (table.includes(key)) return icon;
    }
    return 'fas fa-database';
}

function getTableDisplayName(table) {
    const names = {
        'oswvmstat': 'VMstat (虚拟内存)',
        'oswiostat': 'IOstat (磁盘I/O)',
        'oswifconfig': 'Ifconfig (网络接口)',
        'oswmpstat': 'MPstat (多处理器)',
        'oswtop': 'Top (进程)',
        'oswmeminfo': 'MemInfo (内存)',
        'oswnetstat': 'NetStat (网络)',
        'oswps': 'PS (进程)'
    };
    return names[table] || table;
}

function getTableType(table) {
    const types = {
        'oswvmstat': 'vmstat',
        'oswiostat': 'iostat',
        'oswifconfig': 'ifconfig',
        'oswmpstat': 'mpstat',
        'oswmeminfo': 'meminfo',
        'oswtop': 'top'
    };
    for (const [key, type] of Object.entries(types)) {
        if (table.includes(key)) return type;
    }
    return '';
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Sidebar toggle
document.addEventListener('DOMContentLoaded', function() {
    const toggle = document.getElementById('sidebarToggle');
    const sidebar = document.getElementById('sidebar');
    if (toggle && sidebar) {
        toggle.addEventListener('click', function() {
            sidebar.classList.toggle('collapsed');
        });
    }
});

// Load sidebar navigation
function loadSidebarNav() {
    fetch('/api/statistics')
        .then(r => r.json())
        .then(data => {
            if (data.success && data.data.hasData) {
                const tables = data.data.tables;
                const navItems = {
                    'vmstat': 'nav-data',
                    'iostat': 'nav-iostat-link',
                    'ifconfig': 'nav-ifconfig-link',
                    'mpstat': 'nav-mpstat-link',
                    'meminfo': 'nav-meminfo-link',
                    'top': 'nav-top-link',
                    'ps': 'nav-ps-link'
                };

                for (const [type, navId] of Object.entries(navItems)) {
                    for (const tableName of Object.keys(tables)) {
                        if (tableName.includes('osw' + type)) {
                            const el = document.getElementById(navId);
                            if (el) {
                                el.style.display = 'block';
                                const link = el.querySelector('a');
                                if (link) {
                                    link.href = '/viewer?table=' + encodeURIComponent(tableName);
                                }
                            }
                            break;
                        }
                    }
                }
            }
        })
        .catch(() => {});
}

// Initialize sidebar on page load
document.addEventListener('DOMContentLoaded', loadSidebarNav);

// ---- Archive-mode refresh (manual + auto every 30s). The bar is rendered
// server-side only in -archive mode, so here we just wire up the controls. ----
(function initArchiveRefresh() {
    const bar = document.getElementById('archiveRefreshBar');
    const manualBtn = document.getElementById('manualRefreshBtn');
    const autoChk = document.getElementById('autoRefreshChk');
    const statusEl = document.getElementById('refreshStatus');
    if (!bar || !manualBtn || !autoChk) return; // not in archive mode

    let timer = null;

    function setStatus(msg, isError) {
        if (!statusEl) return;
        statusEl.textContent = msg || '';
        statusEl.className = 'arb-status ' + (isError ? 'text-danger' : 'text-muted');
    }

    function doRefresh() {
        setStatus('刷新中...');
        manualBtn.disabled = true;
        fetch('/api/refresh', { method: 'POST' })
            .then(r => r.json())
            .then(d => {
                if (d && d.success) {
                    // Reload so every page (sidebar, stats, charts) shows fresh data
                    window.location.reload();
                } else {
                    setStatus('刷新失败: ' + ((d && d.message) || '未知错误'), true);
                    manualBtn.disabled = false;
                }
            })
            .catch(e => {
                setStatus('刷新请求失败: ' + e, true);
                manualBtn.disabled = false;
            });
    }

    function startAuto(intervalMs) {
        stopAuto();
        timer = setInterval(doRefresh, intervalMs);
        localStorage.setItem('oswbb_auto_refresh', String(intervalMs));
    }

    function stopAuto() {
        if (timer) { clearInterval(timer); timer = null; }
        localStorage.setItem('oswbb_auto_refresh', '0');
    }

    manualBtn.addEventListener('click', doRefresh);
    autoChk.addEventListener('change', function () {
        if (autoChk.checked) {
            startAuto(30000);
            setStatus('已开启自动刷新');
        } else {
            stopAuto();
            setStatus('已关闭自动刷新');
        }
    });

    // Restore the auto-refresh preference (survives the 30s reload)
    const saved = parseInt(localStorage.getItem('oswbb_auto_refresh') || '0', 10);
    if (saved > 0) {
        autoChk.checked = true;
        startAuto(saved);
        setStatus('自动刷新已开启');
    }
})();
