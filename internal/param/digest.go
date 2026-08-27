package param

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"task283-kemfail/internal/model"
)

// DigestInput 描述参与参数摘要的字段，用于构建参数指纹。
type DigestInput struct {
	KEMAlgorithm string
	Version      string
	Params       map[string]string
}

// ComputeDigest 计算参数集的稳定摘要（sha256），不随创建顺序变化。
// 同一算法 + 版本 + 排序后的参数键值 → 同一摘要，用于参数版本错配比对。
func ComputeDigest(in DigestInput) string {
	h := sha256.New()
	h.Write([]byte(in.KEMAlgorithm))
	h.Write([]byte{0})
	h.Write([]byte(in.Version))
	h.Write([]byte{0})
	keys := make([]string, 0, len(in.Params))
	for k := range in.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{'='})
		h.Write([]byte(in.Params[k]))
		h.Write([]byte{1})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyConsistency 验证一次失败事件使用的参数集与其声明版本一致：
// 若事件携带的参数摘要与库中参数集摘要不同，说明存在版本错配。
//
// 返回 (一致, 证据)。一致时证据为空字符串。
func VerifyConsistency(eventDigest string, stored *model.ParamSet) (bool, string) {
	if eventDigest == "" || stored == nil {
		return false, "empty digest or missing parameter set"
	}
	if eventDigest == stored.ParamsDigest {
		return true, ""
	}
	return false, fmt.Sprintf(
		"param set %s@%s digest mismatch: event=%s stored=%s",
		stored.Name, stored.Version, shortHex(eventDigest), shortHex(stored.ParamsDigest))
}

// MatchingBaseline 在候选基线集合中查找与事件摘要一致的参数集。
// 返回基线 ID 与是否命中。命中表示事件使用的参数版本在既有谱系内。
func MatchingBaseline(eventDigest string, baselines []model.ParamSet) (int64, bool) {
	for _, b := range baselines {
		if b.ParamsDigest == eventDigest {
			return b.ID, true
		}
	}
	return 0, false
}

// FingerprintKey 构造事件的参数域指纹：run_id + 摘要 + 算法。
// 用于判断"同参数 + 同密文特征"是否在多个设备重复出现。
func FingerprintKey(runID int64, digest, algorithm string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s|%s", runID, digest, algorithm)
	return hex.EncodeToString(h.Sum(nil))
}

func shortHex(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[:8] + ".." + s[len(s)-8:]
}

// ValidateName 校验参数集名称合法（非空、无非法字符）。
func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("parameter set name is required")
	}
	return nil
}
