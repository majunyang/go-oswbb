// Viewer page logic - ECharts rendering

const COLORS = ['#667eea', '#ea6a7e', '#7eea66', '#eac866', '#66eac8', '#c866ea', '#ea8a66', '#66c8ea'];
let currentCharts = [];
let allChartData = {}; // Store all fetched chart data for time range filtering

document.addEventListener('DOMContentLoaded', function() {
    // Set page title
    document.getElementById('pageTitle').innerHTML = '<i class="fas fa-chart-line"></i> ' + escapeHtml(TABLE_NAME);

    // Initialize based on table type
    if (TABLE_TYPE === 'vmstat') {
        loadVmstatCharts();
    } else if (TABLE_TYPE === 'iostat') {
        initSingleChart('iostat');
    } else if (TABLE_TYPE === 'ifconfig') {
        initSingleChart('ifconfig');
    } else if (TABLE_TYPE === 'mpstat') {
        initSingleChart('mpstat');
    } else if (TABLE_TYPE === 'meminfo') {
        loadMeminfoCharts();
    } else if (TABLE_TYPE === 'top') {
        loadTopCharts();
    }

    // Time range filter buttons
    document.getElementById('applyTimeRange').addEventListener('click', function() {
        applyTimeRangeFilter();
    });
    document.getElementById('resetTimeRange').addEventListener('click', function() {
        resetTimeRangeFilter();
    });

    // Handle window resize for charts
    window.addEventListener('resize', function() {
        currentCharts.forEach(chart => {
            if (chart && !chart.isDisposed()) chart.resize();
        });
    });
});

// Extract min/max timestamps from chart data categories
function extractTimeRange(chartDataList) {
    let allTimestamps = [];
    chartDataList.forEach(data => {
        if (data && data.categories) {
            allTimestamps = allTimestamps.concat(data.categories);
        }
    });
    if (allTimestamps.length === 0) return null;
    allTimestamps.sort();
    return {
        min: allTimestamps[0],
        max: allTimestamps[allTimestamps.length - 1]
    };
}

// Set default time range inputs
function setDefaultTimeRange(chartDataList) {
    const range = extractTimeRange(chartDataList);
    if (!range) return;

    const startInput = document.getElementById('chartStartTime');
    const endInput = document.getElementById('chartEndTime');

    // Convert "MM-DD HH:mm" format to datetime-local format "YYYY-MM-DDTHH:mm"
    const year = new Date().getFullYear();
    startInput.value = convertToDatetimeLocal(range.min, year);
    endInput.value = convertToDatetimeLocal(range.max, year);
}

// Convert "MM-DD HH:mm" to "YYYY-MM-DDTHH:mm" for datetime-local input
function convertToDatetimeLocal(str, year) {
    if (!str) return '';
    // If already in datetime-local format
    if (str.includes('T')) return str;
    // Parse "MM-DD HH:mm" format
    const parts = str.match(/(\d{2})-(\d{2})\s+(\d{2}):(\d{2})/);
    if (parts) {
        return year + '-' + parts[1] + '-' + parts[2] + 'T' + parts[3] + ':' + parts[4];
    }
    return str;
}

// Filter chart data by time range
function filterChartData(chartData, startTime, endTime) {
    if (!chartData || !chartData.categories) return chartData;

    const startStr = convertToDatetimeLocal(startTime, new Date().getFullYear());
    const endStr = convertToDatetimeLocal(endTime, new Date().getFullYear());

    if (!startStr && !endStr) return chartData;

    const filteredCategories = [];
    const filteredIndices = [];

    chartData.categories.forEach((cat, i) => {
        const catDatetime = convertToDatetimeLocal(cat, new Date().getFullYear());
        let inRange = true;

        if (startStr && catDatetime < startStr) inRange = false;
        if (endStr && catDatetime > endStr) inRange = false;

        if (inRange) {
            filteredCategories.push(cat);
            filteredIndices.push(i);
        }
    });

    if (filteredIndices.length === 0) {
        return { series: [], categories: [] };
    }

    const filteredSeries = chartData.series.map(s => ({
        name: s.name,
        data: filteredIndices.map(i => s.data[i])
    }));

    return {
        series: filteredSeries,
        categories: filteredCategories,
        interfaces: chartData.interfaces,
        devices: chartData.devices,
        cpus: chartData.cpus
    };
}

// Apply time range filter to all charts
function applyTimeRangeFilter() {
    const startTime = document.getElementById('chartStartTime').value;
    const endTime = document.getElementById('chartEndTime').value;

    // Re-render all stored chart data with time range filter
    Object.keys(allChartData).forEach(key => {
        const parts = key.split('|');
        const chartType = parts[0];
        const chartId = parts[1];

        const filtered = filterChartData(allChartData[key], startTime, endTime);

        if (chartType === 'bar') {
            renderBarChart(filtered, chartId);
        } else {
            renderEChart(filtered, chartId);
        }
    });
}

// Reset time range filter
function resetTimeRangeFilter() {
    document.getElementById('chartStartTime').value = '';
    document.getElementById('chartEndTime').value = '';

    // Re-render all charts with full data
    Object.keys(allChartData).forEach(key => {
        const parts = key.split('|');
        const chartType = parts[0];
        const chartId = parts[1];

        if (chartType === 'bar') {
            renderBarChart(allChartData[key], chartId);
        } else {
            renderEChart(allChartData[key], chartId);
        }
    });
}

// ECharts rendering
function renderEChart(chartData, containerId) {
    const container = document.getElementById(containerId);
    if (!container) return null;

    let chart = echarts.getInstanceByDom(container);
    if (chart) chart.dispose();
    chart = echarts.init(container);
    currentCharts.push(chart);

    if (!chartData || !chartData.series || chartData.series.length === 0) {
        chart.setOption({
            title: { text: '暂无数据', left: 'center', top: 'center', textStyle: { color: '#999', fontSize: 14 } }
        });
        return chart;
    }

    const option = {
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'cross' }
        },
        legend: {
            top: 0,
            textStyle: { fontSize: 10 },
            type: 'scroll'
        },
        grid: {
            left: 60,
            right: 20,
            top: 40,
            bottom: 40
        },
        xAxis: {
            type: 'category',
            data: chartData.categories,
            axisLabel: { fontSize: 9, rotate: 30 },
            axisTick: { show: false }
        },
        yAxis: {
            type: 'value',
            axisLabel: { fontSize: 9 },
            splitLine: { lineStyle: { type: 'dashed' } }
        },
        dataZoom: [
            { type: 'inside', start: 0, end: 100 },
            { type: 'slider', start: 0, end: 100, height: 20, bottom: 5 }
        ],
        series: chartData.series.map((s, i) => ({
            name: s.name,
            type: 'line',
            data: s.data,
            smooth: true,
            showSymbol: false,
            lineStyle: { width: 1.5 },
            color: COLORS[i % COLORS.length],
            emphasis: { lineStyle: { width: 3 } }
        }))
    };

    chart.setOption(option);
    return chart;
}

// VMstat charts
function loadVmstatCharts() {
    document.getElementById('vmstatSection').style.display = 'block';
    document.getElementById('meminfoSection').style.display = 'none';
    document.getElementById('topSection').style.display = 'none';
    document.getElementById('singleChartSection').style.display = 'none';

    const metrics = ['cpu', 'procs', 'memory', 'swap', 'io', 'system'];
    const chartDataList = [];

    metrics.forEach(metric => {
        fetch('/api/chart/vmstat/' + metric)
            .then(r => r.json())
            .then(data => {
                if (data.success) {
                    allChartData['line|chart-' + metric] = data.data;
                    chartDataList.push(data.data);
                    renderEChart(data.data, 'chart-' + metric);

                    // Set default time range after all charts loaded
                    if (Object.keys(allChartData).length === metrics.length) {
                        setDefaultTimeRange(chartDataList);
                    }
                }
            })
            .catch(err => console.error('Failed to load vmstat ' + metric + ':', err));
    });
}

// MemInfo charts
function loadMeminfoCharts() {
    document.getElementById('vmstatSection').style.display = 'none';
    document.getElementById('meminfoSection').style.display = 'block';
    document.getElementById('topSection').style.display = 'none';
    document.getElementById('singleChartSection').style.display = 'none';

    const metrics = ['memory', 'swap', 'detail', 'hugepages'];
    const chartDataList = [];

    metrics.forEach(metric => {
        fetch('/api/chart/meminfo/' + metric)
            .then(r => r.json())
            .then(data => {
                if (data.success) {
                    allChartData['line|chart-meminfo-' + metric] = data.data;
                    chartDataList.push(data.data);
                    renderEChart(data.data, 'chart-meminfo-' + metric);

                    if (Object.keys(allChartData).length === metrics.length) {
                        setDefaultTimeRange(chartDataList);
                    }
                }
            })
            .catch(err => console.error('Failed to load meminfo ' + metric + ':', err));
    });
}

// Top charts
function loadTopCharts() {
    document.getElementById('vmstatSection').style.display = 'none';
    document.getElementById('meminfoSection').style.display = 'none';
    document.getElementById('topSection').style.display = 'block';
    document.getElementById('singleChartSection').style.display = 'none';

    const metrics = ['load', 'tasks', 'cpu', 'memory', 'topcpu'];
    const chartDataList = [];

    metrics.forEach(metric => {
        fetch('/api/chart/top/' + metric)
            .then(r => r.json())
            .then(data => {
                if (data.success) {
                    const chartType = metric === 'topcpu' ? 'bar' : 'line';
                    allChartData[chartType + '|chart-top-' + metric] = data.data;
                    chartDataList.push(data.data);
                    renderBarChart(data.data, 'chart-top-' + metric);

                    if (Object.keys(allChartData).length === metrics.length) {
                        setDefaultTimeRange(chartDataList);
                    }
                }
            })
            .catch(err => console.error('Failed to load top ' + metric + ':', err));
    });
}

// Bar chart rendering for top processes
function renderBarChart(chartData, containerId) {
    const container = document.getElementById(containerId);
    if (!container) return null;

    let chart = echarts.getInstanceByDom(container);
    if (chart) chart.dispose();
    chart = echarts.init(container);
    currentCharts.push(chart);

    if (!chartData || !chartData.series || chartData.series.length === 0) {
        chart.setOption({
            title: { text: '暂无数据', left: 'center', top: 'center', textStyle: { color: '#999', fontSize: 14 } }
        });
        return chart;
    }

    // Use bar chart for topcpu, line for others
    const isBarChart = chartData.categories && chartData.categories.length > 0 && chartData.series[0].name.includes('CPU');

    const option = {
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'cross' }
        },
        legend: {
            top: 0,
            textStyle: { fontSize: 10 },
            type: 'scroll'
        },
        grid: {
            left: 60,
            right: 20,
            top: 40,
            bottom: 80
        },
        xAxis: {
            type: 'category',
            data: chartData.categories,
            axisLabel: { fontSize: 9, rotate: 45 },
            axisTick: { show: false }
        },
        yAxis: {
            type: 'value',
            axisLabel: { fontSize: 9 },
            splitLine: { lineStyle: { type: 'dashed' } }
        },
        dataZoom: isBarChart ? [] : [
            { type: 'inside', start: 0, end: 100 },
            { type: 'slider', start: 0, end: 100, height: 20, bottom: 5 }
        ],
        series: chartData.series.map((s, i) => ({
            name: s.name,
            type: isBarChart ? 'bar' : 'line',
            data: s.data,
            smooth: isBarChart ? false : true,
            showSymbol: false,
            lineStyle: { width: 1.5 },
            color: COLORS[i % COLORS.length],
            emphasis: { lineStyle: { width: 3 } },
            barMaxWidth: 30
        }))
    };

    chart.setOption(option);
    return chart;
}

// Single chart section (iostat/ifconfig/mpstat)
function initSingleChart(type) {
    document.getElementById('vmstatSection').style.display = 'none';
    document.getElementById('meminfoSection').style.display = 'none';
    document.getElementById('topSection').style.display = 'none';
    document.getElementById('singleChartSection').style.display = 'block';

    const metricSelect = document.getElementById('metricSelect');
    const applyBtn = document.getElementById('applyFilter');
    const entityLabel = document.getElementById('entityLabel');

    // Set metric options based on type
    let metrics, entityEndpoint, chartEndpoint, entityKey;

    switch (type) {
        case 'iostat':
            metrics = [
                { value: 'iops', label: '📊 IOPS (r/s, w/s)' },
                { value: 'throughput', label: '💾 吞吐量 (KB/s)' },
                { value: 'await', label: '⏱️ 响应时间 (ms)' },
                { value: 'util', label: '📈 利用率 (%)' },
                { value: 'queue', label: '📋 队列深度' },
                { value: 'reqsize', label: '📦 请求大小 (KB)' }
            ];
            entityEndpoint = '/api/chart/iostat-devices';
            chartEndpoint = '/api/chart/iostat/';
            entityKey = 'devices';
            entityLabel.textContent = '🔧 设备筛选';
            break;
        case 'ifconfig':
            metrics = [
                { value: 'throughput', label: '🚀 吞吐量 (MB/s)' },
                { value: 'packets', label: '📦 数据包 (pkts/s)' },
                { value: 'errors', label: '❌ 错误 (errors/s)' },
                { value: 'dropped', label: '⚠️ 丢包 (dropped/s)' }
            ];
            entityEndpoint = '/api/chart/ifconfig-interfaces';
            chartEndpoint = '/api/chart/ifconfig/';
            entityKey = 'interfaces';
            entityLabel.textContent = '🔌 接口筛选';
            break;
        case 'mpstat':
            metrics = [
                { value: 'usage', label: '📊 CPU 使用率 (%)' },
                { value: 'user', label: '👤 用户态 (%)' },
                { value: 'system', label: '⚙️ 系统态 (%)' },
                { value: 'iowait', label: '⏳ I/O 等待 (%)' },
                { value: 'interrupt', label: '🔔 中断 (%)' },
                { value: 'steal', label: '🔀 虚拟化偷取 (%)' }
            ];
            entityEndpoint = '/api/chart/mpstat-cpus';
            chartEndpoint = '/api/chart/mpstat/';
            entityKey = 'cpus';
            entityLabel.textContent = '🖥️ CPU 筛选';
            break;
    }

    // Populate metric select
    metrics.forEach(m => {
        const opt = document.createElement('option');
        opt.value = m.value;
        opt.textContent = m.label;
        metricSelect.appendChild(opt);
    });

    // Load entity list
    loadEntityList(entityEndpoint, entityKey);

    // Load chart on metric change
    metricSelect.addEventListener('change', function() {
        loadSingleChart(chartEndpoint, type, entityKey);
    });

    // Apply filter button
    applyBtn.addEventListener('click', function() {
        loadSingleChart(chartEndpoint, type, entityKey);
    });

    // Initial load
    loadSingleChart(chartEndpoint, type, entityKey);
}

function loadEntityList(endpoint, entityKey) {
    fetch(endpoint)
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                const entities = data.data[entityKey] || [];
                const container = document.getElementById('entityCheckboxes');
                container.innerHTML = '';

                entities.forEach((entity, i) => {
                    const label = document.createElement('label');
                    label.className = 'entity-checkbox';
                    label.innerHTML = '<input type="checkbox" value="' + escapeHtml(entity) + '" checked> ' + escapeHtml(entity);
                    container.appendChild(label);
                });

                // Setup select all/deselect all
                setupSelectAll(entities.length);
            }
        })
        .catch(err => console.error('Failed to load entities:', err));
}

function setupSelectAll(entityCount) {
    const selectAllCb = document.getElementById('selectAllEntities');
    if (!selectAllCb) return;

    // Reset to checked
    selectAllCb.checked = true;

    // Remove old listener by cloning
    const newSelectAll = selectAllCb.cloneNode(true);
    selectAllCb.parentNode.replaceChild(newSelectAll, selectAllCb);

    // Select all toggle
    newSelectAll.addEventListener('change', function() {
        const checked = this.checked;
        const checkboxes = document.querySelectorAll('#entityCheckboxes input[type="checkbox"]');
        checkboxes.forEach(cb => cb.checked = checked);
    });

    // Update select all state when individual checkbox changes
    const container = document.getElementById('entityCheckboxes');
    container.addEventListener('change', function(e) {
        if (e.target.type === 'checkbox') {
            const allCbs = document.querySelectorAll('#entityCheckboxes input[type="checkbox"]');
            const checkedCbs = document.querySelectorAll('#entityCheckboxes input[type="checkbox"]:checked');
            newSelectAll.checked = allCbs.length === checkedCbs.length;
        }
    });
}

function loadSingleChart(chartEndpoint, type, entityKey) {
    const metric = document.getElementById('metricSelect').value;

    // Get selected entities
    const checkboxes = document.querySelectorAll('#entityCheckboxes input[type="checkbox"]:checked');
    const selected = Array.from(checkboxes).map(cb => cb.value);

    let url = chartEndpoint + metric;
    if (selected.length > 0) {
        const paramKey = entityKey === 'devices' ? 'devices' : entityKey === 'interfaces' ? 'interfaces' : 'cpus';
        url += '?' + paramKey + '=' + selected.join(',');
    }

    fetch(url)
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                allChartData['line|chart-main'] = data.data;
                renderEChart(data.data, 'chart-main');
                setDefaultTimeRange([data.data]);
            }
        })
        .catch(err => console.error('Failed to load chart:', err));
}
