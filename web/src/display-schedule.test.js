import { strict as assert } from 'node:assert';
import { displaySchedule } from './display-schedule.js';

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
