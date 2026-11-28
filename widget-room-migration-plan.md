# Chuyển phòng từ màn hình sang widget

Status: Approved — ứng dụng chưa go live; dùng DB mới, không hỗ trợ migration schema cũ.

- Màn hình không gắn phòng; widget `room`, `status`, `upcoming` chọn phòng độc lập và có thể lặp lại.
- Màn hình công khai chỉ trả tên phòng và thời gian, không trả tiêu đề hay người tổ chức. Trạng thái lịch được tính riêng từng phòng.
- Gỡ kiosk token, API kiosk và các trường thiết bị tương ứng.
- Khởi tạo schema phiên bản 19 trực tiếp. DB cũ bị từ chối, không tự xóa hoặc ghi đè; cần chọn DB mới và cấu hình lại dữ liệu.
- Kiểm tra schema mới, từ chối DB cũ không làm thay đổi dữ liệu, quyền riêng tư và toàn bộ regression. Kiểm tra editor và màn hình công khai bằng dữ liệu thử.
- Không thực hiện deploy hoặc xóa DB đang dùng. Nếu cần phục hồi dữ liệu cũ, giữ bản sao và dùng phiên bản cũ tương ứng trong môi trường riêng.
