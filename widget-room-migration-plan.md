# Chuyển phòng từ màn hình sang widget

Status: Approved — ứng dụng chưa go live; người dùng đã cho phép loại bỏ kiosk cũ và liên kết phòng trên màn hình, không cần giai đoạn tương thích consumer.

## Mục tiêu và hợp đồng

- Màn hình không có phòng mặc định; mỗi widget `room`, `status`, `upcoming` chọn một phòng riêng. Cho phép nhiều instance cùng loại, mỗi instance có ID ổn định. Widget chung không có `roomId`.
- Màn hình công khai chỉ trả dữ liệu cần thiết theo các phòng được chọn, không lộ tiêu đề hoặc người tổ chức. Giới hạn số widget/phòng, deduplicate truy vấn và tính độ tin cậy lịch riêng từng phòng. Phòng không khả dụng chỉ ảnh hưởng widget của phòng đó.
- Loại bỏ kiosk token cũ theo quyết định mới của người dùng: ứng dụng chưa go live và không cần duy trì tương thích kiosk.
- Migration phải giữ cấu hình, ID công khai, trạng thái thiết bị, bố cục và liên kết phòng hiện hữu; cấu hình rỗng dùng bố cục mặc định tương đương. Không thay đổi dữ liệu đặt phòng.
- Triển khai theo expand → migrate → verify → contract. Không xóa `devices.room_id` hoặc ngừng API cũ trước khi xác nhận consumer đã chuyển và có phương án rollback; trạng thái đích không còn liên kết phòng ở màn hình.

## Công việc

1. **Chốt hợp đồng và kiểm kê consumer** — Kiểm tra phiên bản code/DB hiện hành, mọi API thiết bị, kiosk, màn hình công khai và frontend; xác định giới hạn phòng/widget, định dạng payload đa phòng, lỗi phòng không hợp lệ, chính sách quyền riêng tư và cách thông báo ngừng kiosk. Xác định chiến lược tương thích khi phiên bản cũ/mới chạy đồng thời; nếu không thể, yêu cầu triển khai dừng toàn bộ phiên bản cũ. Xác minh bằng contract test cho consumer còn hỗ trợ.
2. **Kiểm thử migration trước khi sửa** — Tạo fixture DB cũ: cấu hình rỗng/tùy chỉnh, phòng vô hiệu, thiết bị thu hồi, widget thiếu hoặc đang ẩn; lưu snapshot cấu hình và ID. Kiểm tra migration nguyên tử, chạy lại an toàn, lỗi giữa chừng rollback, khóa ngoại và số lượng bản ghi.
3. **Mở rộng schema và backfill** — Trong `auth.go` và migration liên quan, bổ sung biểu diễn widget có ID và `roomId`; chuyển phòng cũ vào từng widget phòng, kể cả widget mặc định khi cấu hình rỗng. Giữ nguyên widget chung, thứ tự và style. Cập nhật kiểm tra schema/version và test nâng cấp. Không xóa cột cũ ở giai đoạn này; đối soát từng thiết bị trước/sau.
4. **Đổi API và validation** — Trong `display.go`, `devices.go` và test, tạo/sửa màn hình không yêu cầu phòng; validate ID instance, loại widget, phòng tồn tại/khả dụng khi lưu. Trả dữ liệu theo phòng riêng, bảo toàn giới hạn và tính coverage theo từng phòng. Test không phòng, nhiều phòng, trùng loại, phòng bị vô hiệu, lỗi/quá hạn lịch và quyền riêng tư; xác nhận endpoint không còn dựa vào phòng thiết bị sau chuyển đổi.
5. **Đổi editor và renderer** — Trong `web/src/DisplayEditor.svelte`, `DisplayCanvas.svelte`, `display-layout.js` và `PublicDisplay.svelte`, thêm/xóa nhiều instance, chọn phòng trong thuộc tính widget phòng, giữ lựa chọn khi reorder/lưu/tải lại; render trạng thái không khả dụng theo widget. Widget chung hoạt động trên màn hình không phòng. Kiểm tra bàn phím, cảm ứng, responsive và giới hạn UI trùng backend.
6. **Ngừng kiosk và hoàn tất contract** — Sau kiểm kê consumer và chấp thuận triển khai breaking change, gỡ UI/route/API kiosk token và thao tác quản trị chỉ còn phục vụ kiosk; cập nhật tài liệu và test. Sau khi đối soát migration, chuyển toàn bộ reader/writer và đóng cửa sổ rollback, bỏ `devices.room_id` cùng đường tương thích tạm; xác minh không còn phụ thuộc.
7. **Xác minh cuối** — Chạy `leedevkit test all --json`, build frontend theo cấu hình dự án; kiểm tra trên app thật bằng dữ liệu thử: màn hình không phòng, hai widget cùng loại khác phòng, lưu/tải lại, phòng vô hiệu và migration DB cũ. Ghi rõ gate lỗi hoặc bị bỏ qua, không coi test mục tiêu là full regression.

## An toàn và phục hồi

- Trước migration thật: backup có thể khôi phục, diễn tập trên bản sao đại diện, đối soát số lượng, ID, cấu hình và khóa ngoại; theo dõi lỗi, thời gian khóa và độ trễ. Dừng nếu migration lỗi, mất ánh xạ hoặc đối soát không khớp.
- Điểm rollback an toàn cuối: trước khi ghi cấu hình đa phòng/đa instance mà phiên bản cũ không hiểu và trước khi xóa cột cũ. Sau điểm đó không chạy binary cũ trực tiếp: ưu tiên sửa tiến, hoặc khôi phục backup kèm kế hoạch đối soát dữ liệu phát sinh; diễn tập phục hồi trước khi contract.
- Giai đoạn tương thích và xóa dữ liệu cũ cần quyết định triển khai riêng; kế hoạch này không tự cấp phép migration production, xóa dữ liệu hay deploy.
