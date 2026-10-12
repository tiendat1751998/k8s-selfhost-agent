# HIGH-PERFORMANCE GO ARCHITECTURE: PARITY WITH JAVA LMAX DISRUPTOR & NETTY
## PRODUCTION-GRADE SPECIFICATION, MECHANICAL SYMPATHY & FAILURE MODES ANALYSIS

---

## 1. MỔ XẺ VÀ GIẢI PHẪU KHIẾM KHUYẾT CỦA MÃ REFERENCE ("TOY CODE" DECONSTRUCTION)

Đoạn mã tham chiếu trong tài liệu sơ bộ ban đầu mặc dù đã tiếp cận khái niệm Cache-Line Padding và Ring Buffer, nhưng khi đưa vào môi trường xử lý hàng chục triệu sự kiện/giây (High-Frequency Trading, Telemetry Ingestion, Core Gateway) vẫn là một **"Toy Implementation"** và sẽ sập hoặc gây sai lệch dữ liệu nghiêm trọng vì 6 lý do cốt lõi sau:

### Khiếm khuyết 1: Padding 1 chiều (Unidirectional Padding) không triệt tiêu được False Sharing
* **Mã tham chiếu:**
  ```go
  type Sequence struct {
      cursor int64
      _      [56]byte // 64 - 8 = 56
  }
  ```
* **Bản chất phần cứng:** Bộ nhớ trong kiến trúc x86-64 và ARM64 được CPU nạp theo từng khối **64 byte (Cache Line)**. Tuy nhiên, bộ cấp phát bộ nhớ của Go (mcache/mcentral/mheap) chỉ căn chỉnh địa chỉ của biến kiểu `int64` theo bội số của **8 byte**, hoàn toàn **không đảm bảo căn chỉnh 64 byte** cho toàn bộ struct `Sequence`!
* **Hậu quả:** 
  Nếu struct `Sequence` bắt đầu tại offset `0x...38` (56 byte tính từ đầu cache line), thì trường `cursor` (8 byte) sẽ nằm ở những byte cuối cùng của Cache Line A (từ byte 56 đến 64). Toàn bộ 56 byte padding `_ [56]byte` lại nằm trọn trong Cache Line B! Khi đó:
  1. Trường `cursor` vẫn nằm chung Cache Line A với bất kỳ biến nào đứng ngay trước nó trong bộ nhớ.
  2. Bất kỳ thao tác ghi nào lên biến đứng trước sẽ phát tín hiệu **Invalidate** (qua giao thức MESI/MOESI), làm bay màu cache line chứa `cursor` trên tất cả các core khác.
* **Giải pháp chuẩn:** Phải áp dụng **Bi-directional Padding (Padding 2 chiều)**: Đặt padding 56 byte *phía trước* VÀ 56 byte *phía sau* biến `cursor` (hoặc ép struct alignment qua mảng 64 byte), đảm bảo dù struct rơi vào bất kỳ offset 8-byte nào, trường `cursor` luôn chiếm độc quyền một Cache Line riêng biệt mà không chia sẻ với bất kỳ ai.

### Khiếm khuyết 2: Monolithic Struct 75.5 MB – Rủi ro Stack/Heap Allocator & Copy-by-Value
* **Mã tham chiếu:**
  ```go
  type LockFreeRingBuffer struct {
      ring    [BufferSize]Event // BufferSize = 1M, sizeof(Event) = 72 byte => ~75.5 MB
      written Sequence
      read    Sequence
  }
  ```
* **Bản chất phần cứng:** Việc nhúng trực tiếp mảng cố định `[1048576]Event` vào thân struct tạo ra một cấu trúc dữ liệu nguyên khối nặng tới **75,497,472 bytes**.
* **Hậu quả:**
  1. Nếu lập trình viên sơ suất truyền struct này theo giá trị (`func process(rb LockFreeRingBuffer)` thay vì `*LockFreeRingBuffer`), Go runtime buộc phải thực thi lệnh sao chép bộ nhớ `runtime.memmove` 75.5 MB, gây treo toàn bộ CPU thread và tràn stack (stack overflow) ngay lập tức.
  2. Bộ cấp phát TCMalloc trong Go runtime phải tìm kiếm một vùng nhớ liên tục (contiguous span) trên heap lớn hơn 75MB. Việc này làm phân mảnh heap và kích hoạt các bước scan trang nhớ nặng nề.
* **Giải pháp chuẩn:** Tách ring storage thành một vùng đệm phẳng (Flat Slice) được pre-allocate cố định, đóng gói bằng con trỏ và kích thước dạng mũ 2 ($2^N$), tách biệt hoàn toàn với logic điều khiển con trỏ Sequence.

### Khiếm khuyết 3: Race Condition thầm lặng khi Đa luồng sản sinh (Missing Multi-Producer CAS)
* **Mã tham chiếu:**
  ```go
  func (rb *LockFreeRingBuffer) Publish(id int64, payload []byte) {
      currWritten := atomic.LoadInt64(&rb.written.cursor)
      next := currWritten + 1
      // ...
      atomic.StoreInt64(&rb.written.cursor, next)
  }
  ```
* **Hậu quả:** Đoạn mã chỉ hoạt động duy nhất cho mô hình **Single Producer Single Consumer (SPSC)**. Nếu có từ 2 goroutine đồng thời gọi `Publish`, cả hai sẽ cùng đọc được giá trị `currWritten` giống nhau, cùng tính ra `next = currWritten + 1`, cùng ghi đè dữ liệu vào cùng một slot `slot = next & BufferMask`, và sau đó race condition khi ghi đè cursor. Hệ thống sẽ **mất mát dữ liệu thầm lặng (Silent Data Loss)** mà không có bất kỳ panic hay thông báo lỗi nào!
* **Giải pháp chuẩn:** Trong mô hình Multi-Producer (MPMC/MPSC), việc claim sequence phải sử dụng vòng lặp **Compare-And-Swap (`atomic.CompareAndSwapInt64`)** để xí chỗ an toàn, kết hợp với một mảng đánh dấu commit (`commitBuffer` hoặc `availableBuffer`) để consumer biết chính xác slot nào đã được ghi xong hoàn toàn trước khi đọc.

### Khiếm khuyết 4: Điểm nghẽn Atomic trên từng phần tử (Thiếu cơ chế Batching – Amdahl's Law)
* **Bản chất phần cứng:** Thao tác `atomic.StoreInt64` trên kiến trúc x86 thực chất sinh mã assembly `LOCK CMPXCHG` hoặc `XCHG` kèm theo lệnh xả Store Buffer. Thao tác này khóa bus hoặc gửi tín hiệu phát thanh (Broadcast Cache Invalidation) qua bus liên kết giữa các socket/core (Intel UPI / AMD Infinity Fabric).
* **Hậu quả:** Với tốc độ 10,000,000 sự kiện/giây, nếu mỗi sự kiện đều tốn 1 lần atomic load và 1 lần atomic store, CPU sẽ phải thực hiện **20,000,000 thao tác rào cản bộ nhớ (Memory Barrier/Bus Lock) mỗi giây**. Toàn bộ băng thông điều khiển bộ nhớ bị bão hòa, đẩy độ trễ p99 lên hàng trăm microsecond.
* **Bí quyết thực sự của LMAX Disruptor:** LMAX đạt 20 triệu ops/sec nhờ **Batching**: Producer xin cấp phát một dải sequence (ví dụ: từ 1000 đến 1064) chỉ bằng **1 thao tác atomic CAS duy nhất**, sau đó ghi dữ liệu tuần tự với tốc độ CPU pipeline, rồi commit 1 lần. Phía Consumer cũng vậy: đọc con trỏ ghi một lần, nếu thấy Producer đã ghi tới 1064 thì Consumer xử lý toàn bộ 64 sự kiện trong một vòng lặp phẳng mà không tốn thêm bất kỳ atomic fence nào!

### Khiếm khuyết 5: Backpressure ngây thơ bằng `runtime.Gosched()`
* **Bản chất phần cứng:** `runtime.Gosched()` yêu cầu Go Runtime Scheduler đưa Goroutine hiện tại về cuối hàng đợi khả dụng (`runq`) của Processor ($P$).
* **Hậu quả:** Thời gian để Goroutine được kích hoạt lại phụ thuộc vào số lượng goroutine khác đang chờ và chu kỳ điều phối của scheduler (dao động từ vài chục nanosecond đến vài chục microsecond). Điều này phá hủy hoàn toàn mục tiêu Low Latency Jitter (p99/p99.99).
* **Giải pháp chuẩn:** Thiết kế hệ thống **WaitStrategy phân cấp đa tầng (Multi-tier Wait Strategies)**:
  1. *Giai đoạn 1 (Spinning):* Vòng lặp rỗng (Busy Spin) tận dụng lệnh `PAUSE` của CPU để không làm nóng pipeline (giữ latency < 50ns).
  2. *Giai đoạn 2 (Yielding):* Gọi `runtime.Gosched()` khi đã spin quá một số chu kỳ nhất định (ví dụ 100 lần).
  3. *Giai đoạn 3 (Blocking/Park):* Sử dụng Futex / OS Primitive (`sync.Cond`) để nhường CPU hoàn toàn khi hàng đợi cạn kiệt, chống lãng phí điện năng và nhiệt độ chip.

### Khiếm khuyết 6: Cắt cụt dữ liệu thầm lặng và Ô nhiễm bộ nhớ (Memory Pollution)
* **Mã tham chiếu:**
  ```go
  copy(rb.ring[slot].Data[:], payload)
  ```
* **Hậu quả:** Nếu `payload` truyền vào lớn hơn 64 byte, `copy` chỉ sao chép 64 byte đầu tiên và **âm thầm vứt bỏ toàn bộ phần dữ liệu thừa** mà không báo lỗi. Ngược lại, nếu `payload` chỉ có 10 byte, 54 byte còn lại của slot đó vẫn giữ nguyên dữ liệu từ các vòng quay trước của ring buffer. Đây là lỗi **Data Leakage & Stale Data Corruption**.

---

## 2. 5 TRỤ CỘT KỸ THUẬT: ÁNH XẠ NETTY & LMAX DISRUPTOR SANG GO

```
+----------------------------------------------------------------------------------------------------+
|                                  THE 5 ULTRA-PERFORMANCE PILLARS                                    |
+------------------------------------+---------------------------------------------------------------+
| 1. Multi-Reactor I/O (epoll)       | Thay thế Goroutine-per-Connection bằng gnet / netpoll         |
| 2. Lock-Free Ring Buffer           | Triệt tiêu lock contention của Go Channel (hchan.lock)        |
| 3. Bi-directional Cache Padding    | Cách ly 64-byte Cache Line, dập tắt Cache Bouncing (MESI)     |
| 4. Pointerless & Zero-Allocation   | Vô hiệu hóa GC Mark Phase (`haspointers=false`), sync.Pool    |
| 5. Zero-Copy Serialization         | FlatBuffers / vtprotobuf đọc trực tiếp từ raw memory offset    |
+------------------------------------+---------------------------------------------------------------+
```

### Trụ cột 1: Từ Goroutine-Per-Connection sang Multi-Reactor Pattern
* **Hạn chế của `net/http` mặc định trong Go:**
  Go chuẩn tạo 1 goroutine cho mỗi kết nối TCP (`go c.serve(connCtx)`). Khi đối mặt với 500,000 – 1,000,000 kết nối đồng thời:
  1. Mỗi goroutine ngốn tối thiểu 2 KB – 4 KB bộ nhớ stack. Với 1M kết nối, hệ thống mất trắng **2 GB – 4 GB RAM** chỉ để giữ các goroutine nhàn rỗi (idle sockets).
  2. Quá trình quét GC Root phải duyệt qua 1 triệu call stack, khiến thời gian Stop-The-World (STW) và Mark Phase tăng đột biến.
  3. Bất cứ khi nào dữ liệu đến, Scheduler của Go ($G$-$M$-$P$) phải tìm kiếm và đánh thức goroutine, gây bão chuyển đổi ngữ cảnh (Context-Switching Storm).
* **Kiến trúc Multi-Reactor (tương đương Netty epoll):**
  Sử dụng thư viện `cloudwego/netpoll` hoặc `panjf2000/gnet`:
  * **Main Reactor:** 1 hoặc 2 OS thread duy nhất ghim vào CPU core, chỉ làm nhiệm vụ gọi `epoll_wait` để `accept()` kết nối mới, sau đó phân phối fd cho các Sub-Reactors thông qua thuật toán Round-Robin hoặc Least-Connections.
  * **Sub-Reactors:** Một số lượng cố định thread (thường bằng chính xác số Physical CPU Cores), mỗi thread quản lý một instance `epoll_wait` độc lập. Dữ liệu đọc từ socket được nạp thẳng vào bộ đệm vòng (Ring Buffer) hoặc `sync.Pool` byte slice mà không tạo ra bất kỳ goroutine mới nào.

### Trụ cột 2: Thay thế Go Channels bằng Lock-Free Ring Buffer
* **Bản chất struct `hchan` trong Go Runtime (`src/runtime/chan.go`):**
  ```go
  type hchan struct {
      qcount   uint           // total data in the queue
      dataqsiz uint           // size of the circular queue
      buf      unsafe.Pointer // points to an array of dataqsiz elements
      elemsize uint16
      closed   uint32
      elemtype *_type         // element type
      sendx    uint           // send index
      recvx    uint           // receive index
      recvq    waitq          // list of blocked receivers (sudog)
      sendq    waitq          // list of blocked senders (sudog)
      lock     mutex          // <--- ĐIỂM NGHẼN CHẾT NGƯỜI
  }
  ```
  Mỗi thao tác gửi hoặc nhận trên Channel (`ch <- data` hoặc `<-ch`) bắt buộc phải giành được khóa `hchan.lock`. Ở tải 10 triệu RPS với hàng chục core cùng đẩy vào channel, 90% thời gian CPU bị tiêu tốn cho việc tranh chấp Spin-Lock trên cấu trúc `mutex`, đẩy CPU vào trạng thái bão hòa vô ích.
* **Kiến trúc LMAX Disruptor:**
  Loại bỏ hoàn toàn lock. Quản lý trạng thái đọc/ghi thông qua các số nguyên tự tăng tuần tự (`int64 sequence`). Vị trí ô nhớ trong mảng được tính bằng phép toán logic bitmask: `slot = sequence & (Capacity - 1)` (với `Capacity` luôn là lũy thừa của 2).

### Trụ cột 3: Triệt tiêu False Sharing (Mechanical Sympathy)
* **Giao thức MESI trong L1/L2 Cache:**
  Mỗi Cache Line có 4 trạng thái:
  * **M (Modified):** Dữ liệu đã bị sửa đổi trên core hiện tại, các core khác không có dữ liệu này.
  * **E (Exclusive):** Dữ liệu sạch, chỉ tồn tại duy nhất trên core hiện tại.
  * **S (Shared):** Dữ liệu sạch, đang được chia sẻ trên nhiều core.
  * **I (Invalid):** Dữ liệu đã bị vô hiệu hóa vì có core khác vừa ghi vào.
* Khi Core 1 ghi vào con trỏ `written` và Core 2 đọc từ con trỏ `read` nằm chung một 64-byte boundary, Core 1 chuyển cache line sang trạng thái **M**, lập tức gửi tín hiệu Invalidation Message qua bus. Cache line của Core 2 bị giật sang trạng thái **I**. Core 2 buộc phải tạm dừng (stall) chu kỳ lệnh CPU để nạp lại cache line từ L3 Cache (tốn ~40-60 chu kỳ xung nhịp). Hiện tượng này lặp đi lặp lại hàng triệu lần mỗi giây gọi là **Cache-Line Bouncing**, làm sụt giảm throughput tới 80%.

### Trụ cột 4: Triệt tiêu Overhead của Garbage Collector (Pointerless Memory)
* **Cơ chế quét con trỏ của Go GC:**
  Trong giai đoạn Concurrent Mark, Go GC sử dụng giải thuật Tri-Color Mark. Hàm `runtime.scanobject` sẽ duyệt qua từng ô nhớ. Nếu một struct chứa con trỏ (`*Type`, `string`, `slice`, `map`, `interface`), GC bắt buộc phải dereference con trỏ đó để tìm các đối tượng liên quan trên heap.
* **Kỹ thuật Pointerless:**
  Nếu một struct chỉ bao gồm các kiểu dữ liệu nguyên thủy (primitive types) như `int64`, `uint32`, `[64]byte`:
  Go Compiler sẽ đánh dấu cờ `haspointers = false` trong Type Descriptor của struct. Khi GC quét qua mảng chứa các struct này, hàm `runtime.scanobject` **ngay lập tức bỏ qua toàn bộ khối bộ nhớ này mà không quét bất kỳ byte nào!**
  Một Ring Buffer 100 MB chứa 1 triệu sự kiện thuần túy không con trỏ sẽ làm tăng thời gian GC STW thêm **chính xác 0.00 microsecond**.

### Trụ cột 5: Tuần tự hóa Zero-Copy, Không Reflection
* `encoding/json` chuẩn sử dụng reflection (`reflect.TypeOf`, `reflect.ValueOf`), duyệt qua struct tags trong runtime và cấp phát hàng chục đối tượng trung gian trên heap cho mỗi request.
* **Giải pháp FlatBuffers / `vtprotobuf`:**
  Dữ liệu được tổ chức dưới dạng các trường nhị phân có độ dài cố định kèm theo bảng dịch chuyển (vtable offset table). Người nhận chỉ cần ép kiểu con trỏ hoặc đọc trực tiếp giá trị tại offset cụ thể từ mảng byte thô (`rawBytes[offset:offset+8]`) mà không cần tạo ra bất kỳ struct trung gian nào trên heap (Zero-Allocation Deserialization).

---

## 3. TRIỂN KHAI HOÀN CHỈNH ĐẠT CHUẨN ENTERPRISE (PRODUCTION-GRADE RING BUFFER)

```go
package highperf

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// CacheLineSize định nghĩa kích thước chuẩn của một CPU cache line trên x86_64 và ARM64.
const CacheLineSize = 64

// PaddedSequence đóng gói một con trỏ 64-bit với cơ chế đệm 2 chiều (Bi-directional Padding).
// Bố cục bộ nhớ đảm bảo trường value luôn chiếm độc quyền một Cache Line 64-byte,
// bất kể struct được cấp phát ở địa chỉ offset nào trong Go heap/stack.
type PaddedSequence struct {
	// Padding phía trước (56 byte)
	_p1, _p2, _p3, _p4, _p5, _p6, _p7 uint64
	// Trường giá trị chính (8 byte)
	value int64
	// Padding phía sau (56 byte)
	_p8, _p9, _p10, _p11, _p12, _p13, _p14 uint64
}

// NewPaddedSequence khởi tạo Sequence với giá trị ban đầu.
func NewPaddedSequence(initialValue int64) *PaddedSequence {
	return &PaddedSequence{value: initialValue}
}

// Get đọc giá trị con trỏ với atomic acquire load.
func (s *PaddedSequence) Get() int64 {
	return atomic.LoadInt64(&s.value)
}

// Set cập nhật giá trị con trỏ với atomic release store.
func (s *PaddedSequence) Set(val int64) {
	atomic.StoreInt64(&s.value, val)
}

// CompareAndSwap cập nhật giá trị con trỏ bằng thao tác CAS nguyên tử.
func (s *PaddedSequence) CompareAndSwap(oldVal, newVal int64) bool {
	return atomic.CompareAndSwapInt64(&s.value, oldVal, newVal)
}

// WaitStrategy định nghĩa hành vi chờ đợi của Producer hoặc Consumer khi ring buffer đầy hoặc rỗng.
type WaitStrategy interface {
	Wait(checker func() bool)
	SignalAll()
}

// BusySpinWaitStrategy xoay vòng lặp 100% CPU để đạt độ trễ thấp nhất có thể (< 50ns).
// YÊU CẦU: Thread phải được ghim chặt vào một Dedicated CPU Core bằng runtime.LockOSThread().
type BusySpinWaitStrategy struct{}

func (s *BusySpinWaitStrategy) Wait(checker func() bool) {
	for !checker() {
	}
}

func (s *BusySpinWaitStrategy) SignalAll() {}

// YieldingWaitStrategy thực hiện spin-wait một số chu kỳ, sau đó nhường quyền CPU bằng Gosched.
type YieldingWaitStrategy struct {
	spinTries int
}

func NewYieldingWaitStrategy(spinTries int) *YieldingWaitStrategy {
	if spinTries <= 0 {
		spinTries = 100
	}
	return &YieldingWaitStrategy{spinTries: spinTries}
}

func (s *YieldingWaitStrategy) Wait(checker func() bool) {
	counter := s.spinTries
	for !checker() {
		if counter > 0 {
			counter--
		} else {
			runtime.Gosched()
			counter = s.spinTries
		}
	}
}

func (s *YieldingWaitStrategy) SignalAll() {}

// BlockingWaitStrategy sử dụng OS Futex / sync.Cond để đưa goroutine vào trạng thái ngủ khi hàng đợi rỗng.
type BlockingWaitStrategy struct {
	mu   sync.Mutex
	cond *sync.Cond
}

func NewBlockingWaitStrategy() *BlockingWaitStrategy {
	b := &BlockingWaitStrategy{}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (s *BlockingWaitStrategy) Wait(checker func() bool) {
	if checker() {
		return
	}
	s.mu.Lock()
	for !checker() {
		s.cond.Wait()
	}
	s.mu.Unlock()
}

func (s *BlockingWaitStrategy) SignalAll() {
	s.cond.Broadcast()
}

// Event đại diện cho bản tin dữ liệu tải cao.
// CẢNH BÁO KIẾN TRÚC: Tuyệt đối KHÔNG đưa con trỏ, slice, string, map, interface vào struct này.
type Event struct {
	ID         int64
	Timestamp  int64
	TenantID   uint32
	PayloadLen uint32
	Data       [64]byte // Payload cố định 64 byte
}

// RingBuffer cấu trúc đệm vòng lock-free hoàn chỉnh đạt chuẩn LMAX Disruptor.
type RingBuffer struct {
	capacity     int64
	mask         int64
	storage      []Event        // Vùng nhớ sự kiện liên tục
	available    []int32        // Mảng theo dõi commit cho Multi-Producer (0: chưa commit, 1: đã commit)
	claimCursor  PaddedSequence // Con trỏ đăng ký vị trí ghi (Producer Claim)
	commitCursor PaddedSequence // Con trỏ xác nhận vị trí ghi hợp lệ (Producer Commit)
	readCursor   PaddedSequence // Con trỏ đọc của Consumer
	waitStrategy WaitStrategy
	isMultiProd  bool
}

// NewRingBuffer khởi tạo RingBuffer với kích thước lũy thừa 2.
func NewRingBuffer(capacity int64, multiProducer bool, strategy WaitStrategy) (*RingBuffer, error) {
	if capacity <= 0 || (capacity&(capacity-1)) != 0 {
		return nil, fmt.Errorf("dung lượng buffer bắt buộc phải là lũy thừa của 2, giá trị nhận được: %d", capacity)
	}
	if strategy == nil {
		strategy = NewYieldingWaitStrategy(100)
	}

	rb := &RingBuffer{
		capacity:     capacity,
		mask:         capacity - 1,
		storage:      make([]Event, capacity),
		claimCursor:  *NewPaddedSequence(-1),
		commitCursor: *NewPaddedSequence(-1),
		readCursor:   *NewPaddedSequence(-1),
		waitStrategy: strategy,
		isMultiProd:  multiProducer,
	}

	if multiProducer {
		rb.available = make([]int32, capacity)
	}

	return rb, nil
}

// ClaimBatch (Multi-Producer Safe) yêu cầu giữ chỗ an toàn n slots liên tiếp trên buffer.
func (rb *RingBuffer) ClaimBatch(n int64) (int64, int64, error) {
	if n <= 0 || n > rb.capacity {
		return -1, -1, errors.New("kích thước batch không hợp lệ hoặc vượt quá dung lượng buffer")
	}

	for {
		currentClaim := rb.claimCursor.Get()
		nextClaim := currentClaim + n

		wrapPoint := nextClaim - rb.capacity
		cachedGating := rb.readCursor.Get()

		if wrapPoint > cachedGating {
			rb.waitStrategy.Wait(func() bool {
				return wrapPoint <= rb.readCursor.Get()
			})
		}

		if !rb.isMultiProd {
			rb.claimCursor.Set(nextClaim)
			return currentClaim + 1, nextClaim, nil
		}

		if rb.claimCursor.CompareAndSwap(currentClaim, nextClaim) {
			return currentClaim + 1, nextClaim, nil
		}
	}
}

// GetEventPtr trả về con trỏ tới Event tại sequence chỉ định (Zero-Copy Mutation).
func (rb *RingBuffer) GetEventPtr(sequence int64) *Event {
	return &rb.storage[sequence&rb.mask]
}

// CommitBatch (Multi-Producer Safe) xác nhận các slot từ start tới end đã được ghi dữ liệu xong.
func (rb *RingBuffer) CommitBatch(start, end int64) {
	if !rb.isMultiProd {
		rb.commitCursor.Set(end)
		rb.waitStrategy.SignalAll()
		return
	}

	for seq := start; seq <= end; seq++ {
		slot := seq & rb.mask
		atomic.StoreInt32(&rb.available[slot], 1)
	}

	for {
		currCommit := rb.commitCursor.Get()
		nextCommit := currCommit + 1
		slot := nextCommit & rb.mask

		if atomic.LoadInt32(&rb.available[slot]) == 1 {
			atomic.StoreInt32(&rb.available[slot], 0)
			rb.commitCursor.Set(nextCommit)
		} else {
			break
		}
	}

	rb.waitStrategy.SignalAll()
}

// ConsumeBatch xử lý một loạt các sự kiện sẵn có (Single Consumer Batching).
func (rb *RingBuffer) ConsumeBatch(maxBatch int64, handler func(event *Event, sequence int64, isEndOfBatch bool) error) (int64, error) {
	currRead := rb.readCursor.Get()
	nextRead := currRead + 1

	availableSeq := rb.commitCursor.Get()
	if nextRead > availableSeq {
		rb.waitStrategy.Wait(func() bool {
			availableSeq = rb.commitCursor.Get()
			return nextRead <= availableSeq
		})
	}

	limitSeq := availableSeq
	if (limitSeq - nextRead + 1) > maxBatch {
		limitSeq = nextRead + maxBatch - 1
	}

	for seq := nextRead; seq <= limitSeq; seq++ {
		event := &rb.storage[seq&rb.mask]
		isEnd := (seq == limitSeq)
		if err := handler(event, seq, isEnd); err != nil {
			rb.readCursor.Set(seq)
			return seq - currRead, err
		}
	}

	rb.readCursor.Set(limitSeq)
	return limitSeq - currRead, nil
}
```

---

## 4. PHÂN TÍCH ĐIỂM GÃY, NGUY CƠ SẬP VÀ ĐIỀU KIỆN BIÊN (FAILURE MODES & MECHANICAL SYMPATHY)

```
+----------------------------------------------------------------------------------------------------+
|                                    HARDWARE BOTTLENECK PROFILE                                     |
+--------------------------+------------------------------+------------------------------------------+
| Subsystem                | Physical Limit               | Degradation Behavior                     |
+--------------------------+------------------------------+------------------------------------------+
| CPU Invalidation Bus     | ~20M invalidations/sec/core  | Atomic CAS retry storm, bus saturation   |
| Linux NIC Rx Descriptors | 1024 - 4096 descriptors      | rx_missed_errors, packet drop tại NIC    |
| NUMA Node Hop            | Cross-socket UPI: ~40-60ns   | 2.5x - 3x latency penalty                |
| Go Preemption Signal     | sysmon 10ms preempt (SIGURG) | CPU pipeline purge, register spill       |
+--------------------------+------------------------------+------------------------------------------+
```

### 1. Điểm nghẽn phần cứng thực tế (Hardware Physical Limits)
* **Bão hòa Bus liên kết CPU (Inter-Core Invalidation Storms):**
  Trong mô hình Multi-Producer, khi có nhiều hơn 8 goroutine chạy trên 8 core khác nhau cùng thực hiện `ClaimBatch` bằng `CompareAndSwap`, hiện tượng tranh chấp địa chỉ bộ nhớ `claimCursor` sẽ kích hoạt hàng triệu thông điệp vô hiệu hóa cache line qua bus liên kết (Intel UPI / AMD Infinity Fabric). Tần suất CAS thất bại có thể lên tới **95%**, biến CPU thành một máy sưởi tiêu tốn điện năng mà không xử lý được sự kiện nào.
* **Giới hạn Ngắt Mạng (NIC Ring Buffer & `ksoftirqd` Overload):**
  Card mạng 10GbE / 100GbE tiếp nhận gói tin qua phần cứng DMA trực tiếp vào Ring Descriptors (thường có kích thước 1024 đến 4096 slots). Nếu tiến trình Go không gọi `epoll_wait` để đọc dữ liệu ra khỏi buffer socket kịp thời:
  1. Hàng đợi socket bị đầy (`net.core.rmem_max`).
  2. NIC không thể đưa thêm gói tin vào host RAM, buộc phải **drop gói tin ngay tại chip phần cứng NIC** (tăng giá trị `rx_discards_phy` hoặc `rx_missed_errors` trong `ethtool -S`).
  3. Tiến trình `ksoftirqd` trên OS chiếm dụng 100% CPU trên Core 0, gây tắc nghẽn toàn bộ luồng mạng của máy chủ.
* **Ảnh hưởng phân mảnh bộ nhớ NUMA (Non-Uniform Memory Access):**
  Nếu mảng `rb.storage` được cấp phát trên RAM gắn với Socket 0 (NUMA Node 0), nhưng Consumer goroutine lại được HĐH xếp lịch chạy trên Core thuộc Socket 1 (NUMA Node 1), mỗi thao tác đọc bộ đệm sẽ phải đi qua liên kết UPI giữa 2 CPU socket. Độ trễ truy cập RAM sẽ nhảy từ **60ns lên 180ns (tăng gấp 3 lần)**.

### 2. Ba kịch bản sụp đổ hệ thống (System Catastrophic Crash Scenarios)

#### Kịch bản 1: Thảm họa Consumer Chậm trễ (Slow Consumer Cascade & Core Starvation)
* **Diễn biến:** Consumer vì một lý do nào đó bị nghẽn I/O (ví dụ: ghi file log bị block bởi disk flush, hoặc kết nối mạng đến ClickHouse/Postgres bị timeout).
* **Điểm gãy:**
  1. `readCursor` ngừng tịnh tiến.
  2. Toàn bộ các Producer lần lượt chạm đến ngưỡng `wrapPoint > cachedGating`.
  3. Nếu sử dụng `BusySpinWaitStrategy`, **tất cả các CPU cores chứa Producer lập tức bị kéo lên 100% mức sử dụng**.
  4. Hệ thống cạn kiệt CPU cycles. Go Runtime Scheduler không thể xếp lịch cho các background goroutine khác (như goroutine kiểm tra Health Check `/healthz`, goroutine xuất Prometheus metrics, hoặc goroutine duy trì heartbeat của Raft/NATS).
  5. Kubernetes Liveness Probe bị timeout quá 3 lần liên tiếp, Kubernetes kích hoạt lệnh `KILL -9` tiêu diệt Pod, gây sập dịch vụ trên diện rộng.

#### Kịch bản 2: Tràn bộ nhớ (OOM) do Slow Client trong Multi-Reactor
* **Diễn biến:** Mô hình Reactor (`gnet`/`netpoll`) lưu trữ các frame phản hồi chưa gửi đi vào một Outbound Buffer trong heap khi socket phía client chưa sẵn sàng nhận (`EPOLLOUT`).
* **Điểm gãy:**
  Nếu một nhóm client bị nghẽn mạng hoặc cố tình tấn công Slowloris (TCP Window Size co về 0 byte thông qua TCP Zero-Window Probes), máy chủ tiếp tục nạp dữ liệu cần gửi vào bộ nhớ đệm của kết nối đó. Khi có 50,000 kết nối đồng thời bị nghẽn và mỗi kết nối giữ 1 MB buffer chưa thể xả ra cáp mạng, máy chủ sẽ ngốn thêm **50 GB bộ nhớ ngoài dự kiến**, kích hoạt Linux OOM Killer tiêu diệt tiến trình Go ngay lập tức.

#### Kịch bản 3: Độ trễ đột biến do Asynchronous Preemption của Go Runtime
* **Diễn biến:** Từ phiên bản Go 1.14, runtime áp dụng cơ chế Asynchronous Preemption dựa trên tín hiệu HĐH `SIGURG`. Cứ mỗi 10ms, tiến trình giám sát `sysmon` sẽ kiểm tra nếu một goroutine chạy liên tục trên một thread $M$ mà không có điểm gọi hàm (Function Call / Scheduler Co-operative Point), nó sẽ gửi tín hiệu `SIGURG` để ngắt ngang thread đó.
* **Điểm gãy:**
  Vòng lặp `BusySpinWaitStrategy` là một vòng lặp kín không có lệnh gọi hàm. Khi nhận tín hiệu `SIGURG`, CPU phải dừng luồng thực thi, chuyển ngữ cảnh vào Kernel Signal Handler của Linux, sau đó nhảy vào hàm điều phối của Go runtime để context switch. Quá trình này tạo ra các bước nhảy độ trễ (Tail Latency Jitter) bất thường từ 50ns vọt lên **2ms – 5ms**, phá hỏng yêu cầu cam kết SLA siêu thấp.

---

## 5. THIẾT LẬP KERNEL & RUNTIME VÀ TIÊU CHUẨN BENCHMARK THỰC NGHIỆM

### 1. Tinh chỉnh Linux Kernel Chuyên sâu (`/etc/sysctl.conf`)
```ini
# Tăng kích thước hàng đợi backlog tiếp nhận kết nối mới
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535

# Tối đa hóa hàng đợi gói tin chờ xử lý của tầng network device (tránh drop tại NAPI)
net.core.netdev_max_backlog = 250000

# Tái sử dụng cổng TIME_WAIT cho các kết nối outbound tần suất cao
net.ipv4.tcp_tw_reuse = 1
net.ipv4.ip_local_port_range = 1024 65535

# Định cỡ bộ đệm TCP Socket: Min - Default - Max (Tối đa 16MB cho đường truyền tải cao)
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216

# Tắt TCP Slow Start sau khi kết nối rảnh để duy trì tốc độ tức thời
net.ipv4.tcp_slow_start_after_idle = 0

# Tắt TCP Timestamps để tiết kiệm 12 byte header trên mỗi gói tin nếu latency mạng cực thấp
net.ipv4.tcp_timestamps = 0
```

### 2. Thiết lập Ghim CPU (CPU Pinning) và NUMA Node
* **Cô lập Core dành riêng cho Ring Buffer trên Linux Boot Parameters (`/etc/default/grub`):**
  ```text
  GRUB_CMDLINE_LINUX="isolcpus=2-7 nohz_full=2-7 rcu_nocbs=2-7"
  ```
* **Khởi chạy tiến trình Go với NUMA Node Binding:**
  ```bash
  numactl --cpunodebind=0 --membind=0 ./highperf-gateway
  ```

### 3. Thiết lập Môi trường Go Runtime
```bash
# Thiết lập trần bộ nhớ tối đa cho phép trước khi GC cưỡng chế dọn dẹp
export GOMEMLIMIT=14GiB

# Nâng tỷ lệ phần trăm heap để giảm tần suất chạy của GC (đòi hỏi máy chủ RAM dồi dào)
export GOGC=400

# Buộc Go Runtime trả lại bộ nhớ cho OS ngay lập tức thay vì dùng MADV_FREE trì hoãn
export GODEBUG=madvdontneed=1
```

### 4. Công thức định cỡ Ring Buffer dựa trên Định luật Little (Little's Law)
Không bao giờ chọn kích thước Ring Buffer một cách ngẫu nhiên. Dung lượng tối thiểu của Buffer phải tuân theo công thức:

$$\text{Capacity} \ge \text{Target Throughput (eps)} \times \text{Max Consumer Latency } p99.99 \text{ (seconds)} \times \text{Safety Factor}$$

* **Ví dụ thực tế:**
  * Mục tiêu thông lượng: $10,000,000 \text{ ops/sec}$.
  * Độ trễ tiêu thụ lớn nhất của Consumer khi gặp đột biến: $p99.99 = 20 \text{ ms} = 0.02 \text{ s}$.
  * Hệ số an toàn (Safety Margin chống tràn): $\text{Factor} = 2.0$.
  * Áp dụng công thức:
    $$\text{Capacity} = 10,000,000 \times 0.02 \times 2.0 = 400,000 \text{ phần tử}$$
  * Làm tròn lên lũy thừa của 2 gần nhất ($2^N$):
    $$2^{19} = 524,288 \text{ phần tử (Buffer Size tối ưu)}$$

### 5. Tiêu chuẩn Baseline Benchmark bắt buộc trước khi đưa vào Production
```bash
# 1. Kiểm tra tính thoát bộ nhớ (Escape Analysis) - Khẳng định 100% ZERO ALLOCATION
go build -gcflags="-m -m" ./... 2>&1 | grep "escapes to heap"

# 2. Chạy Benchmark đo độ trễ phân vị và số lần cấp phát bộ nhớ
go test -bench=. -benchmem -benchtime=5s -cpu=1,2,4,8 ./...

# 3. Phân tích tranh chấp luồng (Contention Profiling)
go test -bench=. -blockprofile=block.pprof -mutexprofile=mutex.pprof ./...
```
* **Tiêu chí nghiệm thu bắt buộc (Pass/Fail Gate):**
  * `0 allocs/op` trên toàn bộ đường dẫn xử lý sự kiện (Hot Path).
  * Độ trễ phân vị $p50 < 100 \text{ ns}$, $p99 < 500 \text{ ns}$, $p99.9 < 2 \text{ }\mu\text{s}$.
  * Không xuất hiện bất kỳ sự tranh chấp mutex nào (`sync.Mutex` thời gian chờ = 0) trong mutex profile.
