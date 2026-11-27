# Schedule Display

Go API phục vụ SPA Svelte được embed trong binary. SQLite dùng driver Go `modernc.org/sqlite`.

## Yêu cầu

- Docker Compose và LeeDevKit cho môi trường container.
- Khi chạy trực tiếp: Go 1.26, Bun, Air (`go install github.com/air-verse/air@v1.61.7`).

## Đăng nhập quản trị

Ứng dụng mặc định phục vụ HTTP tại `127.0.0.1:8080`, với `APP_ORIGIN=http://localhost:8080`. Khi truy cập từ máy khác, cấu hình reverse proxy phục vụ HTTPS và đặt `APP_ORIGIN` thành origin HTTPS chính xác của trình duyệt. Chỉ cho proxy tin cậy truy cập backend HTTP; không công khai cổng backend.

Để tính giới hạn đăng nhập theo IP client qua proxy, đặt `TRUSTED_PROXY_IP` thành đúng một địa chỉ IP nguồn mà backend nhìn thấy từ proxy. Proxy phải **xóa và ghi đè** `X-Real-IP` bằng IP client thực; không chuyển tiếp giá trị do client gửi. Không cho tiến trình hay container không tin cậy kết nối backend từ IP proxy: chúng có thể giả header. Đặc biệt, `127.0.0.1` không phân biệt proxy với tiến trình cục bộ khác. Khi dùng Docker, kiểm tra IP nguồn thực tế và chỉ cho proxy truy cập mạng backend. Backend bỏ qua `X-Forwarded-For`, và từ chối đăng nhập nếu proxy tin cậy không gửi đúng một IP hợp lệ. Nếu không cấu hình `TRUSTED_PROXY_IP`, giới hạn tính theo IP kết nối; khi đó các client qua cùng proxy có thể cùng bị chặn.

Giới hạn chỉ lưu trong bộ nhớ, reset khi khởi động lại và không ngăn được tấn công phân tán hoặc đổi địa chỉ IPv6. Bảng giới hạn đầy cũng có thể chặn client mới. Cần bổ sung biện pháp tại proxy/mạng và đánh giá rủi ro trước khi vận hành; cấu hình proxy thực tế chưa được kiểm thử.

Sau khi build, tạo tài khoản đầu tiên bằng `DATABASE_PATH=/path/to/schedule.db ./schedule-display bootstrap admin`. Lệnh đọc mật khẩu từ stdin; yêu cầu 12–1024 byte và không chấp nhận bootstrap lại. Khi nhập từ terminal, tắt echo trước khi nhập, kết thúc stdin bằng Ctrl-D rồi bật lại echo; bảo vệ quyền truy cập DB và khóa TLS. Không truyền mật khẩu qua tham số dòng lệnh.

Đọc `tasks/auth-threat-model.md` trước khi triển khai. Chức năng quản lý tài khoản và khôi phục quyền quản trị chưa có.

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
