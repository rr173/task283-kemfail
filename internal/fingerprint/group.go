// Package fingerprint 实现失败指纹的归并：把"同参数 + 同密文特征"
// 的多次失败折叠为单一指纹，识别重复与批量模式。
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// Fingerprint 表示一组失败事件的共性指纹。
type Fingerprint struct {
	Key        string   `json:"key"`         // 指纹键
	ParamName  string   `json:"param_name"`  // 参数集名称
	FailureCode string  `json:"failure_code"` // 失败码
	DeviceIDs  []int64  `json:"device_ids"`  // 涉及设备（去重排序）
	EventCount int      `json:"event_count"` // 归并事件数
	DigestPrefix string `json:"digest_prefix"`
}

// Compute 计算一条失败事件的指纹键：param_id + failure_code + digest 前 16 位。
func Compute(paramID int64, failureCode, digest string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s|%s", paramID, failureCode, short(digest, 16))
	return hex.EncodeToString(h.Sum(nil))
}

// Group 按指纹键归并事件列表。返回指纹集合（按键排序，稳定输出）。
func Group(events []KeyedEvent) []Fingerprint {
	byKey := map[string]*Fingerprint{}
	for _, e := range events {
		k := Compute(e.ParamID, e.FailureCode, e.Digest)
		f, ok := byKey[k]
		if !ok {
			f = &Fingerprint{
				Key:          k,
				ParamName:    e.ParamName,
				FailureCode:  e.FailureCode,
				DigestPrefix: short(e.Digest, 16),
			}
			byKey[k] = f
		}
		f.EventCount++
		if !contains(f.DeviceIDs, e.DeviceID) {
			f.DeviceIDs = append(f.DeviceIDs, e.DeviceID)
		}
	}
	out := make([]Fingerprint, 0, len(byKey))
	for _, f := range byKey {
		sort.Slice(f.DeviceIDs, func(i, j int) bool { return f.DeviceIDs[i] < f.DeviceIDs[j] })
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// IsBurst 判断指纹是否构成批量爆发：事件数 >= 3 且涉及设备 >= 2。
// 批量爆发提示共享根因（如参数版本错配或公共噪声），而非单机随机。
func IsBurst(f Fingerprint) bool {
	return f.EventCount >= 3 && len(f.DeviceIDs) >= 2
}

// KeyedEvent 是 Group 的输入行。
type KeyedEvent struct {
	ParamID    int64
	ParamName  string
	FailureCode string
	Digest     string
	DeviceID   int64
}

func short(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func contains(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
