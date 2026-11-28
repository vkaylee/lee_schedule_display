import { strict as assert } from 'node:assert';
import { instant, localTime, localTimeSeconds, addMinutesLocal, diffMinutesLocal, defaultBookingRange, formatDuration } from './timezone.js';
assert.equal(instant('2024-03-10T02:30', 'America/New_York'), null);
assert.equal(instant('2024-11-03T01:30', 'America/New_York'), null);
assert.equal(instant('2024-03-10T03:30', 'America/New_York'), Date.parse('2024-03-10T07:30:00Z') / 1000);
assert.equal(localTime(Date.parse('2024-01-01T00:00:00Z') / 1000, 'Asia/Ho_Chi_Minh'), '2024-01-01T07:00');
assert.equal(instant('2024-01-01T12:00', ''), null);
assert.equal(localTimeSeconds(1893488459, 'Asia/Ho_Chi_Minh'), '2030-01-01T16:00:59');
assert.equal(localTimeSeconds(1893488400, 'Asia/Ho_Chi_Minh'), '2030-01-01T16:00:00');
assert.equal(instant('2030-01-01T16:00:59', 'Asia/Ho_Chi_Minh'), 1893488459);
assert.equal(instant('2024-03-10T02:30:42', 'America/New_York'), null);
assert.equal(instant('2024-11-03T01:30:42', 'America/New_York'), null);

assert.deepEqual(addMinutesLocal('2026-11-28', '09:15', 45), { date: '2026-11-28', time: '10:00' });
assert.deepEqual(addMinutesLocal('2026-11-28', '23:45', 30), { date: '2026-11-29', time: '00:15' });
assert.equal(diffMinutesLocal('2026-11-28', '09:00', '2026-11-28', '10:30'), 90);
assert.equal(diffMinutesLocal('2026-11-28', '14:00', '2026-11-28', '13:30'), -30);
assert.equal(diffMinutesLocal('2026-11-28', '23:00', '2026-11-29', '01:00'), 120);
assert.deepEqual(defaultBookingRange('Asia/Ho_Chi_Minh', Date.parse('2026-11-28T02:07:00Z') / 1000, 30), {
  startDate: '2026-11-28',
  startTime: '09:15',
  endDate: '2026-11-28',
  endTime: '09:45',
  multiDay: false,
});
assert.deepEqual(defaultBookingRange('Asia/Ho_Chi_Minh', Date.parse('2026-11-28T16:50:00Z') / 1000, 30), {
  startDate: '2026-11-29',
  startTime: '00:00',
  endDate: '2026-11-29',
  endTime: '00:30',
  multiDay: false,
});
assert.deepEqual(defaultBookingRange('Asia/Ho_Chi_Minh', Date.parse('2026-11-28T16:40:00Z') / 1000, 30), {
  startDate: '2026-11-28',
  startTime: '23:45',
  endDate: '2026-11-29',
  endTime: '00:15',
  multiDay: true,
});
assert.equal(formatDuration(45, 'vi'), '45 phút');
assert.equal(formatDuration(90, 'vi'), '1 giờ 30 phút');
assert.equal(formatDuration(90, 'en'), '1 hr 30 min');
assert.equal(formatDuration(1530, 'zh-CN'), '1 天 1 小时 30 分钟');
assert.equal(formatDuration(0, 'vi'), '');
