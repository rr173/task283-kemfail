package model

import "errors"

// 领域错误集合。HTTP 层据此映射为 400/404/409 等状态码。
var (
	// ErrRunNotFound 运行批次不存在。
	ErrRunNotFound = errors.New("run batch not found")
	// ErrParamNotFound 参数集不存在。
	ErrParamNotFound = errors.New("parameter set not found")
	// ErrDeviceNotFound 设备不存在。
	ErrDeviceNotFound = errors.New("device not found")
	// ErrEventNotFound 失败事件不存在。
	ErrEventNotFound = errors.New("failure event not found")
	// ErrCandidateNotFound 来源候选不存在。
	ErrCandidateNotFound = errors.New("source candidate not found")
	// ErrSnapshotNotFound 诊断快照不存在。
	ErrSnapshotNotFound = errors.New("diagnostic snapshot not found")

	// ErrRunSealed 运行已封存，禁止任何写入。
	ErrRunSealed = errors.New("run batch is sealed")
	// ErrDuplicateEvent 事件键已存在（幂等拒绝）。
	ErrDuplicateEvent = errors.New("duplicate failure event")
	// ErrParamLineageCycle 参数谱系存在环。
	ErrParamLineageCycle = errors.New("parameter lineage cycle detected")
	// ErrParamMissingBaseline 参数谱系缺少基线参数。
	ErrParamMissingBaseline = errors.New("parameter lineage missing baseline")
	// ErrParamVersionRegression 参数版本回退（子版本不高于父版本）。
	ErrParamVersionRegression = errors.New("parameter version regression")
	// ErrDeviceStateRegression 设备状态倒退（隔离→活跃需先确认恢复）。
	ErrDeviceStateRegression = errors.New("device state regression")
	// ErrInvalidDigest 密文摘要长度非法。
	ErrInvalidDigest = errors.New("invalid ciphertext digest length")
	// ErrSnapshotFrozen 快照已发布，禁止修改。
	ErrSnapshotFrozen = errors.New("snapshot is frozen")
	// ErrEventExcluded 事件已排除，禁止归因。
	ErrEventExcluded = errors.New("event is excluded")
	// ErrInvalidState 状态机非法流转。
	ErrInvalidState = errors.New("invalid state transition")
)
