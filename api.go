package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// API 封装所有 HTTP handler，依赖 Store、Checker、Scheduler。
type API struct {
	store     *Store
	scheduler *Scheduler
}

// NewAPI 创建 API handler 集合。
func NewAPI(store *Store, scheduler *Scheduler) *API {
	return &API{store: store, scheduler: scheduler}
}

// Routes 注册所有路由到 mux。
func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/services", a.handleServices)
	mux.HandleFunc("/api/services/", a.handleServiceByID)
}

// handleServices 处理 GET（列出）/ POST（创建）。
func (a *API) handleServices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		services, err := a.store.ListServices()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// 为每个服务附加最近 30 次检查，用于前端状态点展示
		type serviceWithHistory struct {
			Service
			Recent []Check `json:"recent"`
		}
		out := make([]serviceWithHistory, 0, len(services))
		for i := range services {
			recent, _ := a.store.RecentChecks(services[i].ID, 30)
			out = append(out, serviceWithHistory{Service: services[i], Recent: reverseChecks(recent)})
		}
		writeJSON(w, out)

	case http.MethodPost:
		var svc Service
		if err := decodeJSON(r, &svc); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		log.Printf("create service request: name=%q type=%q host=%q port=%d path=%q", svc.Name, svc.Type, svc.Host, svc.Port, svc.Path)
		if err := validateService(&svc); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		id, err := a.store.CreateService(&svc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		svc.ID = id
		// 创建后立即触发一次检查，让用户马上看到初始状态
		go a.scheduler.CheckNow(&svc)
		writeJSON(w, svc)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleServiceByID 处理 /api/services/{id}、/check、/history 等子路径。
func (a *API) handleServiceByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/services/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "missing service id")
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	// 子路由：/api/services/{id}/check, /history
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}

	switch {
	case sub == "check" && r.Method == http.MethodPost:
		svc, err := a.store.GetService(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "service not found")
			return
		}
		result := a.scheduler.CheckNow(svc)
		writeJSON(w, result)

	case sub == "history" && r.Method == http.MethodGet:
		hours := 24
		if h := r.URL.Query().Get("hours"); h != "" {
			if v, err := strconv.Atoi(h); err == nil {
				hours = v
			}
		}
		history, err := a.store.History(id, hours)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, history)

	case sub == "" && r.Method == http.MethodPut:
		var svc Service
		if err := decodeJSON(r, &svc); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		svc.ID = id
		if err := validateService(&svc); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := a.store.UpdateService(&svc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, svc)

	case sub == "" && r.Method == http.MethodDelete:
		if err := a.store.DeleteService(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]bool{"deleted": true})

	case sub == "" && r.Method == http.MethodGet:
		svc, err := a.store.GetService(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "service not found")
			return
		}
		writeJSON(w, svc)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// validateService 校验并补全服务字段。
func validateService(svc *Service) error {
	svc.Name = strings.TrimSpace(svc.Name)
	svc.Host = strings.TrimSpace(svc.Host)
	svc.Type = strings.TrimSpace(svc.Type)
	svc.Path = strings.TrimSpace(svc.Path)

	if svc.Name == "" {
		return fmt.Errorf("name is required")
	}
	if svc.Host == "" {
		return fmt.Errorf("host is required")
	}
	switch svc.Type {
	case "tcp", "http", "icmp":
	default:
		return fmt.Errorf("type must be one of: tcp, http, icmp")
	}
	if svc.Type == "tcp" && svc.Port <= 0 {
		return fmt.Errorf("port is required for tcp type")
	}
	if svc.Interval <= 0 {
		svc.Interval = 30
	}
	return nil
}

// ---------- 工具函数 ----------

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// reverseChecks 反转切片，让数据库倒序结果变成时间正序（前端从左到右展示历史）。
func reverseChecks(in []Check) []Check {
	out := make([]Check, len(in))
	for i := range in {
		out[len(in)-1-i] = in[i]
	}
	return out
}
