# Task 062 — Distributed Consensus Fencing & Safe etcd Defragmentation Coordinator (P1)
# Checkpoint: 2026-10-12T13:06:00+07:00

> **Severity**: P1 (Consensus Split-Brain, etcd Defrag Disk Spike, Distributed Lock GC Pause & PgBouncer Poisoning)  
> **Target Agent**: `database-engineer` / `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Monotonic Fencing Tokens, Leader Stepdown Defrag Coordination, PgBouncer Kill Clients Hook  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: etcd Defragmentation Gây Rớt Leader & Election Storm (Scenario 177)
* **Hiện tượng thực tế**: SRE chạy `etcdctl defrag` trực tiếp trên leader node. Quá trình defrag lock disk I/O trong 10 giây, leader không gửi kịp heartbeat Raft, các follower bầu cử liên tục làm tê liệt cụm K8s.
* **Căn nguyên**: Thiếu bộ điều phối (Coordinator) kiểm tra vai trò leader và thiếu cơ chế step-down trước khi defrag.

### Lỗi 2: Distributed Lock Lease Break Do Martin Kleppmann GC Pause Catastrophe (Scenario 178)
* **Hiện tượng thực tế**: Client A giữ lock Redis/etcd trong 10s, bị VM swap freeze 12s. Lock hết hạn, Client B lấy lock và bắt đầu ghi. Client A tỉnh lại tưởng mình còn lock và ghi đè dữ liệu Client B.
* **Căn nguyên**: Thiếu monotonic generation fencing token gửi kèm theo câu lệnh ghi xuống database.

### Lỗi 3: PgBouncer Connection Pool Poisoning Khi PostgreSQL Failover (Scenario 179)
* **Hiện tượng thực tế**: Patroni promote Replica lên Master mới, Node cũ thành Read-Only. PgBouncer vẫn giữ pool socket mở tới node cũ -> Ứng dụng ném lỗi `cannot execute INSERT in a read-only transaction`.
* **Căn nguyên**: Thiếu post-failover hook tự động `PAUSE` và ngắt kết nối stale socket trên PgBouncer.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/consensus/coordinator.go`:
```go
package consensus

import (
    "context"
    "time"
)

type EtcdMemberStatus struct {
    ID         uint64 `json:"id"`
    Endpoint   string `json:"endpoint"`
    IsLeader   bool   `json:"is_leader"`
    DbSize     int64  `json:"db_size"`
    DbSizeInUse int64 `json:"db_size_in_use"`
    RaftIndex  uint64 `json:"raft_index"`
}

type FencingToken struct {
    LockKey    string `json:"lock_key"`
    OwnerID    string `json:"owner_id"`
    Generation int64  `json:"generation"` // Monotonically increasing
    ExpiresAt  time.Time `json:"expires_at"`
}

type ConsensusCoordinator interface {
    AcquireLockWithFencing(ctx context.Context, key, ownerID string, ttl time.Duration) (*FencingToken, error)
    ValidateFencingToken(ctx context.Context, key string, token int64) (bool, error)
    SafeDefragCluster(ctx context.Context, endpoints []string) error
    FlushPgBouncerStaleConnections(ctx context.Context, poolerAddr, newMasterAddr string) error
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Safe Rolling Defragmentation Coordinator
1. Lấy danh sách thành viên etcd qua `member list` và `endpoint status`.
2. Xác định Node đang giữ cờ `IsLeader == true`.
3. Duyệt qua tất cả các **Non-Leader Nodes**:
   - Gửi lệnh `defrag` tới từng endpoint non-leader.
   - Chờ hoàn tất và theo dõi metric `etcd_disk_wal_fsync_duration_seconds`.
   - Nghỉ 30 giây (Cooldown buffer) trước khi sang node tiếp theo.
4. Đối với **Leader Node**:
   - Gửi lệnh `move-leader` sang một trong các node non-leader đã hoàn tất defrag.
   - Chờ leader election mới ổn định (Raft leader ID thay đổi).
   - Tiến hành defrag node cũ (nay đã là follower).

### Quy trình 2: Fencing Token Enforcement
1. Mỗi khi client acquire lock thành công:
   - Atomic increment generation counter: `new_gen = INCR "fencing_gen:" + key`.
   - Gắn `new_gen` vào `FencingToken`.
2. Khi client ghi dữ liệu xuống database:
   - Đính kèm câu truy vấn SQL: `UPDATE ... WHERE fencing_token < $new_gen`.
   - Nếu $0$ row affected: Ném lỗi `ERR_FENCING_TOKEN_STALE`.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_ETCD_DEFRAG_LEADER_BLOCKED`: `HTTP 409 Conflict` — "Cannot defragment etcd leader directly; transfer leadership first."
* `ERR_FENCING_TOKEN_STALE`: `HTTP 412 Precondition Failed` — "Write rejected: Distributed lock generation is stale (current: %d, presented: %d)."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestEtcd_SafeDefrag_StepdownLeaderFirst`: Mock cụm 3 node etcd, assert hàm chuyển quyền leader trước khi defrag node ban đầu.
2. `TestFencingToken_StaleWrite_Rejected`: Acquire token gen 10, giả lập client khác acquire token gen 11, assert write của token gen 10 bị từ chối.
3. `TestPgBouncer_PostFailover_DisconnectsOldMaster`: Giả lập failover, assert hàm gửi lệnh ngắt connection tới node cũ thành công.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Tuyệt đối không defrag cụm etcd khi số lượng healthy members $< (N/2 + 1)$ (mất quorum).
