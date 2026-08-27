package attribution

import (
	"fmt"
	"strings"

	"task283-kemfail/internal/model"
)

// ApplyToEvent 将归因评分结果落到事件状态上：
//   - 最高分为参数相关 → 事件置 param_conflict；
//   - 最高分为设备相关 → 事件置 noise_related；
//   - 密文相关 / 其他 → 保持 raw（等待人工复核）。
//
// 返回最终事件状态。事件已被排除时不作任何修改。
func ApplyToEvent(current model.EventStatus, scores []Score) (model.EventStatus, error) {
	if current == model.EventExcluded {
		return current, nil
	}
	best := Best(scores)
	if best == nil {
		return model.EventRaw, nil
	}
	switch best.Kind {
	case model.CandParam:
		return model.EventParamConflict, nil
	case model.CandDevice:
		return model.EventNoiseRelated, nil
	default:
		return model.EventRaw, nil
	}
}

// Summarize 生成归因结论摘要（用于快照内容与 API 输出）。
func Summarize(scores []Score) string {
	if len(scores) == 0 {
		return "no candidates produced"
	}
	parts := make([]string, 0, len(scores))
	for _, s := range scores {
		parts = append(parts, fmt.Sprintf("%s:%.2f", s.Kind, s.Score))
	}
	return strings.Join(parts, " ")
}

// EvidenceJoin 拼接多条证据。
func EvidenceJoin(evs ...string) string {
	nonEmpty := make([]string, 0, len(evs))
	for _, e := range evs {
		if strings.TrimSpace(e) != "" {
			nonEmpty = append(nonEmpty, e)
		}
	}
	return strings.Join(nonEmpty, "; ")
}
