/**
 * api.js — 后端 REST API 封装
 * 所有网络请求集中在此，UI 层只调用这些方法，不直接 fetch。
 */
const Api = {
  /** 列出所有服务（含最近状态、可用率、历史点） */
  async listServices() {
    const res = await fetch('/api/services');
    if (!res.ok) throw new Error(`列表加载失败 (${res.status})`);
    return res.json();
  },

  /** 获取单个服务 */
  async getService(id) {
    const res = await fetch(`/api/services/${id}`);
    if (!res.ok) throw new Error('服务不存在');
    return res.json();
  },

  /** 创建服务 */
  async createService(body) {
    const res = await fetch('/api/services', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    return Api._unwrap(res, '创建失败');
  },

  /** 更新服务 */
  async updateService(id, body) {
    const res = await fetch(`/api/services/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    return Api._unwrap(res, '更新失败');
  },

  /** 删除服务 */
  async deleteService(id) {
    const res = await fetch(`/api/services/${id}`, { method: 'DELETE' });
    return Api._unwrap(res, '删除失败');
  },

  /** 立即触发一次检查 */
  async checkNow(id) {
    const res = await fetch(`/api/services/${id}/check`, { method: 'POST' });
    return Api._unwrap(res, '检查失败');
  },

  /** 获取历史趋势（默认近 24h） */
  async history(id, hours = 24) {
    const res = await fetch(`/api/services/${id}/history?hours=${hours}`);
    if (!res.ok) throw new Error('历史加载失败');
    return res.json();
  },

  /**
   * 统一拆解响应：HTTP 不ok 时从 body 里取出后端返回的 error 字段。
   * 成功时返回 JSON（若无 body 则返回 {}）。
   */
  async _unwrap(res, fallbackMsg) {
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || `${fallbackMsg} (${res.status})`);
    }
    const text = await res.text();
    return text ? JSON.parse(text) : {};
  },
};
