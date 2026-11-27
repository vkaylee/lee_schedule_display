export function statusAt(data, monotonic, wall, serverNow, uncertainty = 0, refreshDelay = 0) {
  // ponytail: tolerate one second of clock sampling drift; resync after larger jumps or sleep.
  if (!data || !Array.isArray(data.entries) || data.entries.some(e => !e || !Number.isSafeInteger(e.starts) || !Number.isSafeInteger(e.ends) || e.starts < 0 || e.ends > 253402300799 || e.ends <= e.starts) || (data.valid_until != null && !Number.isFinite(data.valid_until)) || !Number.isSafeInteger(serverNow) || serverNow < 0 || serverNow > 253402300799 || ![monotonic, wall, uncertainty, refreshDelay].every(Number.isFinite) || monotonic < 0 || wall < 0 || uncertainty < 0 || refreshDelay < 0 || Math.abs(monotonic - wall) > 1000) return 'Không xác định';
  const elapsed = Math.max(monotonic, wall);
  if (elapsed + uncertainty + refreshDelay >= 900000) return 'Không xác định';
  const now = serverNow + (elapsed + uncertainty + refreshDelay) / 1000 + 1;
  if (data.valid_until != null && now >= data.valid_until) return 'Không xác định';
  const earliest = serverNow + Math.min(monotonic, wall) / 1000 - 1;
  if (data.entries.some(e => e.starts <= earliest && now < e.ends)) return 'Đang họp';
  if (data.entries.some(e => e.starts <= now && e.ends > earliest)) return 'Không xác định';
  return 'Trống';
}
