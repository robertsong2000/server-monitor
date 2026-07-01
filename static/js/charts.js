/**
 * charts.js — 响应时间趋势图（基于 Chart.js）
 * 一个 Chart 实例，重复打开时复用并更新数据，避免内存泄漏。
 */
const Charts = {
  _chart: null,

  /**
   * 渲染趋势图。data 为后端返回的检查记录数组（时间正序）。
   */
  render(data) {
    const labels = data.map(c =>
      new Date(c.checked_at * 1000).toLocaleTimeString([],
        { hour: '2-digit', minute: '2-digit' }));
    const latencies = data.map(c => c.status === 'up' ? c.latency : null);

    const ctx = document.getElementById('trendChart').getContext('2d');

    // 已存在实例则销毁后重建（dataset 结构变化时更稳妥）
    if (this._chart) { this._chart.destroy(); this._chart = null; }

    this._chart = new Chart(ctx, {
      type: 'line',
      data: {
        labels,
        datasets: [{
          label: '延迟 (ms)',
          data: latencies,
          borderColor: '#4f8cff',
          backgroundColor: 'rgba(79,140,255,0.12)',
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          pointHoverRadius: 4,
          spanGaps: true, // down 点为 null 时连线不中断
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: { mode: 'index', intersect: false },
        },
        scales: {
          x: { ticks: { color: '#8b8f9a', maxTicksLimit: 8 }, grid: { color: '#2a2e3a' } },
          y: { ticks: { color: '#8b8f9a' }, grid: { color: '#2a2e3a' }, beginAtZero: true },
        },
      },
    });
  },

  /** 关闭图表时销毁实例，释放资源 */
  destroy() {
    if (this._chart) { this._chart.destroy(); this._chart = null; }
  },
};
