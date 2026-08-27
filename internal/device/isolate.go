// Package device 负责设备噪声画像与隔离状态约束。
// 隔离 → 活跃被视为"恢复"，要求显式确认；活跃 → 隔离任意时刻允许。
package device

import (
	"fmt"

	"task283-kemfail/internal/model"
)

// Isolate 隔离设备：活跃 → 隔离。
// 已隔离设备再次隔离为幂等操作。
func Isolate(dev *model.Device) (model.DeviceStatus, error) {
	if dev == nil {
		return "", model.ErrDeviceNotFound
	}
	switch dev.Status {
	case model.DevIsolated:
		return dev.Status, nil
	case model.DevActive:
		return model.DevIsolated, nil
	default:
		return "", fmt.Errorf("%w: unknown device status %q", model.ErrInvalidState, dev.Status)
	}
}

// Recover 恢复设备：隔离 → 活跃。活跃设备恢复为幂等。
// 恢复要求噪声事件清零（重新建立观测基线）。
func Recover(dev *model.Device) (model.DeviceStatus, error) {
	if dev == nil {
		return "", model.ErrDeviceNotFound
	}
	switch dev.Status {
	case model.DevActive:
		return dev.Status, nil
	case model.DevIsolated:
		return model.DevActive, nil
	default:
		return "", fmt.Errorf("%w: unknown device status %q", model.ErrInvalidState, dev.Status)
	}
}

// GuardTransition 校验状态迁移合法性；非法迁移返回 ErrDeviceStateRegression。
func GuardTransition(from, to model.DeviceStatus) error {
	if from == to {
		return nil
	}
	switch from {
	case model.DevActive:
		if to != model.DevIsolated {
			return model.ErrDeviceStateRegression
		}
	case model.DevIsolated:
		if to != model.DevActive {
			return model.ErrDeviceStateRegression
		}
	default:
		return model.ErrInvalidState
	}
	return nil
}

// ClassifyNoise 将噪声强度映射为标签：low / medium / high / critical。
func ClassifyNoise(level float64) string {
	switch {
	case level >= 0.9:
		return "critical"
	case level >= 0.75:
		return "high"
	case level >= 0.5:
		return "medium"
	default:
		return "low"
	}
}

// NeedsIsolation 判断设备是否应被自动隔离（噪声强度高且事件多）。
func NeedsIsolation(dev *model.Device) bool {
	return dev != nil && dev.NoiseLevel >= 0.8 && dev.NoiseEvents >= 3
}
