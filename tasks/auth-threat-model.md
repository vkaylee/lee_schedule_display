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

## Đề xuất xử lý trước vận hành

- Ứng dụng giới hạn IPv6 theo prefix `/64`, IPv4 theo địa chỉ sau chuẩn hóa. `/64` có thể chặn nhầm người cùng mạng và không ngăn đổi prefix; cần kiểm thử NAT, xem xét hạn mức theo tài khoản và hạn mức tổng theo mạng thực tế, không khóa tài khoản vô thời hạn.
- Khi bảng đầy, ứng dụng chỉ xóa mục hết hạn và từ chối tạm khóa mới, không xóa khóa còn hiệu lực. Cần thêm giới hạn tổng ở proxy/mạng, cảnh báo và quy trình khôi phục. Cách này vẫn có thể làm client mới bị từ chối; cần đánh giá sức chứa và kiểm thử trước triển khai.
- Bộ đếm hiện mất khi khởi động lại và không chia sẻ giữa tiến trình. Chọn cách lưu bền vững/chia sẻ hoặc biện pháp bù tại proxy, kiểm thử khởi động lại và giới hạn số tiến trình trước vận hành. Không coi các đề xuất này là biện pháp đã triển khai.

## Kiểm thử ngày 2026-11-27

- `leedevkit test all --json`: unit, integration, lint đều đạt; không có gate E2E được cấu hình.
- Chrome headless qua Caddy 2.8.4 HTTPS và backend riêng với DB tạm: mật khẩu sai, đăng nhập đúng, tải lại, đăng xuất và tải lại đều đạt. Trình duyệt bỏ qua kiểm tra chứng chỉ CA thử; chưa xác minh chuỗi tin cậy trên thiết bị thực tế.
- Với cùng Caddy, `curl --cacert` xác minh chứng chỉ CA thử. Gửi các header `X-Real-IP` giả khác nhau từ cùng client vẫn dùng chung hạn mức và nhận 429 sau khi hết lượt; kiểm thử dùng DB và mạng tạm, không phải cấu hình triển khai thực tế. Chưa xác minh cách ly backend, nguồn IP, IPv6 hay giới hạn ở proxy trên hạ tầng thực.

## Ranh giới thiết bị kiosk (đang triển khai)

Mã thiết bị là bearer token riêng theo phòng, chỉ cấp một lần và lưu bản băm tại máy chủ; phải bảo vệ mã như mật khẩu, không ghi vào URL hoặc log. Kiosk chỉ đọc dữ liệu phòng được gán, kiểm tra thu hồi và trạng thái phòng trên server; tiêu đề và người tổ chức họp riêng tư không xuất hiện trong payload. Trình duyệt giữ mã trong sessionStorage, nên mọi script cùng origin có thể đọc: cần bảo vệ origin, kiểm soát XSS và hạn chế truy cập máy kiosk. Khi mất mạng, thu hồi không thể có hiệu lực ngay tại màn hình; dữ liệu cũ phải chuyển Không xác định sau tối đa 15 phút. Cần rà soát bảo mật độc lập, thử thiết bị thật và gián đoạn trước vận hành.

## Rủi ro còn lại và vận hành

Mạng nội bộ không thay thế TLS. Trước vận hành, chủ hệ thống phải chỉ định cách cấp chứng chỉ tin cậy, bảo vệ máy chủ và backup, quy trình khôi phục quyền quản trị, xử lý các rủi ro hạn mức nêu trên và kiểm thử proxy trên hạ tầng thực. Rà soát lại mô hình khi thêm API quản trị, thiết bị kiosk, proxy hoặc tích hợp ngoài.
