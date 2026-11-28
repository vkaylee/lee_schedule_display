# Thêm tiếng Trung giản thể

Status: Implementing — chờ kiểm chứng đầy đủ

## Mục tiêu

Thêm `zh-CN` cho giao diện quản trị và màn hình lịch, giữ cơ chế lưu ngôn ngữ theo tài khoản và thiết bị. Giữ nguyên dữ liệu người dùng, ngôn ngữ mặc định và hành vi `vi`/`en`.

## Quyết định database

Nâng cấp database SQLite phiên bản 19 hiện có để giữ dữ liệu; thay đổi chính sách hiện tại vốn yêu cầu database mới. Có kiểm thử nâng cấp, rollback và phương án sao lưu/khôi phục.

## Công việc

- [ ] Nâng schema 19 lên 20 trong giao dịch, giữ dữ liệu account/session/series/device, khóa ngoại và chỉ mục; không xóa database. Test schema 19 gốc, dữ liệu liên quan và rollback; README yêu cầu sao lưu trước khi cập nhật. Cần chạy lại test sau thay đổi migration gần nhất.
- [ ] Mở rộng schema và kiểm định API tài khoản/thiết bị để chấp nhận `zh-CN`; giữ mặc định `vi`, từ chối locale không hỗ trợ. Kiểm thử lưu và đọc lại cả hai loại cài đặt.
- [ ] Bổ sung lựa chọn `简体中文` trong giao diện tài khoản và thiết bị; giữ luồng lưu hiện tại. Không thêm thư viện i18n nếu cơ chế sẵn có đáp ứng.
- [ ] Dịch toàn bộ chuỗi giao diện quản trị và màn hình công khai, bao gồm trạng thái, lỗi, hộp thoại, nhãn truy cập và bản xem trước; không dịch dữ liệu do người dùng nhập. Rà soát các nhánh chọn `vi`/`en` hiện có để tránh bỏ sót.
- [ ] Bổ sung định dạng ngày giờ `zh-CN`, thời lượng và trạng thái lịch; giữ múi giờ và định dạng của hai ngôn ngữ hiện tại. Kiểm tra font, tràn chữ và khả năng đọc trên màn hình thiết bị.
- [ ] Lập danh mục toàn bộ chuỗi do ứng dụng tạo, gồm nhãn, lỗi, placeholder, thông báo, thuộc tính accessibility, bản xem trước và màn hình công khai; phân biệt dữ liệu do người dùng nhập và chuỗi kỹ thuật không cần dịch.
- [ ] Bổ sung kiểm thử tự động đối chiếu danh mục bản dịch của `vi`, `en`, `zh-CN`: thiếu key, giá trị rỗng hoặc thiếu tham số nội suy phải làm test thất bại. Thêm kiểm tra phát hiện chuỗi hiển thị hardcode ngoài danh mục; ngoại lệ phải ghi rõ lý do. Chọn cách tổ chức bản dịch tối thiểu tương thích với mã hiện tại.
- [ ] Kiểm thử frontend theo ba ngôn ngữ cho các màn hình, trạng thái lỗi, dialog và display; kiểm tra ngày giờ, thời lượng, fallback và locale không hợp lệ. Kiểm thử backend lưu/đọc ngôn ngữ; cập nhật README nếu phạm vi ngôn ngữ thay đổi.
- [ ] Kiểm chứng cuối: chạy `leedevkit test all --json`, cùng kiểm thử frontend theo cấu hình dự án; kiểm tra trên trình duyệt việc đổi, tải lại, đăng nhập lại và màn hình công khai theo thiết bị. Ghi rõ nếu không kiểm thử được thiết bị vật lý.

## Hoàn thành khi

- Người dùng và thiết bị chọn, lưu và hiển thị tiếng Trung giản thể sau khi tải lại; `vi` và `en` không hồi quy.
- Mọi chuỗi giao diện thuộc phạm vi danh mục đều có bản dịch không rỗng cho ba ngôn ngữ; kiểm tra tự động bắt được key thiếu, tham số thiếu và chuỗi hardcode theo quy tắc đã xác định. Rà soát thủ công phần kiểm tra tự động không bao phủ; không xem kiểm tra tĩnh là bằng chứng tuyệt đối.
- Database hiện có được xử lý theo chính sách đã chốt; không mất dữ liệu.
- Kiểm thử liên quan đạt hoặc giới hạn được báo rõ. Giữ nguyên các thay đổi chưa commit của người dùng.
