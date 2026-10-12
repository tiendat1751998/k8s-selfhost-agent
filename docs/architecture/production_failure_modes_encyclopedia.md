# Master Production Failure Modes & Effects Analysis (FMEA) Encyclopedia
# Checkpoint: 2026-10-12T10:35:00+07:00

> **Project**: `k8sseflhost` — Hybrid Enterprise DevOps & DevSecOps Platform  
> **Status**: BINDING_PRODUCTION_REFERENCE_CATALOG  
> **Target Audience**: Principal DevOps Engineers, SRE Leads, Security Analysts, Autonomous Subagents  
> **Purpose**: Serves as the definitive operational memory and edge-case encyclopedia for the All-in-One DevOps Command Center & Second Brain.

---

## MỤC LỤC 9 TẦNG SỰ CỐ VẬN HÀNH THỰC CHIẾN

- [TẦNG 1: Phần Cứng Bare-Metal, Linux Kernel & System Calls](#tầng-1-phần-cứng-bare-metal-linux-kernel--system-calls)
- [TẦNG 2: Kubernetes Control Plane & etcd Cluster](#tầng-2-kubernetes-control-plane--etcd-cluster)
- [TẦNG 3: Worker Nodes, Kubelet & Container Runtime](#tầng-3-worker-nodes-kubelet--container-runtime)
- [TẦNG 4: Kubernetes Networking, Ingress & DNS](#tầng-4-kubernetes-networking-ingress--dns)
- [TẦNG 5: Lưu Trữ Phân Tán, CSI & Persistent Volumes](#tầng-5-lưu-trữ-phân-tán-csi--persistent-volumes)
- [TẦNG 6: Cơ Sở Dữ Liệu Chuyên Sâu (Postgres, MySQL, Redis, Mongo)](#tầng-6-cơ-sở-dữ-liệu-chuyên-sâu-postgres-mysql-redis-mongo)
- [TẦNG 7: CI/CD, GitOps & Dựng Ảnh Container](#tầng-7-cicd-gitops--dựng-ảnh-container)
- [TẦNG 8: DevSecOps, Zero-Trust & Identity](#tầng-8-devsecops-zero-trust--identity)
- [TẦNG 9: SRE, Disaster Recovery & Sự Cố Khẩn Cấp](#tầng-9-sre-disaster-recovery--sự-cố-khẩn-cấp)

---

## TẦNG 1: PHẦN CỨNG BARE-METAL, LINUX KERNEL & SYSTEM CALLS

### 1. NVMe/SSD Wear-Out & Chuyển Sang Read-Only Đột Ngột
* **Cơ chế lỗi**: Ổ đĩa SSD ghi cạn chỉ số TBW (Total Bytes Written). Controller bảo vệ dữ liệu bằng cách khóa cứng chip flash sang chế độ Read-Only vĩnh viễn. Mọi lệnh ghi DB panic, hệ điều hành không thể tạo socket/file tạm.
* **Tác động SRE**: Database dừng hoạt động tức thời, pod CrashLoop, không thể ghi WAL/log.
* **Cơ chế tự động hóa**: Đọc chỉ số `Percentage Used` từ NVMe SMART; cảnh báo sớm khi wear-out đạt 85% và 95% qua `Task 029`.

### 2. Hardware RAID BBU (Battery Backup Unit) Hỏng $\implies$ IOPS Rớt 95%
* **Cơ chế lỗi**: Pin dự phòng trên card RAID bị chai/chết. Card tự động chuyển từ chế độ `Write-Back` sang `Write-Through` để tránh mất dữ liệu khi cúp điện. Tốc độ ghi disk từ 80,000 IOPS tụt dốc không phanh xuống 800 IOPS, kéo sập toàn bộ database.
* **Cơ chế tự động hóa**: Quét định kỳ trạng thái pin qua `megacli` / `storcli` CLI.

### 3. Cạn Kiệt Inode Dù Ổ Cứng Còn Trống Hàng Trăm GB
* **Cơ chế lỗi**: Các ứng dụng microservice hoặc cronjob sinh ra hàng triệu file session/cache nhỏ (1KB) lấp đầy 100% Inodes (`df -i` báo 100% dù `df -h` mới dùng 10%). Hệ điều hành từ chối tạo bất kỳ file nào mới.
* **Cơ chế tự động hóa**: Giám sát metric `node_filesystem_files_free`, tự động dọn dẹp các thư mục temp/spool khi inode < 10%.

### 4. Transparent Huge Pages (THP) Gây Đột Biến Latency 5-10 Giây
* **Cơ chế lỗi**: Tính năng THP của Linux Kernel gom bộ nhớ thành các trang 2MB. Khi kernel gom trang (memory compaction), nó khóa (lock) tiến trình, gây ra các cú spike latency từ 5 đến 10 giây trên Redis và PostgreSQL.
* **Cơ chế tự động hóa**: Ansible Day-1 tự động cưỡng chế `echo never > /sys/kernel/mm/transparent_hugepage/enabled`.

### 5. NUMA Node Imbalance Làm Tụt 40% Hiệu Năng CPU
* **Cơ chế lỗi**: Máy chủ 2 CPU Socket. Tiến trình chạy trên Socket 0 nhưng phải đọc bộ nhớ RAM gắn trên Socket 1 qua bus QPI/UPI, gây thắt cổ chai băng thông bộ nhớ.
* **Cơ chế tự động hóa**: Kích hoạt CPU Manager policy `static` trên Kubelet để pin CPU và gán bộ nhớ NUMA cục bộ cho các workload độ trễ thấp.

### 6. Bão Ngắt Mạng (Softirq Saturation) Chiếm Trọn 1 Core CPU
* **Cơ chế lỗi**: Card mạng 10Gbps nhận lưu lượng lớn nhưng không bật RSS (Receive Side Scaling), toàn bộ ngắt mạng dồn vào Core 0 (`ksoftirqd/0` ăn 100% CPU), khiến Core 0 đơ và drop hàng loạt packet.
* **Cơ chế tự động hóa**: Tự động cấu hình `smp_affinity` phân bổ ngắt mạng đều trên tất cả CPU cores.

### 7. Cạn Kiệt Ephemeral Ports & Trạng Thái TIME_WAIT Treo
* **Cơ chế lỗi**: Microservice gọi HTTP client không tái sử dụng connection (`DisableKeepAlives: true`), làm cạn kiệt toàn bộ dải cổng 32768-60999, ứng dụng báo lỗi `bind: address already in use`.
* **Cơ chế tự động hóa**: Tinh chỉnh `net.ipv4.tcp_tw_reuse = 1` và mở rộng `net.ipv4.ip_local_port_range = 1024 65535`.

### 8. SYN Flood Queue Overflow (`tcp_max_syn_backlog`)
* **Cơ chế lỗi**: Lưu lượng người dùng tăng đột biến, hàng đợi SYN bị đầy, kernel âm thầm vứt bỏ kết nối mới mà không phản hồi.
* **Cơ chế tự động hóa**: Nâng `net.ipv4.tcp_max_syn_backlog = 65535` và kích hoạt `net.ipv4.tcp_syncookies = 1`.

### 9. Lệch Pha Đồng Hồ Hệ Thống (Time Drift / NTP Desynchronization)
* **Cơ chế lỗi**: Mạng Air-Gapped không sync NTP làm đồng hồ lệch > 500ms, etcd liên tục mất leader, token JWT bị từ chối 100%.
* **Cơ chế tự động hóa**: Chrony Master tự động đồng bộ giờ nội bộ, watchdog cảnh báo qua `adjtimex` khi offset > 100ms (`Task 029`).

---

## TẦNG 2: KUBERNETES CONTROL PLANE & ETCD CLUSTER

### 10. etcd Backend Quota Exceeded (Chạm Ngưỡng 2GB/8GB)
* **Cơ chế lỗi**: etcd không được bật auto-compaction. Lịch sử revision của events và secret làm etcd chạm ngưỡng dung lượng `quota-backend-bytes`. etcd kích hoạt cờ báo động `NOSPACE` và **chuyển 100% sang chế độ Read-Only**, từ chối mọi lệnh tạo/cập nhật pod trên toàn cụm.
* **Cơ chế tự động hóa**: Tự động chạy cronjob `etcdctl defrag` và `etcdctl compact` khi dung lượng chạm 70% quota.

### 11. etcd Disk Sync Latency Spike (> 10ms Fsync Stall)
* **Cơ chế lỗi**: Ổ cứng chạy etcd dùng chung với các workload nặng ghi I/O. Khi `wal fsync` vượt quá 10ms, Raft heartbeat bị drop, etcd mất leader và khởi động lại liên tục.
* **Cơ chế tự động hóa**: Giám sát metric `etcd_disk_wal_fsync_duration_seconds`.

### 12. API Server OOM Crash Do Lệnh List Resource Không Phân Trang
* **Cơ chế lỗi**: Một controller hoặc script nội bộ gọi `kubectl get pods --all-namespaces` trên cụm có 50,000 pods mà không dùng pagination (`limit` / `continue`). API Server load toàn bộ JSON hàng trăm MB vào RAM, kích hoạt OOM Killer làm sập apiserver.
* **Cơ chế tự động hóa**: API Priority and Fairness (APF) giới hạn concurrency; bắt buộc phân trang cho các query lớn.

### 13. Deadlock Do Validating/Mutating Admission Webhook Chết
* **Cơ chế lỗi**: Một webhook bảo mật bên thứ 3 (như OPA/Kyverno) bị crash hoặc timeout. Mặc định webhook đặt `failurePolicy: Fail`. Toàn bộ cụm K8s bị khóa cứng: không thể tạo pod mới, kể cả pod cứu hộ để sửa chính webhook đó!
* **Cơ chế tự động hóa**: Cấu hình `failurePolicy: Ignore` cho system namespaces; phát hiện webhook timeout và tự động chuyển chế độ break-glass (`Task 030`).

### 14. Hết Hạn Chứng Chỉ Control Plane 1 Năm (Kubeadm Certs Expiry)
* **Cơ chế lỗi**: Đủ 365 ngày kể từ khi dựng cụm, apiserver/kubelet certs hết hạn. Cụm tê liệt hoàn toàn sau một đêm.
* **Cơ chế tự động hóa**: Watchdog quét `kubeadm certs check-expiration` mỗi tuần; tự động renew trước 30 ngày qua Ansible (`Task 023`).

### 15. Treo Cụm Do Xóa CRD Khi Đang Còn Finalizer
* **Cơ chế lỗi**: DevOps xóa một Custom Resource hoặc Namespace, nhưng CR đó chứa một Finalizer trỏ vào controller đã bị xóa từ trước. Namespace kẹt vĩnh viễn ở trạng thái `Terminating`.
* **Cơ chế tự động hóa**: Runbook tự động phát hiện namespace kẹt Terminating và gỡ bỏ orphaned finalizers an toàn.

---

## TẦNG 3: WORKER NODES, KUBELET & CONTAINER RUNTIME

### 16. Kubelet PLEG (Pod Lifecycle Event Generator) Is Not Healthy
* **Cơ chế lỗi**: Container runtime (containerd/docker) bị treo do quá tải I/O đĩa hoặc kernel deadlock. Kubelet không thể nhận được event trạng thái container, đánh dấu node chuyển sang `NotReady`.
* **Cơ chế tự động hóa**: Giám sát metric `kubelet_pleg_relist_duration_seconds`; tự động restart containerd/CRI khi PLEG vượt ngưỡng 3 phút.

### 17. Thundering Herd Eviction Cascade (Domino Sập Cụm)
* **Cơ chế lỗi**: Node A bị cạn RAM, Kubelet kill 30 pod BestEffort/Burstable. 30 pod này dồn sang Node B. Node B lập tức quá tải và lại evict tiếp sang Node C, kéo sập lần lượt từng worker node trong cụm.
* **Cơ chế tự động hóa**: Node Eviction Rate Limiter; tự động Cordon node ngay khi RAM > 90% để ngăn nhận thêm pod mới.

### 18. CPU Throttling Bí Ẩn Do Cơ Chế CFS Quota Dù CPU Chưa Đầy 50%
* **Cơ chế lỗi**: Ứng dụng đa luồng (Go, Java) chia nhỏ tác vụ cho 16 threads. CFS quota cộng dồn thời gian CPU của cả 16 threads và làm chạm quota trong 20ms đầu tiên của chu kỳ 100ms. 80ms còn lại ứng dụng bị đóng băng (throttle), latency vọt lên dù CPU node mới dùng 30%.
* **Cơ chế tự động hóa**: Tự động phát hiện container bị throttle qua `container_cpu_cfs_throttled_periods_total`.

### 19. PodDisruptionBudget (PDB) Gây Treo Vĩnh Viễn Lệnh Node Drain/Upgrade
* **Cơ chế lỗi**: Service có 2 replica, PDB cấu hình `minAvailable: 2` (hoặc `maxUnavailable: 0`). Khi SRE drain node để bảo trì, lệnh drain kẹt vô tận vì không thể evict pod mà không vi phạm PDB.
* **Cơ chế tự động hóa**: Kiểm tra PDB trước khi drain; tự động scale up replica tạm thời lên 3 trước khi drain để thỏa mãn PDB.

---

## TẦNG 4: KUBERNETES NETWORKING, INGRESS & DNS

### 20. CNI IPAM Lease Leakage Gây Treo Pod Mới
* **Cơ chế lỗi**: Daemonset CNI crash làm rò rỉ IP allocation. Node còn thừa RAM/CPU nhưng pod mới kẹt vĩnh viễn ở `ContainerCreating` với lỗi: `no IP addresses available in range`.
* **Cơ chế tự động hóa**: Tự động dọn rác IPAM leak qua daemonset reconciler (`Task 026`).

### 21. CoreDNS Thundering Herd Do `ndots: 5`
* **Cơ chế lỗi**: Mỗi lookup domain ngoài thử 4 lần domain nội bộ, nhân gấp 5 lần số lượng DNS query khi downstream chậm, làm sập CoreDNS toàn cụm.
* **Cơ chế tự động hóa**: Tự động triển khai `NodeLocal DNSCache` và inject `ndots: 2` (`Task 026`).

### 22. ARP Cache Table Overflow Khi Cụm Flat Network Quá Lớn
* **Cơ chế lỗi**: Khi số lượng pod và node vượt quá 1024, bảng ARP của Linux chạm ngưỡng `net.ipv4.neigh.default.gc_thresh3`. Server ngừng phân giải MAC, mất kết nối mạng ngẫu nhiên giữa các pod.
* **Cơ chế tự động hóa**: Nâng `gc_thresh1/2/3` lên `1024/2048/4096`.

### 23. MTU Mismatch (1500 vs 1440 VXLAN) Gây Treo Gói Tin Lớn
* **Cơ chế lỗi**: Gói tin ping nhỏ thông suốt, nhưng TLS Handshake (3KB) bị switch vật lý drop âm thầm do cờ DF (Don't Fragment). Web app treo cứng không báo lỗi.
* **Cơ chế tự động hóa**: Pre-flight MTU auto-discovery và tự động cấu hình MTU CNI = Physical MTU - 60.

### 24. Ingress Buffer Spill Xuống Đĩa (`/tmp/proxy_temp`)
* **Cơ chế lỗi**: Client tải file hoặc response body lớn làm tràn RAM buffer, Ingress ghi xuống disk tạm gây tắc nghẽn I/O disk toàn Ingress node.
* **Cơ chế tự động hóa**: Bounded buffer trong RAM và cảnh báo I/O wait trên Ingress nodes.

---

## TẦNG 5: LƯU TRỮ PHÂN TÁN, CSI & PERSISTENT VOLUMES

### 25. CSI Multi-Attach Error Sau Khi Worker Node Chết Đột Ngột
* **Cơ chế lỗi**: Node mất nguồn vật lý, CSI không kịp unmount volume. K8s attachdetach-controller chờ 6 phút. Pod DB dời sang node mới kẹt vĩnh viễn ở `ContainerCreating` vì disk bị khóa trên node cũ.
* **Cơ chế tự động hóa**: Áp dụng taint `out-of-service` và gỡ `VolumeAttachment` an toàn (`Task 022`).

### 26. StorageClass Không Bật `allowVolumeExpansion` Gây Treo Mở Rộng Disk
* **Cơ chế lỗi**: Disk đầy 100%, DevOps sửa PVC nhưng StorageClass không hỗ trợ online expansion, PVC kẹt vĩnh viễn ở `FileSystemResizePending`.
* **Cơ chế tự động hóa**: Kiểm tra StorageClass capability trước khi patch PVC (`Task 022`).

### 27. Stale NFS / SMB Mounts Treo Kubelet Ở D-State (Uninterruptible Sleep)
* **Cơ chế lỗi**: Máy chủ NAS/NFS bị mất mạng. Các tiến trình đọc ghi file trên mount point bị treo vĩnh viễn ở D-State, lệnh `df -h` bị đơ, Kubelet kiểm tra disk space bị đơ theo, node chuyển sang `NotReady`.
* **Cơ chế tự động hóa**: Cấu hình cờ mount NFS `soft,timeo=30,retrans=2` thay vì `hard` mount vĩnh viễn.

---

## TẦNG 6: CƠ SỞ DỮ LIỆU CHUYÊN SÂU (POSTGRES, MYSQL, REDIS, MONGO)

### 28. PostgreSQL `idle_in_transaction` Cạn Kiệt Pool & Chặn Đứng `VACUUM`
* **Cơ chế lỗi**: Transaction mở quên đóng làm cạn `max_connections` và chặn `VACUUM`, khiến bảng phình to gấp 10 lần (Table Bloat).
* **Cơ chế tự động hóa**: Tự động kill kết nối treo $> 10s$ và cảnh báo Telegram (`Task 027`).

### 29. Transaction ID (XID) Wraparound Làm PostgreSQL Đóng Cửa Khẩn Cấp
* **Cơ chế lỗi**: Số lượng transaction vượt quá 2 tỷ mà autovacuum freeze không kịp dọn dẹp. PostgreSQL tự động **tắt ngấm server và từ chối khởi động lại** để tránh mất dữ liệu do trùng lặp XID.
* **Cơ chế tự động hóa**: Giám sát metric `datfrozenxid`; cảnh báo đỏ khi XID age > 1.5 tỷ transaction (`Task 027`).

### 30. Replication Lag Spikes Gây Hủy Truy Vấn Trên Read Replica
* **Cơ chế lỗi**: Standby chạy query dài hạn, Master gửi WAL yêu cầu dọn dead tuples $\implies$ Replica hủy ngang truy vấn (`canceling statement due to conflict with recovery`).
* **Cơ chế tự động hóa**: Cấu hình `hot_standby_feedback = on` và cân bằng `max_standby_archive_delay` (`Task 027`).

### 31. WAL Archiving Thất Bại Làm Đầy 100% Ổ Cứng Primary
* **Cơ chế lỗi**: Script đẩy WAL lên S3 bị lỗi mạng hoặc sai credential. Thư mục `pg_wal` không thể xóa WAL cũ, phình to chiếm sạch ổ cứng, khiến database crash ngay lập tức.
* **Cơ chế tự động hóa**: Giám sát dung lượng thư mục WAL và số lượng pending WAL files; cảnh báo khẩn cấp khi WAL chiếm > 30% disk.

### 32. DDL Migration Chặn Đứng Toàn Bộ Hệ Thống (Lock Contention)
* **Cơ chế lỗi**: Chạy `ALTER TABLE` thiếu `CONCURRENTLY` đòi hỏi `ACCESS EXCLUSIVE lock`, xếp hàng sau một query chậm và chặn đứng toàn bộ traffic API.
* **Cơ chế tự động hóa**: Bắt buộc `SET lock_timeout = '2s'` và advisory lock (`Task 020`).

### 33. Redis Memory Fragmentation Ratio Spike (> 1.5) Gây OOM Crash
* **Cơ chế lỗi**: Phân mảnh bộ nhớ jemalloc làm RSS RAM tăng vọt gấp 3 lần data thực tế.
* **Cơ chế tự động hóa**: Tự động kích hoạt `CONFIG SET activedefrag yes` khi ratio > 1.4 (`Task 027`).

### 34. Lệnh `KEYS *` Hoặc Big Key HGETALL Đóng Băng Redis
* **Cơ chế lỗi**: Redis là single-threaded. Chạy `KEYS *` trên 10 triệu keys làm Redis đơ trong 5 giây, toàn bộ API cache/session bị timeout đồng loạt.
* **Cơ chế tự động hóa**: Chặn lệnh `KEYS` qua `rename-command KEYS ""`; thay thế bằng `SCAN`.

### 35. Fork OOM Khi Redis Chạy BGSAVE / RDB Snapshot
* **Cơ chế lỗi**: Redis gọi `fork()` để ghi disk. Dưới cơ chế Copy-on-Write, nếu ứng dụng đang ghi nặng, dung lượng RAM cần thiết có thể nhân đôi ngay lập tức, kích hoạt OOM Killer.
* **Cơ chế tự động hóa**: Thiết lập `sysctl vm.overcommit_memory = 1`.

---

## TẦNG 7: CI/CD, GITOPS & DỰNG ẢNH CONTAINER

### 36. Lộ Quyền Root Máy Chủ Qua Mount `/var/run/docker.sock` Trong CI
* **Cơ chế lỗi**: Runner pod mount docker socket để build image $\implies$ hacker thoát container chiếm quyền root cả node.
* **Cơ chế tự động hóa**: Bắt buộc dùng **Kaniko** hoặc **BuildKit Rootless** không cần đặc quyền (`Task 030`).

### 37. DockerHub Rate Limit (HTTP 429) Làm Sập Auto-Scaling Cụm
* **Cơ chế lỗi**: Khi có traffic spike, HPA scale thêm 50 pod. Các worker nodes cùng kéo image từ DockerHub và bị chặn bởi rate limit 100 pulls/6 tiếng. Pod kẹt ở `ImagePullBackOff`.
* **Cơ chế tự động hóa**: Tự động cấu hình Private Harbor Registry Mirror cục bộ trong mạng Air-Gapped.

### 38. GitOps Drift Thầm Lặng (Manual Edit Bị Ghi Đè Đột Ngột)
* **Cơ chế lỗi**: Kỹ sư hotfix thủ công bằng `kubectl edit` trên cluster. Sau đó một commit không liên quan trên Git kích hoạt sync, xóa sạch hotfix mà không ai hay biết.
* **Cơ chế tự động hóa**: Background reconciler chủ động phát hiện drift và gửi cảnh báo so sánh diff về Telegram.

### 39. Bí Mật Rò Rỉ Vĩnh Viễn Trong Git History Dù Đã Xóa File
* **Cơ chế lỗi**: Commit `.env` rồi `git rm` thì key vẫn nằm trong commit tree và reflog.
* **Cơ chế tự động hóa**: Pre-receive webhook chặn đứng secret bằng Gitleaks trước khi commit vào remote repo.

---

## TẦNG 8: DEVSECOPS, ZERO-TRUST & IDENTITY

### 40. Secret Lọt Vào Log File & Memory Crash Dumps
* **Cơ chế lỗi**: Stack trace in toàn bộ chuỗi kết nối DB có password vào `/var/log/pods`.
* **Cơ chế tự động hóa**: Tầng lọc Regex In-Stream Redaction trên `k8s-agent` băm sạch credentials trước khi nén ZSTD (`Task 028`).

### 41. Secret Drift Giữa HashiCorp Vault và K8s Runtime
* **Cơ chế lỗi**: Vault xoay vòng mật khẩu DB, nhưng pod K8s không reload biến môi trường, tiếp tục dùng pass cũ và ăn lỗi 401.
* **Cơ chế tự động hóa**: Reloader controller tự động trigger rolling update từng pod khi secret thay đổi (`Task 023`).

### 42. Automount ServiceAccount Token Tạo Đường Xâm Nhập Ngang (Lateral Movement)
* **Cơ chế lỗi**: Pod mặc định tự động mount token của ServiceAccount tại `/var/run/secrets/kubernetes.io/serviceaccount`. Hacker chiếm được pod dùng token này quét toàn bộ API cụm.
* **Cơ chế tự động hóa**: Webhook tự động chèn `automountServiceAccountToken: false` cho tất cả pod không cần giao tiếp với API Server.

### 43. Cryptominer Qua Port Kubelet 10250 Unauthenticated
* **Cơ chế lỗi**: Cổng Kubelet mở ra ngoài với `anonymous: true` $\implies$ Hacker gọi API spawn pod đào coin ăn 100% CPU.
* **Cơ chế tự động hóa**: Ansible Day-1 đóng chặt firewall port 10250, bắt buộc webhook authorization (`Task 029`).

### 44. Báo Cáo Pass Giả Mạo Khi Scanner Gặp Lỗi (Fake Pass Antipattern)
* **Cơ chế lỗi**: Checkov/Trivy lỗi tự trả về 100% pass giả mạo.
* **Cơ chế tự động hóa**: Triệt tiêu mã fake pass, áp dụng nghiêm ngặt **The Fail-Close Invariant** (`Task 021`).

---

## TẦNG 9: SRE, DISASTER RECOVERY & SỰ CỐ KHẨN CẤP

### 45. "Schrödinger's Backup" (Tưởng Có Backup Nhưng Không Thể Restore)
* **Cơ chế lỗi**: Tối nào cũng báo backup thành công, nhưng chưa từng restore thử. Đến khi thảm họa thật lôi ra thì bản dump lỗi syntax hoặc thiếu khóa giải mã.
* **Cơ chế tự động hóa**: Hàng tuần tự động dựng sandbox tạm, kéo backup về restore test và đo RTO/RPO thực tế (`Task 025`).

### 46. Ransomware Xóa Sạch Bản Sao Lưu Trên S3/MinIO
* **Cơ chế lỗi**: Hacker lấy được root credential và gửi lệnh xóa snapshot backup trên cloud storage.
* **Cơ chế tự động hóa**: Kích hoạt S3 Object Lock ở chế độ `Compliance Mode` bất biến (WORM storage).

### 47. Deploy "Big Bang" 100% Làm Sập Toàn Hệ Thống
* **Cơ chế lỗi**: Deploy thẳng phiên bản mới lên 100% pods, gặp lỗi deadlock DB làm chết toàn bộ cụm.
* **Cơ chế tự động hóa**: Điều phối Canary tự động $10\% \to 25\% \to 50\% \to 100\%$, tự động abort về $0\%$ nếu lỗi tăng $> 0.5\%$ (`Task 031`).

### 48. Bão Cảnh Báo (Alert Storm & Alert Fatigue)
* **Cơ chế lỗi**: Một switch mạng sập sinh ra 1500 cảnh báo trong 2 phút làm tràn ngập điện thoại kỹ sư.
* **Cơ chế tự động hóa**: Thuật toán khử trùng lặp và gom cụm sự cố theo time-window 3 phút trên Telegram bot (`Task 023`).

### 49. Sập Nhà Cung Cấp Định Danh (SSO / IdP Outage)
* **Cơ chế lỗi**: Okta / Keycloak sập làm kỹ sư SRE không thể đăng nhập Web UI hoặc K8s để cứu trợ hệ thống.
* **Cơ chế tự động hóa**: Cơ chế Local Break-Glass Account qua phần cứng YubiKey / MFA cục bộ.

### 50. Cascading Reconnection Storm Khi Khôi Phục Dịch Vụ
* **Cơ chế lỗi**: Sau khi database hoặc API server vừa bật lại, hàng triệu client/app đồng loạt reconnect và gửi lại query dồn ứ cùng 1 giây, đánh sập hệ thống lần 2 ngay lập tức.
* **Cơ chế tự động hóa**: Áp dụng Exponential Backoff with Jitter ở tất cả client và cơ chế Adaptive Load Shedding (`Task 031`).

---

## TẦNG 10: SERVICE MESH, LƯU LƯỢNG NÂNG CAO & REDOS (KỊCH BẢN 51 — 55)

### 51. Istio Sidecar Startup Race Condition (`holdApplicationUntilProxyStarts`)
* **Cơ chế lỗi**: Kubelet mặc định khởi chạy container ứng dụng chính và container `istio-proxy` (Envoy) song song. Ứng dụng khởi động nhanh hơn Envoy và gửi ngay network call ra ngoài (DB migration, fetch Vault secret). `iptables` đã chuyển hướng egress sang port 15001 của Envoy nhưng Envoy chưa nạp xong config từ Istiod, dẫn đến `ECONNREFUSED` hoặc `RST`.
* **Cơ chế tự động hóa**: Cưỡng chế `holdApplicationUntilProxyStarts: true` (hoặc inject annotation `proxy.istio.io/config`); trên K8s 1.28+, áp dụng Native Sidecar Container với `restartPolicy: Always`.

### 52. Envoy Memory Leak do HTTP/2 Multiplexed Stream & Connection Pooling Idle Accumulation
* **Cơ chế lỗi**: Các ứng dụng gRPC hoặc microservice HTTP/2 duy trì persistent connection thời gian dài. Nếu upstream server hoặc client không chủ động gửi `GOAWAY` frame định kỳ khi kết nối nhàn rỗi, `tcmalloc` của Envoy bị phân mảnh và giữ lại các heap chunk đã giải phóng. Envoy memory footprint tăng tuyến tính dẫn đến OOM-Killed (`ExitCode 137`).
* **Cơ chế tự động hóa**: Cấu hình `max_requests_per_connection: 10000` và `idle_timeout: 300s` trong Envoy cluster configuration; thiết lập tcmalloc background release rate qua `MALLOC_CONF="background_thread:true,dirty_decay_ms:10000"`.

### 53. In-Flight Connection Drop do mTLS Certificate Rotation mà Thiếu Graceful Drain
* **Cơ chế lỗi**: Trong kiến trúc Zero-Trust Service Mesh, chứng chỉ x509 ngắn hạn (24h) được SDS tự động renew. Khi TLS Context được reload trong Envoy, nếu thiếu drainage window (`drain_time: 0s`), các kết nối streaming kéo dài (WebSocket, SSE, gRPC bidirectional streams) bị ngắt đột ngột với TLS alert `bad_certificate` hoặc `close_notify`.
* **Cơ chế tự động hóa**: Thiết lập Envoy `drain_time` tối thiểu 45s và `parent_shutdown_time_seconds: 60s` trong `ProxyConfig`; telemetry watchdog kiểm tra delta của `envoy_cluster_ssl_connection_error` trước và sau cert renewal event.

### 54. HTTP Desync / Request Smuggling Bypass WAF do `Transfer-Encoding` vs `Content-Length` Mismatch
* **Cơ chế lỗi**: Giữa Edge Ingress (HAProxy/Envoy) và Backend Service (Node.js/Python/Go) có sự không thống nhất trong cách parse HTTP/1.1 request headers chứa cả `Transfer-Encoding: chunked` và `Content-Length`. Kẻ tấn công gửi request dị biệt (CL.TE hoặc TE.CL desync), phần thân request dư thừa bị sót lại trong socket buffer và biến thành header của request từ người dùng kế tiếp.
* **Cơ chế tự động hóa**: Cấu hình Ingress cưỡng chế `strip_matching_host_port: true`, `merge_slashes: true`, và `headers_with_underscores_action: REJECT_REQUEST`; bật cờ RFC 7230 strict compliance từ chối ngay HTTP 400 nếu request chứa cả `Transfer-Encoding` và `Content-Length`.

### 55. Thảm Họa Cloudflare 2019: Catastrophic Regex Backtracking (ReDoS) Khóa Cứng CPU Trong WAF
* **Cơ chế lỗi**: Post-mortem Cloudflare 2019: Một rule WAF mới chứa regex dạng `(?:.*=.*)+`. Chuỗi ký tự đặc biệt khiến engine Regex NFA tính toán cấp số nhân $O(2^n)$ các nhánh đệ quy (Catastrophic Backtracking). CPU core bị khóa cứng ở 100% spin loop, Ingress pod đơ và bị restart liên tục làm sập toàn cụm.
* **Cơ chế tự động hóa**: Bắt buộc sử dụng engine Regex tuyến tính DFA $O(n)$ như **Google RE2** hoặc **Rust regex** thay cho PCRE trong WAF; pre-merge static linting kiểm tra backtracking bằng `rxxr2`; cấu hình hard timeout cho Regex execution (tối đa 5ms/evaluation).

---

## TẦNG 11: MESSAGE QUEUES & STREAMING PLATFORMS (KỊCH BẢN 56 — 60)

### 56. Kafka Consumer Group Rebalance Storm khi K8s Worker Node Drain / Rolling Restart
* **Cơ chế lỗi**: Khi K8s drain worker node có hàng chục consumer pods. Eager Rebalance truyền thống thu hồi toàn bộ partition của TẤT CẢ các consumers trong group. Xử lý đóng băng, commit offset bị hoãn. Khi 10 pod lần lượt restart rải rác, chuỗi Rebalance Storm kéo dài hàng chục phút làm consumer lag bùng nổ hàng triệu messages.
* **Cơ chế tự động hóa**: Cấu hình bắt buộc `partition.assignment.strategy` sử dụng `CooperativeStickyAssignor` (KIP-429); kích hoạt `group.instance.id` (Static Membership - KIP-345) cho StatefulSets; điều phối rolling update với `maxUnavailable: 1` và `terminationGracePeriodSeconds: 60s`.

### 57. RabbitMQ High Memory Watermark Blocking Publishers Indefinitely & Deadlock Muted Consumers
* **Cơ chế lỗi**: RAM của RabbitMQ broker vượt ngưỡng (`vm_memory_high_watermark = 0.4`), RabbitMQ ngừng đọc dữ liệu từ network socket của toàn bộ TCP connection đang publish. Nếu microservice dùng chung 1 TCP connection cho cả publish lẫn consume, hệ thống rơi vào **Distributed Deadlock**: consumer không thể ack, publisher bị chặn đứng, queue tắc vĩnh viễn.
* **Cơ chế tự động hóa**: Cô lập TCP connection riêng biệt giữa Publisher và Consumer; nâng watermark `vm_memory_high_watermark.relative = 0.6` kết hợp Quorum Queues (Raft) và `vm_memory_high_watermark_paging_ratio = 0.5` để chủ động flush message xuống disk.

### 58. NATS JetStream Consumer Ack-Timeout Cascade & Disk I/O Saturation
* **Cơ chế lỗi**: Downstream chậm, consumer không kịp gửi `+ACK` trước khi hết hạn `ack_wait` (30s). JetStream tự động re-deliver. Hàng ngàn message timeout đồng loạt khiến JetStream đọc liên tục từ file storage, làm cạn kiệt Linux Page Cache, gây bão disk read I/O (`iowait > 70%`), Raft meta cluster drop heartbeat gây split-brain.
* **Cơ chế tự động hóa**: Cấu hình `max_ack_pending` có giới hạn chặt chẽ (1000); áp dụng Dead Letter Queue (`max_deliver: 3`); cấp phát NVMe riêng cho thư mục JetStream storage tách khỏi root disk.

### 59. Apache Kafka Under-Replicated Partitions (URP) Avalanche do Linux `vm.dirty_ratio` Flush Stall
* **Cơ chế lỗi**: Mặc định `vm.dirty_ratio` là 20-30% RAM (50GB-75GB dirty data trên server 256GB). Kernel kích hoạt synchronous writeback xả khối lượng dirty data khổng lồ xuống đĩa vật lý, làm nghẽn I/O queue trong 10-30 giây. Mọi I/O thread của Kafka bị block, leader broker rớt heartbeat, kích hoạt leader election hàng loạt làm hàng ngàn partitions biến thành Under-Replicated (URP Avalanche).
* **Cơ chế tự động hóa**: Tinh chỉnh sysctl bắt buộc cho Kafka host: `vm.dirty_background_ratio = 3`, `vm.dirty_ratio = 10`, `vm.dirty_expire_centisecs = 1000`; giám sát URP > 0 cảnh báo khẩn cấp.

### 60. Thảm Họa AWS Kinesis 2020: OS Process / Thread Exhaustion (`pids.max`) khi Scale-Up Cluster
* **Cơ chế lỗi**: Post-mortem AWS Kinesis 11/2020: Khi thêm capacity cho cụm frontend servers, các máy chủ tự động khởi tạo thêm các thread nội bộ để giao tiếp với các node mới trong mesh. Số lượng OS threads vượt qua `kernel.pid_max` hoặc cgroup `pids.max`. Hệ điều hành từ chối lệnh `fork()` và `pthread_create()` (`EAGAIN`), làm sập toàn bộ dịch vụ phụ thuộc (CloudWatch, Cognito, Auto Scaling).
* **Cơ chế tự động hóa**: Giám sát tỷ lệ `threads_used / pid_max`; thiết lập `kernel.pid_max = 4194304` và `pids.max` của Kubelet tối thiểu 65,535; kiến trúc hóa kết nối bằng Single Thread Event Loop (Epoll) thay vì spawn thread theo số lượng peer servers.

---

## TẦNG 12: LƯU TRỮ PHÂN TÁN QUY MÔ LỚN (KỊCH BẢN 61 — 64)

### 61. Ceph OSD Crashlooping & Peering Deadlock do RocksDB WAL Corruption sau Dirty Power Cut
* **Cơ chế lỗi**: Máy chủ OSD mất điện đột ngột hoặc kernel panic trong lúc ghi nặng, block metadata của RocksDB WAL bị hỏng. Khi OSD khởi động lại, RocksDB crash với `Corruption: bad entry in WAL`. Quorum OSD bị thiếu, Ceph kẹt vĩnh viễn ở `peering` và từ chối mọi I/O đọc/ghi.
* **Cơ chế tự động hóa**: Tự động chạy tool cứu hộ `ceph-bluestore-tool fsck/repair`; cấu hình `bluestore_rocksdb_options` kích hoạt `wal_recovery_mode = 2` (PointInTimeRecovery) để tự động drop trailing corrupted entries thay vì panic crash.

### 62. MinIO Erasure Coding Bit-Rot Healing Stall Dưới Áp Lực Concurrent Write Heavy I/O
* **Cơ chế lỗi**: Scanner crawler chiếm dụng lock đọc để tính toán parity phục hồi block cũ. Sự cạnh tranh tài nguyên I/O giữa background heal scanner và luồng client write dẫn đến disk starvation, crawler bị stall, parity block không được tái tạo kịp thời gây nguy cơ mất dữ liệu vĩnh viễn.
* **Cơ chế tự động hóa**: Tự động điều chỉnh `MINIO_DRIVE_SCANNER_SPEED` động (chuyển về `slowest` khi client write throughput > 80% baseline, tăng `fastest` off-peak); tách biệt drive pools giữa realtime objects và cold archive.

### 63. GlusterFS / CephFS Split-Brain Lock & Inode Quarantine sau Network Partition Intermittent Flapping
* **Cơ chế lỗi**: Hai storage node A và B gặp switch mạng chập chờn. Cả hai cùng tự cấp quyền ghi độc lập cho các client khác nhau vào cùng một inode. Khi mạng nối lại, changelog mâu thuẫn, storage engine đưa inode vào trạng thái `Quarantine / Split-Brain Lock`, chặn hoàn toàn quyền truy cập file (`EIO`).
* **Cơ chế tự động hóa**: Cấu hình bắt buộc `cluster.quorum-type` là `auto` kết hợp Arbiter node; watchdog tự động phát hiện network latency asymmetry > 50ms để chủ động fence node kém ổn định trước khi split-brain xảy ra.

### 64. Thảm Họa GitLab 2017: Postgres DB Replication Desync & Secondary Wipe Disaster
* **Cơ chế lỗi**: Post-mortem GitLab 1/2017: Replication lag vượt quá 4GB WAL. Lệnh xóa dữ liệu `rm -rf` bị chạy nhầm trên node Primary thay vì Secondary, trong khi 5 cơ chế backup khác đều âm thầm hỏng từ trước mà không ai hay biết. GitLab mất 300GB dữ liệu và ngừng hoạt động 18 giờ.
* **Cơ chế tự động hóa**: Chặn đứng lệnh `rm -rf` trên bất kỳ thư mục nào chứa `PG_DATA`; mọi thao tác resync replication phải qua Operator (`pg_rewind` hoặc `patroni`) với xác thực node role qua `SELECT pg_is_in_recovery()`; tự động kích hoạt sandbox restore drill hàng tuần (`Task 025`).

---

## TẦNG 13: LINUX KERNEL CGROUP V2 & EBPF SUBSYSTEM (KỊCH BẢN 65 — 68)

### 65. Linux cgroup v2 `memory.high` Asynchronous Throttling Làm Microservice Bị Đứng Hình Bí Ẩn
* **Cơ chế lỗi**: Trong cgroup v2 (K8s v1.25+), khi container dùng RAM vượt qua `memory.high`, kernel **KHÔNG KILL tiến trình**, mà ép các luồng của tiến trình tham gia Direct Memory Reclaim và đưa syscall vào sleep trì hoãn (Proportional Sleep Penalty). Không có log OOM trong dmesg, nhưng request bị stall từ 5 đến 30 giây trong kernel space.
* **Cơ chế tự động hóa**: Giám sát `container_memory_high_events_total` từ `/sys/fs/cgroup/.../memory.events`; tự động cấu hình `k8s.v1.cgroup2.memory.high.factor = 0` nếu workload không thể chịu được latency jitter; cảnh báo nâng memory request trước khi chạm trần.

### 66. eBPF Hash Map Exhaustion Âm Thầm Drop Socket Packets và Xé Rách Conntrack
* **Cơ chế lỗi**: CNI (Cilium, Calico eBPF) dùng eBPF Hash Maps lưu trữ bảng conntrack và NAT mapping. Dung lượng map cố định (`bpf-ct-global-any-max`). Khi bị port-scan hoặc microservice mở hàng triệu connection ngắn hạn, map đầy 100%. `bpf_map_update_elem()` trả về `-E2BIG`. Chương trình eBPF buộc phải `XDP_DROP` hoặc `TC_ACT_SHOT`.
* **Cơ chế tự động hóa**: Giám sát `cilium_bpf_map_pressure{map_name="cilium_ct4_global"}`; cảnh báo vàng ở mức 70% và đỏ ở mức 85%; tự động trigger dynamic map resizing qua Cilium ConfigMap và giảm `bpf-ct-timeout-regular = 60s`.

### 67. Linux Kernel Slab Dentry & Inode Cache Leak từ Hàng Triệu Short-Lived Temp Files
* **Cơ chế lỗi**: Container sinh hàng triệu file tạm trong `/tmp` rồi xóa ngay. Dù file đã xóa, cấu trúc `dentry` và `inode` trong Linux Kernel Slab Memory (`kmalloc-128`, `dentry`, `inode_cache`) vẫn được giữ lại trong RAM, chiếm tới 80% RAM máy chủ. Kubelet tính RAM bao gồm Unreclaimable Slab, kích hoạt Node Memory Pressure và evict nhầm các pod quan trọng.
* **Cơ chế tự động hóa**: Cấu hình sysctl `vm.vfs_cache_pressure = 150` để ép kernel chủ động dọn dẹp dentry/inode cache; gắn tmpfs (RAM disk in-memory) cho thư mục `/tmp` của container thay vì ghi trực tiếp lên rootfs node.

### 68. eBPF Verifier CPU / Instruction Limit Rejection trong Quá Trình Hot-Reload Network Policy Filter
* **Cơ chế lỗi**: NetworkPolicy quá phức tạp (hàng trăm CIDR blocks hoặc L7 rules). CNI sinh mã bytecode eBPF nạp vào kernel qua syscall `bpf(BPF_PROG_LOAD)`. Kernel eBPF Verifier duyệt chạm trần instruction limit (1 triệu instructions) hoặc tốn 100% CPU của 1 core trong hơn 30 giây, khiến kernel từ chối nạp (`E2BIG` hoặc `EPERM`). CNI daemonset crash hoặc degraded, cô lập toàn bộ pod trên node.
* **Cơ chế tự động hóa**: Triển khai Policy Complexity Linter trong CI/CD pipeline: kiểm tra giới hạn rule CIDR (< 100 per policy) trước khi apply; chia nhỏ NetworkPolicy thành các policy module hóa.

---

## TẦNG 14: BẢO MẬT ZERO-TRUST & IDENTITY PHỨC TẠP (KỊCH BẢN 69 — 72)

### 69. HashiCorp Vault Unseal Key Quorum Deadlock Sau Sự Cố Reboot Toàn Bộ Datacenter
* **Cơ chế lỗi**: Datacenter bị sập nguồn. Khi bật lại, các pod Vault ở trạng thái **Sealed**. Vault yêu cầu Shamir Secret Sharing Quorum (3/5 keys). Nếu hệ thống dựa vào Transit Auto-Unseal mà KMS phụ thuộc vòng lặp (Circular Dependency), hoặc kỹ sư giữ khóa không có mặt, không một dịch vụ nào có thể kết nối DB hay start pod.
* **Cơ chế tự động hóa**: Triển khai Trusted Hardware HSM (PKCS#11) hoặc Multi-Cloud CloudKMS Auto-Unseal độc lập; thiết lập Emergency Break-Glass Unseal Operator có quyền truy cập khóa mã hóa trong TPM 2.0 cục bộ của máy chủ bare-metal an toàn.

### 70. Secret Lease Renewal Thundering Herd Burst Làm Nghẽn Vault Storage Backend
* **Cơ chế lỗi**: Hàng ngàn Pod K8s cùng tạo dynamic DB secret qua Vault Agent với TTL 1 giờ. Tại thời điểm 50 phút (85% TTL), hàng ngàn Pod đồng loạt gửi `POST /v1/sys/leases/renew` trong 10 giây. Vault Core ghi log cập nhật liên tục vào storage backend (Integrated Raft log / Consul). Hàng đợi Raft RPC bị tràn, disk fsync vọt lên 500ms, Vault từ chối các request auth mới làm dịch vụ chết hàng loạt.
* **Cơ chế tự động hóa**: Bắt buộc kích hoạt Random Jitter ($TTL \times (0.6 + 0.3 \times \text{rand}())$) trên Vault Agent sidecar client; đặt lease duration tối thiểu 12h-24h cho ứng dụng tải cao; giới hạn rate limit per-mount trên Vault.

### 71. Multi-Cluster Service Mesh mTLS Trust Domain & Root CA Fingerprint Mismatch
* **Cơ chế lỗi**: Trong Multi-Cluster K8s, mỗi cụm cài Root CA riêng hoặc renew Intermediate CA không đồng bộ. Khi Service A trên Cluster 1 gọi Service B trên Cluster 2, Envoy sidecar Cluster 2 kiểm tra SAN thấy Trust Domain không khớp hoặc CA fingerprint mismatch, lập tức cắt TLS với lỗi `SSL3_GET_SERVER_CERTIFICATE:certificate verify failed`.
* **Cơ chế tự động hóa**: Đồng bộ Unified Root CA chung qua HashiCorp Vault PKI Engine; cấu hình tường minh `trustDomainAliases` trong MeshConfig; automated CI check kiểm tra CA fingerprint hash giữa các cluster trước khi apply cross-cluster routing.

### 72. OIDC / OAuth2 JWKS Cache Expiration & Keyset Endpoint Flapping Làm 100% Request 401
* **Cơ chế lỗi**: API Gateway cache public keys từ OIDC Provider qua `/.well-known/jwks.json` với TTL 5 phút. Khi IdP mạng chập chờn đúng lúc cache hết hạn, API Gateway không fetch được key mới và purge toàn bộ public keys trong RAM. Mọi JWT token hợp lệ của hàng trăm ngàn người dùng đều bị trả về HTTP 401 Unauthorized.
* **Cơ chế tự động hóa**: Cấu hình "Stale-While-Revalidate": nếu JWKS endpoint lỗi, API Gateway BẮT BUỘC giữ lại bộ keys cũ trong tối thiểu 24 giờ và gửi cảnh báo ngầm thay vì xóa cache; thiết lập Circuit Breaker và Local Static JWKS Backup trong ConfigMap dự phòng.

---

## TẦNG 15: THẢM HỌA CLOUD HYPERVISOR & INFRASTRUCTURE (KỊCH BẢN 73 — 77)

### 73. AWS EBS GP2/GP3 I/O Credit Burst Balance Exhaustion (IOPS Tụt Dốc Không Phanh)
* **Cơ chế lỗi**: Ổ đĩa AWS EBS GP2 hoạt động trên cơ chế Burst Bucket (3000 IOPS burst). Khi DB chạy batch job hoặc full table scan, Burst Bucket bị rút cạn về 0%. Hypervisor AWS lập tức bóp nghẹt tốc độ xuống baseline (ổ 100GB chỉ còn đúng 300 IOPS). Disk latency nhảy vọt lên 800ms, database bị treo hoàn toàn.
* **Cơ chế tự động hóa**: Cấm sử dụng volume GP2 trên production; bắt buộc sử dụng **GP3** với IOPS và Throughput được cấp phát cố định độc lập với dung lượng (Provisioned IOPS 3000+, Throughput 125MB/s baseline); cảnh báo tự động khi Disk Queue Length > 10.

### 74. Metadata Service (169.254.169.254) IMDSv1 SSRF Credential Exfiltration & Role Hijack
* **Cơ chế lỗi**: Pod bị lỗi bảo mật SSRF (hàm render PDF, curl preview URL). Kẻ tấn công đọc `http://169.254.169.254/latest/meta-data/iam/security-credentials/<role-name>` qua IMDSv1 không cần header xác thực, lấy được AWS Secret Access Key & Session Token tạm thời của Node IAM Role và chiếm quyền toàn bộ cụm đám mây.
* **Cơ chế tự động hóa**: Cưỡng chế bật **IMDSv2** (`HttpTokens=required`) và thiết lập `HttpPutResponseHopLimit=1` trên EC2 launch template (ngăn Pod bên trong bridge network chạm tới metadata); áp dụng NetworkPolicy chặn hoàn toàn CIDR `169.254.169.254/32` cho toàn bộ ứng dụng người dùng.

### 75. Cloud Object Storage (S3 / GCS) Multi-Region Eventual Consistency Read-After-Write Lag
* **Cơ chế lỗi**: Hệ thống đa vùng Active-Active. Worker Region 1 upload file lên S3 và bắn message qua Kafka. Worker Region 2 nhận message ngay lập tức (trong 50ms) và gọi `GetObject(file_id)`. Do replication giữa 2 datacenter mất 200-500ms, S3 trả về `NoSuchKey` (HTTP 404). Nếu client cache response 404 này, file bị coi là mất vĩnh viễn trong Region 2.
* **Cơ chế tự động hóa**: Thiết lập Exponential Backoff with Jitter cho mọi tác vụ đọc sau ghi; sử dụng Strong Consistency Local Bucket cho luồng xử lý tức thời trước khi kích hoạt Async Cross-Region Replication; gắn metadata `x-amz-version-id` vào message queue để verify object version thực tế.

### 76. Thảm Họa Monzo 2019: etcd Split-Brain & Dual-Master Partition do Raft Election Storm
* **Cơ chế lỗi**: Post-mortem Monzo Bank 2019: Link mạng giữa các node etcd bị nghẽn gói tin do telemetry quá tải. Heartbeat bị trễ quá `heartbeat-interval` (100ms), kích hoạt `election-timeout` (1000ms). Do mạng chập chờn không đối xứng, nhiều cuộc bầu cử nổ ra liên tục (Election Storm), etcd không thể gom đủ đa số quorum (3/5) để chốt Leader. Cụm etcd mất leader trong nhiều giờ, ngân hàng số ngừng hoạt động toàn diện.
* **Cơ chế tự động hóa**: Tách biệt hoàn toàn card mạng vật lý hoặc VLAN riêng cho etcd traffic; nâng `heartbeat-interval = 250ms` và `election-timeout = 1250ms` cho môi trường cross-AZ/cloud; triển khai etcd learner nodes để scale read mà không làm loãng quorum voting.

### 77. Thảm Họa GitHub 2018: Multi-Datacenter Master Failover Partition do 43s Optical Link Disruption
* **Cơ chế lỗi**: Post-mortem GitHub 10/2018: Đứt cáp quang 43 giây giữa US-East và US-West. Orchestrator tự động kết luận US-East đã chết và kích hoạt failover MySQL sang US-West. Tuy nhiên routing nội bộ vẫn còn service trỏ vào US-East. Dữ liệu ghi (writes) bị xé làm 2 dòng hoàn toàn khác nhau trên 2 datacenter (Split-Brain Writes). Sau khi cáp nối lại, replication bị vỡ nát do xung đột khóa chính, GitHub phải bảo trì 24 giờ để đối chiếu thủ công hàng triệu commit/PR.
* **Cơ chế tự động hóa**: Áp dụng cơ chế **Fencing with STONITH (Shoot The Other Node In The Head)**: Không bao giờ promote Secondary nếu chưa xác nhận 100% Primary cũ đã bị cô lập hoàn toàn hoặc tắt nguồn; áp dụng **Grace Period Failover Delay** tối thiểu 120s trước khi chuyển master để tránh phản ứng thái quá với các cú nấc ngắn (transient hiccups); áp dụng Semi-Sync Replication với `rpl_semi_sync_master_wait_no_slave = 1`.

---

## TẦNG 16: SECRET STORE CSI, EXTERNAL SECRETS & RUNTIME DRIFT (KỊCH BẢN 78 — 80)

### 78. External Secrets Operator (ESO) / SecretStore CSI Polling Storm Làm Nghẽn Vault API
* **Cơ chế lỗi**: Trong cụm K8s với 1000 microservices, mỗi service định nghĩa 1 `ExternalSecret` với `refreshInterval: 15s`. Hàng ngàn request đồng thời bắn vào Vault/AWS Secrets Manager mỗi giây. Rate limit của Vault (`sys/leases/renew` hoặc token auth) bị kích hoạt, Vault trả về HTTP 429 Too Many Requests, toàn bộ controller bị backoff, pods mới không thể bootstrap secret và rơi vào `CreateContainerConfigError`.
* **Cơ chế tự động hóa**: Cưỡng chế `refreshInterval` tối thiểu 1h cho static secrets; kích hoạt Push-based Webhook notification từ Vault sang ESO thay vì Pull polling liên tục; triển khai Local In-Cluster Caching Proxy cho Secrets Provider.

### 79. SealedSecrets / SOPS GPG Master Key Mất Mát Làm Tê Liệt Disaster Recovery
* **Cơ chế lỗi**: Triển khai GitOps hoàn chỉnh với SealedSecrets hoặc SOPS. Tuy nhiên, private key giải mã chỉ nằm trên cụm K8s cũ dưới dạng secret in-cluster mà không được sao lưu off-site an toàn. Khi thảm họa xảy ra và cụm K8s bị xóa sạch, ArgoCD kéo toàn bộ Git repo về cụm mới nhưng SealedSecrets Controller không thể giải mã các manifest mật, toàn bộ database credentials và TLS certificates biến thành rác mã hóa vô giá trị.
* **Cơ chế tự động hóa**: Tự động mã hóa và export SealedSecrets private key sang KMS multi-region bất biến trong mỗi chu kỳ backup (`Task 025`); kiểm tra khả năng giải mã dry-run của SOPS/SealedSecret trong CI/CD pipeline trước khi merge PR.

### 80. In-Memory Secret Staleness: Pod Giữ Mật Khẩu Database Cũ Đến Khi Bị Restart Cưỡng Bức
* **Cơ chế lỗi**: Kỹ sư SRE kích hoạt luân chuyển (rotate) mật khẩu database trên Vault và K8s Secret. K8s Secret volume mount được cập nhật atomic, nhưng ứng dụng (Go, Java Spring, Node.js) chỉ đọc environment variable hoặc config file lúc startup (`init()`). Connection pool tiếp tục dùng mật khẩu cũ. Khi DB server kết thúc grace period và thu hồi mật khẩu cũ, hàng ngàn kết nối đang chạy đồng loạt đứt gãy, gây outage diện rộng dù secret trên K8s đã "mới".
* **Cơ chế tự động hóa**: Triển khai `Reloader` controller tự động trigger Rolling Restart (`kubectl rollout restart`) cho Deployment khi ConfigMap/Secret hash thay đổi; hoặc ép buộc ứng dụng sử dụng dynamic SDK lắng nghe file change event qua inotify.

---

## TẦNG 17: EBPF SECURITY, RUNTIME THREATS & KERNEL PROBES (KỊCH BẢN 81 — 83)

### 81. Tetragon / Falco eBPF RingBuffer Dropping Events Gây Mù An Ninh (Silent Audit Blindness)
* **Cơ chế lỗi**: Khi hệ thống bị tấn công brute-force hoặc chạy benchmark nặng, số lượng syscalls (`sys_enter_execve`, `sys_enter_connect`) tăng vọt lên 500,000 events/giây. eBPF Perf RingBuffer (`BPF_MAP_TYPE_RINGBUF`) bị đầy 100%. Kernel âm thầm drop các security audit event để bảo vệ CPU. Kẻ tấn công thực thi shellcode đào coin hoặc leo quyền trong đúng thời điểm RingBuffer overflow mà Falco/Tetragon hoàn toàn không ghi lại bất kỳ alert nào.
* **Cơ chế tự động hóa**: Giám sát metric `tetragon_ringbuf_lost_events_total` và `falco_events_dropped_total`; tự động cảnh báo đỏ khẩn cấp khi lost events > 0; áp dụng kernel adaptive sampling và tăng kích thước `ring_buffer_pages` lên 1024 (4MB per CPU).

### 82. Kernel Kprobe Panic / Instability Sau Khi Cập Nhật Minor Linux Kernel Version
* **Cơ chế lỗi**: Agent eBPF sử dụng Kprobes gắn vào các hàm nội bộ không ổn định của Linux kernel (như `tcp_v4_connect` hoặc `do_exit`). Khi cloud provider tự động nâng cấp kernel từ 5.15.0-88 lên 5.15.0-91, struct layout của kernel thay đổi (thêm field hoặc đổi offset). Bytecode eBPF biên dịch dạng CO-RE (Compile Once - Run Everywhere) đọc sai memory offset hoặc kprobe hook point bị đổi tên, dẫn đến kernel soft lockup hoặc crash node vật lý.
* **Cơ chế tự động hóa**: Bắt buộc sử dụng **fexit/fentry** và **Tracepoints** chuẩn hóa của kernel thay cho kprobes thô; kiểm tra tương thích BTF (BPF Type Format) trong test suite trước khi triển khai agent lên cụm; cơ chế Fail-Safe tự động unhook và unload BPF program nếu kernel ghi nhận exception.

### 83. eBPF Socket Filter CPU Overhead & Tail Call Depth Stack Overflow
* **Cơ chế lỗi**: Một chuỗi eBPF tail calls lồng nhau (tối đa 33 calls trong Linux BPF) được dùng để lọc gói tin L7 (HTTP/gRPC header parsing). Khi payload gói tin phân mảnh hoặc chứa header bất thường, chương trình lặp đệ quy đạt giới hạn tail call depth hoặc ngốn tới 25% CPU core chỉ cho việc thực thi bộ lọc kernel trên mỗi network packet, làm throughput của cụm giảm 40%.
* **Cơ chế tự động hóa**: Giới hạn độ sâu tail call tối đa 4 tầng; kiểm tra benchmark `bpf_prog_run_time_ns`; tự động bypass L7 inspection cho internal service-to-service traffic tin cậy.

---

## TẦNG 18: HYBRID WAN, SITE-TO-SITE VPN & MTU BLACK HOLE (KỊCH BẢN 84 — 86)

### 84. WireGuard / IPSec Site-to-Site VPN MTU Black Hole & TCP MSS Clamping Failure
* **Cơ chế lỗi**: Kết nối giữa datacenter On-Premise và Cloud qua WireGuard/IPSec tunnel. Interface vật lý có MTU 1500, nhưng tunnel thêm 60-80 bytes header (ESP/WireGuard outer header), làm inner MTU giảm xuống 1420. Client gửi packet lớn với cờ `DF (Don't Fragment) = 1`. Switch trung gian drop gói và gửi lại ICMP `Fragmentation Needed (Type 3, Code 4)`. Nhưng firewall chặn mất gói ICMP này. Kết quả: Kết nối TCP handshake (SYN/ACK nhỏ) thành công, nhưng lệnh `curl` lớn hoặc SSL handshake bị treo vĩnh viễn (MTU Black Hole).
* **Cơ chế tự động hóa**: Áp dụng bắt buộc `iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu` trên toàn bộ gateway; ping probe tự động với payload size 1472 (`ping -M do -s 1472`) để cảnh báo MTU mismatch.

### 85. Tailscale / WireGuard DERP Relay Latency Spike Khi Đứt Kết Nối P2P Trực Tiếp
* **Cơ chế lỗi**: Hai cụm Kubernetes giao tiếp qua mesh VPN (Tailscale/Nebula). Ban đầu kết nối P2P Direct qua UDP thành công (latency 5ms). Khi ISP thay đổi NAT type sang Symmetric NAT hoặc firewall chặn UDP, kết nối âm thầm fallback sang DERP Relay Server qua HTTPS (TCP 443). Mọi luồng traffic dữ liệu DB/storage bị chuyển hướng qua server trung gian đặt tại khu vực khác, latency vọt từ 5ms lên 180ms làm timeout hàng loạt gRPC calls.
* **Cơ chế tự động hóa**: Watchdog liên tục kiểm tra trạng thái `tailscale status` phát hiện cờ `relay` thay vì `direct`; tự động cảnh báo và kích hoạt STUN/UPnP port mapping dự phòng.

### 86. Asymmetric Routing & State Drop Do Dual-Uplink BGP Multi-Homing
* **Cơ chế lỗi**: Server kết nối 2 ISP độc lập qua giao thức BGP. Gói tin đi ra qua ISP-1 (Primary), nhưng gói tin phản hồi từ internet quay về qua ISP-2 (Secondary). Linux kernel bật tính năng `rp_filter = 1` (Reverse Path Filtering Strict Mode). Kernel thấy gói tin vào từ interface không khớp với routing table chiều đi, coi đây là IP spoofing và âm thầm DROP toàn bộ gói tin (`IP: Drop reverse path check failed`).
* **Cơ chế tự động hóa**: Cấu hình sysctl `net.ipv4.conf.all.rp_filter = 2` (Loose Mode) trên tất cả multi-homed routers; kiểm tra đồng bộ asymmetric flows qua conntrack telemetry.

---

## TẦNG 19: ANALYTICAL BIG DATA, TSDB & LOGGING ENGINES (KỊCH BẢN 87 — 89)

### 87. ClickHouse `Too Many Parts` Do Micro-Inserts Bỏ Qua In-Memory Buffer
* **Cơ chế lỗi**: Các microservices hoặc log shipper ghi log trực tiếp vào ClickHouse MergeTree table với tốc độ 1000 inserts/giây, mỗi insert chỉ 1-5 dòng. ClickHouse tạo ra 1 folder "part" mới trên ổ đĩa cho mỗi lệnh insert. Background merge thread không thể đuổi kịp tốc độ sinh parts, số lượng active parts vượt qua ngưỡng trần (`max_parts_in_total = 300`). ClickHouse từ chối mọi lệnh ghi với lỗi `Code: 252. DB::Exception: Too many parts in all data parts in table...`.
* **Cơ chế tự động hóa**: Bắt buộc gom batch tối thiểu 10,000 dòng hoặc 5 giây trước khi insert; hoặc sử dụng engine `Buffer` / `Kafka Engine` làm lớp đệm in-memory trước khi xả xuống MergeTree; cảnh báo khi số lượng active parts trong `system.parts` > 150.

### 88. VictoriaMetrics / Prometheus TSDB Out-of-Order Chunks Rejection Từ Edge Agents
* **Cơ chế lỗi**: Edge agents tại các chi nhánh offline trong 2 giờ, khi có mạng trở lại, agent đẩy 2 giờ metrics dồn ứ lên central VictoriaMetrics/Prometheus. TSDB truyền thống không chấp nhận dữ liệu có timestamp cũ hơn append head window (thường là 1-2 giờ). TSDB âm thầm reject hoặc crash với lỗi `out of order sample`, toàn bộ metrics lịch sử quan trọng của sự cố mạng bị mất trắng.
* **Cơ chế tự động hóa**: Cấu hình VictoriaMetrics với cờ `-search.latencyOffset` và `-insert.maxQueueDuration`; hoặc kích hoạt `out_of_order_time_window` trong Prometheus 2.39+; thiết lập dead-letter spillover bucket trên edge agent.

### 89. OpenSearch / Elasticsearch Red Cluster State Do Circuit Breaker Trip Trên JVM Heap
* **Cơ chế lỗi**: Kỹ sư tìm kiếm log bằng query aggregation quá rộng (wildcard trên hàng chục tỷ documents). JVM Heap của OpenSearch bị đẩy lên 95%. Cơ chế Parent Circuit Breaker (`indices.breaker.total.use_real_memory`) lập tức cắt đứt query và đánh dấu shard đang xử lý là corrupt. Cluster status chuyển từ Green sang **Red**, master node bị freeze không phản hồi REST API.
* **Cơ chế tự động hóa**: Khóa giới hạn JVM Heap tối đa 31GB (để tận dụng Compressed OOPs); cấu hình circuit breaker `indices.breaker.total.limit = 70%`; tự động timeout cho mọi wildcard query vượt quá 10 giây.

---

## TẦNG 20: CONTAINER RUNTIME, INODE DEPLETION & P2P IMAGE DISTRIBUTION (KỊCH BẢN 90 — 92)

### 90. Containerd Content Store Inode Exhaustion Do Tích Tụ Unpacked Layer Tarballs
* **Cơ chế lỗi**: CI/CD runner build và pull hàng trăm Docker images mỗi ngày trên worker node. Mặc dù dung lượng ổ đĩa (`df -h`) còn trống 50%, nhưng số lượng **Inodes** (`df -i`) đã đạt 100% (hàng triệu file nhỏ từ node_modules trong các layer image cũ). Containerd không thể giải nén image mới, Kubelet báo lỗi `failed to extract layer: no space left on device` dù đĩa còn hàng trăm GB trống.
* **Cơ chế tự động hóa**: Định dạng ổ đĩa với `mkfs.ext4 -i 8192` (tăng gấp đôi mật độ inode); triển khai DaemonSet tự động kích hoạt `crictl rmi --prune` khi Inode usage vượt quá 80%; dọn dẹp thư mục `/var/lib/containerd/io.containerd.content.v1.content/ingest/` định kỳ.

### 91. Docker Hub Rate Limit (HTTP 429) Làm Chết Rolling Update Trong Giờ Cao Điểm
* **Cơ chế lỗi**: Một Deployment scale từ 5 lên 50 pods trong giờ cao điểm. Base image trỏ vào `golang:alpine` hoặc `python:slim` từ Docker Hub mà không có credentials hoặc mirror. Docker Hub chặn IP NAT của công ty với lỗi `HTTP 429 Too Many Requests: toomanyrequests: You have reached your pull rate limit`. Các pod mới kẹt ở `ImagePullBackOff`, trong khi các pod cũ đã bị terminate theo rolling update policy, làm dịch vụ sập hoàn toàn.
* **Cơ chế tự động hóa**: Bắt buộc 100% container image phải được kéo từ **Harbor Private Registry** nội bộ; cấu hình Containerd Mirror (`/etc/containerd/certs.d/docker.io/hosts.toml`) tự động redirect sang registry mirror cục bộ; Admission Webhook từ chối mọi manifest chứa image domain `docker.io` trần.

### 92. P2P Image Streaming (Spegel / Dragonfly) DHT Routing Loop Khi Scale Đột Biến
* **Cơ chế lỗi**: Sử dụng mạng P2P (Spegel) để chia sẻ image layers giữa các worker nodes mà không tải từ registry trung tâm. Khi 100 worker nodes mới cùng boot đồng thời trong Auto-Scaling group, Distributed Hash Table (DHT) chưa hội tụ (churn rate quá cao). Các node liên tục query lẫn nhau tạo thành routing loop, CPU của containerd-shim vọt lên 100%, image pull bị timeout 10 phút.
* **Cơ chế tự động hóa**: Cấu hình fallback timeout chặt chẽ (15s): nếu Spegel P2P không tìm thấy layer trong 15s, lập tức fallback thẳng về Harbor registry trung tâm; giới hạn số lượng concurrent P2P downloads trên mỗi node.

---

## TẦNG 21: AI / LLM INFRASTRUCTURE & GPU ACCELERATOR SRE (KỊCH BẢN 93 — 95)

### 93. NVIDIA GPU XID 31 / 43 Memory Page Fault & Zombie GPU Drop Trong Huấn Luyện Phân Tán
* **Cơ chế lỗi**: Khi chạy mô hình LLM lớn trên cụm GPU (A100/H100), một lỗi phần cứng hoặc kernel driver sinh mã `XID 31 (MMU Fault)` hoặc `XID 43 (GPU stopped processing)`. Driver NVIDIA đưa GPU vào trạng thái zombie (Unusable). Pod K8s vẫn tiếp tục chạy vì tiến trình Python chưa exit, nhưng mọi lệnh gọi CUDA bị freeze vĩnh viễn. Cả cụm huấn luyện phân tán 128 GPU bị treo chờ đợi (NCCL Barrier timeout) gây lãng phí hàng ngàn USD chi phí điện toán mỗi giờ.
* **Cơ chế tự động hóa**: Triển khai `NVIDIA GPU Operator` kết hợp `DCGM (Data Center GPU Manager)` exporter; tự động phát hiện XID critical errors qua eBPF/dmesg scanner; tự động taint node `nvidia.com/gpu=faulty:NoSchedule` và kill ngay pod bị kẹt để giải phóng barrier.

### 94. vLLM / Triton PagedAttention KV-Cache Fragmentation Dẫn Đến OOM CUDA Đột Ngột
* **Cơ chế lỗi**: Server phục vụ LLM (vLLM) phân bổ bộ nhớ VRAM cho PagedAttention KV-cache (`gpu_memory_utilization = 0.90`). Khi nhiều user gửi prompt dài đột biến (32K tokens) đồng thời, fragmentation trên VRAM xảy ra. Tiến trình vLLM không thể cấp phát thêm block bộ nhớ và kích hoạt `CUDA Out of Memory`. Pod crash ngay lập tức, làm đứt toàn bộ WebSocket/streaming responses của hàng trăm client đang chat.
* **Cơ chế tự động hóa**: Thiết lập `gpu_memory_utilization = 0.85` an toàn; kích hoạt dynamic request preempting / swap KV-cache sang CPU RAM khi VRAM áp lực > 95%; áp dụng Token Bucket Rate Limiting chặt chẽ ở tầng API Gateway để chặn các prompt dài vượt dự kiến.

### 95. NCCL AllReduce Ring Deadlock Do RoCE / InfiniBand Packet Drop
* **Cơ chế lỗi**: Trong huấn luyện LLM đa máy chủ qua mạng RoCEv2 (RDMA over Converged Ethernet), switch mạng bị drop 1 gói tin PFC (Priority Flow Control). Thuật toán NCCL Ring AllReduce yêu cầu mọi node phải truyền tensor tuần tự theo vòng tròn. Khi Node 3 không nhận được tensor từ Node 2, toàn bộ 64 servers trong cụm rơi vào **Distributed Deadlock**. Không có lỗi crash, GPU vẫn báo 100% utilization (do busy-polling spin loop), nhưng không có tiến độ huấn luyện nào được thực hiện.
* **Cơ chế tự động hóa**: Cấu hình `NCCL_ASYNC_ERROR_HANDLING=1` và `NCCL_HEARTBEAT_TIMEOUT_SEC=120`; watchdog tự động đo delta của Loss function: nếu sau 5 phút không có step mới được ghi nhận, tự động kill job và rollback về checkpoint gần nhất.

---

## TẦNG 22: AIR-GAPPED DISASTER RECOVERY & VELERO SNAPSHOT LOCKS (KỊCH BẢN 96 — 98)

### 96. Velero / Kopia Snapshot Repository Lock Bị Kẹt Sau Khi Node Backup Bị Chết Đột Ngột
* **Cơ chế lỗi**: Trong lúc Velero đang snapshot cụm K8s xuống MinIO/S3, node chạy backup controller bị mất điện hoặc OOM-killed. Kopia/Restic để lại một file lock độc quyền (`exclusive lock`) trong bucket lưu trữ. Khi controller khởi động lại hoặc ca backup kế tiếp chạy, Velero từ chối thực hiện vì `Repository is locked by another instance`. Các ca backup tự động suốt 7 ngày tiếp theo đều âm thầm thất bại.
* **Cơ chế tự động hóa**: Tự động chạy lệnh `velero backup-location get` và kiểm tra lock age; nếu lock tồn tại quá 60 phút mà không có tiến trình active, tự động thực thi `kopia maintenance --break-lock` và kích hoạt lại backup; gửi cảnh báo khẩn cấp lên Telegram SRE Copilot nếu 2 ca backup liên tiếp bị skip.

### 97. SQLite / Raft Metadata Split Trên Edge Nodes Do Mất Điện Đột Ngột (Dirty Flush)
* **Cơ chế lỗi**: Tại các trạm biên (Edge/IoT node), cơ sở dữ liệu nhúng (k3s embedded etcd hoặc SQLite) đang ghi WAL journal thì nguồn điện bị ngắt đột ngột. Ổ cứng SSD tiêu dùng (consumer grade không có tụ điện PLP - Power Loss Protection) làm mất cache trong DRAM của ổ cứng. Metadata bị hỏng một nửa (`SQLITE_CORRUPT`). Khi cấp điện trở lại, node không thể gia nhập cụm và kẹt ở trạng thái khởi động.
* **Cơ chế tự động hóa**: Cấu hình SQLite ở chế độ `PRAGMA synchronous = FULL; PRAGMA journal_mode = WAL;`; tự động kích hoạt script recovery tự động rebuild database từ snapshot gần nhất lưu trong phân vùng read-only dự phòng.

### 98. KMS Key Policy Deletion Làm Toàn Bộ Bản Sao Lưu Bất Biến Biến Thành Rác Vô Nghiệm
* **Cơ chế lỗi**: Doanh nghiệp lưu snapshot backup trên S3/GCS đã mã hóa bằng KMS Customer Managed Key (CMK). Một kỹ sư cloud vô tình xóa IAM Role hoặc schedule delete KMS Key vì tưởng là tài nguyên không dùng. Sau 7 ngày, KMS Key bị tiêu hủy vĩnh viễn. Mặc dù hàng trăm Terabytes bản sao lưu vẫn nằm nguyên vẹn trên đĩa, nhưng không còn bất kỳ thuật toán nào có thể giải mã được dữ liệu.
* **Cơ chế tự động hóa**: Kích hoạt KMS Key Deletion Protection; cấu hình Multi-Party Quorum Approval cho mọi thao tác xóa KMS Key; tự động sao lưu bản mã hóa dữ liệu với Multi-Cloud Split Key (1 khóa AWS KMS + 1 khóa Vault On-Premise).

---

## TẦNG 23: FINOPS, CROSS-AZ EGRESS & CLUSTER ORPHAN ZOMBIES (KỊCH BẢN 99 — 100)

### 99. Cross-Availability Zone Data Egress Cost Explosion Do Pods Giao Tiếp Chéo Vùng
* **Cơ chế lỗi**: Một dịch vụ Kafka và microservices được deploy trên cụm đa vùng (AZ-1, AZ-2, AZ-3). Do K8s Service mặc định cân bằng tải round-robin ngẫu nhiên, 66% lưu lượng mạng phải chạy xuyên qua các đường cáp quang giữa các AZs (Inter-AZ Data Transfer). Cloud provider tính phí 0.01$/GB cho mỗi chiều. Cuối tháng, hóa đơn băng thông Cross-AZ nhảy vọt lên 25,000$ vượt xa chi phí thuê máy chủ.
* **Cơ chế tự động hóa**: Bật tính năng `topologyAwareHints: true` (hoặc `topology.kubernetes.io/zone`) trên tất cả K8s Services để ép Kube-proxy ưu tiên 100% traffic nội bộ cùng AZ; chỉ cho phép traffic liên AZ khi pods trong cùng AZ quá tải.

### 100. Zombie Cloud Load Balancers & Unattached PVC Volumes Sau Khi Xóa K8s Namespaces
* **Cơ chế lỗi**: Khi xóa một namespace thử nghiệm (`kubectl delete ns staging`), do lỗi webhook hoặc finalizer bị kẹt, K8s Cloud Controller Manager không thể gửi lệnh deprovisioning cho AWS ALB / NLB và EBS volumes. Namespace biến mất khỏi K8s, nhưng hàng chục Load Balancer và hàng chục Terabytes ổ đĩa đắt tiền vẫn tiếp tục chạy âm thầm trên tài khoản AWS/GCP, tiêu tốn hàng ngàn USD mỗi tháng mà không phục vụ bất kỳ ai.
* **Cơ chế tự động hóa**: Triển khai `Cloud Orphan Resource Reaper` quét định kỳ giữa danh sách K8s Ingress/PVC và AWS/GCP API; phát hiện mọi tài nguyên mang tag `kubernetes.io/cluster` nhưng không còn tồn tại trong K8s API server; tự động gửi thông báo phê duyệt dọn dẹp qua Telegram bot sau 48 giờ.

---

## TẦNG 24: KERNEL TCP/SOCKET LOW-LEVEL, CONGESTION & TIME_WAIT SATURATION (KỊCH BẢN 101 — 105)

### 101. TCP TIME_WAIT Accumulation & Local Ephemeral Port Starvation Dưới Tải Microservices
* **Cơ chế lỗi**: Khi microservices thực hiện hàng chục ngàn HTTP/1.1 requests ngắn (short-lived without keep-alive) tới internal services hoặc external APIs, các TCP connections bị đóng sẽ duy trì ở trạng thái `TIME_WAIT` trong $2 \times \text{MSL}$ (khoảng 60 giây). Dải cổng cục bộ (`net.ipv4.ip_local_port_range`, mặc định 32768–60999 = 28,231 ports) bị cạn kiệt hoàn toàn. Các lệnh gọi mạng mới thất bại với lỗi `bind: address already in use` hoặc `dial: cannot assign requested address`, làm tê liệt toàn bộ microservices dù CPU và RAM còn rất rảnh.
* **Cơ chế tự động hóa**: Bật HTTP Client Connection Pooling (Keep-Alive, `MaxIdleConnsPerHost >= 100`); tinh chỉnh sysctl kernel qua daemonset node-initializer: `net.ipv4.tcp_tw_reuse = 1`, mở rộng `net.ipv4.ip_local_port_range = 10240 65535`; tự động giám sát metric `node_sockstat_TCP_tw` và gửi alert khi tiệm cận 80% ngưỡng dải port.

### 102. TCP Listen Backlog Overflow (`somaxconn` & `tcp_max_syn_backlog`) Âm Thầm Drop Gói SYN
* **Cơ chế lỗi**: Khi có đợt traffic spike bất ngờ, hàng ngàn client cùng gửi gói TCP SYN tới API server (Go / Nginx). Hàng đợi TCP listen backlog bị đầy do giá trị mặc định của Linux kernel quá thấp (`net.core.somaxconn = 128` hoặc `4096`). Kernel âm thầm vứt bỏ (silent drop) các gói SYN mới mà không gửi RST. Client bị timeout từ 3 đến 21 giây do phải TCP retransmit, người dùng thấy ứng dụng quay đơ không phản hồi, trong khi server logs hoàn toàn không ghi nhận bất kỳ access log nào vì kết nối chưa bao giờ hoàn tất bắt tay 3 bước.
* **Cơ chế tự động hóa**: Tự động cấu hình sysctl `net.core.somaxconn = 65535` và `net.ipv4.tcp_max_syn_backlog = 65535` trong initContainer của Node; đặt `listen(fd, 65535)` trong backend Go; cấu hình eBPF probe theo dõi bộ đếm `ListenOverflows` và `ListenDrops` từ `/proc/net/netstat` để kích hoạt autoscaling pod trước khi backlog tràn.

### 103. TCP Window Starvation & Bandwidth-Delay Product (BDP) Collapse Trên Cross-Region VPC Peering
* **Cơ chế lỗi**: Hai cụm Kubernetes đồng bộ dữ liệu Kafka hoặc database backup qua đường truyền VPC Peering xuyên lục địa (ví dụ: us-east-1 tới ap-southeast-1, RTT ~200ms, băng thông 10Gbps). Kích thước TCP buffer mặc định của Linux (`net.ipv4.tcp_wmem` và `net.ipv4.tcp_rmem` giới hạn ở 4MB) không đủ lớn để lấp đầy BDP ($\text{BDP} = 10\text{Gbps} \times 0.2\text{s} = 250\text{MB}$). Tốc độ truyền tải thực tế bị bóp nghẹt xuống chỉ còn 20–30MB/s (1% dung lượng mạng), làm việc sao lưu và đồng bộ chậm trễ hàng chục giờ.
* **Cơ chế tự động hóa**: Tự động phát hiện các kết nối liên vùng (RTT > 50ms) và áp dụng thuật toán TCP BBR (`net.ipv4.tcp_congestion_control = bbr`); mở rộng kích thước buffer tối đa: `net.core.rmem_max = 67108864`, `net.core.wmem_max = 67108864`; bật TCP Window Scaling (`net.ipv4.tcp_window_scaling = 1`).

### 104. ARP Cache Table Saturation (`gc_thresh3`) Gây Silent Packet Drop Trong Cụm /16 Flat Network
* **Cơ chế lỗi**: Trong cụm Kubernetes baremetal lớn sử dụng CNI Flat Network (Host-GW hoặc Calico mà không có overlay) với dải mạng `/16` (hơn 1,000 node và hàng chục ngàn Pod IPs). Khi một node cố gắng giao tiếp với các pods khác, bảng ARP cache của kernel vượt quá ngưỡng dọn dẹp rác (`net.ipv4.neigh.default.gc_thresh3 = 1024`). Kernel từ chối thêm entry ARP mới và in lỗi `neighbor table overflow`. Các kết nối mạng tới các node/pod ngẫu nhiên trong cụm bị rớt hoàn toàn gói tin mà không có lý do rõ ràng.
* **Cơ chế tự động hóa**: Giám sát kích thước ARP cache table; cấu hình tự động nâng ngưỡng qua tuning daemonset: `gc_thresh1 = 4096`, `gc_thresh2 = 8192`, `gc_thresh3 = 16384`; khuyến nghị chuyển đổi sang eBPF-based IP routing (Cilium) để thay thế hoàn toàn bảng ARP truyền thống bằng eBPF maps.

### 105. Kernel Conntrack Table Saturation (`nf_conntrack_max`) Đánh Sập Toàn Bộ Outbound/NAT Traffic
* **Cơ chế lỗi**: Tất cả traffic đi qua Kube-proxy (iptables mode) hoặc NAT Gateway trên node Linux đều phải tạo entry trong bảng Conntrack để theo dõi trạng thái luồng. Khi microservices mở hàng loạt kết nối UDP DNS hoặc short-lived HTTP không tái sử dụng connection, số lượng entry vượt quá `net.netfilter.nf_conntrack_max`. Kernel từ chối nhận diện các gói tin mới và drop sạch: `nf_conntrack: table full, dropping packet`. Toàn bộ node bị cô lập mạng: CoreDNS không phân giải được, database connection bị đứt, Kubelet không thể heartbeat về API server.
* **Cơ chế tự động hóa**: Giám sát tỷ lệ lấp đầy conntrack (`node_nf_conntrack_entries / node_nf_conntrack_entries_limit`); tự động nâng `nf_conntrack_max = 1048576`; giảm `nf_conntrack_tcp_timeout_close_wait = 10` và `nf_conntrack_tcp_timeout_time_wait = 30`; tự động bypass conntrack cho các port nội bộ hoặc chuyển đổi sang IPVS / Cilium eBPF mode.

---

## TẦNG 25: LINUX VFS, INODES, CORRUPTED METADATA & D-STATE LOCKUPS (KỊCH BẢN 106 — 110)

### 106. Inode Exhaustion Với 90% Ổ Đĩa Còn Trống Do Hàng Triệu File Rác 0-Byte
* **Cơ chế lỗi**: Ứng dụng PHP/Node.js/Python sinh ra hàng triệu session files hoặc temporary uploads có kích thước 0 byte hoặc vài bytes trong `/tmp` hoặc thư mục log. Ổ đĩa báo dung lượng còn trống 500GB (chỉ dùng 10% blocks), nhưng số lượng Inodes khả dụng (`df -i`) đã chạm 100%. Mọi thao tác tạo file, ghi log, hoặc khởi tạo socket mới của hệ thống đều báo lỗi `No space left on device` (ENOSPC). Kubelet không thể khởi động container mới, hệ điều hành tê liệt.
* **Cơ chế tự động hóa**: Triển khai Inode Watchdog kiểm tra metric `node_filesystem_files_free / node_filesystem_files`; khi Inode usage > 85%, tự động định vị top 5 thư mục có mật độ file cao nhất bằng thuật toán tối ưu `find / -xdev -printf '%h\n' | sort | uniq -c | sort -nr | head -5`; tự động kích hoạt cronjob dọn dẹp file tạm cũ hơn 24 giờ.

### 107. Uninterruptible Sleep (D-State) Process Lockup Không Thể Kill Bằng SIGKILL
* **Cơ chế lỗi**: Một tiến trình I/O (ví dụ backup agent hoặc database writer) thực hiện lệnh `write()` lên một NFS volume hoặc Ceph mount đang bị đứt cáp mạng. Tiến trình rơi vào trạng thái Uninterruptible Sleep (D-State trong kernel). Tiến trình này nắm giữ file lock độc quyền hoặc semaphore bộ nhớ. Kỹ sư thực hiện `kill -9 <PID>` nhưng lệnh hoàn toàn vô hiệu vì kernel không phân phối tín hiệu cho tiến trình đang chờ hardware/NFS I/O. Cụm Kubernetes không thể terminate pod, pod kẹt ở `Terminating` nhiều ngày.
* **Cơ chế tự động hóa**: Giám sát số lượng D-State processes trên node qua `/proc/[pid]/status`; sử dụng eBPF hoặc `dmesg` để phát hiện kernel call trace bị nghẽn trong VFS; tự động ngắt kết nối NFS bị treo bằng `umount -f -l <mount_point>` (lazy unmount); nếu process nắm giữ tài nguyên quan trọng quá 15 phút mà không nhả, kích hoạt SysRq reboot an toàn (`echo b > /proc/sysrq-trigger`).

### 108. Zombie Process Fork Bomb & Cạn Kiệt PID Table (`pid_max`) Thiếu Subreaper Tini
* **Cơ chế lỗi**: Container chạy ứng dụng Node.js hoặc Python sinh ra các tiến trình con bằng `fork()` / `exec()` nhưng không gọi `waitpid()` để thu dọn exit status. Do trong container ứng dụng đóng vai trò PID 1 nhưng không được tích hợp logic init system (subreaper), các tiến trình chết biến thành Zombie (`<defunct>`). Khi số lượng Zombie chạm ngưỡng `/proc/sys/kernel/pid_max` (thường là 32,768 hoặc 4,194,304), hệ điều hành máy chủ không thể cấp phát thêm PID mới. Bất kỳ lệnh nào chạy trên host (kể cả `ps`, `ls`, `ssh`) đều báo lỗi `fork: Cannot allocate memory`.
* **Cơ chế tự động hóa**: Bắt buộc cấu hình Pod Security Standard hoặc OCI runtime bật `shareProcessNamespace: false` và sử dụng `tini` / `dumb-init` làm entrypoint cho mọi container; thiết lập `pidsLimit` trong K8s Pod spec (`spec.containers[].resources.limits.pids = 1000`) để ngăn chặn một pod làm cạn kiệt PID của toàn bộ máy chủ.

### 109. Transparent HugePages (THP) Defragmentation Latency Spikes Gây Đơ Hệ Thống Hàng Giây
* **Cơ chế lỗi**: Linux kernel mặc định bật tính năng Transparent HugePages (`/sys/kernel/mm/transparent_hugepage/enabled = always`). Khi cơ sở dữ liệu như Redis, PostgreSQL, hoặc JVM phân bổ bộ nhớ động, tiến trình `khugepaged` chạy ngầm để nén và dọn dẹp phân mảnh (memory defragmentation). Quá trình này khóa bộ nhớ ảo (memory lock) trên toàn hệ thống. Redis latency đột ngột nhảy vọt từ 0.5ms lên 8,000ms. Clients bị timeout hàng loạt, các probe Kubernetes tưởng Redis chết nên kill pod, tạo thành chu kỳ crash loop thảm họa.
* **Cơ chế tự động hóa**: Tạo daemonset tinh chỉnh kernel lúc boot máy chủ: thiết lập `echo madvise > /sys/kernel/mm/transparent_hugepage/enabled` và `echo never > /sys/kernel/mm/transparent_hugepage/defrag`; tự động kiểm tra trạng thái THP trong checklist sức khỏe của các node chạy database.

### 110. NFS Stale File Handle (`ESTALE`) Cascade Trên Shared Storage Sau Khi Server Khởi Động Lại
* **Cơ chế lỗi**: Cụm Kubernetes sử dụng NFS server (hoặc EFS/Filestore) cho shared storage giữa nhiều pods. Khi NFS server khởi động lại hoặc thay đổi inode id của thư mục sau một thao tác failover, các client K8s nodes vẫn giữ file handle cũ trong bộ nhớ đệm VFS. Mọi thao tác đọc/ghi file từ ứng dụng lập tức ném lỗi `ESTALE (Stale file handle)`. Ứng dụng crash, pod restart nhưng vẫn mount lại đúng mountpoint bị stale nên tiếp tục crash.
* **Cơ chế tự động hóa**: Triển khai Storage Health Checker phát hiện lỗi `ESTALE`; tự động thực hiện remount với cờ `-o remount` hoặc lazy unmount và mount lại volume mới; gửi alert yêu cầu kiểm tra tính nhất quán dữ liệu của NFS server.

---

## TẦNG 26: KUBERNETES CONTROL PLANE, WEBHOOK DEADLOCKS & PLEG FREEZES (KỊCH BẢN 111 — 115)

### 111. Mutating/Validating Webhook Timeout Deadlock Làm Đóng Băng Việc Tạo Mọi Pod Cụm
* **Cơ chế lỗi**: Một Mutating/Validating Webhook được cấu hình để kiểm tra bảo mật (ví dụ OPA Gatekeeper, Kyverno, hoặc Istio Injector) với chính sách `failurePolicy: Fail`. Do sự cố mạng hoặc thiếu tài nguyên, toàn bộ các Pod của webhook controller bị chết hoặc node chứa webhook bị khởi động lại. Khi K8s API server cố gắng tạo lại Pod cho chính webhook controller đó, request tạo Pod lại bị chuyển hướng đến webhook service (vốn đang chết!). Cả cụm rơi vào **Tử Huyệt Deadlock**: Không thể tạo bất kỳ Pod nào trong toàn bộ cụm, kể cả pod cứu hộ!
* **Cơ chế tự động hóa**: Cấu hình `failurePolicy: Ignore` hoặc thiết lập `namespaceSelector` loại trừ tuyệt đối namespace hệ thống (`kube-system`, `gatekeeper-system`); cấu hình `timeoutSeconds: 3` (không để mặc định 10-30s); webhook controllers phải sử dụng `priorityClassName: system-cluster-critical` và chạy tối thiểu 3 replicas trên các node vật lý khác nhau; k8s-selfhost watchdog tự động phát hiện webhook timeout và tự động patch emergency disable webhook khi phát hiện deadlock.

### 112. Orphaned Finalizers Deadlock Ghi Kẹt Namespace Ở Trạng Thái `Terminating` Vĩnh Viễn
* **Cơ chế lỗi**: Khi xóa một namespace (`kubectl delete ns staging`), K8s controller chờ tất cả tài nguyên con được dọn dẹp. Một Custom Resource (CR) trong namespace mang một `finalizer` trỏ tới một Custom Resource Definition (CRD) hoặc APIService đã bị xóa trước đó. K8s namespace controller không thể liên lạc với API handler để thực thi dọn dẹp tài nguyên con, dẫn đến việc namespace bị kẹt ở trạng thái `Terminating` suốt nhiều tháng. Các công cụ CI/CD cố gắng tạo lại namespace cùng tên đều thất bại.
* **Cơ chế tự động hóa**: Tự động rà soát các namespaces kẹt `Terminating` > 10 phút; truy quét các custom resources mồ côi nằm bên trong bằng script liệt kê discovery API; tự động bóc tách và xóa bỏ finalizers bị kẹt (`kubectl patch cr <name> -p '{"metadata":{"finalizers":null}}' --type=merge`) để giải phóng namespace an toàn.

### 113. Kubelet PLEG (Pod Lifecycle Event Generator) Timeout Lockup Do Containerd Treo I/O
* **Cơ chế lỗi**: Kubelet định kỳ gọi Container Runtime Interface (CRI) để liệt kê trạng thái của tất cả containers trên node thông qua cơ chế PLEG (chu kỳ 3 phút). Khi một container thực hiện quá nhiều thao tác I/O trên ổ cứng đĩa chậm hoặc containerd daemon bị deadlock bộ nhớ, lệnh CRI gRPC `ListContainers` bị treo quá 3 phút. Kubelet ghi nhận lỗi `PLEG is not healthy` và chuyển node sang trạng thái `NotReady`. Controller manager lập tức trục xuất (evict) hàng loạt Pods khỏi node, dồn tải sang các node khác và gây sập dây chuyền toàn cụm.
* **Cơ chế tự động hóa**: Giám sát latency của metric `kubelet_pleg_relist_duration_seconds`; tách riêng ổ đĩa chứa root filesystem và ổ đĩa chứa thư mục `/var/lib/containerd`; tự động restart containerd daemon khi CRI calls bị timeout 3 lần liên tiếp trước khi PLEG đánh sập node.

### 114. Controller-Manager & Scheduler Leader Election Flapping Do Disk Latency Trên etcd
* **Cơ chế lỗi**: Các thành phần K8s Control Plane (`kube-controller-manager`, `kube-scheduler`) sử dụng cơ chế Leader Election dựa trên Lease API lưu trong etcd (`leaseDuration: 15s`, `renewDeadline: 10s`). Khi đĩa SSD chạy etcd có `fsync` latency tăng đột biến từ 2ms lên 50ms (do chạy chung với các ứng dụng ghi nặng hoặc cloud EBS throttling), các node Master không thể gia hạn Lease đúng hạn. Hai master controllers liên tục tranh cướp quyền lãnh đạo (Flapping), dẫn đến các quyết định reconcile pod bị đảo lộn, pod bị kill và tạo liên tục gây nhiễu loạn toàn hệ thống.
* **Cơ chế tự động hóa**: Giám sát chặt chẽ `etcd_disk_wal_fsync_duration_seconds` (phải < 10ms ở 99th percentile); cấp phát ổ đĩa NVMe chuyên dụng với IOPS cao nhất cho etcd WAL; tự động cách ly các workload không thuộc control plane ra khỏi master nodes.

### 115. ConfigMap / Secret 1MB etcd Limit Exceeded Đánh Sập Deployment Đột Ngột
* **Cơ chế lỗi**: etcd có giới hạn cứng về kích thước của một object lưu trữ là 1.5MB (mặc định khuyến nghị của K8s là 1MB). Khi một developer vô tình bundle tài sản nhị phân (như file model AI nhỏ, database seed dump, hoặc file cấu hình lớn) vào bên trong K8s ConfigMap hoặc Secret, API server từ chối request với lỗi `etcdserver: request is too large`. Các pipeline Helm/GitOps bị crash giữa chừng, để lại release ở trạng thái `failed/pending-upgrade` và không thể rollback tự động.
* **Cơ chế tự động hóa**: Kiểm tra kích thước của tất cả manifest ConfigMap/Secret trong CI/CD pipeline trước khi deploy; nếu kích thước vượt quá 500KB, tự động kích hoạt cảnh báo và chuyển hướng lưu trữ sang External Storage (S3 / Persistent Volume) với cơ chế mount trong initContainer.

---

## TẦNG 27: DATABASE & CACHE CRITICAL CATASTROPHES (KỊCH BẢN 116 — 120)

### 116. PostgreSQL Transaction ID (XID) Wraparound Freeze Đưa Database Về Chế Độ Read-Only Cưỡng Chế
* **Cơ chế lỗi**: PostgreSQL sử dụng số nguyên 32-bit cho Transaction ID (tối đa ~4 tỷ giao dịch). Cơ chế MVCC yêu cầu `autovacuum` phải chạy liên tục để đóng băng (freeze) các tuple cũ. Nếu autovacuum bị chặn bởi một transaction dài (long-running query hoặc uncommitted transaction bị bỏ quên) suốt nhiều tuần, số lượng transaction khả dụng giảm dần về 0. Khi chạm ngưỡng 10 triệu transactions cuối cùng, PostgreSQL tự động phát lệnh ngắt khẩn cấp và chuyển toàn bộ database sang chế độ **Chỉ Đọc (Read-Only)** để bảo vệ dữ liệu khỏi bị ghi đè. Mọi ứng dụng ghi đều sập hoàn toàn.
* **Cơ chế tự động hóa**: Giám sát metric `pg_database_wraparound_age`; tự động gửi cảnh báo khẩn cấp khi tuổi transaction vượt quá 1 tỷ; cấu hình tự động kill các transactions chạy quá 1 giờ (`statement_timeout = 3600000`) và tinh chỉnh `autovacuum_freeze_max_age` cùng `autovacuum_cost_limit = 2000` để autovacuum luôn đủ tài nguyên I/O hoàn tất trước khi nguy cấp.

### 117. Redis BGSAVE Fork Copy-on-Write (CoW) Memory Spike Kích Hoạt Linux OOM Killer
* **Cơ chế lỗi**: Redis lưu trữ 25GB dữ liệu trên máy chủ có 32GB RAM. Khi Redis thực thi lệnh lưu snapshot RDB (`BGSAVE`) hoặc ghi AOF rewrite, nó gọi kernel hàm `fork()` để tạo tiến trình con. Nếu lúc đó ứng dụng có lưu lượng ghi cao, cơ chế Copy-on-Write của Linux phải nhân bản các trang bộ nhớ bị sửa đổi. Tổng lượng RAM tiêu thụ vọt lên 45GB. Linux OOM Killer lập tức can thiệp và tiêu diệt tiến trình cha Redis Master, đánh mất toàn bộ cache nóng và gây sập dây chuyền xuống toàn bộ hệ thống frontend.
* **Cơ chế tự động hóa**: Cấu hình quy tắc an toàn: Không bao giờ cấp phát dữ liệu Redis vượt quá 50-60% dung lượng RAM vật lý của máy chủ; cấu hình sysctl `vm.overcommit_memory = 1`; tắt hoàn toàn background RDB snapshotting trên Redis Master và chuyển trách nhiệm snapshot sang Redis Read-Replica chuyên dụng.

### 118. Redis Replication Buffer Overflow Gây Vòng Lặp Vô Tận Full Resync Storm
* **Cơ chế lỗi**: Khi mạng giữa Redis Master và Redis Replica bị chập chờn trong 10 giây hoặc replica bị chậm I/O, master lưu các thay đổi vào bộ đệm replication (`client-output-buffer-limit replica`). Khi bộ đệm vượt quá giới hạn (ví dụ 256MB), master ngắt kết nối replica. Khi kết nối lại, replica yêu cầu Partial Resynchronization (PSYNC) nhưng offset đã bị mất, buộc master phải thực hiện Full Resync (sinh lại toàn bộ file RDB và gửi qua mạng). Quá trình gửi RDB lớn lại làm đầy buffer, master lại ngắt kết nối, tạo thành **Vòng Lặp Resync Bão Táp (Full Resync Storm)** vắt kiệt CPU và băng thông mạng của Master.
* **Cơ chế tự động hóa**: Tự động cấu hình `client-output-buffer-limit replica 1gb 512mb 60`; mở rộng kích thước `repl-backlog-size = 512mb` để tăng thời gian chịu đựng mất kết nối tạm thời; giám sát metric `connected_slaves` và replication lag để tự động ngắt kết nối replica lỗi trước khi kéo sập master.

### 119. Kafka Consumer Group Head-of-Line Blocking Do Poison Pill Message Serialization Bug
* **Cơ chế lỗi**: Một producer gửi vào Kafka topic một message bị lỗi cấu trúc dữ liệu hoặc phiên bản schema mới mà consumer không thể parse được (Poison Pill). Consumer ném ngoại lệ `SerializationException` hoặc `UnmarshalError` và crash trước khi commit offset. K8s khởi động lại consumer pod, pod lại đọc đúng message lỗi đó ở đầu partition và crash tiếp. Toàn bộ Consumer Group bị đình trệ (Head-of-Line Blocking), độ trễ (Consumer Lag) tăng lên hàng triệu messages, dữ liệu thời gian thực bị đóng băng hoàn toàn.
* **Cơ chế tự động hóa**: Triển khai mẫu hình **Dead Letter Queue (DLQ)** tự động trong framework consumer Go/Java: khi unmarshal thất bại, bắt exception, gửi payload lỗi cùng stack trace sang `topic-error-dlq`, sau đó commit offset để tiếp tục xử lý các message tiếp theo; gửi alert tới đội ngũ phát triển kèm link xem message bị lỗi trên dashboard.

### 120. Elasticsearch High Disk Watermark Flood Stage Đóng Băng Mọi Chỉ Mục Log Read-Only
* **Cơ chế lỗi**: Cụm Elasticsearch/Opensearch lưu trữ log hệ thống và audit trail. Khi dung lượng ổ đĩa trên một data node chạm ngưỡng 95% (`cluster.routing.allocation.disk.watermark.flood_stage`), Elasticsearch tự động chuyển tất cả các index trên cụm sang chế độ `read_only_allow_delete = true`. Mọi request đẩy log từ Vector, Fluentbit, hoặc Logstash đều bị từ chối với lỗi `HTTP 403 ClusterBlockException`. Mất mát toàn bộ log và giám sát trong giai đoạn xảy ra sự cố.
* **Cơ chế tự động hóa**: Giám sát dung lượng đĩa trước ngưỡng 80%; tự động cấu hình **Index Lifecycle Management (ILM)** xóa các chỉ mục log cũ hơn 30 ngày hoặc tự động di chuyển các index sang Cold/Frozen tier lưu trên object storage rẻ hơn; tự động chạy lệnh unblock `index.blocks.read_only_allow_delete: null` ngay sau khi dung lượng đĩa được giải phóng.

---

## TẦNG 28: ADVANCED DEVSECOPS, ZERO-TRUST & SUPPLY CHAIN ATTACKS (KỊCH BẢN 121 — 125)

### 121. Dependency Confusion & Typosquatting Tấn Công Chiếm Quyền CI/CD Pipeline
* **Cơ chế lỗi**: Công ty sử dụng các thư viện nội bộ (ví dụ `@company/auth-utils` trong NPM hoặc `github.com/company/common` trong Go). Kẻ tấn công đăng ký một package trùng tên nhưng có số version cực cao (ví dụ `99.9.9`) trên public NPM registry hoặc public proxy. Khi build runner trong CI/CD tải dependencies mà không chỉ định private registry an toàn, hệ thống tự động ưu tiên kéo phiên bản độc hại từ public registry về. Mã độc chạy ngay trong giai đoạn postinstall và gửi toàn bộ biến môi trường (AWS Keys, K8s tokens) về máy chủ của hacker.
* **Cơ chế tự động hóa**: Cấu hình bắt buộc scoped registry private (`.npmrc`, `GOPRIVATE`); triển khai Dependency Firewall (Sonatype Nexus hoặc JFrog Artifactory) chặn kéo các package có scope nội bộ từ Internet; kích hoạt quét SCA tự động trước khi build và khóa phiên bản bằng lockfile nghiêm ngặt (`package-lock.json`, `go.sum`).

### 122. Compromised CI/CD Runner Privilege Escalation Qua Docker Socket (`docker.sock`)
* **Cơ chế lỗi**: Để hỗ trợ kỹ sư build container image trong pipeline (Docker-in-Docker), một số admin gắn volume `/var/run/docker.sock` từ máy chủ vật lý vào bên trong container của GitLab Runner hoặc GitHub Actions runner. Một commit chứa pull request từ bên ngoài thực thi script gọi Docker API trên host, khởi tạo một container mới với cờ `--privileged -v /:/host` và chiếm quyền điều khiển root hoàn toàn trên máy chủ mẹ, cài mã độc đào coin và đánh cắp credential của cụm.
* **Cơ chế tự động hóa**: Cấm tuyệt đối việc mount `/var/run/docker.sock` trong bất kỳ Pod nào; thay thế hoàn toàn công nghệ build bằng **Rootless Container Builders** chạy hoàn toàn trong userspace (như Kaniko, Buildah, hoặc Google distroless); kích hoạt OPA Gatekeeper policy cấm mount hostPath socket.

### 123. Sidecar Injection Race Condition Làm Rò Rỉ Traffic Không Mã Hóa Ra Mạng Ngoài
* **Cơ chế lỗi**: Trong Service Mesh (Istio / Linkerd), một initContainer hoặc sidecar Envoy được inject để định tuyến mTLS. Khi pod khởi động, container ứng dụng chính (App) có thể khởi động nhanh hơn sidecar Envoy. Trong vài giây đầu tiên, ứng dụng gửi các bản tin HTTP không mã hóa hoặc kết nối tới các dịch vụ bên ngoài trực tiếp mà không đi qua chính sách kiểm soát của proxy. Kẻ tấn công trên cùng mạng có thể nghe lén dữ liệu nhạy cảm hoặc bypass authentication.
* **Cơ chế tự động hóa**: Bật tính năng Kubernetes Native Sidecar Containers (từ K8s 1.28+) với cờ `restartPolicy: Always` trong `initContainers` để bảo đảm proxy khởi động và sẵn sàng 100% trước khi app container được phép boot; cấu hình `holdApplicationUntilProxyStarts: true` trong cấu hình Istio.

### 124. Cloud Instance Metadata IPv6 / DNS Rebinding SSRF Bỏ Qua Chặn Metadata IPv4
* **Cơ chế lỗi**: Hệ thống có cơ chế chống SSRF (Server-Side Request Forgery) bằng cách lọc các URL trỏ về IPv4 `169.254.169.254`. Kẻ tấn công khai thác địa chỉ IPv6 của AWS Nitro Metadata (`[fd00:ec2::254]`) hoặc sử dụng kỹ thuật DNS Rebinding (tên miền trỏ về IP nội bộ sau khi vượt qua vòng kiểm tra DNS đầu tiên). Ứng dụng tải URL do hacker cung cấp, lấy được IAM Role Temporary Credentials của EC2 instance và chiếm đoạt tài khoản cloud.
* **Cơ chế tự động hóa**: Sử dụng NetGuard SSRF Validator kiểm tra toàn diện cả IPv4 và IPv6 (chặn dải Link-Local `fe80::/10` và `fd00::/8`); bắt buộc kích hoạt **AWS IMDSv2** với `HttpTokens=required` và `HttpPutResponseHopLimit=1` để triệt tiêu hoàn toàn khả năng khai thác SSRF từ container.

### 125. Stale Projected ServiceAccount Token Replay Attack Trên Cụm Phân Tán
* **Cơ chế lỗi**: Kubernetes cấp phát ServiceAccount token cho pod. Nếu sử dụng static token trong Secret cũ, token này không bao giờ hết hạn. Kẻ tấn công trích xuất token từ bộ nhớ hoặc log, sau đó tái sử dụng token từ một máy chủ ở bên ngoài Internet để gọi K8s API Server nhiều tháng sau đó mà không bị phát hiện.
* **Cơ chế tự động hóa**: Bắt buộc sử dụng **Bound Service Account Token Volume** (`BoundServiceAccountTokenVolume`) với thời gian sống ngắn (`expirationSeconds: 3600`) và gắn chặt với identity của Pod; kích hoạt Token Audience (`audiences`) để token chỉ có giá trị khi gửi đến đúng dịch vụ mục tiêu; tự động vô hiệu hóa token ngay khi pod bị xóa khỏi cụm.

---

## TẦNG 29: CLOUD HARDWARE, SPOT TERMINATION & HYBRID DISASTERS (KỊCH BẢN 126 — 130)

### 126. AWS EBS IOPS Burst Credit Exhaustion Làm Đóng Băng Hoàn Toàn Ổ Đĩa Dữ Liệu
* **Cơ chế lỗi**: Ứng dụng chạy trên volume AWS EBS gp2 dung lượng nhỏ (ví dụ 100GB). gp2 sử dụng cơ chế cấp điểm Burst Credit để đạt 3,000 IOPS trong thời gian ngắn, nhưng tốc độ cơ bản (baseline) chỉ là $3 \times 100 = 300\text{ IOPS}$. Khi ứng dụng có đợt ghi tải nặng kéo dài vài giờ, toàn bộ Burst Credits bị cạn kiệt. AWS bóp nghẹt tốc độ đĩa xuống mức baseline tối thiểu. Thao tác ghi của PostgreSQL hoặc Elasticsearch bị nghẽn, Disk I/O Queue nhảy vọt lên hàng trăm, ứng dụng hoàn toàn bị freeze không phản hồi.
* **Cơ chế tự động hóa**: Di chuyển 100% volumes từ gp2 sang **gp3** với baseline 3,000 IOPS độc lập với dung lượng; giám sát metric AWS CloudWatch `BurstBalance` (đối với gp2) và tự động kích hoạt resize volume hoặc tăng IOPS qua AWS API trước khi credit chạm mức 20%.

### 127. Cloud Provider API Throttling (`RequestLimitExceeded`) Làm Tê Liệt Cluster Autoscaler
* **Cơ chế lỗi**: Trong một đợt scale-up khẩn cấp, hàng chục tools trong cụm (Cluster Autoscaler, ExternalDNS, AWS Load Balancer Controller, Karpenter, Datadog Agent) đồng thời gửi hàng ngàn requests tới AWS/GCP API (ví dụ `DescribeInstances`, `DescribeSubnets`). Cloud provider áp dụng giới hạn Rate Limit và trả về lỗi `HTTP 429 Too Many Requests` hoặc `RequestLimitExceeded`. Cluster Autoscaler không thể gọi lệnh tạo thêm node mới. Cụm bị kẹt với hàng trăm Pods ở trạng thái `Pending` trong khi người dùng không thể truy cập hệ thống.
* **Cơ chế tự động hóa**: Triển khai AWS API Proxy Cache (local caching cho read-only API calls); cấu hình Exponential Backoff với Jitter cho tất cả controllers; giảm tần suất polling của Cluster Autoscaler (`--scan-interval=30s`); tích hợp thông báo alert khi AWS API Throttling metric bắt đầu xuất hiện.

### 128. Cloud Spot Instance 2-Minute Termination Warning Bị Bỏ Quên Gây Mất Dữ Liệu
* **Cơ chế lỗi**: Doanh nghiệp sử dụng Spot/Preemptible Instances để tiết kiệm 70% chi phí. Khi cloud provider cần thu hồi máy chủ, họ gửi một cảnh báo Termination Notice qua Instance Metadata Service trước đúng 2 phút. Nếu cụm không có daemon lắng nghe cảnh báo này, máy chủ sẽ bị ngắt điện đột ngột sau 120 giây. Các tiến trình đang xử lý dở giao dịch thanh toán hoặc đang ghi file bị mất dữ liệu, các kết nối TCP bị drop đột ngột gây ra hàng ngàn lỗi 502/504 cho người dùng.
* **Cơ chế tự động hóa**: Cài đặt `AWS Node Termination Handler` (hoặc GCP Preemptible Killer); liên tục polling endpoint metadata mỗi 5 giây; ngay khi nhận được tín hiệu cảnh báo 2 phút, lập tức thực hiện `kubectl cordon` node và `kubectl drain` với `grace-period=90s`, chuyển hướng traffic sang các node khác một cách êm ái trước khi máy chủ bị tắt.

### 129. Cloud NAT Gateway SNAT Port Exhaustion Gây Đứt Kết Nối Ra Mạng Ngoài Ngẫu Nhiên
* **Cơ chế lỗi**: Hàng trăm Pods trên nhiều node trong private subnet gửi concurrent requests tới các dịch vụ bên ngoài (Payment Gateways, Twilio, OpenAI API) thông qua một Cloud NAT Gateway duy nhất. Một địa chỉ IP của NAT Gateway chỉ cung cấp tối đa 64,512 SNAT ports. Khi số lượng kết nối đồng thời tới cùng một destination IP:port vượt quá số port khả dụng, NAT Gateway âm thầm drop các gói SYN mới. Khoảng 5-10% các giao dịch thanh toán của khách hàng bị timeout ngẫu nhiên dù server nội bộ hoàn toàn bình thường.
* **Cơ chế tự động hóa**: Giám sát CloudWatch metric `ErrorPortAllocation`; tự động gán thêm các Elastic IP dự phòng vào NAT Gateway (mỗi IP bổ sung thêm 64,000 ports); cấu hình Connection Pooling chặt chẽ trên microservices để tái sử dụng outbound TCP sockets; triển khai VPC Endpoints (PrivateLink) cho các dịch vụ AWS nội bộ để tránh đi qua NAT Gateway.

### 130. KMS Key Revocation / Expiration Đánh Sập Toàn Bộ Hệ Thống Trong Một Giờ
* **Cơ chế lỗi**: Toàn bộ Secret, Config, và EBS Volumes của cụm Kubernetes được mã hóa bằng Customer Managed Key (CMK) trên AWS KMS hoặc HashiCorp Vault. Do sơ suất hành chính, khóa KMS bị hết hạn (expiration date) hoặc chính sách IAM Key Policy bị chỉnh sửa sai quyền. Khi Pod mới khởi động hoặc Kubelet cần mount lại volume, lệnh giải mã bị từ chối với lỗi `KMS.AccessDeniedException`. Các pod crash loop không thể đọc mật khẩu database, toàn bộ cụm rơi vào tình trạng đóng băng trên diện rộng.
* **Cơ chế tự động hóa**: Giám sát thời hạn của KMS Keys và Vault Certificates trước 30 ngày; thiết lập Canary Secret Decryption Probe chạy mỗi 60 giây: nếu probe không giải mã được canary secret, lập tức kích hoạt P0 alert khẩn cấp lên Telegram SRE Copilot trước khi các ứng dụng chính bị ảnh hưởng; duy trì Break-Glass Offline Recovery Key được niêm phong an toàn.
