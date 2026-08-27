package device

import (
	"math"
)

// NoiseProfile 描述设备的噪声画像（滑动窗口统计）。
type NoiseProfile struct {
	Mean     float64 `json:"mean"`      // 滑动均值
	Variance float64 `json:"variance"`  // 滑动方差
	Count    int     `json:"count"`     // 窗口内观测数
	MaxLevel float64 `json:"max_level"` // 峰值强度
}

// UpdateExponential 用指数滑动平均更新噪声画像。
// 保持计算无状态：给定旧画像与新的观测值，返回更新后的画像。
func UpdateExponential(p NoiseProfile, level float64, alpha float64) NoiseProfile {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.3
	}
	p.Count++
	if p.Count == 1 {
		p.Mean = level
		p.Variance = 0
		p.MaxLevel = level
		return p
	}
	prevMean := p.Mean
	p.Mean = alpha*level + (1-alpha)*prevMean
	// 在线方差更新：Var = (1-alpha) * (Var + alpha * (level - prevMean)^2)
	p.Variance = (1-alpha)*(p.Variance+alpha*(level-prevMean)*(level-prevMean))
	p.MaxLevel = math.Max(p.MaxLevel, level)
	return p
}

// AnomalyScore 计算噪声观测的异常度 [0,1]：
// z-score 映射为 sigmoid，高均值 + 高方差时更保守。
func AnomalyScore(p NoiseProfile, level float64) float64 {
	if p.Count == 0 {
		return 0.5 // 无基线时中性
	}
	std := math.Sqrt(p.Variance)
	if std < 1e-6 {
		std = 0.1
	}
	z := (level - p.Mean) / std
	// sigmoid(z) 映射到 [0,1]
	return 1.0 / (1.0 + math.Exp(-z))
}

// Combine 合并两台设备的画像（用于批量归因时的设备组噪声）。
func Combine(a, b NoiseProfile) NoiseProfile {
	n := a.Count + b.Count
	if n == 0 {
		return NoiseProfile{}
	}
	mean := (a.Mean*float64(a.Count) + b.Mean*float64(b.Count)) / float64(n)
	// 合并方差按加权平均近似
	variance := (a.Variance*float64(a.Count) + b.Variance*float64(b.Count)) / float64(n)
	return NoiseProfile{
		Mean: mean, Variance: variance, Count: n,
		MaxLevel: math.Max(a.MaxLevel, b.MaxLevel),
	}
}
