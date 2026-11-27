# Môi trường Playwright riêng

Stack này chỉ dùng dữ liệu thử, không mount volume `dev_data`. SQLite nằm trên tmpfs và mất khi container dừng. Không đưa dữ liệu thật vào stack này; mạng Docker được chia sẻ với môi trường dev, chỉ kết nối các container tin cậy.

1. Khởi chạy môi trường dev bằng `leedevkit dev up` để tạo mạng Docker.
2. Chọn mật khẩu dùng riêng cho môi trường thử, tối thiểu 12 byte; không dùng mật khẩu thật. Đặt biến `PLAYWRIGHT_DEV_PASSWORD` trong shell, không lưu vào repository.
3. Chạy `docker compose -p schedule-ui-audit -f .compose/docker-compose.playwright.yml up -d --build`. Container tạo DB mới và bootstrap tài khoản `admin` mỗi lần khởi động.
4. Khởi động lại phiên MCP để nhận endpoint mới. MCP truy cập `http://ui-audit:8080` qua mạng Docker; đăng nhập `admin` bằng giá trị `PLAYWRIGHT_DEV_PASSWORD`. Không mở cổng app thử trên host. Có thể tạo phòng, lịch và thiết bị mẫu qua giao diện.

Bản build thử dùng Go overlay để chấp nhận HTTP origin `ui-audit` và dùng cookie `dev-session` không có cờ `Secure`. Đây là ngoại lệ **chỉ dành cho dữ liệu giả trên mạng Docker tin cậy**; traffic và cookie không được mã hóa. Bản build thông thường vẫn giữ nguyên HTTPS và cookie `__Host-session` bảo mật. Overlay dừng build nếu mã nguồn không còn khớp.
5. Dừng bằng `docker compose -p schedule-ui-audit -f .compose/docker-compose.playwright.yml down`. Dữ liệu thử sẽ mất. Lệnh không xóa dữ liệu dev chính.

Đổi endpoint MCP trong `leedevkit.toml` về `http://go:8080` nếu cần dùng DB dev chính. Không áp dụng mật khẩu thử hay lệnh bootstrap này cho môi trường thật.
