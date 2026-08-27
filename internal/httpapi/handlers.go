package httpapi

import (
	"encoding/json"
	"net/http"

	"task283-kemfail/internal/event"
	"task283-kemfail/internal/model"
)

// ---- 运行批次 ----

type createRunReq struct {
	Name  string `json:"name"`
	Owner string `json:"owner"`
}

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	var req createRunReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	if req.Name == "" {
		writeErr(w, model.ErrInvalidState)
		return
	}
	run, err := s.Svc.CreateRun(req.Name, req.Owner)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.Svc.ListRuns()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	run, err := s.Svc.GetRun(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) sealRun(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	run, err := s.Svc.SealRun(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) runAttribution(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	created, err := s.Svc.RunAttribution(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run_id": id, "candidates_created": created})
}

// ---- 参数集 ----

type createParamReq struct {
	RunID        int64             `json:"run_id"`
	Name         string            `json:"name"`
	KEMAlgorithm string            `json:"kem_algorithm"`
	Version      string            `json:"version"`
	ParentID     *int64            `json:"parent_id,omitempty"`
	Params       map[string]string `json:"params"`
}

func (s *Server) createParam(w http.ResponseWriter, r *http.Request) {
	var req createParamReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	p := &model.ParamSet{
		RunID:        req.RunID,
		Name:         req.Name,
		KEMAlgorithm: req.KEMAlgorithm,
		Version:      req.Version,
		ParentID:     req.ParentID,
	}
	created, err := s.Svc.RegisterParamSet(p, req.Params)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) listParams(w http.ResponseWriter, r *http.Request) {
	runID, err := queryInt64(r, "run_id")
	if err != nil {
		writeErr(w, err)
		return
	}
	params, err := s.Svc.ListParamSets(runID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, params)
}

func (s *Server) getParam(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	p, err := s.Svc.GetParamSet(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) setParamBaseline(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.Svc.SetParamBaseline(id); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "baseline": true})
}

func (s *Server) getLineage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	lin, err := s.Svc.LineageOf(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"depth":     lin.Depth,
		"has_base":  lin.HasBase,
		"cyclic":    lin.Cyclic,
		"describe":  lin.Describe(),
		"sets":      lin.Sets,
	})
}

// ---- 设备 ----

type createDeviceReq struct {
	RunID       int64   `json:"run_id"`
	Name        string  `json:"name"`
	Model       string  `json:"model"`
	NoiseLevel  float64 `json:"noise_level"`
	Firmware    string  `json:"firmware"`
}

func (s *Server) createDevice(w http.ResponseWriter, r *http.Request) {
	var req createDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	dev := &model.Device{
		RunID:      req.RunID,
		Name:       req.Name,
		Model:      req.Model,
		NoiseLevel: req.NoiseLevel,
		Firmware:   req.Firmware,
	}
	created, err := s.Svc.RegisterDevice(dev)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	runID, err := queryInt64(r, "run_id")
	if err != nil {
		writeErr(w, err)
		return
	}
	devices, err := s.Svc.ListDevices(runID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	dev, err := s.Svc.GetDevice(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dev)
}

type noiseReq struct {
	Level float64 `json:"level"`
}

func (s *Server) recordNoise(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var req noiseReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	dev, err := s.Svc.RecordNoise(id, req.Level)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dev)
}

func (s *Server) isolateDevice(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	dev, err := s.Svc.IsolateDevice(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dev)
}

func (s *Server) recoverDevice(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	dev, err := s.Svc.RecoverDevice(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dev)
}

// ---- 失败事件 ----

type ingestEventReq struct {
	RunID            int64   `json:"run_id"`
	ParamID          int64   `json:"param_id"`
	DeviceID         int64   `json:"device_id"`
	CiphertextDigest string  `json:"ciphertext_digest"`
	FailureCode      string  `json:"failure_code"`
	NoiseLevel       float64 `json:"noise_level"`
}

func (s *Server) ingestEvent(w http.ResponseWriter, r *http.Request) {
	var req ingestEventReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	e, isNew, err := s.Svc.IngestEvent(event.IngestInput{
		RunID:            req.RunID,
		ParamID:          req.ParamID,
		DeviceID:         req.DeviceID,
		CiphertextDigest: req.CiphertextDigest,
		FailureCode:      req.FailureCode,
		NoiseLevel:       req.NoiseLevel,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	status := http.StatusCreated
	if !isNew {
		status = http.StatusOK // 幂等命中，返回既有记录
	}
	writeJSON(w, status, e)
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	runID, err := queryInt64(r, "run_id")
	if err != nil {
		writeErr(w, err)
		return
	}
	events, err := s.Svc.ListEvents(runID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) getEvent(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	e, err := s.Svc.GetEvent(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) excludeEvent(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	e, err := s.Svc.ExcludeEvent(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) listFingerprints(w http.ResponseWriter, r *http.Request) {
	runID, err := queryInt64(r, "run_id")
	if err != nil {
		writeErr(w, err)
		return
	}
	fps, err := s.Svc.FingerprintsOf(runID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fps)
}
