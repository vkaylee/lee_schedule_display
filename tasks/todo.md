# Checklist triển khai MVP

Chi tiết, điều kiện nghiệm thu, kiểm thử và phụ thuộc: xem [plan.md](plan.md). Tác vụ 1 chưa hoàn tất: rủi ro giới hạn đăng nhập và kiểm thử trên hạ tầng proxy thực còn chặn triển khai; bằng chứng và đề xuất tại [auth-threat-model.md](auth-threat-model.md).

- [ ] 1. Bootstrap tài khoản đầu tiên và đăng nhập an toàn. Tạm bỏ qua theo yêu cầu; các rủi ro và kiểm thử proxy thực vẫn chưa được giải quyết, không coi là sẵn sàng triển khai.
- [x] 2. Quản lý tài khoản quản trị. Đã kiểm thử API, giao diện, giao dịch cạnh tranh và rà soát bảo mật; checkpoint A vẫn phụ thuộc proxy triển khai thực.
- [ ] Checkpoint A: HTTPS, tài khoản và kiểm tra truy cập.
- [x] 3. Quản lý phòng và cấu hình riêng tư. Đã kiểm thử API, trình duyệt và lưu lịch khi phòng bị vô hiệu hóa; đã xử lý các điểm rà soát phòng.
- [ ] 4. Đặt lịch một lần, kiểm tra trùng nguyên tử. Đã có API, giao diện múi giờ chung, test cạnh tranh, DST và trình duyệt; đã bổ sung kiểm soát phiên bản và thử lại trình duyệt. Còn kiểm thử hạ tầng thực tại checkpoint B.
- [ ] Checkpoint B: kiểm thử đặt trùng đồng thời và lưu bền vững.
- [ ] 5. Ghép thiết bị và hiển thị lịch trên kiosk. Đã có đăng ký, ghép, thu hồi, quyền đọc theo phòng; đã rà soát bảo mật và sửa các vấn đề phát hiện. Còn kiểm thử kiosk thực và trạng thái biên.
- [ ] 6. Xử lý mất kết nối và dữ liệu quá 15 phút. Đã kiểm thử Chrome chuyển Không xác định khi mất kết nối và phục hồi; còn kiểm thử đồng hồ/tab ngủ và lưu lịch sau khởi động lại. Cách xử lý hiện tại ưu tiên an toàn hơn hiển thị offline theo kế hoạch.
- [ ] Checkpoint C: kiểm thử kiosk, thu hồi và riêng tư.
- [ ] 7. Tạo và xem lịch lặp có giới hạn.
- [ ] 8. Sửa hoặc hủy một buổi trong chuỗi.
- [ ] Checkpoint D: kiểm thử chuỗi, ngoại lệ và xung đột.
- [ ] 9. Sửa hoặc hủy từ buổi này hoặc toàn chuỗi.
- [ ] 10. Lịch lặp vô hạn và cảnh báo xung đột định kỳ.
- [ ] Checkpoint E: kiểm thử cảnh báo khi cửa sổ dịch chuyển.
- [ ] 11. Giao diện tiếng Việt và tiếng Anh.
- [ ] 12. Sao lưu, khôi phục thử và quy trình vận hành.
- [ ] Checkpoint phát hành: toàn bộ gate và luồng đầu cuối.
