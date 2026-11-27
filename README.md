# Schedule Display

Go API phục vụ SPA Svelte được embed trong binary. SQLite dùng driver Go `modernc.org/sqlite`.

## Yêu cầu

- Docker Compose và LeeDevKit cho môi trường container.
- Khi chạy trực tiếp: Go 1.26, Bun, Air (`go install github.com/air-verse/air@v1.61.7`).

## Đăng nhập quản trị

Ứng dụng mặc định phục vụ HTTP tại `127.0.0.1:8080`, với `APP_ORIGIN=http://localhost:8080`. Khi truy cập từ máy khác, cấu hình reverse proxy phục vụ HTTPS và đặt `APP_ORIGIN` thành origin HTTPS chính xác của trình duyệt. Chỉ cho proxy tin cậy truy cập backend HTTP; không công khai cổng backend.

Để tính giới hạn đăng nhập theo IP client qua proxy, đặt `TRUSTED_PROXY_IP` thành đúng một địa chỉ IP nguồn mà backend nhìn thấy từ proxy. Proxy phải **xóa và ghi đè** `X-Real-IP` bằng IP client thực; không chuyển tiếp giá trị do client gửi. Không cho tiến trình hay container không tin cậy kết nối backend từ IP proxy: chúng có thể giả header. Đặc biệt, `127.0.0.1` không phân biệt proxy với tiến trình cục bộ khác. Khi dùng Docker, kiểm tra IP nguồn thực tế và chỉ cho proxy truy cập mạng backend. Backend bỏ qua `X-Forwarded-For`, và từ chối đăng nhập nếu proxy tin cậy không gửi đúng một IP hợp lệ. Nếu không cấu hình `TRUSTED_PROXY_IP`, giới hạn tính theo IP kết nối; khi đó các client qua cùng proxy có thể cùng bị chặn.

Giới hạn lưu trong bộ nhớ, theo địa chỉ IPv4 hoặc prefix IPv6 `/64`; client cùng prefix có thể bị chặn chung. Bộ đếm reset khi khởi động lại, không chia sẻ giữa các tiến trình và không ngăn được tấn công phân tán hoặc đổi prefix. Bảng 10.000 khóa đầy sẽ từ chối client mới cho đến khi khóa cũ hết hạn. Trước vận hành, cần giới hạn tốc độ tổng tại proxy/mạng, cảnh báo và quy trình xử lý bảng đầy; kiểm thử cấu hình proxy thực tế và cách ly backend.

Sau khi build, tạo tài khoản đầu tiên bằng `DATABASE_PATH=/path/to/schedule.db ./schedule-display bootstrap admin`. Lệnh đọc mật khẩu từ stdin; yêu cầu 12–1024 byte và không chấp nhận bootstrap lại. Khi nhập từ terminal, tắt echo trước khi nhập, kết thúc stdin bằng Ctrl-D rồi bật lại echo; bảo vệ quyền truy cập DB và khóa TLS. Không truyền mật khẩu qua tham số dòng lệnh.

Quản trị viên có thể tạo, vô hiệu hóa tài khoản và đặt lại mật khẩu qua giao diện; thao tác vô hiệu hóa hoặc đổi mật khẩu thu hồi mọi phiên của tài khoản liên quan. Không thể vô hiệu hóa tài khoản hoạt động cuối cùng. Thao tác được ghi trong bảng `account_audit` không chứa mật khẩu hoặc token. Chưa có chức năng kích hoạt lại tài khoản hoặc khôi phục quyền quản trị tự động.

Schema phiên bản 2 bổ sung trạng thái tài khoản và nhật ký; phiên bản 3 bổ sung phòng, phiên bản 4 bổ sung lịch một lần, phiên bản 5 bổ sung thiết bị và phiên bản 6 bổ sung kiểm soát phiên bản lịch, phiên bản 7 lưu ngôn ngữ theo thiết bị (mặc định tiếng Việt). Quản trị viên có thể chọn tiếng Việt hoặc tiếng Anh cho kiosk; nội dung lịch do người dùng nhập không được dịch. Kiosk tại `/kiosk` đang trong giai đoạn thử nghiệm, chưa sẵn sàng vận hành: chưa kiểm chứng đầy đủ mất mạng, đồng hồ và thiết bị thực. Đặt `APP_TIMEZONE` thành múi giờ IANA chung (mặc định `UTC`) trước khi chạy; giao diện nhập và hiển thị theo múi giờ này, từ chối giờ không tồn tại hoặc mơ hồ do DST. Không đổi múi giờ tùy tiện khi đã có lịch lặp. Bản cũ từ chối mở schema mới. Không chạy đồng thời binary cũ và mới: binary cũ không kiểm tra trạng thái tài khoản. Khi cần quay lại phiên bản cũ, ưu tiên khôi phục bản sao lưu nhất quán trước nâng cấp; dữ liệu thay đổi sau bản sao lưu cần được đối soát, không tự động hạ schema hoặc ghi đè dữ liệu đang dùng. Đọc `tasks/auth-threat-model.md` trước khi triển khai.

Kiosk chưa có watchdog độc lập: nếu luồng trình duyệt bị treo hoặc hẹn giờ trễ quá 30 giây, màn hình có thể tiếp tục báo Trống sau khi cuộc họp bắt đầu. Nếu thiết bị ngủ và đồng hồ bị chỉnh đồng thời mà không có sự kiện vòng đời, hai đồng hồ có thể không phát hiện sai lệch. Không dùng kiosk làm nguồn quyết định phòng trống đáng tin cậy cho đến khi có watchdog độc lập hoặc giải pháp thiết bị tương đương và kiểm thử thực tế.

## Sao lưu thử nghiệm

Sau khi build, chạy `./schedule-display backup /path/to/schedule.db /path/to/new-backup.db`. Nguồn được mở chỉ đọc; SQLite tạo snapshot nhất quán. Khi dùng rollback journal, snapshot có thể chặn commit và khiến ứng dụng trả lỗi `SQLITE_BUSY`; chưa kiểm thử và xử lý đầy đủ tương tranh này. Chỉ sao lưu khi đã dừng ghi cho đến khi cơ chế chờ và kiểm thử đồng thời được xác minh. Đích phải chưa tồn tại, không có hậu tố `-wal`, `-shm`, `-journal`, và nằm trên filesystem hỗ trợ hard link; bản sao có quyền `0600`. Không dùng bản sao lưu làm DB đang chạy. Bảo vệ thư mục sao lưu, quyền truy cập và vị trí lưu như dữ liệu gốc; chưa có mã hóa hay chính sách lưu giữ tự động. Nếu tiến trình bị dừng đột ngột, thư mục ẩn `.sqlite-backup-*` có thể còn lại và chứa dữ liệu nhạy cảm; kiểm tra trước khi xóa thủ công.

Để diễn tập, sao chép bản sao lưu đến một đường dẫn mới trong môi trường riêng, mở bằng SQLite và chạy `PRAGMA integrity_check`, sau đó kiểm tra dữ liệu và luồng nghiệp vụ. Không khôi phục đè DB thật nếu chưa được duyệt riêng. Cần chốt lịch sao lưu, lưu giữ, mã hóa và mục tiêu khôi phục trước vận hành.

## Phát triển

Chạy tại `http://localhost:8080` trên chính máy phát triển:

```sh
export APP_PORT=8080
leedevkit dev up
leedevkit dev ps
# Xem cổng được Docker cấp:
docker compose -p lee-schedule-display-dev port go 8080
```

Nếu thay `APP_PORT`, đặt `APP_ORIGIN` khớp origin trình duyệt, ví dụ `http://localhost:8081`. Compose chỉ publish cổng lên loopback máy chủ; container vẫn lắng nghe trên mạng Docker, nên chỉ kết nối nó với các container tin cậy. `LISTEN_ADDR` có thể thay đổi địa chỉ lắng nghe khi chạy trực tiếp; nếu mở ra ngoài loopback, phải bảo đảm chỉ reverse proxy tin cậy truy cập được. SQLite dev nằm trong volume `dev_data`; `leedevkit dev down` giữ dữ liệu, còn `leedevkit dev clean` xóa volume và dữ liệu.

Chạy trực tiếp ngoài container:

```sh
bun install --cwd web
air
```

Air build lại Svelte, sau đó build và khởi động lại Go khi mã nguồn thay đổi. Tải lại trình duyệt tại cổng được Docker cấp để xem thay đổi. Không có browser HMR.

## Build và kiểm thử

```sh
leedevkit test all --json
```

Test chạy trên stack riêng, build frontend trong volume test trước mỗi gate. Lần đầu cần mạng để tải image và dependencies. Kiểm thử hoặc build trực tiếp:

```sh
bun run --cwd web build
go test ./...
go build -o schedule-display .
./schedule-display
```

Bước build frontend bắt buộc trước `go test` hoặc `go build` trên checkout mới. Đặt `DATABASE_PATH` để chọn đường dẫn SQLite; mặc định `schedule.db`. API kiểm tra trạng thái: `GET /api/health`.
