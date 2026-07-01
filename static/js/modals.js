/**
 * modals.js — 模态框逻辑
 * 两个模态框：服务添加/编辑表单、历史趋势图。
 * 表单读写集中在此，app.js 只负责打开与事件绑定。
 */
const Modals = {
  /* ---------- 服务表单（添加/编辑共用） ---------- */

  /** 以「新增」模式打开表单 */
  openAdd() {
    this._fillForm({
      id: '', name: '', type: 'tcp', host: '', port: '', path: '', interval: 30, enabled: true,
    });
    document.getElementById('modalTitle').textContent = '添加服务';
    this._open('serviceModal');
    setTimeout(() => document.getElementById('f_name').focus(), 50);
  },

  /** 以「编辑」模式打开表单，用服务数据填充 */
  openEdit(s) {
    this._fillForm(s);
    document.getElementById('modalTitle').textContent = '编辑服务';
    this._open('serviceModal');
  },

  /** 根据类型显隐 端口/路径 字段，并给 http 一个默认端口 */
  onTypeChange() {
    const type = document.getElementById('f_type').value;
    document.getElementById('portGroup').style.display = (type === 'icmp') ? 'none' : '';
    document.getElementById('pathGroup').style.display = (type === 'http') ? '' : 'none';
    if (type === 'http' && !document.getElementById('f_port').value) {
      document.getElementById('f_port').value = 80;
    }
  },

  /** 收集表单数据为服务对象 */
  readForm() {
    const rawId = document.getElementById('editId').value;
    return {
      // id 为数字类型（后端 Service.ID 是 int64），新建时为空则不发送
      ...(rawId ? { id: parseInt(rawId, 10) } : {}),
      name: document.getElementById('f_name').value.trim(),
      type: document.getElementById('f_type').value,
      host: document.getElementById('f_host').value.trim(),
      port: parseInt(document.getElementById('f_port').value) || 0,
      path: document.getElementById('f_path').value.trim(),
      interval: parseInt(document.getElementById('f_interval').value) || 30,
      enabled: document.getElementById('f_enabled').checked,
    };
  },

  /** 关闭服务表单 */
  closeService() { this._close('serviceModal'); },

  /* ---------- 历史趋势图 ---------- */

  /** 打开趋势图模态框 */
  openChart(name) {
    document.getElementById('chartTitle').textContent = `${name} · 响应时间趋势（近 24h）`;
    this._open('chartModal');
  },

  /** 关闭趋势图模态框 */
  closeChart() {
    this._close('chartModal');
    Charts.destroy();
  },

  /* ---------- 内部方法 ---------- */

  /** 用服务数据填充表单字段 */
  _fillForm(s) {
    document.getElementById('editId').value = s.id ?? '';
    document.getElementById('f_name').value = s.name ?? '';
    document.getElementById('f_type').value = s.type ?? 'tcp';
    document.getElementById('f_host').value = s.host ?? '';
    document.getElementById('f_port').value = s.port ?? '';
    document.getElementById('f_path').value = s.path ?? '';
    document.getElementById('f_interval').value = s.interval ?? 30;
    document.getElementById('f_enabled').checked = s.enabled !== false;
    this.onTypeChange();
  },

  _open(id) { document.getElementById(id).classList.add('active'); },
  _close(id) { document.getElementById(id).classList.remove('active'); },
};
