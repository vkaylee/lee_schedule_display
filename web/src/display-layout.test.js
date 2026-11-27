import { strict as assert } from 'node:assert';
import { defaultDisplayLayout, validDisplayLayout } from './display-layout.js';

assert.equal(validDisplayLayout(defaultDisplayLayout), true);
assert.equal(validDisplayLayout([]), true);
assert.equal(validDisplayLayout([...defaultDisplayLayout, defaultDisplayLayout[0]]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 11, y: 0, w: 2, h: 1 }]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 0, y: 0, w: 2, h: 1 }, { type: 'clock', x: 1, y: 0, w: 2, h: 1 }]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 0, y: 0, w: 1, h: 25 }]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 0.5, y: 0, w: 1, h: 1 }]), false);
assert.equal(validDisplayLayout([{ type: 'unknown', x: 0, y: 0, w: 1, h: 1 }]), false);
