# Mô hình đe dọa: Đăng nhập quản trị

Trạng thái: cần rà soát độc lập trước khi đưa vào sử dụng. Người chịu trách nhiệm vận hành: chủ hệ thống, cần chỉ định trước khi triển khai.

## Tài sản và ranh giới

Mật khẩu và phiên quản trị bảo vệ quyền quản lý lịch, tài khoản và thiết bị. Trình duyệt, mạng nội bộ, máy chủ HTTP và SQLite là các ranh giới tin cậy riêng. Người truy cập mạng và trang web bên ngoài đều có thể gửi yêu cầu không đáng tin. Người có quyền quản trị máy chủ hoặc đọc DB/backup nằm ngoài phạm vi xác thực ứng dụng.

## Đe dọa và kiểm soát cần xác minh

- Giả mạo/mò mật khẩu: mật khẩu băm bằng thuật toán chuẩn, lỗi đăng nhập chung, giới hạn thử; bootstrap chỉ qua thao tác vận hành cục bộ và chỉ khi chưa có tài khoản.
- Sửa hoặc phát lại yêu cầu: TLS tại reverse proxy cho truy cập ngoài localhost, backend HTTP chỉ kết nối tới proxy tin cậy, session ngẫu nhiên, cookie HttpOnly/Secure/SameSite, bảo vệ CSRF ở server; không tin header proxy từ nguồn bất kỳ. HTTP chỉ dùng trực tiếp trên localhost.
- Chối bỏ: không ghi thông tin xác thực hoặc token vào log; audit thay đổi tài khoản thuộc tác vụ tiếp theo.
- Lộ thông tin: không đặt mật khẩu/token trong URL, argv, log hoặc lỗi; bảo vệ quyền đọc SQLite và bản sao lưu.
- Cạn tài nguyên: giới hạn kích thước request, tốc độ đăng nhập và thời gian xử lý; theo dõi lỗi và hạn mức. Hệ thống đơn nút không bảo đảm chống tấn công phân tán.
- Leo thang quyền: từ chối mặc định ở server; kiểm tra mọi endpoint quản trị và thu hồi phiên khi đăng xuất.

## Rà soát độc lập

Rà soát độc lập xác minh timeout đọc body và cookie `__Host-`. Có thể cấu hình một IP proxy tin cậy để xác định IP client; proxy phải ghi đè `X-Real-IP`, và backend phải được cách ly khỏi mọi nguồn khác có cùng IP. Rà soát phát hiện giới hạn IPv6 theo từng địa chỉ có thể bị vượt, bảng 10.000 IP có thể bị lấp khiến client mới bị chặn, và bộ đếm reset sau khởi động lại. Chủ hệ thống cần xử lý các rủi ro này và kiểm thử proxy thực tế trước vận hành; chưa kiểm thử tấn công động.

## Rủi ro còn lại và vận hành

Mạng nội bộ không thay thế TLS. Trước vận hành, chủ hệ thống phải chỉ định cách cấp chứng chỉ tin cậy, bảo vệ máy chủ và backup, quy trình khôi phục quyền quản trị, và xem xét giới hạn đăng nhập sau khi khởi động lại. Rà soát lại mô hình khi thêm API quản trị, thiết bị kiosk, proxy hoặc tích hợp ngoài.
