<script>
  import { displaySchedule } from './display-schedule.js';
  import { canvasScale, gridCell, moveBlock, resizeEdges } from './display-layout.js';
  let { config, data, status = 'Không xác định', zone = 'UTC', clock = new Date(), scheduleNow = null, editing = false, selected = '', zoomMode = 'width', zoom = 1, onmove = () => {}, onresize = () => {}, onselect = () => {} } = $props();
  let viewportWidth = $state(0), windowHeight = $state(0);
  let resizeStart = null;
  let dragging = $state(null);
  let dragOffset = { x: 0, y: 0 };
  let dragPreview = $state(null);
  let rowCount = $derived(config.rows || Math.max(7, ...config.blocks.map(block => block.y + block.h)));
  let scale = $derived(canvasScale(config.width, config.height, viewportWidth, editing ? Math.max(280, windowHeight * 0.7) : windowHeight, editing ? zoomMode : 'fit', editing ? zoom : 1));
  let schedule = $derived(displaySchedule(data?.entries || [], scheduleNow ?? 0, data?.valid_until));
  const labels = { clock: 'Giờ và ngày', branding: 'Tổ chức', room: 'Tên phòng', status: 'Trạng thái', upcoming: 'Lịch tiếp theo', footer: 'Kết nối', text:'Chữ', rectangle:'Hình chữ nhật' };
  const key = block => block.id || block.type;
  const time = (seconds) => new Date(seconds * 1000).toLocaleTimeString('vi-VN', { timeZone: zone, hour: '2-digit', minute: '2-digit' });
  function cellAt(canvas, clientX, clientY) {
    const rect = canvas.getBoundingClientRect();
    const style = getComputedStyle(canvas);
    const left = parseFloat(style.paddingLeft), right = parseFloat(style.paddingRight);
    const top = parseFloat(style.paddingTop), bottom = parseFloat(style.paddingBottom);
    const columnGap = parseFloat(style.columnGap), rowGap = parseFloat(style.rowGap);
    const factor = rect.width / canvas.offsetWidth || 1;
    return { x: gridCell((clientX - rect.left) / factor, 0, canvas.offsetWidth, left, right, columnGap, 12), y: gridCell((clientY - rect.top) / factor, 0, canvas.offsetHeight, top, bottom, rowGap, rowCount) };
  }
  function startDrag(event, block) {
    const cell = cellAt(event.currentTarget.parentElement, event.clientX, event.clientY);
    onselect(key(block));
    dragging = { key:key(block), block:{ ...block } };
    dragOffset = { x: cell.x - block.x, y: cell.y - block.y };
    dragPreview = { key:key(block), ...moveBlock(block, 0, 0, rowCount) };
    event.dataTransfer.setData('text/plain', key(block));
    event.dataTransfer.effectAllowed = 'move';
  }
  function previewDrag(event) {
    event.preventDefault();
    if (!editing || !dragging) return;
    const cell = cellAt(event.currentTarget, event.clientX, event.clientY);
    dragPreview = { key:dragging.key, ...moveBlock(dragging.block, cell.x - dragOffset.x - dragging.block.x, cell.y - dragOffset.y - dragging.block.y, rowCount) };
  }
  function drop(event) {
    previewDrag(event);
    if (dragging && dragPreview) onmove(dragging.key, dragPreview.x, dragPreview.y);
    dragging = null;
    dragPreview = null;
  }
  function cancelDrag() {
    dragging = null;
    dragPreview = null;
  }
  function startResize(event, block, edges) {
    if (!event.isPrimary || event.button !== 0 || resizeStart) return;
    event.preventDefault();
    event.stopPropagation();
    onselect(key(block));
    const canvas = event.currentTarget.closest('.canvas');
    resizeStart = { block: { ...block }, canvas, edges, cell:cellAt(canvas, event.clientX, event.clientY), pointer:event.pointerId };
    event.currentTarget.setPointerCapture(event.pointerId);
  }
  function updateResize(event) {
    if (!resizeStart || event.pointerId !== resizeStart.pointer) return;
    const { block, canvas, edges, cell } = resizeStart;
    const end = cellAt(canvas, event.clientX, event.clientY);
    onresize(key(block), resizeEdges(block, edges, end.x - cell.x, end.y - cell.y, rowCount));
  }
  function finishResize(event) {
    if (!resizeStart || event.pointerId !== resizeStart.pointer) return;
    updateResize(event);
    resizeStart = null;
  }
  function cancelResize() {
    if (!resizeStart) return;
    const { block } = resizeStart;
    resizeStart = null;
    onresize(key(block), { x:block.x, y:block.y, w:block.w, h:block.h });
  }
  function handleKeyNudge(event, block) {
    if (!editing) return;
    const deltas = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] };
    const delta = deltas[event.key];
    if (!delta) return;
    event.preventDefault();
    const [dx, dy] = delta;
    if (event.shiftKey) {
      const edges = `${dx < 0 ? 'w' : dx > 0 ? 'e' : ''}${dy < 0 ? 'n' : dy > 0 ? 's' : ''}`;
      onresize(key(block), resizeEdges(block, edges, dx, dy, rowCount));
    } else {
      const moved = moveBlock(block, dx, dy, rowCount);
      onmove(key(block), moved.x, moved.y);
    }
  }
</script>
<svelte:window bind:innerHeight={windowHeight} onkeydown={(event) => { if (event.key === 'Escape' && resizeStart) { event.preventDefault(); cancelResize(); } }} onblur={cancelResize} />
<div class="viewport" class:fixed={!!config.width} class:editing bind:clientWidth={viewportWidth}>
<div class="canvas-size" style={config.width ? `width:${config.width * scale}px;height:${config.height * scale}px` : ''}>
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="canvas" class:editing class:fixed={!!config.width} style={`--rows:${rowCount};${config.width ? `width:${config.width}px;height:${config.height}px;transform:scale(${scale});` : ''}`} ondragover={previewDrag} ondrop={drop}>
  {#each config.blocks as block (key(block))}
    {#if editing && dragPreview?.key === key(block) && (dragPreview.x !== dragging.block.x || dragPreview.y !== dragging.block.y)}<div class="drag-origin" aria-hidden="true" style={`grid-column:${dragging.block.x + 1} / span ${block.w};grid-row:${dragging.block.y + 1} / span ${block.h}`}></div>{/if}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <section
      class="block {block.type}"
      class:available={block.type === 'status' && status === 'Trống'}
      class:busy-status={block.type === 'status' && status === 'Đang họp'}
      class:unknown-status={block.type === 'status' && status === 'Không xác định'}
      class:is-selected={editing && selected === key(block)}
      class:drag-preview={dragPreview?.key === key(block)}
      style={`grid-column:${(dragPreview?.key === key(block) ? dragPreview.x : block.x) + 1} / span ${block.w};grid-row:${(dragPreview?.key === key(block) ? dragPreview.y : block.y) + 1} / span ${block.h};${block.type === 'rectangle' && /^#[0-9a-f]{6}$/i.test(block.color) ? `background:${block.color}` : ''}${block.type === 'text' ? `;padding:${Number.isInteger(block.padding) ? block.padding : 16}px;margin:${Number.isInteger(block.margin) ? block.margin : 0}px;justify-content:${({ top:'flex-start', center:'center', bottom:'flex-end' })[block.valign] || 'center'};align-items:${({ left:'flex-start', center:'center', right:'flex-end' })[block.align] || 'flex-start'};text-align:${block.align || 'left'};font-size:${Number.isInteger(block.fontSize) && block.fontSize >= 8 ? block.fontSize : 24}px` : ''}`}
      draggable={editing}
      ondragstart={(event) => startDrag(event, block)}
      ondragend={cancelDrag}
      onpointerdown={() => { if (editing) onselect(key(block)); }}
    >
      {#if editing}
        <button type="button" class="block-label" onkeydown={(event) => handleKeyNudge(event, block)} onclick={() => onselect(key(block))}>{labels[block.type]}</button>
      {/if}
      {#if block.type === 'clock'}
        <strong class="clock">{clock.toLocaleTimeString('vi-VN', { timeZone: zone, hour:'2-digit', minute:'2-digit' })}</strong>
        <small class="clock-date">{clock.toLocaleDateString('vi-VN', { timeZone: zone, weekday:'long', day:'numeric', month:'long', year:'numeric' })}</small>
      {:else if block.type === 'branding'}
        <strong>{config.organization}</strong>{#if config.logo}<img src={config.logo} alt="Logo" />{/if}
      {:else if block.type === 'room'}
        <h1>{data?.room || 'Phòng họp'}</h1>
      {:else if block.type === 'status'}
        <small class="kicker">HIỆN TẠI</small>
        <strong class="state" class:available={status === 'Trống'} class:busy={status === 'Đang họp'}>{status}</strong>
        {#if status !== 'Không xác định' && scheduleNow !== null}
          {#if status === 'Đang họp' && schedule.current}<p class="status-until">Đến {time(schedule.current.ends)}</p>
          {:else if status === 'Trống' && schedule.upcoming.length}<p class="status-until">Đến {time(schedule.upcoming[0].starts)}</p>
          {/if}
        {/if}
      {:else if block.type === 'upcoming'}
        <small class="kicker">LỊCH TIẾP THEO</small>
        {#if status !== 'Không xác định' && scheduleNow !== null && schedule.upcoming.length}
          <ul>{#each schedule.upcoming as entry}<li>{time(entry.starts)} – {time(entry.ends)}</li>{/each}</ul>
        {:else}
          <p class="upcoming-empty">{status === 'Không xác định' ? 'Không thể xác minh lịch' : 'Không có lịch sắp tới'}</p>
        {/if}
      {:else if block.type === 'text'}
        <span>{block.text}</span>
      {:else if block.type === 'footer'}
        <small class="footer-row">
          <span class="footer-dot" class:is-offline={status === 'Không xác định'} aria-hidden="true"></span>
          <span>{status === 'Không xác định' ? 'Không thể xác minh kết nối' : 'Hệ thống đang hoạt động'}</span>
        </small>
      {/if}
      {#if editing}
        {#each ['n','s','e','w','ne','nw','se','sw'] as edge}
          <span class="resize {edge}" aria-hidden="true" onpointerdown={(event) => startResize(event, block, edge)} onpointermove={updateResize} onpointerup={finishResize} onpointercancel={cancelResize} onlostpointercapture={cancelResize}></span>
        {/each}
      {/if}
    </section>
  {/each}
</div>
</div>
</div>
<style>
  .viewport.editing {
    background: var(--display-stage-bg, #d8e1d6);
    border: 1px solid var(--display-stage-border, #c2cec0);
    border-radius: var(--radius-md, 0.65rem);
    padding: 0.5rem;
  }
  .viewport.editing.fixed { overflow-x: auto; }
  .canvas-size { position: relative; }
  .viewport:not(.fixed) .canvas-size { width: 100%; }
  .block.text { background: transparent; border-color: transparent; border-radius: 0; pointer-events: none; box-shadow: none; }
  .editing .block.text { pointer-events: auto; border: 1px dashed rgba(38, 63, 51, 0.25); border-radius: 0.35rem; }
  .editing .resize { position: absolute; z-index: 2; touch-action: none; min-width: 0; min-height: 0; }
  .resize.n, .resize.s { height: 12px; left: 12px; right: 12px; cursor: ns-resize; }
  .resize.n { top: 0; bottom: auto; }
  .resize.s { bottom: 0; }
  .resize.e, .resize.w { width: 12px; top: 12px; bottom: 12px; cursor: ew-resize; }
  .resize.e { right: 0; }
  .resize.w { left: 0; right: auto; }
  .resize.ne, .resize.nw, .resize.se, .resize.sw { width: 16px; height: 16px; }
  .resize.ne { top: 0; right: 0; bottom: auto; cursor: nesw-resize; }
  .resize.nw { top: 0; left: 0; bottom: auto; right: auto; cursor: nwse-resize; }
  .resize.se { bottom: 0; right: 0; cursor: nwse-resize; }
  .resize.sw { bottom: 0; left: 0; right: auto; cursor: nesw-resize; }

  .viewport.fixed:not(.editing) { overflow: hidden; }
  .canvas.fixed {
    box-sizing: border-box;
    min-height: 0;
    transform-origin: top left;
    grid-template-rows: repeat(var(--rows), minmax(0, 1fr));
    container-type: size;
    padding: 1.5rem;
    gap: 0.75rem;
  }
  .canvas.fixed .block { min-height: 0; padding: 1.35rem; }
  .canvas.fixed .clock { font-size: clamp(1.5rem, 3cqw, 2.9rem); }
  .canvas.fixed .room h1 { font-size: clamp(1.3rem, 3.5cqw, 3rem); }
  .canvas.fixed .state { font-size: clamp(1.8rem, 5cqw, 4.5rem); }
  .canvas.fixed small { font-size: clamp(0.75rem, 1cqw, 0.95rem); }

  .canvas {
    display: grid;
    grid-template-columns: repeat(12, minmax(0, 1fr));
    grid-template-rows: repeat(var(--rows), minmax(4.2rem, 1fr));
    gap: 0.75rem;
    background: var(--display-canvas-bg, #ecefe7);
    padding: clamp(0.75rem, 1.5vw, 1.5rem);
    color: var(--ink-primary, #182c23);
    font-family: var(--font-sans, system-ui, sans-serif);
    min-height: 100vh;
    min-height: 100dvh;
  }
  .canvas.editing {
    min-height: 28rem;
    border-radius: 0.5rem;
    background-image:
      linear-gradient(to right, rgba(24, 44, 35, 0.04) 1px, transparent 1px),
      linear-gradient(to bottom, rgba(24, 44, 35, 0.04) 1px, transparent 1px);
    background-size: calc(100% / 12) calc(100% / var(--rows));
  }

  .block {
    position: relative;
    min-width: 0;
    overflow: auto;
    background: var(--display-card-bg, #fffefa);
    border: 1px solid var(--display-card-border, #d7e0d4);
    border-radius: 0.75rem;
    padding: clamp(0.75rem, 1.5vw, 1.4rem);
    display: flex;
    flex-direction: column;
    justify-content: center;
    box-shadow: 0 2px 10px rgba(24, 44, 35, 0.04);
  }
  .editing .block { cursor: grab; }
  .editing .block.is-selected {
    outline: 2.5px solid var(--focus-ring, #1d5fa3);
    outline-offset: 2px;
    z-index: 2;
  }
  .block-label {
    align-self: flex-start;
    display: inline-block;
    min-height: 0 !important;
    margin-bottom: 0.35rem;
    padding: 0.15rem 0.45rem !important;
    border: 1px solid rgba(24, 44, 35, 0.15) !important;
    border-radius: 0.3rem !important;
    background: rgba(255, 254, 250, 0.9) !important;
    color: var(--ink-secondary, #3b5448) !important;
    font-size: 0.7rem !important;
    font-weight: 700 !important;
    letter-spacing: 0.03em;
    cursor: pointer;
  }
  .editing .block.is-selected .block-label {
    background: var(--focus-ring, #1d5fa3) !important;
    color: var(--ink-on-accent, #ffffff) !important;
    border-color: var(--focus-ring, #1d5fa3) !important;
  }

  .drag-origin {
    z-index: 0;
    border: 2px dashed var(--ink-secondary, #486457);
    border-radius: 0.65rem;
    background: repeating-linear-gradient(135deg, #4864571c 0 8px, transparent 8px 16px);
    pointer-events: none;
  }
  .drag-preview {
    z-index: 3;
    outline: 3px solid var(--accent, #1b6340);
    outline-offset: 2px;
    opacity: 0.88;
    cursor: grabbing !important;
  }

  .clock {
    font-size: clamp(1.5rem, 3vw, 2.9rem);
    line-height: 1.05;
    letter-spacing: -0.025em;
    font-variant-numeric: tabular-nums;
    color: var(--ink-primary, #182c23);
  }
  .clock-date {
    margin-top: 0.25rem;
    color: var(--ink-muted, #4f665a);
    text-transform: capitalize;
  }

  .branding {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }
  .branding strong {
    font-size: clamp(1rem, 1.6vw, 1.35rem);
    letter-spacing: -0.015em;
  }
  .branding img { max-width: 22%; max-height: 3.2rem; object-fit: contain; }

  .room h1 {
    font-size: clamp(1.3rem, 3.5vw, 3rem);
    letter-spacing: -0.03em;
    line-height: 1.1;
    margin: 0;
  }

  .kicker {
    font-weight: 700;
    letter-spacing: 0.12em;
    color: var(--ink-muted, #4f665a);
    margin-bottom: 0.35rem;
  }

  .status.available {
    background: var(--display-avail-bg, #e1f0e5);
    border-color: var(--display-avail-border, #b7d6c0);
  }
  .status.busy-status {
    background: var(--display-busy-bg, #fcefe9);
    border-color: var(--display-busy-border, #e7c0b2);
  }
  .status.unknown-status {
    background: var(--display-unknown-bg, #e7eae6);
    border-color: var(--display-unknown-border, #c8d0c7);
  }

  .state {
    font-size: clamp(1.8rem, 5vw, 4.5rem);
    letter-spacing: -0.035em;
    line-height: 1.08;
    color: var(--display-unknown-ink, #3f5048);
  }
  .state.available { color: var(--display-avail-ink, #115833); }
  .state.busy { color: var(--display-busy-ink, #962c1c); }

  .status-until {
    margin: 0.5rem 0 0;
    font-size: clamp(0.95rem, 1.4vw, 1.25rem);
    font-weight: 600;
    color: var(--ink-secondary, #3b5448);
    font-variant-numeric: tabular-nums;
  }

  .upcoming ul {
    margin: 0.5rem 0 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 0.5rem;
  }
  .upcoming li {
    padding: 0.5rem 0.75rem;
    border-left: 3px solid var(--accent, #1b6340);
    border-radius: 0.35rem;
    background: var(--bg-subtle, #f4f7f2);
    font-variant-numeric: tabular-nums;
    font-weight: 600;
    color: var(--ink-primary, #182c23);
  }
  .upcoming-empty {
    margin: 0.5rem 0 0;
    color: var(--ink-muted, #556e62);
  }

  .footer {
    background: var(--display-footer-bg, #173126);
    border-color: var(--display-footer-bg, #173126);
    color: var(--display-footer-ink, #edf3ee);
  }
  .footer-row {
    display: inline-flex;
    align-items: center;
    gap: 0.55rem;
  }
  .footer-dot {
    width: 0.55rem;
    height: 0.55rem;
    border-radius: 50%;
    background: var(--display-dot-ok, #5dd890);
    flex-shrink: 0;
  }
  .footer-dot.is-offline {
    background: var(--display-dot-warn, #e9a34c);
  }
  .footer small, small { font-size: clamp(0.75rem, 1vw, 0.95rem); }

  @media (max-width: 600px) {
    .canvas { padding: 0.5rem; gap: 0.35rem; grid-template-rows: repeat(var(--rows), minmax(3rem, 1fr)); }
    .block { padding: 0.55rem; }
    .clock { font-size: 1.2rem; }
  }
</style>
