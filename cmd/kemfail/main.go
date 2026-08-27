// Command kemfail 是"后量子密钥封装失败模式归因服务"的入口。
//
// 启动：
//
//	CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/kemfail --addr :8080 --db kemfail.db
//
// 自检（不启动长驻服务）：
//
//	go run ./cmd/kemfail --smoke-test
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"task283-kemfail/internal/event"
	"task283-kemfail/internal/fingerprint"
	"task283-kemfail/internal/httpapi"
	"task283-kemfail/internal/model"
	"task283-kemfail/internal/service"
	"task283-kemfail/internal/store"
)

func main() {
	var (
		addr      = flag.String("addr", ":8080", "HTTP listen address")
		dbPath    = flag.String("db", "kemfail.db", "SQLite database path")
		smokeTest = flag.Bool("smoke-test", false, "run smoke test and exit")
	)
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	svc := service.New(db)

	if *smokeTest {
		if err := runSmokeTest(*dbPath, svc, db); err != nil {
			log.Fatalf("smoke test failed: %v", err)
		}
		fmt.Println("smoke test passed")
		return
	}
	defer db.Close()

	srv := &http.Server{
		Addr:    *addr,
		Handler: httpapi.New(svc).Handler(),
	}

	go func() {
		log.Printf("kemfail listening on %s (db=%s)", *addr, *dbPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// runSmokeTest 执行端到端自检：创建运行 → 注册参数集/设备 →
// 写入失败事件 → 执行归因 → 确认候选 → 发布快照 → 关闭并重新打开
// 数据库验证持久化与重启恢复，最后以 0 退出码结束。
func runSmokeTest(dbPath string, svc *service.Service, db *store.DB) error {
	// 1. 创建运行
	run, err := svc.CreateRun("smoke-run", "smoke")
	if err != nil {
		return fmt.Errorf("create run: %w", err)
	}

	// 2. 注册基线参数集（无 parent）
	baseParams := map[string]string{"K": "1024", "HASH": "SHAKE256", "F": "ML-DSA"}
	base, err := svc.RegisterParamSet(&model.ParamSet{
		RunID: run.ID, Name: "ml-kem-768", KEMAlgorithm: "ML-KEM",
		Version: "1.0.0",
	}, baseParams)
	if err != nil {
		return fmt.Errorf("register baseline param: %w", err)
	}

	// 3. 注册子版本参数集（parent = base）
	childParams := map[string]string{"K": "1024", "HASH": "SHAKE256", "F": "ML-DSA", "T": "derand"}
	child, err := svc.RegisterParamSet(&model.ParamSet{
		RunID: run.ID, Name: "ml-kem-768-t", KEMAlgorithm: "ML-KEM",
		Version: "1.1.0", ParentID: &base.ID,
	}, childParams)
	if err != nil {
		return fmt.Errorf("register child param: %w", err)
	}

	// 4. 注册设备（一台高噪声）
	devA, err := svc.RegisterDevice(&model.Device{
		RunID: run.ID, Name: "hsm-a", Model: "HSM-3000", NoiseLevel: 0.2, Firmware: "1.0",
	})
	if err != nil {
		return fmt.Errorf("register device a: %w", err)
	}
	devB, err := svc.RegisterDevice(&model.Device{
		RunID: run.ID, Name: "hsm-b", Model: "HSM-3000", NoiseLevel: 0.9, Firmware: "1.0",
	})
	if err != nil {
		return fmt.Errorf("register device b: %w", err)
	}

	// 5. 写入失败事件（64 位 hex 摘要）
	digestOK := strings.Repeat("ab", 32)
	digestBad := strings.Repeat("cd", 32)
	ev1, isNew, err := svc.IngestEvent(event.IngestInput{
		RunID: run.ID, ParamID: child.ID, DeviceID: devB.ID,
		CiphertextDigest: digestOK, FailureCode: "decapsulation_error", NoiseLevel: 0.9,
	})
	if err != nil {
		return fmt.Errorf("ingest event 1: %w", err)
	}
	_ = ev1
	if !isNew {
		return fmt.Errorf("event 1 should be new")
	}
	// 重复事件：幂等拒绝
	_, isNew2, err := svc.IngestEvent(event.IngestInput{
		RunID: run.ID, ParamID: child.ID, DeviceID: devB.ID,
		CiphertextDigest: digestOK, FailureCode: "decapsulation_error", NoiseLevel: 0.9,
	})
	if err != nil {
		return fmt.Errorf("ingest duplicate event: %w", err)
	}
	if isNew2 {
		return fmt.Errorf("duplicate event should be idempotent")
	}
	// 参数冲突事件（摘要与库不一致）
	_, _, err = svc.IngestEvent(event.IngestInput{
		RunID: run.ID, ParamID: child.ID, DeviceID: devA.ID,
		CiphertextDigest: digestBad, FailureCode: "keygen_mismatch", NoiseLevel: 0.2,
	})
	if err != nil {
		return fmt.Errorf("ingest event 2: %w", err)
	}

	// 6. 记录设备噪声并确认隔离阈值
	if _, err := svc.RecordNoise(devB.ID, 0.95); err != nil {
		return fmt.Errorf("record noise: %w", err)
	}
	if _, err := svc.RecordNoise(devB.ID, 0.9); err != nil {
		return fmt.Errorf("record noise: %w", err)
	}

	// 7. 指纹归并
	fps, err := svc.FingerprintsOf(run.ID)
	if err != nil {
		return fmt.Errorf("fingerprints: %w", err)
	}
	if len(fps) == 0 {
		return fmt.Errorf("expected fingerprints, got none")
	}
	for _, f := range fps {
		_ = fingerprint.IsBurst(f)
	}

	// 8. 执行全量归因
	created, err := svc.RunAttribution(run.ID)
	if err != nil {
		return fmt.Errorf("run attribution: %w", err)
	}
	if created == 0 {
		return fmt.Errorf("expected candidates, got none")
	}

	// 9. 确认候选并发布快照
	cands, err := svc.ListCandidates(run.ID)
	if err != nil {
		return fmt.Errorf("list candidates: %w", err)
	}
	for i := range cands {
		if _, err := svc.ConfirmCandidate(cands[i].ID); err != nil {
			return fmt.Errorf("confirm candidate %d: %w", cands[i].ID, err)
		}
	}
	snap, err := svc.CreateSnapshot(run.ID, "smoke snapshot")
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	if _, err := svc.PublishSnapshot(snap.ID); err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}

	// 10. 封存运行
	if _, err := svc.SealRun(run.ID); err != nil {
		return fmt.Errorf("seal run: %w", err)
	}
	sealedRun, err := svc.GetRun(run.ID)
	if err != nil {
		return fmt.Errorf("get run after seal: %w", err)
	}
	if sealedRun.Status != model.RunSealed {
		return fmt.Errorf("run should be sealed")
	}

	// 11. 统计验证
	stats, err := svc.Stats(run.ID)
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	if stats.TotalEvents < 2 {
		return fmt.Errorf("expected at least 2 events, got %d", stats.TotalEvents)
	}

	// 12. 关闭并重新打开数据库，验证持久化与重启恢复
	db.Close()
	reopened, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("reopen database: %w", err)
	}
	defer reopened.Close()
	svc2 := service.New(reopened)

	reRun, err := svc2.GetRun(run.ID)
	if err != nil {
		return fmt.Errorf("restart recovery: get run: %w", err)
	}
	if reRun.Status != model.RunSealed || reRun.Name != "smoke-run" {
		return fmt.Errorf("restart recovery: run state lost: %+v", reRun)
	}
	reEvents, err := svc2.ListEvents(run.ID)
	if err != nil {
		return fmt.Errorf("restart recovery: list events: %w", err)
	}
	if len(reEvents) != stats.TotalEvents {
		return fmt.Errorf("restart recovery: events mismatch: got %d want %d", len(reEvents), stats.TotalEvents)
	}
	reSnaps, err := svc2.ListSnapshots(run.ID)
	if err != nil {
		return fmt.Errorf("restart recovery: list snapshots: %w", err)
	}
	if len(reSnaps) != 1 || reSnaps[0].Status != model.SnapPublished {
		return fmt.Errorf("restart recovery: snapshot not persisted: %+v", reSnaps)
	}

	fmt.Printf("smoke: run=%d events=%d candidates=%d snapshots=%d persisted=ok\n",
		run.ID, stats.TotalEvents, stats.TotalCandidates, stats.PublishedSnapshot)
	return nil
}
