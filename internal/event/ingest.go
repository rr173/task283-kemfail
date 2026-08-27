// Package event 处理失败事件的接收边界：幂等键构造、密文摘要
// 合法性校验与接收时的预分类（重复 / 参数冲突 / 噪声相关）。
package event

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"task283-kemfail/internal/model"
)

// IngestInput 描述一条待写入的失败事件（HTTP 层解析后传入）。
type IngestInput struct {
	RunID            int64
	ParamID          int64
	DeviceID         int64
	CiphertextDigest string
	FailureCode      string
	NoiseLevel       float64 // 设备当前噪声强度（0..1），用于预分类
}

// DigestLength 密文摘要的合法长度（hex 编码，32 字节 → 64 字符）。
const DigestLength = 64

// BuildEventKey 构造幂等键：runID|paramID|deviceID|digestPrefix。
// digestPrefix 取前 12 位 hex。
func BuildEventKey(runID, paramID, deviceID int64, digest string) string {
	prefix := digest
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	h := sha256.New()
	fmt.Fprintf(h, "%d|%d|%d|%s", runID, paramID, deviceID, prefix)
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateDigest 校验密文摘要合法性（64 位 hex）。
func ValidateDigest(digest string) error {
	if len(digest) != DigestLength {
		return fmt.Errorf("%w: got %d chars, want %d", model.ErrInvalidDigest, len(digest), DigestLength)
	}
	for _, c := range digest {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return fmt.Errorf("%w: non-hex char %q", model.ErrInvalidDigest, c)
		}
	}
	return nil
}

// ValidateFailureCode 校验失败码非空且长度受限。
func ValidateFailureCode(code string) error {
	if code == "" {
		return fmt.Errorf("failure code is required")
	}
	if len(code) > 32 {
		return fmt.Errorf("failure code too long")
	}
	return nil
}

// Preclassify 根据设备噪声与参数一致性给出接收时的预分类。
//
//   - 参数摘要与库中参数集不一致 → param_conflict；
//   - 设备噪声强度 > 0.75 且噪声事件计数高 → noise_related；
//   - 否则 raw（等待归因引擎）。
//
// 返回预分类状态与证据描述。
func Preclassify(noiseLevel float64, noiseEvents int, paramConsistent bool) (model.EventStatus, string) {
	if !paramConsistent {
		return model.EventParamConflict, "parameter digest inconsistent with stored set"
	}
	if noiseLevel > 0.75 && noiseEvents >= 3 {
		return model.EventNoiseRelated, fmt.Sprintf(
			"device noise level %.2f with %d events", noiseLevel, noiseEvents)
	}
	return model.EventRaw, ""
}
