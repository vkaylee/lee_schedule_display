# Checklist triển khai MVP

Chi tiết, điều kiện nghiệm thu, kiểm thử và phụ thuộc: xem [plan.md](plan.md). Tác vụ 1 chưa hoàn tất: rủi ro giới hạn đăng nhập và kiểm thử trên hạ tầng proxy thực còn chặn triển khai; bằng chứng và đề xuất tại [auth-threat-model.md](auth-threat-model.md).

- [ ] 1. Bootstrap tài khoản đầu tiên và đăng nhập an toàn. Tạm bỏ qua theo yêu cầu; các rủi ro và kiểm thử proxy thực vẫn chưa được giải quyết, không coi là sẵn sàng triển khai.
- [x] 2. Quản lý tài khoản quản trị. Đã kiểm thử API, giao diện, giao dịch cạnh tranh và rà soát bảo mật; checkpoint A vẫn phụ thuộc proxy triển khai thực.
- [ ] Checkpoint A: HTTPS, tài khoản và kiểm tra truy cập.
- [x] 3. Quản lý phòng và cấu hình riêng tư. Đã kiểm thử API, trình duyệt và lưu lịch khi phòng bị vô hiệu hóa; đã xử lý các điểm rà soát phòng.
- [ ] 4. Đặt lịch một lần, kiểm tra trùng nguyên tử. Đã có API, giao diện múi giờ chung, test cạnh tranh, DST và trình duyệt; đã bổ sung kiểm soát phiên bản, chặn lưu khi chưa tải múi giờ và thử lại trình duyệt trước thay đổi mới. Còn kiểm thử hạ tầng thực tại checkpoint B.
- [ ] Checkpoint B: đã kiểm thử API hai tài khoản đặt trùng đồng thời và mở lại DB giữ lịch; còn kiểm thử qua trình duyệt và khởi động lại trên hạ tầng thực.
- [ ] 5. Ghép thiết bị và hiển thị lịch trên kiosk. Đã có đăng ký, ghép, thu hồi, quyền đọc theo phòng; test API kiểm tra riêng tư và mốc an toàn khi lịch vượt 100 mục. Còn kiểm thử kiosk thực và trạng thái biên.
- [ ] 6. Xử lý mất kết nối và dữ liệu quá 15 phút. Đã kiểm thử Chrome chuyển Không xác định khi mất kết nối và phục hồi; test đơn vị kiểm tra lệch đồng hồ, độ trễ và biên cập nhật. Còn kiểm thử vòng đời/tab ngủ, hẹn giờ trễ trên kiosk thực và lưu lịch sau khởi động lại. Nếu thiết bị ngủ và thay đổi đồng hồ đồng thời mà trình duyệt không phát sự kiện, hai đồng hồ có thể không phát hiện được; luồng giao diện treo hoặc hẹn giờ trễ hơn 30 giây cũng có thể giữ trạng thái Trống quá hạn. Cần watchdog hiển thị độc lập hoặc giải pháp thiết bị tương đương; chưa đủ bảo đảm an toàn tuyệt đối. Cách xử lý hiện tại ưu tiên an toàn hơn hiển thị offline theo kế hoạch.
- [ ] Checkpoint C: kiểm thử kiosk, thu hồi và riêng tư.
- [ ] 7. Tạo và xem lịch lặp có giới hạn. Đã thêm lát thử nghiệm tạo/xem chuỗi ngày, tuần, tháng và kiểm tra trùng; hiện từ chối chuỗi có buổi kết thúc quá 12 tháng, hẹp hơn kế hoạch. Chromium trên DB tạm đã tạo chuỗi ngày/tuần/tháng, kiểm tra lịch đơn trùng và kiosk ẩn nội dung riêng tư; snapshot bản tích hợp đã kiểm tra hiển thị chuỗi tuần theo múi giờ Asia/Ho_Chi_Minh và qua tải lại. Cần kiểm thử đầy đủ các quy tắc, giây khác 00 trên trình duyệt và hỗ trợ chuỗi kết thúc xa.
- [ ] 8. Sửa hoặc hủy một buổi trong chuỗi.
- [ ] Checkpoint D: kiểm thử chuỗi, ngoại lệ và xung đột.
- [ ] 9. Sửa hoặc hủy từ buổi này hoặc toàn chuỗi.
- [ ] 10. Lịch lặp vô hạn và cảnh báo xung đột định kỳ.
- [ ] Checkpoint E: kiểm thử cảnh báo khi cửa sổ dịch chuyển.
- [ ] 11. Giao diện tiếng Việt và tiếng Anh. Đã thêm ngôn ngữ theo thiết bị và tài khoản, dịch giao diện quản trị và sửa thông báo đang hiện theo ngôn ngữ được chọn; test API và migration đạt. Chromium trên DB tạm đã kiểm tra đăng nhập, thông báo tạo tài khoản và lỗi tên phòng trùng đổi ngôn ngữ tức thời, nhãn chính của phòng/lịch/thiết bị, cùng lựa chọn qua tải lại; kiosk ghép bằng giao diện, đổi Việt sang Anh và báo thu hồi bằng tiếng Anh. Còn kiểm thử đầy đủ các trạng thái lỗi, lỗi tải lại và kiosk trên thiết bị thực.
- [ ] 12. Sao lưu, khôi phục thử và quy trình vận hành. Đã thêm lệnh tạo snapshot SQLite, test khôi phục DB tạm khi có ghi chưa commit và đọc/tạo lịch qua API trên bản sao mà không đổi DB nguồn; đã kiểm thử snapshot trước/sau commit ở rollback journal và WAL, lỗi khóa độc quyền không công bố file; chưa kiểm thử commit trùng lúc snapshot đang đọc, đã diễn tập trên DB tạm với lịch đơn và chuỗi lặp, kiểm tra integrity, API và ghi riêng trên bản khôi phục; còn diễn tập nghiệp vụ đầy đủ và chốt chính sách vận hành. Tạm yêu cầu dừng ghi trước khi sao lưu.
- [ ] Checkpoint phát hành: toàn bộ gate và luồng đầu cuối.
