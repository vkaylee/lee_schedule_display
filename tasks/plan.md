# Kế hoạch: Quản lý lịch và màn hình phòng họp

## Context
Xây MVP cho dưới 10 phòng trong mạng nội bộ: quản trị viên quản lý lịch, trình duyệt kiosk hiển thị trạng thái phòng. Lịch lặp nâng cao, bảo vệ dữ liệu và không báo phòng trống sai là yêu cầu cốt lõi. Lịch hiện nằm trong ứng dụng; chưa xây kết nối hệ thống ngoài.

Repo hiện là skeleton Go/chi + SQLite (`main.go` mở DB nhưng chưa đưa vào handler), Svelte 5/Vite (`web/src/App.svelte` là placeholder), frontend embed vào binary. Giữ stack và tái dùng router/asset handler, driver SQLite, test `httptest`; không thêm framework khi chưa cần. `leedevkit.toml` cấu hình các gate unit, integration, lint. Chưa có đặc tả lưu trong repo được xác nhận; cuộc hội thoại là nguồn yêu cầu.

## Quyết định và giới hạn
- Một múi giờ IANA chung cấu hình cho hệ thống; lưu thời điểm tuyệt đối và quy tắc lặp theo giờ địa phương. Định nghĩa và kiểm thử chuyển giờ mùa hè trước khi hỗ trợ múi giờ có DST; không âm thầm đổi giờ không tồn tại hoặc mơ hồ.
- Chuỗi lặp hỗ trợ mỗi N ngày/tuần/tháng, nhiều thứ trong tuần, ngày tháng hoặc thứ tự thứ trong tháng, kết thúc theo ngày/số buổi/vô hạn. Ngày 31 không tồn tại thì bỏ qua. Xác định giới hạn hợp lý cho N, độ dài cuộc họp và đầu vào để tránh phép tính không có giới hạn.
- Kiểm tra trùng bằng khoảng nửa mở `[bắt đầu, kết thúc)` trong transaction ghi SQLite, bao gồm mọi occurrence giao cửa sổ và các ngoại lệ. Lịch một lần luôn được kiểm tra đúng thời điểm của nó, kể cả xa hơn 12 tháng. Với chuỗi: kiểm tra 12 tháng từ thời điểm hiện tại hoặc ngày bắt đầu trong tương lai, lấy mốc muộn hơn; hiển thị rõ ngày hết hạn kiểm tra. Kiểm tra định kỳ cửa sổ trượt 12 tháng, lưu cảnh báo có thể xử lý trong trang quản trị, không gửi email. Không khẳng định không có xung đột ngoài cửa sổ; khi phát hiện không tự hủy lịch.
- Ngoại lệ gắn với định danh occurrence gốc. Thao tác một buổi / từ buổi này / cả chuỗi phải nguyên tử, giữ hoặc xử lý rõ các ngoại lệ đã tồn tại. Nếu thay đổi quy tắc khiến không thể bảo toàn ngoại lệ, dừng và yêu cầu xác nhận theo chính sách sẽ chốt trước khi triển khai; không âm thầm xóa.
- Mặc định màn hình phòng không lộ tiêu đề/người tổ chức; họp riêng tư luôn ẩn. Mỗi thiết bị có danh tính và quyền chỉ đọc giới hạn phòng được gán, có thể thu hồi. Không đặt thông tin xác thực trong URL hoặc log. Tài khoản quản trị cùng quyền, xác thực ở server, giới hạn đăng nhập, session an toàn và CSRF; thiết lập HTTPS nội bộ tin cậy cho mật khẩu và danh tính thiết bị.
- Kiosk lưu bản lịch gần nhất để hiển thị khi gián đoạn; quá 15 phút từ lần đồng bộ thành công chuyển sang Không xác định. Kiểm thử đồng hồ lệch, dữ liệu không có, tab ngủ và phục hồi; dùng thời gian đơn điệu khi khả dụng và không tin thời gian máy bị điều chỉnh để kéo dài tuổi dữ liệu.
- Chỉ chuẩn bị trường nguồn/mã ngoài khi thiết kế DB cần thiết; không tạo adapter hay đồng bộ trước. Sao lưu SQLite nhất quán và kiểm tra khôi phục trong môi trường riêng, không ghi đè dữ liệu thật.

## Phụ thuộc
`bootstrap + đăng nhập` → `phòng` → `lịch một lần` → `thiết bị + kiosk` → `riêng tư/offline/i18n`.
`lịch một lần` → `lặp có giới hạn` → `ngoại lệ/sửa chuỗi` → `vô hạn + kiểm tra trượt/cảnh báo`. Các nhánh hội tụ ở kiểm thử đầu cuối, sao lưu và triển khai. Sửa schema theo migration có phiên bản, cập nhật mỗi lát thay vì dựng toàn bộ schema trước.

## Các lát triển khai
Mỗi tác vụ gồm lưu trữ + endpoint + UI phù hợp và test; vị trí file cụ thể chốt theo cấu trúc hiện có khi thực hiện. Mỗi checkpoint chạy `leedevkit test all --json` và kiểm thử thủ công các luồng đã hoàn thành; lỗi hoặc gate không chạy phải báo rõ.

1. **Tài khoản đầu tiên và đăng nhập.** Bootstrap an toàn bằng thao tác vận hành một lần; không có mật khẩu mặc định, không cho bootstrap lại khi đã có tài khoản. Session có hết hạn, đăng xuất, bảo vệ CSRF, giới hạn thử mật khẩu; không lộ secrets. Test đăng nhập đúng/sai, truy cập không quyền, bootstrap lặp. Dự kiến `main.go`, package auth/migration mới, `web/src/` và test. Phụ thuộc: không.
2. **Quản lý tài khoản.** Quản trị viên tạo, vô hiệu hóa và đặt lại mật khẩu tài khoản; ngăn vô hiệu hóa tài khoản quản trị cuối; thu hồi session khi cần, ghi nhật ký không chứa bí mật. Test quyền và các đường lỗi. Phụ thuộc: 1.

**Checkpoint A:** đăng nhập trên HTTPS nội bộ, quản lý hai tài khoản và xác nhận truy cập không đăng nhập bị từ chối. Chốt cách cấp chứng chỉ/triển khai HTTPS trước vận hành.

3. **Quản lý phòng.** Tạo, đổi tên, xem phòng; cấu hình quyền hiển thị tiêu đề/người tổ chức, mặc định ẩn. Không xóa dữ liệu lịch vô tình khi vô hiệu hóa phòng. Test hợp lệ, phân quyền, mặc định và lưu bền vững. Phụ thuộc: 1.
4. **Đặt lịch một lần.** Tạo/xem/sửa/hủy lịch phòng gồm tiêu đề, người tổ chức, riêng tư, thời gian; cho phép mọi giờ. Giao dịch kiểm tra và ghi nguyên tử, phát hiện trùng tại thời điểm thực tế kể cả lịch xa; cho phép hai buổi liền giờ, báo lỗi rõ. Test hai kết nối đặt đồng thời, biên thời gian và audit. Phụ thuộc: 3.

**Checkpoint B:** hai quản trị viên cùng thử đặt trùng; chỉ một ghi thành công, lịch còn nguyên sau khởi động lại.

5. **Ghép màn hình và hiển thị lịch.** Quản trị viên đăng ký/gán thiết bị phòng, thu hồi và xem lần kết nối cuối; kiosk chỉ đọc phòng được gán, hiển thị giờ, phòng, trống/đang họp và lịch tới. Test thiết bị khác phòng, thu hồi, trạng thái tại ranh giới và không lộ dữ liệu riêng tư trong payload. Phụ thuộc: 3–4.
6. **Mất mạng và dữ liệu cũ.** Đồng bộ định kỳ và tự phục hồi; sau 15 phút hoặc không có dữ liệu đáng tin hiển thị Không xác định, kèm lần đồng bộ cuối. Test ngắt mạng, tab ngủ, đổi đồng hồ, khôi phục. Phụ thuộc: 5.

**Checkpoint C:** thử kiosk thực tế với gián đoạn và thu hồi thiết bị; không xuất hiện thông tin riêng tư hoặc trạng thái trống không đáng tin.

7. **Lịch lặp có giới hạn.** Tạo/xem chuỗi với các quy tắc và ngày kết thúc/số buổi; kiểm tra các buổi trong cửa sổ đã chốt, kể cả occurrence giao biên. Test tháng thiếu ngày, tuần nhiều thứ, biên và múi giờ/DST. Ưu tiên thư viện recurrence được bảo trì nếu không có giải pháp sẵn; đánh giá license, tính đúng và giới hạn trước khi thêm dependency. Phụ thuộc: 4.
8. **Ngoại lệ một buổi.** Sửa/hủy một occurrence, giữ định danh gốc và kiểm tra trùng cả khi chuyển sang ngày khác; không làm thay đổi buổi khác. Test ngoại lệ di chuyển vào/ra cửa sổ và cạnh tranh ghi. Phụ thuộc: 7.

**Checkpoint D:** kiểm thử chuỗi qua tháng thiếu ngày, ngoại lệ và xung đột; so sánh UI, DB và kiosk.

9. **Sửa/hủy từ buổi này và toàn chuỗi.** Chia chuỗi nguyên tử và bảo toàn số buổi, ngoại lệ và lịch sử; xử lý thay đổi quy tắc theo chính sách được duyệt, từ chối nếu không thể bảo toàn an toàn. Test occurrence đầu/cuối, ngoại lệ trước/sau điểm chia, xung đột không gây ghi một phần. Phụ thuộc: 8.
10. **Lặp vô hạn và cảnh báo.** Cho chuỗi không có ngày kết thúc, hiển thị khoảng đã kiểm tra, chạy kiểm tra định kỳ trượt và lưu cảnh báo có thể xem/xử lý; chạy lại không nhân đôi cảnh báo, dọn trạng thái khi xung đột được giải quyết. Test lịch khởi đầu xa, xung đột mới vượt biên cũ, chạy lại và khởi động lại. Phụ thuộc: 9.

**Checkpoint E:** diễn tập xung đột xuất hiện sau khi cửa sổ dịch; không tự hủy, quản trị viên nhìn thấy cảnh báo; màn hình không báo trống sai.

11. **Song ngữ.** Giao diện quản trị Việt/Anh theo tài khoản, kiosk cấu hình riêng theo thiết bị; không dịch nội dung nhập. Test cả hai ngôn ngữ, lưu lựa chọn và hiển thị trạng thái lỗi. Phụ thuộc: 2, 5, 10.
12. **Sao lưu, khôi phục và vận hành.** Thêm quy trình sao lưu SQLite nhất quán, kiểm tra khôi phục trên bản sao; cấu hình lưu dữ liệu bền vững, múi giờ, HTTPS, khởi động lại kiosk và nhật ký không lộ secrets. Không thực hiện khôi phục đè dữ liệu thật nếu chưa được duyệt riêng. Test backup đang hoạt động và restore vào DB tạm. Phụ thuộc: 1–11.

**Checkpoint phát hành:** chạy toàn bộ gate, kiểm thử E2E thực trên trình duyệt: đăng nhập → tạo phòng → lịch đơn/lặp → sửa từng phạm vi → ghép kiosk → quyền riêng tư → mất mạng → cảnh báo → khôi phục backup tạm. Kiểm thử bảo mật server-side, đồng thời SQLite, quyền thiết bị và CSRF; ghi lại kết quả và rủi ro chưa xác minh.

## Kiểm thử và triển khai
- Theo `leedevkit.toml`: `leedevkit test all --json`; lặp nhanh với `leedevkit test go --unit-only --pattern <regex>` nếu phù hợp. Build hermetic dùng `leedevkit exec go -- <command>`; frontend cần `bun run --cwd web build` trước Go build vì `go:embed`.
- Các gate hiện tại chưa có E2E; thêm kiểm thử trình duyệt phù hợp hoặc ghi lại các bước thủ công, không tuyên bố có gate tự động khi chưa thiết lập.
- Trước mỗi lát đọc các rulebook tương ứng cho migration, bảo mật, giao diện và vận hành. Migration bảo vệ dữ liệu; rollback ưu tiên restore/roll-forward khi rollback phá dữ liệu.
- Sau khi người dùng duyệt và thoát plan mode: ghi kế hoạch vào `tasks/plan.md` và checklist `tasks/todo.md` theo yêu cầu skill, rồi mới triển khai nếu được yêu cầu. Hai file này chưa được tạo vì plan mode chỉ cho phép sửa file kế hoạch hiện tại.

## Đánh giá recurrence

`github.com/teambition/rrule-go` là ứng viên MIT, README mô tả port một phần RFC 2445; [mã nguồn](https://github.com/teambition/rrule-go/blob/master/rrule.go) cho thấy iterator bắt đầu từ `dtstart`, kiểm tra giới hạn số nhưng không xác thực DST gap/fold. Truy vấn xa có thể phải duyệt từ đầu; chưa có bằng chứng về phiên bản, tình trạng bảo trì và giới hạn công việc đủ đáp ứng yêu cầu. Chưa thêm dependency; cần kiểm tra phiên bản cố định, license và test DST, hiệu năng trước khi chọn. Prototype tự viết cũng chưa đạt các biên này.

## Điểm cần làm rõ trước tác vụ liên quan
- Nếu chỉnh hoặc tách chuỗi khiến occurrence gốc của ngoại lệ biến mất, từ chối thay đổi và yêu cầu xử lý ngoại lệ trước; không tự xóa.
- Với giờ địa phương không tồn tại hoặc mơ hồ do DST, từ chối và giải thích để người dùng chọn thời điểm khác.
- Nơi chạy HTTPS nội bộ và phương pháp cấp chứng chỉ tin cậy cho máy quản trị và kiosk.
- Khoảng kiểm tra 12 tháng là giới hạn có chủ đích; ghi rõ tới người dùng, kể cả đối với chuỗi có ngày kết thúc xa.
