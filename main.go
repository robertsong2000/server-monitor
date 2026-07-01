package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", getenv("ADDR", ":8080"), "HTTP listen address")
	dbPath := flag.String("db", getenv("DB_PATH", "data.db"), "SQLite database path")
	staticDir := flag.String("static", getenv("STATIC_DIR", "static"), "static files directory")
	flag.Parse()

	// 确保数据库所在目录存在（容器挂载卷时可能尚未创建）
	if dir := filepath.Dir(*dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("create db dir: %v", err)
		}
	}

	// 打开数据库
	store, err := Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	// 创建检查器与调度器
	checker := NewChecker(store)
	scheduler := NewScheduler(store, checker)
	scheduler.Start()
	defer scheduler.Stop()

	api := NewAPI(store, scheduler)

	mux := http.NewServeMux()
	api.Routes(mux)
	// 静态前端：根路径映射到指定目录
	mux.Handle("/", http.FileServer(http.Dir(*staticDir)))

	srv := &http.Server{
		Addr:              *addr,
		Handler:           withLogging(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 优雅关闭
	go func() {
		log.Printf("server monitor listening on http://localhost%s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("bye")
}

// withLogging 是一个简单的访问日志中间件。
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// getenv 读取环境变量，未设置则返回 fallback。
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
