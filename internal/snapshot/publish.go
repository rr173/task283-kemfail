// Package snapshot 提供诊断快照的发布与替代约束：
// 发布后不可修改（冻结），替代通过新建快照完成。
package snapshot

import (
	"fmt"

	"task283-kemfail/internal/model"
)

// NewDraft 构造一个草稿快照。要求运行存在、参数基线 ID 合法、
// 候选 ID 列表非空（无候选不允许发布诊断）。
func NewDraft(runID, paramBaseline int64, title string, candidateIDs []int64) (*model.DiagnosticSnapshot, error) {
	if runID <= 0 {
		return nil, fmt.Errorf("run id is required")
	}
	if paramBaseline <= 0 {
		return nil, fmt.Errorf("param baseline is required")
	}
	if len(candidateIDs) == 0 {
		return nil, fmt.Errorf("snapshot requires at least one source candidate")
	}
	if title == "" {
		title = fmt.Sprintf("diagnostic snapshot for run %d", runID)
	}
	return &model.DiagnosticSnapshot{
		RunID:         runID,
		Title:         title,
		Status:        model.SnapDraft,
		ParamBaseline: paramBaseline,
		CandidateIDs:  candidateIDs,
	}, nil
}

// GuardPublish 校验发布前置条件：
//   - 草稿才能发布；
//   - 参数基线必须真实存在（由调用方保证 ID 有效性）；
//   - 候选必须全部处于 confirmed 状态（未确认的候选不允许发布）。
func GuardPublish(s *model.DiagnosticSnapshot, candidates []model.SourceCandidate) error {
	if s == nil {
		return model.ErrSnapshotNotFound
	}
	if s.Status != model.SnapDraft {
		return model.ErrSnapshotFrozen
	}
	if len(candidates) == 0 {
		return fmt.Errorf("no candidates to publish")
	}
	for _, c := range candidates {
		if c.Status != model.CandConfirmed {
			return fmt.Errorf("candidate %d not confirmed (status=%s)", c.ID, c.Status)
		}
	}
	return nil
}

// GuardSupersede 校验替代操作：旧快照必须已发布，新快照必须为草稿。
func GuardSupersede(old, new *model.DiagnosticSnapshot) error {
	if old == nil || new == nil {
		return model.ErrSnapshotNotFound
	}
	if old.Status != model.SnapPublished {
		return fmt.Errorf("%w: source snapshot must be published", model.ErrSnapshotFrozen)
	}
	if new.Status != model.SnapDraft {
		return fmt.Errorf("%w: replacement must be draft", model.ErrInvalidState)
	}
	if old.RunID != new.RunID {
		return fmt.Errorf("%w: snapshots belong to different runs", model.ErrInvalidState)
	}
	return nil
}

// FreezeGuard 返回是否可修改：仅草稿可改。
func FreezeGuard(s *model.DiagnosticSnapshot) error {
	if s == nil {
		return model.ErrSnapshotNotFound
	}
	if s.Status != model.SnapDraft {
		return model.ErrSnapshotFrozen
	}
	return nil
}
