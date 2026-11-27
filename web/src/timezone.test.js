import { strict as assert } from 'node:assert';
import { instant, localTime } from './timezone.js';
assert.equal(instant('2024-03-10T02:30', 'America/New_York'), null);
assert.equal(instant('2024-11-03T01:30', 'America/New_York'), null);
assert.equal(instant('2024-03-10T03:30', 'America/New_York'), Date.parse('2024-03-10T07:30:00Z') / 1000);
assert.equal(localTime(Date.parse('2024-01-01T00:00:00Z') / 1000, 'Asia/Ho_Chi_Minh'), '2024-01-01T07:00');
