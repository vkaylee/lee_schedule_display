# Thiết kế màn hình thiết bị

Status: Approved

## Mục tiêu
Quản trị viên chọn mẫu giống ảnh tham khảo, tùy chỉnh khối trong lưới và lưu riêng cho từng thiết bị; `/display/{id}` hiển thị công khai không cần token.

## Ràng buộc
- URL dễ đoán: chỉ trả tên phòng, trạng thái, giờ lịch và cấu hình hiển thị; tuyệt đối không trả tên cuộc họp hoặc người đặt, kể cả khi phòng cho phép hiện thông tin đó trên kiosk hiện tại.
- Giữ nguyên `/kiosk` và API token hiện có. Thiết bị bị thu hồi hoặc phòng ngừng hoạt động không được công khai dữ liệu. Không cập nhật `last_seen` từ lượt xem công khai.
- Mẫu mặc định: đầu trang giờ/ngày, tên tổ chức và logo; tên phòng; trạng thái và thời gian còn lại; lịch sắp tới; trạng thái kết nối và thời điểm cập nhật.
- Giới hạn lưới và loại khối cố định; kiểm tra dữ liệu server, không cho HTML/CSS tùy ý. Hỗ trợ thao tác bằng bàn phím bên cạnh kéo thả. Bố cục thích ứng màn hình mà không chồng khối.
- Tên tổ chức là văn bản tùy chọn; logo dùng đường dẫn nội bộ được xác thực, chưa có tải tệp lên. Mặc định để trống. Khi mất kết nối hoặc dữ liệu quá hạn, hiện trạng thái không xác định, không khẳng định phòng còn trống.
- Migration không xóa dữ liệu; giữ tương thích thiết bị hiện có và tài liệu hóa sao lưu/rollback. Bảo toàn các thay đổi chưa commit.

## Công việc
- [ ] Chốt hợp đồng cấu hình phiên bản đầu: tập khối, tọa độ/kích thước theo lưới, giới hạn nội dung và cách biểu diễn mẫu mặc định; kiểm thử cấu hình lỗi, trùng hoặc vượt lưới.
- [ ] Thêm migration tại `auth.go` lưu cấu hình theo `devices.id`; thiết bị cũ dùng mẫu mặc định. Kiểm thử nâng cấp DB từ schema hiện tại và rollback giao dịch lỗi.
- [ ] Bổ sung API quản trị đọc/lưu cấu hình theo ID trong `devices.go`, dùng phiên đăng nhập, kiểm tra CSRF/origin như thao tác hiện tại; giới hạn kích thước và xác thực toàn bộ payload. Kiểm thử quyền, dữ liệu lỗi và lưu/đọc.
- [ ] Thêm API đọc công khai theo ID: truy vấn chỉ các trường được phép, lọc thiết bị thu hồi/phòng ngừng hoạt động, trả `no-store` và dữ liệu thời gian cần thiết. Kiểm thử không rò rỉ title/organizer với mọi thiết lập phòng và lịch riêng tư; ID không hợp lệ hoặc bị thu hồi trả 404.
- [ ] Thêm màn hình thiết kế từ `web/src/Devices.svelte`: chọn mẫu, bật/tắt khối, kéo thả/đổi kích thước theo lưới, điều khiển bàn phím, xem trước, lưu và báo lỗi. Lưu ý trạng thái thay đổi chưa lưu và phản hồi khi lưu thất bại.
- [ ] Thêm route `/display/{id}` trong `web/src/App.svelte` và màn hình hiển thị dùng chung renderer với xem trước; tính trạng thái từ dữ liệu thời gian, xử lý hết hạn/mất mạng, hiển thị trạng thái cập nhật. Duy trì `/kiosk` như cũ.
- [ ] Cập nhật README về URL công khai dễ đoán, dữ liệu được công khai, cấu hình logo và giới hạn mất mạng; nêu rõ bước sao lưu và khôi phục khi nâng schema.
- [ ] **Xác minh cuối:** chạy `leedevkit test all --json`; kiểm tra trình duyệt ở màn hình ngang/dọc và kích thước nhỏ, thao tác bàn phím/kéo thả, tải lại sau lưu, hai thiết bị độc lập, thu hồi, mất mạng và dữ liệu quá hạn. Báo riêng mọi kiểm tra không chạy được.

## Hoàn tất khi
Người quản trị lưu và xem trước bố cục riêng cho từng thiết bị; URL công khai hiển thị đúng mà không lộ tên cuộc họp/người đặt; thiết bị cũ và kiosk token tiếp tục hoạt động.
