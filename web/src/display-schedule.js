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
