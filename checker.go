package main

import (
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// Checker 负责执行各类探测，并把结果写入 Store。
type Checker struct {
	store *Store

	// icmpPermissionFailed 记录 raw socket 是否不可用，便于回退到 TCP。
	icmpPermissionFailed bool
	icmpOnce             sync.Once

	httpClient *http.Client
}

// NewChecker 创建检查器。
func NewChecker(store *Store) *Checker {
	return &Checker{
		store: store,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			// 不跟随重定向，只关心目标是否存活
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// CheckResult 是一次检查的内部结果。
type CheckResult struct {
	Status  string  // up | down
	Latency float64 // 毫秒
	Error   string
}

// Check 对给定服务执行一次检查。
func (c *Checker) Check(svc *Service) CheckResult {
	switch svc.Type {
	case "tcp":
		return c.checkTCP(svc)
	case "http":
		return c.checkHTTP(svc)
	case "icmp":
		return c.checkICMP(svc)
	default:
		return CheckResult{Status: "down", Error: "unknown check type: " + svc.Type}
	}
}

// checkTCP 尝试建立 TCP 连接。
func (c *Checker) checkTCP(svc *Service) CheckResult {
	if svc.Port <= 0 {
		return CheckResult{Status: "down", Error: "missing port"}
	}
	addr := net.JoinHostPort(svc.Host, fmt.Sprintf("%d", svc.Port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	latency := msSince(start)
	if err != nil {
		return CheckResult{Status: "down", Latency: latency, Error: err.Error()}
	}
	_ = conn.Close()
	return CheckResult{Status: "up", Latency: latency}
}

// checkHTTP 发起 HTTP GET 并接受 2xx/3xx 视为存活。
func (c *Checker) checkHTTP(svc *Service) CheckResult {
	if svc.Port == 0 {
		// 未指定端口时按默认端口推断
		svc.Port = 80
	}
	scheme := "http"
	if svc.Port == 443 {
		scheme = "https"
	}
	path := svc.Path
	if path == "" {
		path = "/"
	}
	url := fmt.Sprintf("%s://%s:%d%s", scheme, svc.Host, svc.Port, path)

	start := time.Now()
	resp, err := c.httpClient.Get(url)
	latency := msSince(start)
	if err != nil {
		return CheckResult{Status: "down", Latency: latency, Error: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return CheckResult{Status: "up", Latency: latency}
	}
	return CheckResult{
		Status:  "down",
		Latency: latency,
		Error:   fmt.Sprintf("HTTP %d", resp.StatusCode),
	}
}

// checkICMP 发送 ICMP echo；若无权限则回退到 TCP ping（探测 80 端口可达性近似判断主机存活）。
func (c *Checker) checkICMP(svc *Service) CheckResult {
	// 首次调用时探测是否有 ICMP 权限；后续直接复用结论。
	c.icmpOnce.Do(func() {
		if err := c.probeICMPCapability(); err != nil {
			log.Printf("icmp: raw socket unavailable (%v), will fall back to TCP ping", err)
			c.icmpPermissionFailed = true
		}
	})

	if !c.icmpPermissionFailed {
		if r := c.doICMP(svc.Host); r.Error == "" {
			return r
		}
		// 即便有权限也可能个别包丢失，这里不立即回退，直接返回结果
	}

	// 回退：对目标主机的常用端口做 TCP 握手来近似判断可达性
	return c.tcpFallbackPing(svc.Host)
}

// probeICMPCapability 尝试打开一个 ICMP socket 以判断权限。
func (c *Checker) probeICMPCapability() error {
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// doICMP 实际发送 ICMP echo request 并等待 reply。
func (c *Checker) doICMP(host string) CheckResult {
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return CheckResult{Status: "down", Error: err.Error()}
	}
	defer conn.Close()

	dst, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return CheckResult{Status: "down", Error: err.Error()}
	}

	// 构造 ICMP echo request（type 8, code 0）
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID:   1,
			Seq:  1,
			Data: []byte("SERVERMONITOR-PING"),
		},
	}
	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		return CheckResult{Status: "down", Error: err.Error()}
	}

	start := time.Now()
	_, err = conn.WriteTo(msgBytes, dst)
	if err != nil {
		return CheckResult{Status: "down", Latency: msSince(start), Error: err.Error()}
	}

	// 设置 5 秒读超时
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1500)
	n, _, err := conn.ReadFrom(buf)
	latency := msSince(start)
	if err != nil {
		return CheckResult{Status: "down", Latency: latency, Error: err.Error()}
	}
	_, err = icmp.ParseMessage(1, buf[:n]) // protocol 1 = ICMPv4
	if err != nil {
		return CheckResult{Status: "down", Latency: latency, Error: "parse reply: " + err.Error()}
	}
	return CheckResult{Status: "up", Latency: latency}
}

// tcpFallbackPing 在没有 ICMP 权限时，用对常见端口的 TCP 握手近似判断主机是否可达。
func (c *Checker) tcpFallbackPing(host string) CheckResult {
	ports := []int{80, 443, 22, 8080}
	var lastErr string
	for _, p := range ports {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", p)), 3*time.Second)
		latency := msSince(start)
		if err == nil {
			_ = conn.Close()
			return CheckResult{Status: "up", Latency: latency}
		}
		lastErr = err.Error()
	}
	return CheckResult{Status: "down", Error: "icmp fallback (tcp) failed: " + lastErr}
}

func msSince(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000.0
}

// Scheduler 定时轮询所有已启用的服务并执行检查。
type Scheduler struct {
	store   *Store
	checker *Checker
	stop    chan struct{}
	wg      sync.WaitGroup
}

// NewScheduler 创建调度器。
func NewScheduler(store *Store, checker *Checker) *Scheduler {
	return &Scheduler{
		store:   store,
		checker: checker,
		stop:    make(chan struct{}),
	}
}

// Start 启动后台调度循环。
func (s *Scheduler) Start() {
	s.wg.Add(1)
	go s.run()
}

// Stop 停止调度。
func (s *Scheduler) Stop() {
	close(s.stop)
	s.wg.Wait()
}

// run 每 10 秒扫描一次服务表，为到点的服务执行检查。
func (s *Scheduler) run() {
	defer s.wg.Done()
	// 记录每个服务上次检查时间，按各自 interval 调度
	lastChecked := make(map[int64]time.Time)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// 启动后立即跑一轮
	s.runOnce(lastChecked)

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.runOnce(lastChecked)
		}
	}
}

// runOnce 执行一轮调度判断。
func (s *Scheduler) runOnce(lastChecked map[int64]time.Time) {
	services, err := s.store.ListServices()
	if err != nil {
		log.Printf("scheduler: list services: %v", err)
		return
	}
	now := time.Now()
	for i := range services {
		svc := &services[i]
		if !svc.Enabled {
			continue
		}
		interval := time.Duration(svc.Interval) * time.Second
		if interval <= 0 {
			interval = 30 * time.Second
		}
		last, ok := lastChecked[svc.ID]
		if ok && now.Sub(last) < interval {
			continue // 还没到下一次检查时间
		}
		lastChecked[svc.ID] = now
		// 每个检查独立 goroutine，避免慢服务阻塞其他服务
		s.wg.Add(1)
		go func(svc *Service) {
			defer s.wg.Done()
			s.checkAndRecord(svc)
		}(svc)
	}
}

// checkAndRecord 执行单次检查并写入存储。
func (s *Scheduler) checkAndRecord(svc *Service) {
	result := s.checker.Check(svc)
	err := s.store.RecordCheck(Check{
		ServiceID: svc.ID,
		Status:    result.Status,
		Latency:   math.Round(result.Latency*100) / 100,
		Error:     result.Error,
		CheckedAt: time.Now().Unix(),
	})
	if err != nil {
		log.Printf("scheduler: record check for service %d: %v", svc.ID, err)
	}
}

// CheckNow 立即对单个服务执行一次检查（用于前端「立即检查」按钮）。
func (s *Scheduler) CheckNow(svc *Service) CheckResult {
	result := s.checker.Check(svc)
	_ = s.store.RecordCheck(Check{
		ServiceID: svc.ID,
		Status:    result.Status,
		Latency:   math.Round(result.Latency*100) / 100,
		Error:     result.Error,
		CheckedAt: time.Now().Unix(),
	})
	return result
}
