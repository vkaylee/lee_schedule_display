export function displaySchedule(entries, now, validUntil = null) {
  const sorted = [...entries].sort((a, b) => a.starts - b.starts);
  let current = sorted.find(entry => entry.starts <= now && entry.ends > now);
  if (current) {
    current = { ...current };
    let busyUntil = current.ends;
    // ponytail: merge overlapping conflicts for display end, and track contiguous busy horizon for valid_until.
    for (const entry of sorted) {
      if (entry.starts > busyUntil) break;
      if (entry.starts < current.ends && entry.ends > current.ends) current.ends = entry.ends;
      if (entry.ends > busyUntil) busyUntil = entry.ends;
    }
    if (validUntil != null && busyUntil >= validUntil) current = undefined;
  }
  const upcoming = sorted.filter(entry => entry.starts > now).slice(0, 3);
  return { current, upcoming };
}

export function formatDuration(seconds, language = 'vi') {
  if (!Number.isFinite(seconds) || seconds <= 0) return '';
  if (language === 'zh-CN') {
    if (seconds < 60) return '不到 1 分钟';
    const total = Math.round(seconds / 60);
    const days = Math.floor(total / 1440), hours = Math.floor(total % 1440 / 60), minutes = total % 60;
    return [days && `${days} 天`, hours && `${hours} 小时`, minutes && `${minutes} 分钟`].filter(Boolean).join(' ');
  }
  if (seconds < 60) return language === 'en' ? '< 1 min' : 'Dưới 1 phút';
  const totalMinutes = Math.round(seconds / 60);
  const days = Math.floor(totalMinutes / 1440);
  const hours = Math.floor((totalMinutes % 1440) / 60);
  const minutes = totalMinutes % 60;
  if (days) return language === 'en' ? `${days}d${hours ? ` ${hours}h` : ''}${minutes ? ` ${minutes}m` : ''}` : `${days} ngày${hours ? ` ${hours} giờ` : ''}${minutes ? ` ${minutes} phút` : ''}`;
  if (hours === 0) return language === 'en' ? `${totalMinutes} min` : `${totalMinutes} phút`;
  if (minutes === 0) return language === 'en' ? `${hours}h` : `${hours} giờ`;
  return language === 'en' ? `${hours}h ${minutes}m` : `${hours}g ${minutes}p`;
}

export function formatScheduleDateTime(seconds, now, zone = 'UTC', locale = 'vi-VN') {
  const date = new Date(seconds * 1000);
  const current = new Date(now * 1000);
  const dateKey = value => new Intl.DateTimeFormat('en-CA', { timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(value);
  const time = new Intl.DateTimeFormat(locale, { timeZone: zone, hour: '2-digit', minute: '2-digit' }).format(date);
  return dateKey(date) === dateKey(current) ? time : `${new Intl.DateTimeFormat(locale, { timeZone: zone, day: 'numeric', month: 'numeric' }).format(date)} ${time}`;
}

export function meetingProgress(current, now) {
  if (!current || now == null || !Number.isFinite(now) || current.ends <= current.starts) return 0;
  return Math.max(0, Math.min(1, (now - current.starts) / (current.ends - current.starts)));
}
