/**
 * services.js — 服务列表的渲染逻辑
 * 负责：概览统计 + 卡片网格 + 空状态。
 * 交互动作（检查/编辑/删除/历史）通过 data-action 委托给 app.js 绑定。
 */
const Services = {
  /** 渲染整个主区域（概览 + 网格） */
  render(services) {
    Services._renderSummary(services);
    Services._renderGrid(services);
  },

  /** 顶部统计胶囊 */
  _renderSummary(services) {
    const total = services.length;
    const up = services.filter(s => s.last_status === 'up').length;
    const down = services.filter(s => s.last_status === 'down').length;
    const avgUptime = total
      ? (services.reduce((a, s) => a + (s.uptime || 0), 0) / total)
      : 0;
    document.getElementById('summary').innerHTML = `
      <div class="pill">监控服务<b>${total}</b></div>
      <div class="pill up">正常<b>${up}</b></div>
      <div class="pill down">异常<b>${down}</b></div>
      <div class="pill">平均可用率<b>${avgUptime.toFixed(1)}%</b></div>
    `;
  },

  /** 卡片网格 / 空状态 */
  _renderGrid(services) {
    const grid = document.getElementById('grid');
    if (!services.length) {
      grid.innerHTML = `
        <div class="empty-state" style="grid-column:1/-1">
          <div class="icon">📭</div>
          <div>还没有任何监控服务</div>
          <div style="margin-top:8px">点击右上角「+ 添加服务」开始监控</div>
        </div>`;
      return;
    }
    grid.innerHTML = services.map(Services._cardHTML).join('');
  },

  /** 单张卡片 HTML */
  _cardHTML(s) {
    const dotClass = s.last_status === 'up' ? 'up'
                   : (s.last_status === 'down' ? 'down' : '');
    const latency = s.last_status === 'up' ? `${s.last_latency.toFixed(0)} ms` : '—';
    const uptimeColor = s.uptime >= 99 ? 'up' : (s.uptime >= 90 ? 'warn' : 'down');
    const hasData = s.last_status || (s.recent && s.recent.length);
    const uptimeText = hasData ? `${s.uptime.toFixed(1)}%` : '—';
    const target = Utils.formatTarget(s);
    const nameAttr = Utils.escapeAttr(s.name);

    const bars = Services._historyBars(s.recent || [], 30);
    const errLine = (s.last_error && s.last_status === 'down')
      ? `<div class="hint" style="color:var(--red);margin-top:10px">⚠ ${Utils.escapeHtml(s.last_error)}</div>`
      : '';

    return `
      <div class="card ${s.last_status === 'down' ? 'down' : ''}">
        <div class="card-head">
          <div class="card-title">
            <span class="status-dot ${dotClass}"></span>
            ${Utils.escapeHtml(s.name)}
          </div>
          <span class="type-badge">${s.type}</span>
        </div>
        <div class="card-meta">${Utils.escapeHtml(target)}</div>
        <div class="history-bar">${bars}</div>
        <div class="card-stats">
          <div class="stat">当前延迟<b>${latency}</b></div>
          <div class="stat uptime">可用率<b class="${uptimeColor}">${uptimeText}</b></div>
          <div class="stat">最后检查<b style="font-size:13px">${s.last_checked || '从未'}</b></div>
        </div>
        <div class="card-actions">
          <button class="btn-ghost btn-sm" data-action="check" data-id="${s.id}">立即检查</button>
          <button class="btn-ghost btn-sm" data-action="history" data-id="${s.id}" data-name="${nameAttr}">历史趋势</button>
          <button class="btn-ghost btn-sm" data-action="edit" data-id="${s.id}">编辑</button>
          <button class="btn-danger btn-sm" data-action="delete" data-id="${s.id}" data-name="${nameAttr}">删除</button>
        </div>
        ${errLine}
      </div>`;
  },

  /**
   * 生成最近 N 次检查的状态点条 HTML。
   * @param {Array} recent 时间正序的检查记录
   * @param {number} total 共显示多少格（不足的用空格占位）
   */
  _historyBars(recent, total) {
    const slice = recent.slice(-total);
    const bars = [];
    for (let i = 0; i < total; i++) {
      if (i < slice.length) {
        const c = slice[i];
        const tip = `${Utils.fmtTime(c.checked_at)} · ${c.status.toUpperCase()}`
                  + (c.latency ? ` · ${c.latency.toFixed(0)}ms` : '');
        bars.push(`<div class="bar ${c.status}" title="${Utils.escapeAttr(tip)}"></div>`);
      } else {
        bars.push(`<div class="bar empty"></div>`);
      }
    }
    return bars.join('');
  },
};
