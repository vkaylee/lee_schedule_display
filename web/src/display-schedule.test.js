import { strict as assert } from 'node:assert';
import { displaySchedule } from './display-schedule.js';

const entries = [{ starts: 100, ends: 200 }, { starts: 300, ends: 400 }, { starts: 500, ends: 600 }, { starts: 700, ends: 800 }, { starts: 900, ends: 1000 }];
assert.deepEqual(displaySchedule(entries, 150), { current: entries[0], upcoming: entries.slice(1, 4) });
assert.deepEqual(displaySchedule(entries, 200), { current: undefined, upcoming: entries.slice(1, 4) });
assert.deepEqual(displaySchedule(entries, 300), { current: entries[1], upcoming: entries.slice(2) });
assert.deepEqual(displaySchedule([], 300), { current: undefined, upcoming: [] });
