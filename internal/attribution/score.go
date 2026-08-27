// Package attribution 实现来源归因评分：对一次失败事件按
// 密文损坏 / 参数版本错配 / 设备硬件噪声三个来源分别打分，
// 输出候选及证据。分数越高代表该来源越可能。
package attribution

import (
	"fmt"
	"sort"

	"task283-kemfail/internal/model"
)

// Input 描述归因引擎所需的全部上下文（由 service 层组装）。
type Input struct {
	RunID            int64
	EventID          int64
	ParamConsistent  bool   // 参数摘要是否与库中一致
	LineageDepth     int    // 谱系深度（版本距基线步数）
	HasBaseline      bool   // 谱系是否有基线
	DeviceNoise      float64 // 设备噪声强度 0..1
	DeviceNoiseEvents int   // 设备噪声事件数
	DeviceIsolated   bool   // 设备是否已隔离
	BurstEvents      int    // 同指纹归并事件数
	BurstDevices     int    // 同指纹涉及设备数
	DigestUnknown    bool   // 密文摘要无法解析（长度异常等）
}

// Score 表示某一来源的评分结果。
type Score struct {
	Kind     model.CandidateKind
	Score    float64
	Evidence string
}

// Evaluate 对一次失败事件执行三路归因评分，返回按分数降序的候选。
//
// 评分规则（各来源独立计分，取最高分作为该来源分）：
//
//	密文相关 ciphertext：
//	  + 摘要无法解析 → 0.9
//	  + 参数一致但同一摘要跨多设备失败 → 0.75（密文本身损坏）
//	  + 失败码为解密类（如 invalid_ciphertext）→ 0.65
//	参数相关 param：
//	  + 参数摘要与库不一致 → 0.85
//	  + 谱系无基线 / 版本错配 → 0.7
//	  + 谱系深度 > 0 且批量爆发（多设备同指纹）→ 0.6
//	设备相关 device：
//	  + 噪声强度 > 0.75 且噪声事件 >= 3 → 0.8
//	  + 噪声强度 > 0.5 → 0.55
//	  + 设备已隔离但仍有新失败 → 0.7（隔离未生效）
//
// 边界：得分区间 [0,1]；证据字符串必须非空。
func Evaluate(in Input) []Score {
	out := make([]Score, 0, 3)

	// 密文相关
	cipherScore := 0.0
	cipherEv := ""
	switch {
	case in.DigestUnknown:
		cipherScore = 0.9
		cipherEv = "ciphertext digest unparseable"
	case !in.ParamConsistent:
		// 参数不一致优先归因参数，密文分压低
		cipherScore = 0.2
		cipherEv = "digest present but parameter mismatch dominates"
	case in.BurstDevices >= 2 && in.BurstEvents >= 2:
		cipherScore = 0.75
		cipherEv = fmt.Sprintf("same digest across %d devices (%d events)", in.BurstDevices, in.BurstEvents)
	default:
		cipherScore = 0.3
		cipherEv = "single-device failure with valid parameters"
	}
	if cipherScore > 0 {
		out = append(out, Score{Kind: model.CandCiphertext, Score: cipherScore, Evidence: cipherEv})
	}

	// 参数相关
	paramScore := 0.0
	paramEv := ""
	switch {
	case !in.ParamConsistent:
		paramScore = 0.85
		paramEv = "parameter digest inconsistent with stored set"
	case !in.HasBaseline:
		paramScore = 0.7
		paramEv = "lineage missing baseline parameter"
	case in.LineageDepth > 0 && in.BurstEvents >= 3:
		paramScore = 0.6
		paramEv = fmt.Sprintf("deep lineage (depth %d) with burst of %d events", in.LineageDepth, in.BurstEvents)
	default:
		paramScore = 0.25
		paramEv = "parameters consistent; version mismatch unlikely"
	}
	if paramScore > 0 {
		out = append(out, Score{Kind: model.CandParam, Score: paramScore, Evidence: paramEv})
	}

	// 设备相关
	devScore := 0.0
	devEv := ""
	switch {
	case in.DeviceIsolated:
		devScore = 0.7
		devEv = "device isolated yet new failure received"
	case in.DeviceNoise > 0.75 && in.DeviceNoiseEvents >= 3:
		devScore = 0.8
		devEv = fmt.Sprintf("noise level %.2f with %d events", in.DeviceNoise, in.DeviceNoiseEvents)
	case in.DeviceNoise > 0.5:
		devScore = 0.55
		devEv = fmt.Sprintf("elevated noise level %.2f", in.DeviceNoise)
	default:
		devScore = 0.15
		devEv = "device noise within normal range"
	}
	if devScore > 0 {
		out = append(out, Score{Kind: model.CandDevice, Score: devScore, Evidence: devEv})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// Best 返回最高分候选；空输入返回 nil。
func Best(scores []Score) *Score {
	if len(scores) == 0 {
		return nil
	}
	s := scores[0]
	return &s
}
