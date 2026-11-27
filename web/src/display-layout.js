export const displayBlocks = ['clock', 'branding', 'room', 'status', 'upcoming', 'footer'];

export const widgetParts = {
  clock: ['zone', 'date'],
  branding: ['eyebrow', 'logo'],
  room: ['eyebrow', 'accentBar'],
  status: ['pill', 'icon', 'until', 'remaining', 'progress'],
  upcoming: ['count', 'relative', 'duration'],
  footer: ['badge', 'zone']
};

export const hexColorRe = /^#[a-fA-F0-9]{6}$/;

export function contrastRatio(hexA, hexB) {
  if (!hexColorRe.test(hexA || '') || !hexColorRe.test(hexB || '')) return null;
  const lum = (hex) => {
    const ch = [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16) / 255).map(v => v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);
    return 0.2126 * ch[0] + 0.7152 * ch[1] + 0.0722 * ch[2];
  };
  const a = lum(hexA), b = lum(hexB);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

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

export function canvasScale(width, height, availableWidth, availableHeight, mode, zoom = 1) {
  if (!width || !height || !availableWidth) return 1;
  const fitWidth = availableWidth / width;
  return mode === 'width' ? fitWidth * zoom : Math.min(fitWidth, (availableHeight || height) / height) * zoom;
}

export function moveBlock(block, dx, dy, rows) {
  return {
    x: Math.max(0, Math.min(12 - block.w, block.x + dx)),
    y: Math.max(0, Math.min(rows - block.h, block.y + dy))
  };
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
    const optHex = (v) => v === undefined || v === '' || hexColorRe.test(v);
    const optAlign = (v) => v === undefined || v === '' || ['left', 'center', 'right'].includes(v);
    const optValign = (v) => v === undefined || v === '' || ['top', 'center', 'bottom'].includes(v);
    const optPadding = (v) => v === undefined || Number.isInteger(v) && v >= 0 && v <= 128;
    const optLs = (v) => v === undefined || Number.isInteger(v) && v >= -4 && v <= 20;
    const widgetStyleValid = custom || (
      optHex(block?.bgColor) && optHex(block?.textColor) && optHex(block?.accentColor) &&
      (block?.fontScale === undefined || block.fontScale === 0 || Number.isInteger(block.fontScale) && block.fontScale >= 60 && block.fontScale <= 180) &&
      optLs(block?.letterSpacing) && optPadding(block?.padding) &&
      optAlign(block?.align) && optValign(block?.valign) &&
      (block?.subLayout === undefined || block.subLayout === '' || ['default', 'reverse', 'compact'].includes(block.subLayout)) &&
      (block?.hideParts === undefined || Array.isArray(block.hideParts) && block.hideParts.length <= 8 &&
        new Set(block.hideParts).size === block.hideParts.length &&
        block.hideParts.every(p => (widgetParts[block.type] || []).includes(p)))
    );
    const textStyleValid = block?.type !== 'text' || (
      optHex(block.bgColor) && optHex(block.textColor) && optLs(block.letterSpacing) &&
      (block.fontSize === undefined || Number.isInteger(block.fontSize) && block.fontSize >= 8 && block.fontSize <= 256) &&
      optPadding(block.padding) &&
      (block.margin === undefined || Number.isInteger(block.margin) && block.margin >= 0 && block.margin <= 128) &&
      optAlign(block.align) && optValign(block.valign)
    );
    const key = block?.id || block?.type;
    if (!block || (!custom && !displayBlocks.includes(block.type)) || seen.has(key) ||
        (!custom && block.id) || (custom && (!block.id || !/^layer_[a-zA-Z0-9_-]{1,58}$/.test(block.id))) || !widgetStyleValid || !textStyleValid ||
        ![block.x, block.y, block.w, block.h].every(Number.isSafeInteger) ||
        block.x < 0 || block.y < 0 || block.w < 1 || block.h < 1 ||
        block.x + block.w > 12 || block.y + block.h > 24) return false;
    seen.add(key);
  }
  return true;
}
