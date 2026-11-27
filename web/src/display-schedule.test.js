import { strict as assert } from 'node:assert';
import { displaySchedule, formatDuration, meetingProgress } from './display-schedule.js';

const entries = [{ starts: 100, ends: 200 }, { starts: 300, ends: 400 }, { starts: 500, ends: 600 }, { starts: 700, ends: 800 }, { starts: 900, ends: 1000 }];
assert.deepEqual(displaySchedule(entries, 150), { current: entries[0], upcoming: entries.slice(1, 4) });
assert.deepEqual(displaySchedule(entries, 200), { current: undefined, upcoming: entries.slice(1, 4) });
assert.deepEqual(displaySchedule(entries, 300), { current: entries[1], upcoming: entries.slice(2) });
assert.deepEqual(displaySchedule([], 300), { current: undefined, upcoming: [] });
const overlapping = [{ starts: 100, ends: 200 }, { starts: 150, ends: 250 }];
assert.equal(displaySchedule(overlapping, 175).current.ends, 250);
assert.equal(displaySchedule([{ starts: 250, ends: 300 }, ...overlapping], 175).current.ends, 300);
assert.equal(displaySchedule([{ starts: 251, ends: 300 }, ...overlapping], 175).current.ends, 250);
assert.equal(overlapping[0].ends, 200);
assert.equal(displaySchedule([{ starts: 100, ends: 1000 }], 150, 300).current, undefined);
assert.equal(displaySchedule([{ starts: 100, ends: 300 }], 150, 300).current, undefined);
assert.equal(displaySchedule([{ starts: 100, ends: 299 }], 150, 300).current.ends, 299);
assert.deepEqual(displaySchedule([{ starts: 500, ends: 600 }, { starts: 300, ends: 400 }], 150).upcoming.map(x => x.starts), [300, 500]);

assert.equal(formatDuration(0), '');
assert.equal(formatDuration(-10), '');
assert.equal(formatDuration(45), 'Dưới 1 phút');
assert.equal(formatDuration(45, 'en'), '< 1 min');
assert.equal(formatDuration(1800), '30 phút');
assert.equal(formatDuration(1800, 'en'), '30 min');
assert.equal(formatDuration(3600), '1 giờ');
assert.equal(formatDuration(3600, 'en'), '1h');
assert.equal(formatDuration(5400), '1g 30p');
assert.equal(formatDuration(5400, 'en'), '1h 30m');

assert.equal(meetingProgress(null, 150), 0);
assert.equal(meetingProgress({ starts: 100, ends: 100 }, 100), 0);
assert.equal(meetingProgress({ starts: 100, ends: 200 }, 50), 0);
assert.equal(meetingProgress({ starts: 100, ends: 200 }, 150), 0.5);
assert.equal(meetingProgress({ starts: 100, ends: 200 }, 250), 1);
