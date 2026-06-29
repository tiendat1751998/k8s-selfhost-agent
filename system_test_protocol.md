# Quy Trình Kiểm Thử Toàn Diện Hệ Thống (QA/Manual Test Protocol)

Tài liệu này cung cấp quy trình kiểm thử từng bước (manual test cases) cho tất cả các phân hệ và menu trên Dashboard của dự án **K8S Control Plane — Enterprise Edition**.

---

## 1. Nhóm Quản Trị Tài Nguyên & Cluster (Cluster & Resource Explorer)

### Case 1.1: Trình Duyệt Tài Nguyên (Resource Explorer)
* **Đường dẫn**: `#explorer` (Menu **Resource Explorer**)
* **Các bước thực hiện**:
  1. Click vào **Resource Explorer** trên thanh Sidebar.
  2. Chọn bộ lọc `Namespace` hoặc `Resource Kind` (Pod, Service, Deployment, ReplicaSet).
  3. Kiểm tra xem danh sách các tài nguyên có hiển thị đầy đủ và phản ánh dữ liệu từ API `/api/v1/explorer` hay không.
* **Kết quả mong đợi**: Trả về danh sách tài nguyên thực tế từ cluster đang kết nối. Nếu cluster chưa cấu hình, bảng sẽ hiển thị thông báo lỗi/trống an toàn thay vì mock data.

### Case 1.2: Quản Lý Nodes & Pods
* **Đường dẫn**: `#nodes` (Menu **Nodes**) và `#pods` (Menu **Pods**)
* **Các bước thực hiện**:
  1. Click vào menu **Nodes** để xem danh sách các nodes trong cluster.
  2. Click vào menu **Pods** để xem trạng thái sẵn sàng của các Container.
* **Kết quả mong đợi**: 
  - Hiển thị danh sách các Node thực tế (IP, trạng thái Ready/NotReady, vai trò Manager/Worker).
  - Hiển thị danh sách các Pod kèm theo các số liệu CPU/Memory thực tế.

---

## 2. Nhóm GitOps & Tự Động Hóa (GitOps & CI/CD Pipelines)

### Case 2.1: Phát Hiện Lệch Cấu Hình (GitOps Drift Detection)
* **Đường dẫn**: `#drift` (Menu **GitOps Drift**)
* **Các bước thực hiện**:
  1. Click vào **GitOps Drift**.
  2. Bấm nút **Scan Now** để quét cấu hình.
  3. Tìm tài nguyên bị gắn tag `Drifted` (Lệch cấu hình) và click **View Diff**.
* **Kết quả mong đợi**: 
  - Trạng thái đồng bộ (In Sync / Drifted) hiển thị chính xác.
  - Hộp thoại Diff mở ra hiển thị các dòng màu đỏ/xanh biểu thị sự khác biệt giữa cấu hình Git baseline và trạng thái chạy thực tế.

### Case 2.2: Tích Hợp Git & CI/CD
* **Đường dẫn**: `#git-providers` và `#cicd`
* **Các bước thực hiện**:
  1. Mở menu **Git Providers** xem danh sách các repo đã liên kết (GitHub/GitLab). Bấm **+ Add Provider** để thử thêm kết nối.
  2. Mở menu **CI/CD Pipelines** xem lịch sử các lượt chạy build/deploy.
* **Kết quả mong đợi**: Hiển thị chính xác thời gian webhook hoạt động và trạng thái lượt chạy (success/failing).

---

## 3. Nhóm Vận Hành & Giám Sát (Operations & Observability)

### Case 3.1: Trung Tâm Xử Lý Sự Cố & Phân Tích Nguyên Nhân Gốc (Incidents & RCA)
* **Đường dẫn**: `#incidents` (Menu **Incidents**)
* **Các bước thực hiện**:
  1. Click vào **Incidents**.
  2. Nhấp chọn một Incident cụ thể có độ nghiêm trọng cao (Critical).
  3. Xem phần phân tích nguyên nhân gốc tự động từ AI (RCA Analysis) bằng Ollama.
* **Kết quả mong đợi**: AI tự động đưa ra dự đoán nguyên nhân (ví dụ: CrashLoopBackOff, Out of Memory) kèm theo nút đề xuất sửa lỗi (Runbook recommendation).

### Case 3.2: Trung Tâm Sức Khỏe Hệ Thống (Health Center)
* **Đường dẫn**: `#health`
* **Các bước thực hiện**:
  1. Click vào **Health Center**.
  2. Theo dõi các card trạng thái: Frontend, Backend, WebSocket, Database, AI Engine.
  3. Bấm **Refresh** để kích hoạt quét động.
* **Kết quả mong đợi**: Trả về kết quả Ping/Status thời gian thực. Nếu cơ sở dữ liệu Postgres bị ngắt kết nối, card Database lập tức chuyển sang trạng thái đỏ (`Down`).

---

## 4. Nhóm Trí Tuệ Nhân Tạo (AI & Copilot)

### Case 4.1: Chatbot Trợ Lý AI Ops
* **Đường dẫn**: `#ai-ops`
* **Các bước thực hiện**:
  1. Gõ câu hỏi vào khung chat: *"How to scale my application?"* hoặc *"Check system health"*
  2. Nhấn Enter và đợi phản hồi.
* **Kết quả mong đợi**: Mô hình LLM (Ollama/Gemini) phản hồi trực tiếp, trích xuất thông tin hệ thống và hướng dẫn các lệnh `kubectl` hoặc `docker` tương ứng.

---

## 5. Nhóm Quản Trị Doanh Nghiệp (Enterprise & Platform Admin)

### Case 5.1: Phân Quyền & Vai Trò (Enterprise RBAC)
* **Đường dẫn**: `#enterprise` (Tab **RBAC**)
* **Các bước thực hiện**:
  1. Truy cập phân hệ **Enterprise** và chọn tab **RBAC**.
  2. Tích chọn/bỏ tích chọn các quyền tương ứng của các vai trò (Developer, DevOps, Viewer).
* **Kết quả mong đợi**: Quyền hạn được lưu động vào AppState. User có vai trò Viewer sẽ bị chặn các nút tương tác ghi (như Scale/Restart).

### Case 5.2: Tối Ưu Chi Phí (Cost Management)
* **Đường dẫn**: `#cost-management`
* **Các bước thực hiện**:
  1. Xem biểu đồ phân bổ chi phí theo Namespace và Resource Type.
  2. Đọc các đề xuất tối ưu hóa (ví dụ: *"Idle node pool detected, downscale recommended"*).
* **Kết quả mong đợi**: Dữ liệu hiển thị trực quan, giúp quản trị viên đưa ra quyết định cắt giảm tài nguyên thừa.
