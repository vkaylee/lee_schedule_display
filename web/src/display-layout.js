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

export function validDisplayLayout(blocks) {
  if (!Array.isArray(blocks) || blocks.length > displayBlocks.length) return false;
  const seen = new Set();
  for (const block of blocks) {
    if (!block || !displayBlocks.includes(block.type) || seen.has(block.type) ||
        ![block.x, block.y, block.w, block.h].every(Number.isSafeInteger) ||
        block.x < 0 || block.y < 0 || block.w < 1 || block.h < 1 ||
        block.x + block.w > 12 || block.y + block.h > 24) return false;
    seen.add(block.type);
  }
  return blocks.every((a, i) => blocks.slice(i + 1).every(b =>
    a.x + a.w <= b.x || b.x + b.w <= a.x || a.y + a.h <= b.y || b.y + b.h <= a.y
  ));
}
