/**
 * utils.js — 通用工具函数
 * 挂载到全局 Utils 命名空间，避免污染 window。
 */
const Utils = {
  /**
   * HTML 转义，防止 XSS（服务名/错误信息等用户/外部数据插入 DOM 时使用）。
   */
  escapeHtml(s) {
    return String(s ?? '').replace(/[&<>"']/g, c => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    }[c]));
  },

  /**
   * 转义字符串使其可安全嵌入单引号属性（如 onclick='f("...")'）。
   */
  escapeAttr(s) {
    return String(s ?? '').replace(/'/g, "\\'");
  },

  /**
   * unix 时间戳（秒）→ 人类可读时间。
   */
  fmtTime(ts) {
    return new Date(ts * 1000).toLocaleString([], {
      month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'
    });
  },

  /**
   * 把服务对象格式化成可读的目标地址串。
   */
  formatTarget(s) {
    if (s.type === 'icmp') return `ping ${s.host}`;
    if (s.type === 'http') {
      const scheme = s.port === 443 ? 'https' : 'http';
      return `${scheme}://${s.host}:${s.port}${s.path || ''}`;
    }
    return `${s.host}:${s.port}`;
  },

  /* ---------- toast 轻提示 ---------- */
  _toastTimer: null,
  toast(msg, isError) {
    const t = document.getElementById('toast');
    if (!t) return;
    t.textContent = msg;
    t.className = 'toast show' + (isError ? ' error' : '');
    clearTimeout(this._toastTimer);
    this._toastTimer = setTimeout(() => { t.className = 'toast'; }, 3000);
  },
};
