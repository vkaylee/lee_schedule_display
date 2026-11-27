# Schedule Display

Go API phục vụ SPA Svelte được embed trong binary. SQLite dùng driver Go `modernc.org/sqlite`.

## Yêu cầu

- Docker Compose và LeeDevKit cho môi trường container.
- Khi chạy trực tiếp: Go 1.26, Bun, Air (`go install github.com/air-verse/air@v1.61.7`).

## Phát triển

```sh
leedevkit dev up
leedevkit dev ps
# Xem cổng được Docker cấp:
docker compose -p lee-schedule-display-dev port go 8080
```

Đặt `APP_PORT=8080` trước `leedevkit dev up` nếu muốn cổng cố định. SQLite dev nằm trong volume `dev_data`; `leedevkit dev down` giữ dữ liệu, còn `leedevkit dev clean` xóa volume và dữ liệu.

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
