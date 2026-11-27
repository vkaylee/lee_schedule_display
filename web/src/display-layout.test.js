import { strict as assert } from 'node:assert';
import { defaultDisplayLayout, gridCell, validDisplayLayout } from './display-layout.js';

assert.equal(gridCell(126, 100, 262, 10, 10, 2, 12), 0);
assert.equal(gridCell(132, 100, 262, 10, 10, 2, 12), 1);
assert.equal(gridCell(342, 100, 262, 10, 10, 2, 12), 11);
assert.equal(gridCell(109, 100, 262, 10, 10, 2, 12), -1);
assert.equal(validDisplayLayout(defaultDisplayLayout), true);
assert.equal(validDisplayLayout([]), true);
assert.equal(validDisplayLayout([...defaultDisplayLayout, defaultDisplayLayout[0]]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 11, y: 0, w: 2, h: 1 }]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 0, y: 0, w: 2, h: 1 }, { type: 'clock', x: 1, y: 0, w: 2, h: 1 }]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 0, y: 0, w: 1, h: 25 }]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 0.5, y: 0, w: 1, h: 1 }]), false);
assert.equal(validDisplayLayout([{ type: 'unknown', x: 0, y: 0, w: 1, h: 1 }]), false);
