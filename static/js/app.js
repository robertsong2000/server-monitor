/**
 * app.js — 应用主入口
 * 职责：定时轮询刷新、事件委托（卡片按钮）、模态框保存/删除等业务编排。
 * 依赖：Utils / Api / Services / Charts / Modals
 */
const App = {
  REFRESH_MS: 5000, // 前端轮询间隔
  _timer: null,

  /** 启动：首次加载 + 定时轮询 + 绑定全局事件 */
  start() {
    this.refresh();
    this._timer = setInterval(() => this.refresh(), this.REFRESH_MS);
    this._bindEvents();
  },

  /** 拉取最新服务列表并渲染 */
  async refresh() {
    try {
      const services = await Api.listServices();
      Services.render(services);
      const time = new Date().toLocaleTimeString();
      document.getElementById('refreshLabel').textContent = `已更新 ${time}`;
    } catch (e) {
      Utils.toast('加载失败: ' + e.message, true);
    }
  },

  /** 全局事件绑定（用事件委托，避免对每张卡片重复绑定） */
  _bindEvents() {
    // 卡片内按钮：通过 data-action 委托
    document.getElementById('grid').addEventListener('click', e => {
      const btn = e.target.closest('[data-action]');
      if (!btn) return;
      const { action, id, name } = btn.dataset;
      if (action === 'check')   this.checkNow(id);
      if (action === 'history') this.showHistory(id, name);
      if (action === 'edit')    this.editService(id);
      if (action === 'delete')  this.deleteService(id, name);
    });

    // 点击遮罩关闭模态框
    document.querySelectorAll('.modal-overlay').forEach(o => {
      o.addEventListener('click', e => {
        if (e.target === o) o.classList.remove('active');
        if (o.id === 'chartModal') Charts.destroy();
      });
    });

    // ESC 关闭模态框
    document.addEventListener('keydown', e => {
      if (e.key === 'Escape') {
        Modals.closeService();
        Modals.closeChart();
      }
    });
  },

  /* ---------- 业务动作 ---------- */

  /** 立即触发一次检查 */
  async checkNow(id) {
    try {
      const r = await Api.checkNow(id);
      this.refresh();
      const ok = r.Status === 'up';
      const msg = `检查完成: ${ok ? '✓ 正常' : '✗ 异常'} (${(r.Latency || 0).toFixed(0)}ms)`
                + (r.Error ? ' - ' + r.Error : '');
      Utils.toast(msg, !ok);
    } catch (e) { Utils.toast(e.message, true); }
  },

  /** 打开历史趋势图 */
  async showHistory(id, name) {
    Modals.openChart(name);
    try {
      const data = await Api.history(id, 24);
      Charts.render(data);
    } catch (e) {
      Utils.toast('历史加载失败: ' + e.message, true);
    }
  },

  /** 打开编辑表单（先拉取最新数据） */
  async editService(id) {
    try {
      const s = await Api.getService(id);
      Modals.openEdit(s);
    } catch (e) { Utils.toast(e.message, true); }
  },

  /** 保存（新增或更新） */
  async saveService() {
    const body = Modals.readForm();
    if (!body.name || !body.host) {
      Utils.toast('请填写名称和主机', true);
      return;
    }
    try {
      if (body.id) {
        await Api.updateService(body.id, body);
        Utils.toast('已更新');
      } else {
        await Api.createService(body);
        Utils.toast('已添加，正在首次检查...');
      }
      Modals.closeService();
      this.refresh();
    } catch (e) { Utils.toast(e.message, true); }
  },

  /** 删除服务 */
  async deleteService(id, name) {
    if (!confirm(`确定删除「${name}」？所有历史记录也会被清除。`)) return;
    try {
      await Api.deleteService(id);
      this.refresh();
      Utils.toast('已删除');
    } catch (e) { Utils.toast('删除失败: ' + e.message, true); }
  },
};

// DOM 就绪后启动
document.addEventListener('DOMContentLoaded', () => App.start());
