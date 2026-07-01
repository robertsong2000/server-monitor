package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Service 描述一个被监控的服务。
type Service struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // tcp | http | icmp
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Path      string    `json:"path,omitempty"`
	Interval  int       `json:"interval"` // 秒
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`

	// 以下为运行时聚合字段，并非直接来自 services 表
	LastStatus  string  `json:"last_status"`  // up | down | "" (无记录)
	LastLatency float64 `json:"last_latency"` // 毫秒
	LastError   string  `json:"last_error"`
	LastChecked string  `json:"last_checked"` // 人类可读时间
	Uptime      float64 `json:"uptime"`       // 最近 N 次的可用率，0-100
}

// Check 表示一次检查的结果记录。
type Check struct {
	ID        int64   `json:"id"`
	ServiceID int64   `json:"service_id"`
	Status    string  `json:"status"` // up | down
	Latency   float64 `json:"latency"` // 毫秒
	Error     string  `json:"error"`
	CheckedAt int64   `json:"checked_at"` // unix 时间戳
}

// Store 封装 SQLite 访问。
type Store struct {
	db *sql.DB
}

// Open 打开数据库并创建表结构。
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// 开启 WAL 提升并发读写；关闭同步等待以加快写入。
	_, _ = db.Exec("PRAGMA journal_mode=WAL;")
	_, _ = db.Exec("PRAGMA busy_timeout=5000;")

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS services (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    type       TEXT    NOT NULL,
    host       TEXT    NOT NULL,
    port       INTEGER NOT NULL DEFAULT 0,
    path       TEXT    NOT NULL DEFAULT '',
    interval   INTEGER NOT NULL DEFAULT 30,
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS checks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id INTEGER NOT NULL,
    status     TEXT    NOT NULL,
    latency    REAL    NOT NULL DEFAULT 0,
    error      TEXT    NOT NULL DEFAULT '',
    checked_at INTEGER NOT NULL,
    FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_checks_service_time ON checks(service_id, checked_at);
`)
	return err
}

// Close 关闭数据库。
func (s *Store) Close() error { return s.db.Close() }

// ListServices 返回所有服务，附带最近状态与可用率聚合数据。
func (s *Store) ListServices() ([]Service, error) {
	rows, err := s.db.Query(`
SELECT id, name, type, host, port, path, interval, enabled, created_at
FROM services ORDER BY id;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var svc Service
		var enabled int
		var createdAt string
		if err := rows.Scan(&svc.ID, &svc.Name, &svc.Type, &svc.Host, &svc.Port,
			&svc.Path, &svc.Interval, &enabled, &createdAt); err != nil {
			return nil, err
		}
		svc.Enabled = enabled == 1
		t, _ := time.Parse("2006-01-02 15:04:05", createdAt)
		svc.CreatedAt = t
		services = append(services, svc)
	}

	// 为每个服务填充运行时聚合数据
	for i := range services {
		s.fillRuntimeStats(&services[i])
	}
	return services, rows.Err()
}

// fillRuntimeStats 填充最近状态、延迟、可用率（最近 100 次检查中 up 的比例）。
func (s *Store) fillRuntimeStats(svc *Service) {
	// 最近一次检查
	var status, errMsg string
	var latency float64
	var checkedAt sql.NullInt64
	err := s.db.QueryRow(`
SELECT status, latency, error, checked_at FROM checks
WHERE service_id=? ORDER BY checked_at DESC LIMIT 1;`, svc.ID).Scan(&status, &latency, &errMsg, &checkedAt)
	if err == nil {
		svc.LastStatus = status
		svc.LastLatency = latency
		svc.LastError = errMsg
		if checkedAt.Valid {
			svc.LastChecked = time.Unix(checkedAt.Int64, 0).Format("01-02 15:04:05")
		}
	}

	// 可用率：最近 100 次里 up 的比例
	var total, up int
	_ = s.db.QueryRow(`
SELECT COUNT(*), COALESCE(SUM(CASE WHEN status='up' THEN 1 ELSE 0 END),0)
FROM (SELECT status FROM checks WHERE service_id=? ORDER BY checked_at DESC LIMIT 100);`,
		svc.ID).Scan(&total, &up)
	if total > 0 {
		svc.Uptime = float64(up) / float64(total) * 100
	}
}

// GetService 按 ID 查询单个服务。
func (s *Store) GetService(id int64) (*Service, error) {
	var svc Service
	var enabled int
	var createdAt string
	err := s.db.QueryRow(`
SELECT id, name, type, host, port, path, interval, enabled, created_at
FROM services WHERE id=?;`, id).Scan(
		&svc.ID, &svc.Name, &svc.Type, &svc.Host, &svc.Port,
		&svc.Path, &svc.Interval, &enabled, &createdAt)
	if err != nil {
		return nil, err
	}
	svc.Enabled = enabled == 1
	t, _ := time.Parse("2006-01-02 15:04:05", createdAt)
	svc.CreatedAt = t
	s.fillRuntimeStats(&svc)
	return &svc, nil
}

// CreateService 插入新服务，返回新 ID。
func (s *Store) CreateService(svc *Service) (int64, error) {
	if svc.Interval <= 0 {
		svc.Interval = 30
	}
	res, err := s.db.Exec(`
INSERT INTO services (name, type, host, port, path, interval, enabled)
VALUES (?,?,?,?,?,?,?);`,
		svc.Name, svc.Type, svc.Host, svc.Port, svc.Path, svc.Interval, boolToInt(svc.Enabled))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateService 更新服务配置。
func (s *Store) UpdateService(svc *Service) error {
	if svc.Interval <= 0 {
		svc.Interval = 30
	}
	_, err := s.db.Exec(`
UPDATE services SET name=?, type=?, host=?, port=?, path=?, interval=?, enabled=?
WHERE id=?;`,
		svc.Name, svc.Type, svc.Host, svc.Port, svc.Path, svc.Interval, boolToInt(svc.Enabled), svc.ID)
	return err
}

// DeleteService 删除服务及其所有检查记录。
func (s *Store) DeleteService(id int64) error {
	_, err := s.db.Exec(`DELETE FROM services WHERE id=?;`, id)
	return err
}

// RecordCheck 写入一条检查记录，并在超过保留上限时清理旧数据。
func (s *Store) RecordCheck(c Check) error {
	_, err := s.db.Exec(`
INSERT INTO checks (service_id, status, latency, error, checked_at)
VALUES (?,?,?,?,?);`,
		c.ServiceID, c.Status, c.Latency, c.Error, c.CheckedAt)
	if err != nil {
		return err
	}
	// 每个服务最多保留 5000 条历史，避免数据无限增长
	_, _ = s.db.Exec(`
DELETE FROM checks WHERE service_id=? AND id NOT IN (
    SELECT id FROM checks WHERE service_id=? ORDER BY checked_at DESC LIMIT 5000
);`, c.ServiceID, c.ServiceID)
	return nil
}

// History 返回某服务在最近 hours 小时内的检查记录（按时间正序），用于绘制趋势图。
func (s *Store) History(serviceID int64, hours int) ([]Check, error) {
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
	rows, err := s.db.Query(`
SELECT id, service_id, status, latency, error, checked_at
FROM checks WHERE service_id=? AND checked_at>=? ORDER BY checked_at ASC;`,
		serviceID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Check
	for rows.Next() {
		var c Check
		if err := rows.Scan(&c.ID, &c.ServiceID, &c.Status, &c.Latency, &c.Error, &c.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RecentChecks 返回某服务最近 n 次检查记录（按时间倒序），用于状态点展示。
func (s *Store) RecentChecks(serviceID int64, n int) ([]Check, error) {
	if n <= 0 {
		n = 30
	}
	rows, err := s.db.Query(`
SELECT id, service_id, status, latency, error, checked_at
FROM checks WHERE service_id=? ORDER BY checked_at DESC LIMIT ?;`,
		serviceID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Check
	for rows.Next() {
		var c Check
		if err := rows.Scan(&c.ID, &c.ServiceID, &c.Status, &c.Latency, &c.Error, &c.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// formatDuration 简单的人类可读时长（用于日志/调试）。
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}
	return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
}
