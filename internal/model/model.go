// Package model 定义后量子密钥封装（KEM）失败模式归因服务的领域实体、
// 状态机与共享错误。所有表结构、状态流转与不变量均以本包为准。
package model

import (
	"time"
)

// RunStatus 表示一个归因运行批次的整体状态。
//
// 状态机：接收中 receiving → 待归因 pending → 需复核 review → 已发布 published → 封存 sealed。
// 封存后任何写入与状态变更均被拒绝。
type RunStatus string

const (
	RunReceiving RunStatus = "receiving" // 接收中：允许写入失败事件与参数集
	RunPending   RunStatus = "pending"   // 待归因：已接收完毕，等待自动归因
	RunReview    RunStatus = "review"    // 需复核：归因完成，等待人工确认
	RunPublished RunStatus = "published" // 已发布：诊断快照已发布
	RunSealed    RunStatus = "sealed"    // 封存：不可修改，固定参数基准
)

// EventStatus 表示单条解封装失败事件在归因过程中的状态。
//
// 状态机：原始 raw → 参数冲突 param_conflict / 噪声相关 noise_related / 重复 duplicate / 排除 excluded。
type EventStatus string

const (
	EventRaw          EventStatus = "raw"            // 原始：已接收尚未归因
	EventParamConflict EventStatus = "param_conflict" // 参数冲突：参数版本谱系错配导致
	EventNoiseRelated EventStatus = "noise_related"  // 噪声相关：设备硬件噪声导致
	EventDuplicate   EventStatus = "duplicate"       // 重复：与既有事件指纹相同
	EventExcluded    EventStatus = "excluded"        // 排除：人工排除，不参与归因
)

// CandidateKind 表示来源候选的类型：密文相关 / 参数相关 / 设备相关。
type CandidateKind string

const (
	CandCiphertext CandidateKind = "ciphertext" // 密文相关：密文损坏导致
	CandParam      CandidateKind = "param"      // 参数相关：参数版本错配导致
	CandDevice     CandidateKind = "device"     // 设备相关：硬件噪声导致
)

// CandidateStatus 表示来源候选的确认状态。
//
// 状态机：候选 candidate → 确认 confirmed / 否决 rejected。
type CandidateStatus string

const (
	CandOpen      CandidateStatus = "candidate" // 候选：待人工确认
	CandConfirmed CandidateStatus = "confirmed" // 确认：人工确认该来源
	CandRejected  CandidateStatus = "rejected"  // 否决：人工否决该来源
)

// SnapshotStatus 表示诊断快照的状态。
//
// 状态机：草稿 draft → 发布 published → 替代 superseded。发布后不可修改；
// 替代通过创建新快照并旧快照置 superseded 完成。
type SnapshotStatus string

const (
	SnapDraft      SnapshotStatus = "draft"      // 草稿：尚未发布
	SnapPublished  SnapshotStatus = "published"  // 发布：对外可见，固定参数基准
	SnapSuperseded SnapshotStatus = "superseded" // 替代：已被新快照取代
)

// DeviceStatus 表示硬件设备的状态。
//
// 状态机：活跃 active ↔ 隔离 isolated。隔离后该设备事件只读标记，
// 不再产生新的来源候选，直到解除隔离。
type DeviceStatus string

const (
	DevActive   DeviceStatus = "active"   // 活跃：正常参与归因
	DevIsolated DeviceStatus = "isolated" // 隔离：暂停参与归因
)

// ParamSet 表示一次封装所使用的参数集（算法、版本、共享参数摘要）。
// 参数集通过 ParentID 构成版本谱系链，用于检测参数版本错配。
type ParamSet struct {
	ID           int64     `json:"id"`
	RunID        int64     `json:"run_id"`
	Name         string    `json:"name"`
	KEMAlgorithm string    `json:"kem_algorithm"`
	Version      string    `json:"version"`
	ParentID     *int64    `json:"parent_id,omitempty"`
	ParamsDigest string    `json:"params_digest"`
	IsBaseline   bool      `json:"is_baseline"`
	CreatedAt    time.Time `json:"created_at"`
}

// Device 表示执行封装的硬件设备及其实时噪声状态。
type Device struct {
	ID          int64        `json:"id"`
	RunID       int64        `json:"run_id"`
	Name        string       `json:"name"`
	Model       string       `json:"model"`
	Status      DeviceStatus `json:"status"`
	NoiseLevel  float64      `json:"noise_level"`  // 0..1 归一化噪声强度
	NoiseEvents int          `json:"noise_events"` // 观测到的噪声事件计数
	Firmware    string       `json:"firmware"`
	CreatedAt   time.Time    `json:"created_at"`
}

// FailureEvent 表示一条解封装失败事件。EventKey 是幂等键（run_id + param_id +
// device_id + ciphertext_digest 的前缀），重复写入被拒绝。
type FailureEvent struct {
	ID               int64       `json:"id"`
	RunID            int64       `json:"run_id"`
	ParamID          int64       `json:"param_id"`
	DeviceID         int64       `json:"device_id"`
	EventKey         string      `json:"event_key"`
	CiphertextDigest string      `json:"ciphertext_digest"`
	FailureCode      string      `json:"failure_code"`
	Status           EventStatus `json:"status"`
	ReceivedAt       time.Time   `json:"received_at"`
}

// SourceCandidate 表示归因引擎为一次失败产生的来源候选及评分。
type SourceCandidate struct {
	ID          int64           `json:"id"`
	RunID       int64           `json:"run_id"`
	EventID     int64           `json:"event_id"`
	Kind        CandidateKind   `json:"kind"`
	Score       float64         `json:"score"`
	Evidence    string          `json:"evidence"`
	Status      CandidateStatus `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
}

// DiagnosticSnapshot 表示归因诊断快照：固定参数基准与候选集，
// 发布后不可修改，可被新快照替代。
type DiagnosticSnapshot struct {
	ID            int64          `json:"id"`
	RunID         int64          `json:"run_id"`
	Title         string         `json:"title"`
	Status        SnapshotStatus `json:"status"`
	ParamBaseline int64          `json:"param_baseline"`
	CandidateIDs  []int64        `json:"candidate_ids"`
	CreatedAt     time.Time      `json:"created_at"`
	PublishedAt   *time.Time     `json:"published_at,omitempty"`
}

// RunBatch 表示一次归因运行批次。
type RunBatch struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     RunStatus `json:"status"`
	Owner      string    `json:"owner"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	SealedAt   *time.Time `json:"sealed_at,omitempty"`
}

// Stats 表示运行级别的统计摘要。
type Stats struct {
	RunID             int64  `json:"run_id"`
	TotalEvents       int    `json:"total_events"`
	ParamConflict     int    `json:"param_conflict"`
	NoiseRelated      int    `json:"noise_related"`
	Duplicates        int    `json:"duplicates"`
	Excluded          int    `json:"excluded"`
	TotalCandidates   int    `json:"total_candidates"`
	Confirmed         int    `json:"confirmed"`
	Rejected          int    `json:"rejected"`
	IsolatedDevices   int    `json:"isolated_devices"`
	PublishedSnapshot int    `json:"published_snapshot"`
}
