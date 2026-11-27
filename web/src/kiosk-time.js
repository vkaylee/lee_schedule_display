export function statusAt(data, monotonic, wall, serverNow, uncertainty = 0) {
  if (!data || monotonic < 0 || wall < 0) return 'Không xác định';
  const elapsed = Math.max(monotonic, wall);
  if (elapsed >= 900000) return 'Không xác định';
  const now = serverNow + elapsed / 1000;
  if (data.valid_until != null && now >= data.valid_until) return 'Không xác định';
  const earliest = now - uncertainty / 1000 - 1;
  if (data.entries.some(e => e.starts <= earliest && now < e.ends)) return 'Đang họp';
  if (data.entries.some(e => e.starts <= now && e.ends > earliest)) return 'Không xác định';
  return 'Trống';
}
