import { strict as assert } from 'node:assert';
import { defaultDisplayLayout, gridCell, resizeEdges, validDisplayLayout } from './display-layout.js';

const rectangle = { x:2, y:2, w:4, h:3 };
assert.deepEqual(resizeEdges(rectangle, 'nw', -2, -1, 7), { x:0, y:1, w:6, h:4 });
assert.deepEqual(resizeEdges(rectangle, 'se', 20, 20, 7), { x:2, y:2, w:10, h:5 });
assert.deepEqual(resizeEdges(rectangle, 'w', 20, 0, 7), { x:5, y:2, w:1, h:3 });

assert.equal(gridCell(126, 100, 262, 10, 10, 2, 12), 0);
assert.equal(gridCell(132, 100, 262, 10, 10, 2, 12), 1);
assert.equal(gridCell(342, 100, 262, 10, 10, 2, 12), 11);
assert.equal(gridCell(109, 100, 262, 10, 10, 2, 12), -1);
assert.equal(validDisplayLayout(defaultDisplayLayout), true);
assert.equal(validDisplayLayout([]), true);
assert.equal(validDisplayLayout([...defaultDisplayLayout, defaultDisplayLayout[0]]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 11, y: 0, w: 2, h: 1 }]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 0, y: 0, w: 2, h: 1 }, { type: 'clock', x: 1, y: 0, w: 2, h: 1 }]), true);
assert.equal(validDisplayLayout([{ type: 'text', id: 'layer_t1', x: 0, y: 0, w: 2, h: 1 }, { type: 'text', id: 'layer_t2', x: 0, y: 0, w: 2, h: 1 }]), true);
assert.equal(validDisplayLayout([{ type: 'text', id: 'layer_t1', x: 0, y: 0, w: 2, h: 1 }, { type: 'text', id: 'layer_t1', x: 0, y: 0, w: 2, h: 1 }]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 0, y: 0, w: 1, h: 25 }]), false);
assert.equal(validDisplayLayout([{ type: 'room', x: 0.5, y: 0, w: 1, h: 1 }]), false);
assert.equal(validDisplayLayout([{ type: 'unknown', x: 0, y: 0, w: 1, h: 1 }]), false);
