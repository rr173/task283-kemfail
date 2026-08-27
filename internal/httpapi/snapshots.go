package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"task283-kemfail/internal/model"
)

// ---- 来源候选 ----

func (s *Server) listCandidates(w http.ResponseWriter, r *http.Request) {
	runID, err := queryInt64(r, "run_id")
	if err != nil {
		writeErr(w, err)
		return
	}
	cands, err := s.Svc.ListCandidates(runID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cands)
}

func (s *Server) confirmCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	c, err := s.Svc.ConfirmCandidate(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) rejectCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	c, err := s.Svc.RejectCandidate(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// ---- 诊断快照 ----

type createSnapshotReq struct {
	RunID int64  `json:"run_id"`
	Title string `json:"title"`
}

func (s *Server) createSnapshot(w http.ResponseWriter, r *http.Request) {
	var req createSnapshotReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	ss, err := s.Svc.CreateSnapshot(req.RunID, req.Title)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ss)
}

func (s *Server) listSnapshots(w http.ResponseWriter, r *http.Request) {
	runID, err := queryInt64(r, "run_id")
	if err != nil {
		writeErr(w, err)
		return
	}
	snaps, err := s.Svc.ListSnapshots(runID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

func (s *Server) getSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	ss, err := s.Svc.GetSnapshot(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ss)
}

func (s *Server) publishSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	ss, err := s.Svc.PublishSnapshot(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ss)
}

func (s *Server) supersedeSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var req struct {
		NewID int64 `json:"new_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	ss, err := s.Svc.SupersedeSnapshot(id, req.NewID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ss)
}

// ---- 统计与自检 ----

func (s *Server) getStats(w http.ResponseWriter, r *http.Request) {
	runID, err := queryInt64(r, "run_id")
	if err != nil {
		writeErr(w, err)
		return
	}
	st, err := s.Svc.Stats(runID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

type selfCheckResult struct {
	OK      bool     `json:"ok"`
	Checks  []string `json:"checks"`
	Errors  []string `json:"errors,omitempty"`
	APICount int     `json:"api_count"`
}

// selfCheck 验证服务健康：数据库连通 + 运行创建/读取闭环。
func (s *Server) selfCheck(w http.ResponseWriter, r *http.Request) {
	res := selfCheckResult{OK: true, APICount: apiCount}
	// 运行创建/读取
	run, err := s.Svc.CreateRun("selfcheck-"+strconv.FormatInt(runSeq(), 10), "selfcheck")
	if err != nil {
		res.OK = false
		res.Errors = append(res.Errors, "create run: "+err.Error())
	} else {
		res.Checks = append(res.Checks, "run create/read")
		if _, err := s.Svc.GetRun(run.ID); err != nil {
			res.OK = false
			res.Errors = append(res.Errors, "get run: "+err.Error())
		}
	}
	if res.OK {
		writeJSON(w, http.StatusOK, res)
	} else {
		writeJSON(w, http.StatusServiceUnavailable, res)
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

var apiCount = 31

var runCounter int64

func runSeq() int64 {
	runCounter++
	return runCounter
}

var _ = model.ErrRunNotFound // 保持 model 引用（辅助函数可能未全部使用）
