# Lớp và tương tác thiết kế màn hình

Status: Approved

## Mục tiêu
Cho phép các lớp độc lập chồng lên nhau, thêm chữ và hình chữ nhật, đổi kích thước từ cạnh/góc; giữ bố cục cũ và chưa hỗ trợ nhóm.

## Ràng buộc
- Bảo toàn thay đổi chưa commit; cấu hình cũ tiếp tục đọc và hiển thị đúng. Không thay đổi quyền truy cập hoặc dữ liệu lịch công khai.
- Lưu ID ổn định cho mỗi lớp mới, thứ tự lớp theo thứ tự mảng; cho phép nhiều text/hình, giới hạn số lượng và độ dài. Chỉ nhận văn bản thuần và thuộc tính hiển thị được xác thực; không nhận HTML/CSS tùy ý.
- Giữ lưới 12 cột, không vượt khung; chồng lớp được phép và không tự di chuyển lớp khác. Duy trì cách chỉnh bằng bàn phím và bảo vệ thay đổi chưa lưu.
- Thử Interact.js trước khi chọn; kiểm tra tương thích Svelte 5, bundle, touch và lifecycle. Không thay phụ thuộc nếu thử nghiệm không đạt; báo giới hạn thay vì tuyên bố đã xác minh.

## Công việc
- [ ] Thiết kế và kiểm thử hợp đồng lớp trong `display.go` và `web/src/display-layout.js`: ID, thứ tự, văn bản, màu hình; xác thực giới hạn và tương thích JSON cũ, thử round-trip nhiều lớp trùng loại và chồng nhau.
- [ ] Thử tương tác Interact.js với canvas đang zoom; xác minh cạnh/góc, chuột, touch, hủy thao tác và dọn listener; chỉ thêm dependency khi thử nghiệm đạt.
- [ ] Cập nhật `web/src/DisplayEditor.svelte`: thêm/xóa text và hình, chọn lớp kể cả khi bị che, đổi thứ tự, chỉnh thuộc tính và vị trí; xác nhận thao tác mất dữ liệu.
- [ ] Cập nhật `web/src/DisplayCanvas.svelte`: render lớp theo thứ tự, kéo cạnh/góc bắt lưới, giới hạn khung; hỗ trợ chọn và chỉnh bằng bàn phím. Xem trước và trang công khai dùng chung renderer.
- [ ] Cập nhật kiểm thử API, logic và trình duyệt: dữ liệu cũ, lớp chồng nhau, chuyển thứ tự, lưu/tải lại, kiểm thử quyền và nội dung không an toàn, zoom, touch, hủy và bàn phím.
- [ ] Xác minh cuối bằng `leedevkit test all --json` và trình duyệt; cập nhật hướng dẫn. Báo rõ bước không chạy được.

## Hoàn tất khi
Thiết kế cũ không đổi; text chồng lên hình, chọn và đổi thứ tự được, lưu/tải lại và hiển thị công khai đúng. Không hỗ trợ nhóm trong đợt này.
