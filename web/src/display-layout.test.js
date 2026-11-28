import { strict as assert } from 'node:assert';
import { readFileSync } from 'node:fs';
import { blockKey, canvasScale, contrastRatio, defaultDisplayLayout, gridCell, hasInvalidRoomAssignment, moveBlock, resizeEdges, validDisplayLayout } from './display-layout.js';

assert.equal(canvasScale(1080, 1920, 810, 700, 'width'), 0.75);
assert.equal(canvasScale(1080, 1920, 810, 700, 'fit'), 700 / 1920);
assert.equal(canvasScale(1080, 1920, 810, 700, 'width', 1.5), 1.125);
assert.equal(canvasScale(1080, 1920, 0, 700, 'width'), 1);

const rectangle = { x:2, y:2, w:4, h:3 };
assert.deepEqual(moveBlock(rectangle, 2, -1, 7), { x:4, y:1 });
assert.deepEqual(moveBlock(rectangle, 99, 99, 7), { x:8, y:4 });
assert.deepEqual(moveBlock(rectangle, -99, -99, 7), { x:0, y:0 });
assert.deepEqual(moveBlock(rectangle, 99, 99, 7), { x:8, y:4 });
assert.deepEqual(moveBlock(rectangle, -99, -99, 7), { x:0, y:0 });
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
assert.equal(validDisplayLayout([{ id: 'room', type: 'room', x: 11, y: 0, w: 2, h: 1 }]), false);
assert.equal(validDisplayLayout([{ id: 'room', type: 'room', x: 0, y: 0, w: 2, h: 1 }, { type: 'clock', x: 1, y: 0, w: 2, h: 1 }]), true);
assert.equal(validDisplayLayout([{ type: 'text', id: 'layer_t1', x: 0, y: 0, w: 2, h: 1, fontSize: 48, align: 'center', padding: 12, margin: 8 }]), true);
assert.equal(validDisplayLayout([{ type: 'text', id: 'layer_t1', x: 0, y: 0, w: 2, h: 1, fontSize: 257 }]), false);
assert.equal(validDisplayLayout([{ type: 'text', id: 'layer_t1', x: 0, y: 0, w: 2, h: 1, align: 'justify' }]), false);
assert.equal(validDisplayLayout([{ type: 'text', id: 'layer_t1', x: 0, y: 0, w: 2, h: 1, padding: -1 }]), false);
assert.equal(validDisplayLayout([{ type: 'text', id: 'layer_t1', x: 0, y: 0, w: 2, h: 1, margin: 129 }]), false);
assert.equal(validDisplayLayout([{ type: 'text', id: 'layer_t1', x: 0, y: 0, w: 2, h: 1 }, { type: 'text', id: 'layer_t2', x: 0, y: 0, w: 2, h: 1 }]), true);
assert.equal(validDisplayLayout([{ type: 'text', id: 'layer_t1', x: 0, y: 0, w: 2, h: 1 }, { type: 'text', id: 'layer_t1', x: 0, y: 0, w: 2, h: 1 }]), false);
assert.equal(validDisplayLayout([{ id: 'room', type: 'room', x: 0, y: 0, w: 1, h: 25 }]), false);
assert.equal(validDisplayLayout([{ id: 'room', type: 'room', x: 0.5, y: 0, w: 1, h: 1 }]), false);
assert.equal(validDisplayLayout([{ type: 'unknown', x: 0, y: 0, w: 1, h: 1 }]), false);
assert.equal(validDisplayLayout([{ id: 'status', type: 'status', x: 0, y: 0, w: 6, h: 4, bgColor: '#0f172a', textColor: '#f8fafc', accentColor: '#22c55e', fontScale: 115, letterSpacing: 3, padding: 18, align: 'center', valign: 'center', subLayout: 'reverse', hideParts: ['pill', 'title', 'organizer', 'progress'] }]), true);
assert.equal(validDisplayLayout([{ id: 'upcoming', type: 'upcoming', x: 0, y: 0, w: 6, h: 4, hideParts: ['count', 'title', 'organizer', 'relative', 'duration'] }]), true);
assert.equal(validDisplayLayout([{ id: 'status', type: 'status', x: 0, y: 0, w: 6, h: 4, bgColor: 'invalid' }]), false);
assert.equal(validDisplayLayout([{ id: 'status', type: 'status', x: 0, y: 0, w: 6, h: 4, fontScale: 250 }]), true);
assert.equal(validDisplayLayout([{ id: 'status', type: 'status', x: 0, y: 0, w: 6, h: 4, fontScale: 251 }]), false);
assert.equal(validDisplayLayout([{ id: 'status', type: 'status', x: 0, y: 0, w: 6, h: 4, fontScale: 50 }]), false);
assert.equal(validDisplayLayout([{ id: 'status', type: 'status', x: 0, y: 0, w: 6, h: 4, letterSpacing: 25 }]), false);
assert.equal(validDisplayLayout([{ id: 'status', type: 'status', x: 0, y: 0, w: 6, h: 4, subLayout: 'diagonal' }]), false);
assert.equal(validDisplayLayout([{ type: 'clock', x: 0, y: 0, w: 3, h: 1, hideParts: ['progress'] }]), false);
assert.equal(validDisplayLayout([{ type: 'clock', x: 0, y: 0, w: 3, h: 1, hideParts: ['zone', 'zone'] }]), false);
const roomA = { id: 'room_a', type: 'room', roomId: 1, x: 0, y: 0, w: 2, h: 1 };
const roomB = { ...roomA, id: 'room_b', roomId: 2 };
assert.equal(blockKey({ type: 'clock' }), 'clock');
assert.equal(blockKey(roomA), 'room_a');
const rooms = [{ id: 1, active: false }, { id: 2, active: true }];
assert.equal(hasInvalidRoomAssignment([roomA], rooms, [roomA]), false);
assert.equal(hasInvalidRoomAssignment([{ ...roomA, x: 4 }], rooms, [roomA]), false);
assert.equal(hasInvalidRoomAssignment([{ ...roomA, id: 'new' }], rooms, [roomA]), true);
assert.equal(hasInvalidRoomAssignment([{ ...roomA, type: 'status' }], rooms, [roomA]), true);
assert.equal(hasInvalidRoomAssignment([roomA], rooms, [roomB]), true);
assert.equal(hasInvalidRoomAssignment([roomB], rooms, []), false);
assert.equal(hasInvalidRoomAssignment([{ ...roomA, roomId: undefined }], rooms, []), false);
assert.equal(validDisplayLayout([roomA, roomB]), true);
assert.equal(validDisplayLayout([roomA, { ...roomB, id: roomA.id }]), false);
assert.equal(validDisplayLayout([{ ...roomA, roomId: '1' }]), false);
assert.equal(validDisplayLayout([{ ...roomA, roomId: -1 }]), false);
assert.equal(validDisplayLayout([{ type: 'clock', roomId: 1, x: 0, y: 0, w: 1, h: 1 }]), false);
assert.equal(validDisplayLayout([{ ...roomA, id: '' }]), false);
assert.equal(contrastRatio('#000000', '#ffffff') > 20, true);
assert.equal(contrastRatio('#777777', '#888888') < 2, true);
assert.equal(contrastRatio('bad', '#ffffff'), null);

// ponytail: checks explicit widget typography rules; use browser tests for computed layout and clipping.
const canvas = readFileSync(new URL('./DisplayCanvas.svelte', import.meta.url), 'utf8');
for (const selector of ['clock-zone', 'widget-eyebrow', 'room-eyebrow', 'status-pill', 'status-remaining', 'status-note', 'kicker', 'upcoming-count', 'upcoming-sep', 'upcoming-relative', 'upcoming-duration', 'upcoming-empty', 'footer-row', 'footer-badge', 'footer-zone']) {
  assert.match(canvas, new RegExp(`\\.${selector}\\s*\\{[^}]*font-size:\\s*calc\\([^;]*var\\(--w-font-scale, 1\\)`), `${selector} must scale`);
}
