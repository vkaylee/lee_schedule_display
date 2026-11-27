export function displaySchedule(entries, now, validUntil = null) {
  const sorted = [...entries].sort((a, b) => a.starts - b.starts);
  let current = sorted.find(entry => entry.starts <= now && entry.ends > now);
  if (current) {
    current = { ...current };
    // ponytail: merge known busy intervals; a truncated schedule still requires valid_until.
    for (const entry of sorted) {
      if (entry.starts > current.ends) break;
      if (entry.ends > current.ends) current.ends = entry.ends;
    }
    if (validUntil != null && current.ends >= validUntil) current = undefined;
  }
  const upcoming = sorted.filter(entry => entry.starts > now).slice(0, 3);
  return { current, upcoming };
}

export function formatDuration(seconds, language = 'vi') {
  if (!Number.isFinite(seconds) || seconds <= 0) return '';
  if (seconds < 60) return language === 'en' ? '< 1 min' : 'Dưới 1 phút';
  const totalMinutes = Math.round(seconds / 60);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  if (hours === 0) return language === 'en' ? `${totalMinutes} min` : `${totalMinutes} phút`;
  if (minutes === 0) return language === 'en' ? `${hours}h` : `${hours} giờ`;
  return language === 'en' ? `${hours}h ${minutes}m` : `${hours}g ${minutes}p`;
}

export function meetingProgress(current, now) {
  if (!current || now == null || !Number.isFinite(now) || current.ends <= current.starts) return 0;
  return Math.max(0, Math.min(1, (now - current.starts) / (current.ends - current.starts)));
}
