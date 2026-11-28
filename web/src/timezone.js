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

const DATE_RE = /^(\d{4})-(\d{2})-(\d{2})$/;
const TIME_RE = /^(\d{2}):(\d{2})$/;
const pad2 = n => String(n).padStart(2, '0');

export function addMinutesLocal(dateStr, timeStr, minutes) {
  const dm = DATE_RE.exec(dateStr);
  const tm = TIME_RE.exec(timeStr);
  if (!dm || !tm || !Number.isFinite(minutes)) return null;
  const dt = new Date(Date.UTC(+dm[1], +dm[2] - 1, +dm[3], +tm[1], +tm[2] + Math.round(minutes)));
  if (Number.isNaN(dt.getTime())) return null;
  return {
    date: `${String(dt.getUTCFullYear()).padStart(4, '0')}-${pad2(dt.getUTCMonth() + 1)}-${pad2(dt.getUTCDate())}`,
    time: `${pad2(dt.getUTCHours())}:${pad2(dt.getUTCMinutes())}`,
  };
}

export function diffMinutesLocal(startDate, startTime, endDate, endTime) {
  const sd = DATE_RE.exec(startDate);
  const st = TIME_RE.exec(startTime);
  const ed = DATE_RE.exec(endDate);
  const et = TIME_RE.exec(endTime);
  if (!sd || !st || !ed || !et) return null;
  const a = Date.UTC(+sd[1], +sd[2] - 1, +sd[3], +st[1], +st[2]);
  const b = Date.UTC(+ed[1], +ed[2] - 1, +ed[3], +et[1], +et[2]);
  return Math.round((b - a) / 60000);
}

export function defaultBookingRange(zone, nowEpochSec = Math.floor(Date.now() / 1000), durationMin = 30) {
  if (!zone) return { startDate: '', startTime: '', endDate: '', endTime: '', multiDay: false };
  const [date, time] = localTime(nowEpochSec, zone).split('T');
  const mm = Number(time.slice(3, 5));
  const step = 15 - (mm % 15);
  const start = addMinutesLocal(date, time, step);
  const end = addMinutesLocal(start.date, start.time, durationMin);
  return {
    startDate: start.date,
    startTime: start.time,
    endDate: end.date,
    endTime: end.time,
    multiDay: end.date !== start.date,
  };
}

export function formatDuration(minutes, language = 'vi') {
  if (!Number.isFinite(minutes) || minutes <= 0) return '';
  const total = Math.round(minutes);
  const d = Math.floor(total / 1440);
  const h = Math.floor((total % 1440) / 60);
  const m = total % 60;
  const parts = [];
  if (language === 'en') {
    if (d > 0) parts.push(`${d}d`);
    if (h > 0) parts.push(`${h} hr`);
    if (m > 0) parts.push(`${m} min`);
  } else {
    if (d > 0) parts.push(`${d} ngày`);
    if (h > 0) parts.push(`${h} giờ`);
    if (m > 0) parts.push(`${m} phút`);
  }
  return parts.join(' ');
}
