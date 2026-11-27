const formatters = new Map();
function formatter(zone) {
  if (!formatters.has(zone)) formatters.set(zone, new Intl.DateTimeFormat('en-CA', { timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23' }));
  return formatters.get(zone);
}
export function localTime(epoch, zone) {
  const parts = Object.fromEntries(formatter(zone).formatToParts(new Date(epoch * 1000)).map(({type,value}) => [type,value]));
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}`;
}
export function localTimeSeconds(epoch, zone) {
  const parts = Object.fromEntries(formatter(zone).formatToParts(new Date(epoch * 1000)).map(({type,value}) => [type,value]));
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}:${parts.second}`;
}
export function instant(local, zone) {
  if (!zone || !/^\d{4}-\d\d-\d\dT\d\d:\d\d(?::\d\d)?$/.test(local)) return null;
  const normalized = local.length === 16 ? local + ':00' : local;
  const nominal = Date.parse(normalized + 'Z');
  if (!Number.isFinite(nominal)) return null;
  const offsets = new Set();
  for (let h = -48; h <= 48; h += 6) {
    const sample = nominal + h * 3600000;
    offsets.add(Date.parse(localTimeSeconds(sample / 1000, zone) + 'Z') - sample);
  }
  const matches = [...offsets].map(offset => (nominal - offset) / 1000).filter(epoch => localTimeSeconds(epoch, zone) === normalized);
  return matches.length === 1 ? matches[0] : null;
}
