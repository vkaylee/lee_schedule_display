# Tối ưu trình thiết kế màn hình

Status: Approved

## Mục tiêu
Chọn mẫu rồi kéo và đổi kích thước khối theo lưới; mỗi thiết bị có độ phân giải riêng, xem trước đúng tỷ lệ và giữ bố cục khi đổi độ phân giải.

## Ràng buộc
- Giữ nguyên khối, lưới 12 cột, giới hạn 24 hàng và quy tắc không chồng khối. Không ghi đè thay đổi chưa commit.
- Lưu độ phân giải cùng cấu hình hiển thị từng thiết bị; kiểm tra số nguyên dương và giới hạn hợp lý ở server. Cấu hình cũ thiếu độ phân giải tiếp tục đọc và hiển thị như cũ; yêu cầu khai báo trước khi dùng xem trước theo độ phân giải, không đoán thông số thiết bị.
- Giữ nguyên bố cục khi đổi độ phân giải. Ổn định số hàng của khung thiết kế để thêm, di chuyển hoặc xóa khối không làm các khối khác đổi kích thước; chuyển đổi cấu hình cũ không làm mất bố cục.
- Xem trước và màn hình công khai dùng chung quy tắc dựng hình; không thay đổi phạm vi dữ liệu công khai hoặc quyền truy cập. Có thao tác tương đương bằng bàn phím, cảnh báo thay đổi chưa lưu và báo lỗi rõ ràng.

## Công việc
- [ ] Xác định hợp đồng độ phân giải và số hàng trong `display.go`, `web/src/display-layout.js`; kiểm thử dữ liệu cũ, thiếu trường, giới hạn và lưu/đọc hai thiết bị độc lập.
- [ ] Bổ sung nhập độ phân giải vào `web/src/DisplayEditor.svelte`; đổi độ phân giải không thay đổi tọa độ khối. Giữ an toàn khi đóng, đổi mẫu hoặc lưu lỗi.
- [ ] Cập nhật `web/src/DisplayCanvas.svelte`: khung đúng tỷ lệ, thu phóng vừa vùng làm việc, lưới hàng ổn định; chọn, kéo và đổi kích thước theo ô, ngăn tràn và chồng khối. Giữ nhập tọa độ bằng bàn phím.
- [ ] Đảm bảo màn hình công khai phản ánh bố cục đã lưu, kiểm tra nội dung bị cắt/tràn ở tỷ lệ ngang và dọc; không thay đổi hiển thị cũ khi thiết bị chưa khai báo độ phân giải.
- [ ] Xác minh cuối: `leedevkit test all --json`; kiểm tra trình duyệt với kích thước ngang/dọc, kéo thả, đổi kích thước, bàn phím, đổi độ phân giải, tải lại sau lưu, thiết bị cũ và mất kết nối. Báo rõ kiểm tra không chạy được.

## Hoàn tất khi
Thiết bị có thể lưu và xem đúng bố cục riêng theo độ phân giải khai báo; thay đổi tỷ lệ không làm mất vị trí lưới; cấu hình cũ vẫn hoạt động.
