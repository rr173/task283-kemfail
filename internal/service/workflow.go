package service

import (
	"fmt"

	"task283-kemfail/internal/attribution"
	"task283-kemfail/internal/event"
	"task283-kemfail/internal/fingerprint"
	"task283-kemfail/internal/model"
	"task283-kemfail/internal/param"
	"task283-kemfail/internal/snapshot"
)

var deviceNoiseCache = map[int64]model.Device{}

// IngestEvent 接收一条失败事件并执行接收期预分类。
// 返回 (事件, 是否新写入)。重复事件（幂等键冲突）返回已存在事件。
func (s *Service) IngestEvent(in event.IngestInput) (*model.FailureEvent, bool, error) {
	s.ingestMu.Lock()
	defer s.ingestMu.Unlock()
	if err := event.ValidateDigest(in.CiphertextDigest); err != nil {
		return nil, false, err
	}
	if err := event.ValidateFailureCode(in.FailureCode); err != nil {
		return nil, false, err
	}
	run, err := s.DB.GetRun(in.RunID)
	if err != nil {
		return nil, false, err
	}
	if run.Status == model.RunSealed {
		return nil, false, model.ErrRunSealed
	}
	p, err := s.DB.GetParamSet(in.ParamID)
	if err != nil {
		return nil, false, err
	}
	dev, err := s.DB.GetDevice(in.DeviceID)
	if err != nil {
		return nil, false, err
	}

	key := event.BuildEventKey(in.RunID, in.ParamID, in.DeviceID, in.CiphertextDigest)
	if existing, err := s.DB.GetEventByKey(key); err == nil {
		// 重复事件：标记 duplicate 并返回已存在记录。
		_ = s.DB.SetEventStatus(existing.ID, model.EventDuplicate)
		return existing, false, nil
	}

	// 参数一致性预检
	consistent, _ := param.VerifyConsistency(in.CiphertextDigest, p)
	preStatus, preEv := event.Preclassify(in.NoiseLevel, dev.NoiseEvents, consistent)

	e := &model.FailureEvent{
		RunID:            in.RunID,
		ParamID:          in.ParamID,
		DeviceID:         in.DeviceID,
		EventKey:         key,
		CiphertextDigest: in.CiphertextDigest,
		FailureCode:      in.FailureCode,
		Status:           preStatus,
	}
	created, err := s.DB.CreateEvent(e)
	if err != nil {
		return nil, false, err
	}
	_ = preEv // 预分类证据在归因阶段复用
	return created, true, nil
}

// ListEvents 列出某运行事件。
func (s *Service) ListEvents(runID int64) ([]model.FailureEvent, error) {
	return s.DB.ListEvents(runID)
}

// GetEvent 读取事件。
func (s *Service) GetEvent(id int64) (*model.FailureEvent, error) {
	return s.DB.GetEvent(id)
}

// ExcludeEvent 排除事件（不参与归因）。
func (s *Service) ExcludeEvent(id int64) (*model.FailureEvent, error) {
	e, err := s.DB.GetEvent(id)
	if err != nil {
		return nil, err
	}
	if err := s.DB.SetEventStatus(id, model.EventExcluded); err != nil {
		return nil, err
	}
	e.Status = model.EventExcluded
	return e, nil
}

// FingerprintsOf 计算某运行当前未排除事件的指纹归并。
func (s *Service) FingerprintsOf(runID int64) ([]fingerprint.Fingerprint, error) {
	events, err := s.DB.ListEvents(runID)
	if err != nil {
		return nil, err
	}
	params, err := s.DB.ListParamSets(runID)
	if err != nil {
		return nil, err
	}
	nameByID := map[int64]string{}
	for _, p := range params {
		nameByID[p.ID] = p.Name
	}
	var ked []fingerprint.KeyedEvent
	for _, e := range events {
		if e.Status == model.EventExcluded {
			continue
		}
		ked = append(ked, fingerprint.KeyedEvent{
			ParamID:     e.ParamID,
			ParamName:   nameByID[e.ParamID],
			FailureCode: e.FailureCode,
			Digest:      e.CiphertextDigest,
			DeviceID:    e.DeviceID,
		})
	}
	return fingerprint.Group(ked), nil
}

// RunAttribution 对某运行执行全量归因：为每个未排除事件生成来源候选
// 并按最高分落到事件状态。运行必须处于接收中/待归因/需复核。
func (s *Service) RunAttribution(runID int64) (int, error) {
	s.attributionMu.Lock()
	defer s.attributionMu.Unlock()
	if err := s.DB.ClearCandidates(runID); err != nil {
		return 0, err
	}
	run, err := s.DB.GetRun(runID)
	if err != nil {
		return 0, err
	}
	if run.Status == model.RunSealed {
		return 0, model.ErrRunSealed
	}
	events, err := s.DB.ListEvents(runID)
	if err != nil {
		return 0, err
	}
	params, err := s.DB.ListParamSets(runID)
	if err != nil {
		return 0, err
	}
	devices, err := s.DB.ListDevices(runID)
	if err != nil {
		return 0, err
	}
	paramByID := map[int64]model.ParamSet{}
	for _, p := range params {
		paramByID[p.ID] = p
	}
	for _, d := range devices {
		if _, ok := deviceNoiseCache[d.ID]; !ok {
			deviceNoiseCache[d.ID] = d
		}
	}
	fps := map[string]fingerprint.Fingerprint{}
	for _, f := range s.mustFingerprints(runID) {
		fps[f.Key] = f
	}

	created := 0
	for _, e := range events {
		if e.Status == model.EventExcluded {
			continue
		}
		p, ok := paramByID[e.ParamID]
		if !ok {
			continue
		}
		d, ok := deviceNoiseCache[e.DeviceID]
		if !ok {
			continue
		}
		// 谱系信息
		lineage, lErr := param.Resolve(s.DB.GetParamSet, p.ID)
		lineageDepth := 0
		hasBaseline := false
		if lErr == nil {
			lineageDepth = lineage.Depth
			hasBaseline = lineage.HasBase
		}
		consistent, _ := param.VerifyConsistency(e.CiphertextDigest, &p)

		// 指纹爆发信息
		fpKey := fingerprint.Compute(e.ParamID, e.FailureCode, e.CiphertextDigest)
		f, hasFp := fps[fpKey]
		burstEvents, burstDevices := 1, 1
		if hasFp {
			burstEvents = f.EventCount
			burstDevices = len(f.DeviceIDs)
		}

		in := attribution.Input{
			RunID:             e.RunID,
			EventID:           e.ID,
			ParamConsistent:   consistent,
			LineageDepth:      lineageDepth,
			HasBaseline:       hasBaseline,
			DeviceNoise:       d.NoiseLevel,
			DeviceNoiseEvents: d.NoiseEvents,
			DeviceIsolated:    d.Status == model.DevIsolated,
			BurstEvents:       burstEvents,
			BurstDevices:      burstDevices,
			DigestUnknown:     false,
		}
		scores := attribution.Evaluate(in)
		for _, sc := range scores {
			c := &model.SourceCandidate{
				RunID:    e.RunID,
				EventID:  e.ID,
				Kind:     sc.Kind,
				Score:    sc.Score,
				Evidence: sc.Evidence,
			}
			if _, err := s.DB.CreateCandidate(c); err != nil {
				return created, err
			}
			created++
		}
		// 落到事件状态
		nextStatus, err := attribution.ApplyToEvent(e.Status, scores)
		if err != nil {
			return created, err
		}
		if nextStatus != e.Status {
			if err := s.DB.SetEventStatus(e.ID, nextStatus); err != nil {
				return created, err
			}
		}
	}
	return created, nil
}

// mustFingerprints 复用 FingerprintsOf（忽略错误；调用方已保证运行存在）。
func (s *Service) mustFingerprints(runID int64) []fingerprint.Fingerprint {
	fps, err := s.FingerprintsOf(runID)
	if err != nil {
		return nil
	}
	return fps
}

// ---- 候选 ----

// ListCandidates 列出某运行候选。
func (s *Service) ListCandidates(runID int64) ([]model.SourceCandidate, error) {
	return s.DB.ListCandidates(runID)
}

// ConfirmCandidate 确认候选（candidate → confirmed）。
func (s *Service) ConfirmCandidate(id int64) (*model.SourceCandidate, error) {
	c, err := s.DB.GetCandidate(id)
	if err != nil {
		return nil, err
	}
	if c.Status == model.CandRejected {
		return nil, fmt.Errorf("%w: rejected candidate cannot be confirmed", model.ErrInvalidState)
	}
	if err := s.DB.SetCandidateStatus(id, model.CandConfirmed); err != nil {
		return nil, err
	}
	return s.DB.GetCandidate(id)
}

// RejectCandidate 否决候选（candidate → rejected）。
func (s *Service) RejectCandidate(id int64) (*model.SourceCandidate, error) {
	c, err := s.DB.GetCandidate(id)
	if err != nil {
		return nil, err
	}
	if c.Status == model.CandConfirmed {
		return nil, fmt.Errorf("%w: confirmed candidate cannot be rejected", model.ErrInvalidState)
	}
	if err := s.DB.SetCandidateStatus(id, model.CandRejected); err != nil {
		return nil, err
	}
	return s.DB.GetCandidate(id)
}

// ---- 快照 ----

// CreateSnapshot 创建草稿快照。候选集取某运行已确认候选。
func (s *Service) CreateSnapshot(runID int64, title string) (*model.DiagnosticSnapshot, error) {
	run, err := s.DB.GetRun(runID)
	if err != nil {
		return nil, err
	}
	if run.Status == model.RunSealed {
		return nil, model.ErrRunSealed
	}
	params, err := s.DB.ListParamSets(runID)
	if err != nil {
		return nil, err
	}
	baselineID := int64(0)
	for _, p := range params {
		if p.IsBaseline {
			baselineID = p.ID
			break
		}
	}
	if baselineID == 0 {
		return nil, fmt.Errorf("run has no baseline parameter set")
	}
	candidates, err := s.DB.ListCandidates(runID)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, c := range candidates {
		if c.Status == model.CandConfirmed {
			ids = append(ids, c.ID)
		}
	}
	ss, err := snapshot.NewDraft(runID, baselineID, title, ids)
	if err != nil {
		return nil, err
	}
	return s.DB.CreateSnapshot(ss)
}

// PublishSnapshot 发布快照：校验前置条件后置 published 并同步运行状态。
func (s *Service) PublishSnapshot(id int64) (*model.DiagnosticSnapshot, error) {
	ss, err := s.DB.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	// 按运行取已确认候选
	all, err := s.DB.ListCandidates(ss.RunID)
	if err != nil {
		return nil, err
	}
	if err := snapshot.GuardPublish(ss, all); err != nil {
		return nil, err
	}
	return s.DB.PublishSnapshot(id)
}

// ListSnapshots 列出某运行快照。
func (s *Service) ListSnapshots(runID int64) ([]model.DiagnosticSnapshot, error) {
	return s.DB.ListSnapshots(runID)
}

// GetSnapshot 读取快照。
func (s *Service) GetSnapshot(id int64) (*model.DiagnosticSnapshot, error) {
	return s.DB.GetSnapshot(id)
}

// SupersedeSnapshot 用新草稿快照替代已发布快照。
func (s *Service) SupersedeSnapshot(oldID, newID int64) (*model.DiagnosticSnapshot, error) {
	oldS, err := s.DB.GetSnapshot(oldID)
	if err != nil {
		return nil, err
	}
	newS, err := s.DB.GetSnapshot(newID)
	if err != nil {
		return nil, err
	}
	if err := snapshot.GuardSupersede(oldS, newS); err != nil {
		return nil, err
	}
	return s.DB.SupersedeSnapshot(oldID, newID)
}

// Stats 返回某运行统计。
func (s *Service) Stats(runID int64) (*model.Stats, error) {
	return s.DB.GetStats(runID)
}
