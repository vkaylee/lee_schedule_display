export function displaySchedule(entries, now) {
  const current = entries.find(entry => entry.starts <= now && entry.ends > now);
  const upcoming = entries.filter(entry => entry.starts > now).slice(0, 3);
  return { current, upcoming };
}
