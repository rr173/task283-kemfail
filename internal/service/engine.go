// Package service 编排业务闭环：接收失败事件 → 参数谱系校验 →
// 指纹归并 → 来源归因评分 → 隔离设备/确认候选 → 发布诊断快照。
// 所有跨 store 的调用链都经由此层，保证状态机与不变量一致。
package service

import (
	"fmt"
	"sync"

	"task283-kemfail/internal/device"
	"task283-kemfail/internal/fingerprint"
	"task283-kemfail/internal/model"
	"task283-kemfail/internal/param"
	"task283-kemfail/internal/store"
)

// Service 聚合存储与业务包，暴露领域操作。
type Service struct {
	DB               *store.DB
	ingestMu         sync.Mutex
	attributionMu   sync.Mutex
	fingerprintCache map[int64][]fingerprint.Fingerprint
}

// New 创建服务实例。
func New(db *store.DB) *Service {
	return &Service{DB: db, fingerprintCache: map[int64][]fingerprint.Fingerprint{}}
}

// ---- 运行批次 ----

// CreateRun 创建运行批次（接收中）。
func (s *Service) CreateRun(name, owner string) (*model.RunBatch, error) {
	return s.DB.CreateRun(name, owner)
}

// GetRun 读取运行批次。
func (s *Service) GetRun(id int64) (*model.RunBatch, error) {
	return s.DB.GetRun(id)
}

// ListRuns 列出全部运行批次。
func (s *Service) ListRuns() ([]model.RunBatch, error) {
	return s.DB.ListRuns()
}

// SealRun 封存运行：仅接收中/待归因/需复核可封存。
func (s *Service) SealRun(id int64) (*model.RunBatch, error) {
	run, err := s.DB.GetRun(id)
	if err != nil {
		return nil, err
	}
	if run.Status == model.RunSealed {
		return run, nil
	}
	switch run.Status {
	case model.RunReceiving, model.RunPending, model.RunReview, model.RunPublished:
	default:
		return nil, fmt.Errorf("%w: run status %s", model.ErrInvalidState, run.Status)
	}
	if err := s.DB.SetRunStatus(id, model.RunSealed); err != nil {
		return nil, err
	}
	return s.DB.GetRun(id)
}

// ---- 参数集 ----

// RegisterParamSet 注册参数集并校验谱系不变量（无环、版本递增）。
// parentID 为空表示基线（第一个注册）。
func (s *Service) RegisterParamSet(p *model.ParamSet, params map[string]string) (*model.ParamSet, error) {
	run, err := s.DB.GetRun(p.RunID)
	if err != nil {
		return nil, err
	}
	if run.Status == model.RunSealed {
		return nil, model.ErrRunSealed
	}
	p.ParamsDigest = param.ComputeDigest(param.DigestInput{
		KEMAlgorithm: p.KEMAlgorithm,
		Version:      p.Version,
		Params:       params,
	})
	// 基线判定：运行内尚无任何参数集 → 首个即为基线。
	existing, err := s.DB.ListParamSets(p.RunID)
	if err != nil {
		return nil, err
	}
	p.IsBaseline = len(existing) == 0
	if p.ParentID != nil {
		// 校验父参数集存在且同运行
		parent, err := s.DB.GetParamSet(*p.ParentID)
		if err != nil {
			return nil, err
		}
		if parent.RunID != p.RunID {
			return nil, fmt.Errorf("parent parameter set belongs to different run")
		}
		// 谱系无环 + 版本递增
		if _, err := param.Resolve(s.DB.GetParamSet, *p.ParentID); err != nil {
			return nil, err
		}
	}
	return s.DB.CreateParamSet(p)
}

// ListParamSets 列出某运行参数集。
func (s *Service) ListParamSets(runID int64) ([]model.ParamSet, error) {
	return s.DB.ListParamSets(runID)
}

// GetParamSet 读取参数集。
func (s *Service) GetParamSet(id int64) (*model.ParamSet, error) {
	return s.DB.GetParamSet(id)
}

// SetParamBaseline 将指定参数集标记为基线。
func (s *Service) SetParamBaseline(id int64) error {
	p, err := s.DB.GetParamSet(id)
	if err != nil {
		return err
	}
	run, err := s.DB.GetRun(p.RunID)
	if err != nil {
		return err
	}
	if run.Status == model.RunSealed {
		return model.ErrRunSealed
	}
	return s.DB.MarkParamBaseline(id)
}

// LineageOf 返回参数集的谱系描述（用于证据）。
func (s *Service) LineageOf(id int64) (*param.Lineage, error) {
	return param.Resolve(s.DB.GetParamSet, id)
}

// ---- 设备 ----

// RegisterDevice 注册设备。
func (s *Service) RegisterDevice(dev *model.Device) (*model.Device, error) {
	run, err := s.DB.GetRun(dev.RunID)
	if err != nil {
		return nil, err
	}
	if run.Status == model.RunSealed {
		return nil, model.ErrRunSealed
	}
	return s.DB.CreateDevice(dev)
}

// ListDevices 列出某运行设备。
func (s *Service) ListDevices(runID int64) ([]model.Device, error) {
	return s.DB.ListDevices(runID)
}

// GetDevice 读取设备。
func (s *Service) GetDevice(id int64) (*model.Device, error) {
	return s.DB.GetDevice(id)
}

// RecordNoise 记录一次设备噪声观测。
func (s *Service) RecordNoise(id int64, level float64) (*model.Device, error) {
	return s.DB.RecordDeviceNoise(id, level)
}

// IsolateDevice 隔离设备（活跃 → 隔离）。
func (s *Service) IsolateDevice(id int64) (*model.Device, error) {
	dev, err := s.DB.GetDevice(id)
	if err != nil {
		return nil, err
	}
	next, err := device.Isolate(dev)
	if err != nil {
		return nil, err
	}
	if err := device.GuardTransition(dev.Status, next); err != nil {
		return nil, err
	}
	if err := s.DB.SetDeviceStatus(id, next); err != nil {
		return nil, err
	}
	return s.DB.GetDevice(id)
}

// RecoverDevice 恢复设备（隔离 → 活跃）。
func (s *Service) RecoverDevice(id int64) (*model.Device, error) {
	dev, err := s.DB.GetDevice(id)
	if err != nil {
		return nil, err
	}
	next, err := device.Recover(dev)
	if err != nil {
		return nil, err
	}
	if err := device.GuardTransition(dev.Status, next); err != nil {
		return nil, err
	}
	if err := s.DB.SetDeviceStatus(id, next); err != nil {
		return nil, err
	}
	return s.DB.GetDevice(id)
}
