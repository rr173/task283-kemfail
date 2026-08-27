# BENZHI 评测说明

基于 Go 实现的后量子密钥封装失败模式归因后端服务，一款后端服务，完成失败事件幂等接收与预分类、参数集谱系无环校验与版本递增检查、同参数同密文特征指纹归并、密文损坏/参数版本错配/设备硬件噪声三路来源评分、高噪声设备隔离降级与不可变诊断快照封存。

## 启动

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/kemfail --addr :8080 --db kemfail.db
```

## 自检（不启动长驻服务）

```bash
go run ./cmd/kemfail --smoke-test
```

`--smoke-test` 会真实创建运行批次、注册基线/子版本参数集与高噪声设备、写入失败事件与重复事件（幂等拒绝）、记录设备噪声、执行指纹归并与全量归因、确认候选、发布诊断快照并封存运行，关闭并重新打开数据库验证持久化与重启恢复，最后以 0 退出码结束。

## 构建门禁

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/kemfail --smoke-test
```

## HTTP API（前缀 /api）

运行批次：`POST /api/runs`、`GET /api/runs`、`GET /api/runs/{id}`、`POST /api/runs/{id}/seal`、`POST /api/runs/{id}/attribution`
参数集：`POST /api/params`、`GET /api/params?run_id=`、`GET /api/params/{id}`、`POST /api/params/{id}/baseline`、`GET /api/params/{id}/lineage`
设备：`POST /api/devices`、`GET /api/devices?run_id=`、`GET /api/devices/{id}`、`POST /api/devices/{id}/noise`、`.../isolate`、`.../recover`
失败事件：`POST /api/events`、`GET /api/events?run_id=`、`GET /api/events/{id}`、`POST /api/events/{id}/exclude`、`GET /api/fingerprints?run_id=`
来源候选：`GET /api/candidates?run_id=`、`POST /api/candidates/{id}/confirm`、`.../reject`
诊断快照：`POST /api/snapshots`、`GET /api/snapshots?run_id=`、`GET /api/snapshots/{id}`、`POST /api/snapshots/{id}/publish`、`.../supersede`
统计与自检：`GET /api/stats?run_id=`、`GET /api/selfcheck`、`GET /api/health`

## 持久化

SQLite（modernc.org/sqlite，CGO 无关）。建表：run_batches、param_sets、devices、failure_events、source_candidates、diagnostic_snapshots。事件键幂等；封存运行不可写入；冻结快照不改变既有归因证据。
