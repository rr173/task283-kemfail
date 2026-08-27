// Package httpapi 提供 HTTP API 层，路由统一以 /api 前缀。
// 处理 JSON 编解码、错误映射与请求分发。
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"task283-kemfail/internal/model"
	"task283-kemfail/internal/service"
)

// Server 聚合服务与路由。
type Server struct {
	Svc *service.Service
	mux *http.ServeMux
}

// New 构建 HTTP 服务器，注册全部路由。
func New(svc *service.Service) *Server {
	s := &Server{Svc: svc, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 返回 http.Handler（供 http.ListenAndServe 使用）。
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	// 运行批次
	s.mux.HandleFunc("POST /api/runs", s.createRun)
	s.mux.HandleFunc("GET /api/runs", s.listRuns)
	s.mux.HandleFunc("GET /api/runs/{id}", s.getRun)
	s.mux.HandleFunc("POST /api/runs/{id}/seal", s.sealRun)
	s.mux.HandleFunc("POST /api/runs/{id}/attribution", s.runAttribution)

	// 参数集
	s.mux.HandleFunc("POST /api/params", s.createParam)
	s.mux.HandleFunc("GET /api/params", s.listParams)
	s.mux.HandleFunc("GET /api/params/{id}", s.getParam)
	s.mux.HandleFunc("POST /api/params/{id}/baseline", s.setParamBaseline)
	s.mux.HandleFunc("GET /api/params/{id}/lineage", s.getLineage)

	// 设备
	s.mux.HandleFunc("POST /api/devices", s.createDevice)
	s.mux.HandleFunc("GET /api/devices", s.listDevices)
	s.mux.HandleFunc("GET /api/devices/{id}", s.getDevice)
	s.mux.HandleFunc("POST /api/devices/{id}/noise", s.recordNoise)
	s.mux.HandleFunc("POST /api/devices/{id}/isolate", s.isolateDevice)
	s.mux.HandleFunc("POST /api/devices/{id}/recover", s.recoverDevice)

	// 失败事件
	s.mux.HandleFunc("POST /api/events", s.ingestEvent)
	s.mux.HandleFunc("GET /api/events", s.listEvents)
	s.mux.HandleFunc("GET /api/events/{id}", s.getEvent)
	s.mux.HandleFunc("POST /api/events/{id}/exclude", s.excludeEvent)
	s.mux.HandleFunc("GET /api/fingerprints", s.listFingerprints)

	// 来源候选
	s.mux.HandleFunc("GET /api/candidates", s.listCandidates)
	s.mux.HandleFunc("POST /api/candidates/{id}/confirm", s.confirmCandidate)
	s.mux.HandleFunc("POST /api/candidates/{id}/reject", s.rejectCandidate)

	// 诊断快照
	s.mux.HandleFunc("POST /api/snapshots", s.createSnapshot)
	s.mux.HandleFunc("GET /api/snapshots", s.listSnapshots)
	s.mux.HandleFunc("GET /api/snapshots/{id}", s.getSnapshot)
	s.mux.HandleFunc("POST /api/snapshots/{id}/publish", s.publishSnapshot)
	s.mux.HandleFunc("POST /api/snapshots/{id}/supersede", s.supersedeSnapshot)

	// 统计与自检
	s.mux.HandleFunc("GET /api/stats", s.getStats)
	s.mux.HandleFunc("GET /api/selfcheck", s.selfCheck)
	s.mux.HandleFunc("GET /api/health", s.health)
}

// ---- 公共辅助 ----

// pathID 解析路径参数 {id} 为 int64。
func pathID(r *http.Request) (int64, error) {
	v := r.PathValue("id")
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

// queryInt64 解析查询参数为 int64。
func queryInt64(r *http.Request, key string) (int64, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0, errors.New("missing query parameter " + key)
	}
	return strconv.ParseInt(v, 10, 64)
}

// writeJSON 写 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

// writeErr 映射领域错误到 HTTP 状态码。
func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, model.ErrRunNotFound),
		errors.Is(err, model.ErrParamNotFound),
		errors.Is(err, model.ErrDeviceNotFound),
		errors.Is(err, model.ErrEventNotFound),
		errors.Is(err, model.ErrCandidateNotFound),
		errors.Is(err, model.ErrSnapshotNotFound):
		status = http.StatusNotFound
	case errors.Is(err, model.ErrRunSealed),
		errors.Is(err, model.ErrDuplicateEvent),
		errors.Is(err, model.ErrParamLineageCycle),
		errors.Is(err, model.ErrParamMissingBaseline),
		errors.Is(err, model.ErrParamVersionRegression),
		errors.Is(err, model.ErrDeviceStateRegression),
		errors.Is(err, model.ErrInvalidDigest),
		errors.Is(err, model.ErrSnapshotFrozen),
		errors.Is(err, model.ErrEventExcluded),
		errors.Is(err, model.ErrInvalidState):
		status = http.StatusConflict
	default:
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
