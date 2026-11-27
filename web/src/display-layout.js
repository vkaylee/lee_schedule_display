export const displayBlocks = ['clock', 'branding', 'room', 'status', 'upcoming', 'footer'];

export const defaultDisplayLayout = [
  { type: 'clock', x: 0, y: 0, w: 3, h: 1 },
  { type: 'branding', x: 3, y: 0, w: 9, h: 1 },
  { type: 'room', x: 0, y: 1, w: 12, h: 1 },
  { type: 'status', x: 0, y: 2, w: 7, h: 4 },
  { type: 'upcoming', x: 7, y: 2, w: 5, h: 4 },
  { type: 'footer', x: 0, y: 6, w: 12, h: 1 }
];

export function gridCell(position, start, size, paddingStart, paddingEnd, gap, count) {
  const cell = (size - paddingStart - paddingEnd - gap * (count - 1)) / count;
  return Math.floor((position - start - paddingStart) / (cell + gap));
}

export function resizeEdges(block, edges, dx, dy, rows) {
  const left = edges.includes('w') ? Math.max(0, Math.min(block.x + block.w - 1, block.x + dx)) : block.x;
  const right = edges.includes('e') ? Math.min(12, Math.max(block.x + 1, block.x + block.w + dx)) : block.x + block.w;
  const top = edges.includes('n') ? Math.max(0, Math.min(block.y + block.h - 1, block.y + dy)) : block.y;
  const bottom = edges.includes('s') ? Math.min(rows, Math.max(block.y + 1, block.y + block.h + dy)) : block.y + block.h;
  return { x:left, y:top, w:right-left, h:bottom-top };
}

export function validDisplayLayout(blocks) {
  if (!Array.isArray(blocks) || blocks.length > 32) return false;
  const seen = new Set();
  for (const block of blocks) {
    const custom = block?.type === 'text' || block?.type === 'rectangle';
    const key = block?.id || block?.type;
    if (!block || (!custom && !displayBlocks.includes(block.type)) || seen.has(key) ||
        (!custom && block.id) || (custom && (!block.id || !/^layer_[a-zA-Z0-9_-]{1,58}$/.test(block.id))) ||
        ![block.x, block.y, block.w, block.h].every(Number.isSafeInteger) ||
        block.x < 0 || block.y < 0 || block.w < 1 || block.h < 1 ||
        block.x + block.w > 12 || block.y + block.h > 24) return false;
    seen.add(key);
  }
  return true;
}
