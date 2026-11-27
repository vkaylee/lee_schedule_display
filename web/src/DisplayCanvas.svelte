<script>
  import Icon from './Icon.svelte';
  import { displaySchedule, formatDuration, meetingProgress } from './display-schedule.js';
  import { canvasScale, gridCell, moveBlock, resizeEdges } from './display-layout.js';

  let { config, data, status = 'Không xác định', zone = 'UTC', clock = new Date(), scheduleNow = null, language = 'vi', editing = false, selected = '', zoomMode = 'width', zoom = 1, onmove = () => {}, onresize = () => {}, onselect = () => {} } = $props();

  const t = (vi, en) => language === 'en' ? en : vi;
  let locale = $derived(language === 'en' ? 'en-US' : 'vi-VN');
  let statusText = $derived(language === 'en' ? ({ 'Không xác định':'Unknown', 'Đang họp':'In meeting', 'Trống':'Available' }[status] || status) : status);
  let viewportWidth = $state(0), windowHeight = $state(0);
  let resizeStart = null;
  let dragging = $state(null);
  let dragOffset = { x: 0, y: 0 };
  let dragPreview = $state(null);

  let rowCount = $derived(config.rows || Math.max(7, ...config.blocks.map(block => block.y + block.h)));
  let scale = $derived(canvasScale(config.width, config.height, viewportWidth, editing ? Math.max(280, windowHeight * 0.7) : windowHeight, editing ? zoomMode : 'fit', editing ? zoom : 1));
  let schedule = $derived(displaySchedule(data?.entries || [], scheduleNow ?? 0, data?.valid_until));
  let progress = $derived(status === 'Đang họp' && scheduleNow !== null ? meetingProgress(schedule.current, scheduleNow) : 0);

  let labels = $derived(language === 'en'
    ? { clock: 'Clock & date', branding: 'Organization', room: 'Room name', status: 'Status', upcoming: 'Upcoming', footer: 'Connection', text:'Text', rectangle:'Rectangle' }
    : { clock: 'Giờ và ngày', branding: 'Tổ chức', room: 'Tên phòng', status: 'Trạng thái', upcoming: 'Lịch tiếp theo', footer: 'Kết nối', text:'Chữ', rectangle:'Hình chữ nhật' });

  const key = block => block.id || block.type;
  const time = (seconds) => new Date(seconds * 1000).toLocaleTimeString(locale, { timeZone: zone, hour: '2-digit', minute: '2-digit' });

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
      class="block widget-{block.type} {block.type}"
      class:available={(block.type === 'status' || block.type === 'room') && status === 'Trống'}
      class:busy-status={(block.type === 'status' || block.type === 'room') && status === 'Đang họp'}
      class:unknown-status={(block.type === 'status' || block.type === 'room') && status === 'Không xác định'}
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
        <div class="clock-head">
          <strong class="clock">{clock.toLocaleTimeString(locale, { timeZone: zone, hour:'2-digit', minute:'2-digit' })}</strong>
          <span class="clock-zone mono">{zone}</span>
        </div>
        <small class="clock-date">{clock.toLocaleDateString(locale, { timeZone: zone, weekday:'long', day:'numeric', month:'long', year:'numeric' })}</small>

      {:else if block.type === 'branding'}
        <div class="branding-copy">
          <span class="widget-eyebrow">{t('ĐƠN VỊ', 'ORGANIZATION')}</span>
          <strong>{config.organization || 'Schedule Display'}</strong>
        </div>
        {#if config.logo}
          <div class="branding-logo-frame">
            <img src={config.logo} alt="Logo" />
          </div>
        {:else}
          <div class="branding-mark" aria-hidden="true">
            <Icon name="brand" />
          </div>
        {/if}

      {:else if block.type === 'room'}
        <div class="room-eyebrow">
          <Icon name="rooms" />
          <span>{t('PHÒNG HỌP', 'MEETING ROOM')}</span>
        </div>
        <h1>{data?.room || t('Phòng họp', 'Meeting room')}</h1>

      {:else if block.type === 'status'}
        <div class="status-top">
          <span class="status-pill">
            <span class="status-beacon" aria-hidden="true"></span>
            <span>{t('HIỆN TẠI', 'CURRENT')}</span>
          </span>
        </div>
        <div class="status-main">
          <Icon name={status === 'Trống' ? 'check-circle' : status === 'Đang họp' ? 'clock' : 'warning'} class="widget-status-icon" />
          <strong class="state" class:available={status === 'Trống'} class:busy={status === 'Đang họp'}>{statusText}</strong>
        </div>
        {#if status !== 'Không xác định' && scheduleNow !== null}
          {#if status === 'Đang họp' && schedule.current}
            {@const rem = formatDuration(schedule.current.ends - scheduleNow, language)}
            <div class="status-meta">
              <div class="status-window">
                <p class="status-until">{t('Đến', 'Until')} {time(schedule.current.ends)}</p>
                {#if rem}<span class="status-remaining">{t('Còn', 'Remaining')} {rem}</span>{/if}
              </div>
              <div class="status-progress" aria-hidden="true">
                <span class="status-progress-fill" style={`width:${Math.round(progress * 100)}%`}></span>
              </div>
            </div>
          {:else if status === 'Trống' && schedule.upcoming.length}
            {@const rem = formatDuration(schedule.upcoming[0].starts - scheduleNow, language)}
            <div class="status-meta">
              <div class="status-window">
                <p class="status-until">{t('Trống đến', 'Free until')} {time(schedule.upcoming[0].starts)}</p>
                {#if rem}<span class="status-remaining">{t('Còn', 'Remaining')} {rem}</span>{/if}
              </div>
            </div>
          {:else if status === 'Trống'}
            <p class="status-note">{t('Không có lịch kế tiếp', 'No upcoming meetings')}</p>
          {/if}
        {:else if status === 'Không xác định'}
          <p class="status-note">{t('Đang chờ xác minh trạng thái', 'Awaiting server verification')}</p>
        {/if}

      {:else if block.type === 'upcoming'}
        <div class="upcoming-head">
          <span class="kicker">{t('LỊCH TIẾP THEO', 'UPCOMING')}</span>
          {#if status !== 'Không xác định' && scheduleNow !== null && schedule.upcoming.length}
            <span class="upcoming-count mono">{schedule.upcoming.length}</span>
          {/if}
        </div>
        {#if status !== 'Không xác định' && scheduleNow !== null && schedule.upcoming.length}
          <ul class="upcoming-list">
            {#each schedule.upcoming as entry, index}
              {@const untilStart = index === 0 ? formatDuration(entry.starts - scheduleNow, language) : ''}
              {@const slotDuration = formatDuration(entry.ends - entry.starts, language)}
              <li class="upcoming-item" class:is-next={index === 0}>
                <div class="upcoming-time-wrap">
                  <span class="upcoming-dot" aria-hidden="true"></span>
                  <div>
                    <strong class="upcoming-range">{time(entry.starts)} – {time(entry.ends)}</strong>
                    {#if index === 0}
                      <span class="upcoming-relative">{untilStart ? `${t('Kế tiếp', 'Next')} · ${t('Sau', 'In')} ${untilStart}` : t('Kế tiếp', 'Next')}</span>
                    {/if}
                  </div>
                </div>
                {#if slotDuration}<span class="upcoming-duration">{slotDuration}</span>{/if}
              </li>
            {/each}
          </ul>
        {:else}
          <div class="upcoming-empty-box">
            <Icon name={status === 'Không xác định' ? 'warning' : 'calendar'} />
            <p class="upcoming-empty">{status === 'Không xác định' ? t('Không thể xác minh lịch', 'Schedule cannot be verified') : t('Không có lịch sắp tới', 'No upcoming meetings')}</p>
          </div>
        {/if}

      {:else if block.type === 'text'}
        <span>{block.text}</span>

      {:else if block.type === 'footer'}
        <div class="footer-bar">
          <small class="footer-row">
            <span class="footer-dot" class:is-offline={status === 'Không xác định'} aria-hidden="true"></span>
            <span>{status === 'Không xác định' ? t('Không thể xác minh kết nối', 'Connection cannot be verified') : t('Hệ thống đang hoạt động', 'System online')}</span>
          </small>
          <span class="footer-meta">
            <span class="footer-badge">{status === 'Không xác định' ? t('Ngoại tuyến', 'Offline') : t('Trực tuyến', 'Live')}</span>
            <span class="footer-zone mono">{zone}</span>
          </span>
        </div>
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

  .editing .resize { position: absolute; z-index: 3; touch-action: none; min-width: 0; min-height: 0; }
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

  .editing .block.is-selected .resize.ne::after,
  .editing .block.is-selected .resize.nw::after,
  .editing .block.is-selected .resize.se::after,
  .editing .block.is-selected .resize.sw::after {
    content: '';
    position: absolute;
    width: 7px;
    height: 7px;
    border-radius: 2px;
    background: #ffffff;
    border: 2px solid var(--focus-ring, #1d5fa3);
  }
  .editing .block.is-selected .resize.nw::after { top: 2px; left: 2px; }
  .editing .block.is-selected .resize.ne::after { top: 2px; right: 2px; }
  .editing .block.is-selected .resize.se::after { bottom: 2px; right: 2px; }
  .editing .block.is-selected .resize.sw::after { bottom: 2px; left: 2px; }

  .viewport.fixed:not(.editing) { overflow: hidden; }
  .canvas.fixed {
    box-sizing: border-box;
    min-height: 0;
    transform-origin: top left;
    grid-template-rows: repeat(var(--rows), minmax(0, 1fr));
    container-type: size;
    padding: clamp(0.65rem, 2cqmin, 1.5rem);
    gap: clamp(0.4rem, 1.1cqmin, 0.85rem);
  }
  .canvas.fixed .block {
    min-height: 0;
    padding: clamp(0.45rem, 1.8cqmin, 1.35rem);
  }
  .canvas.fixed .widget-room {
    padding-left: clamp(0.95rem, 2.6cqmin, 1.75rem);
  }
  .canvas.fixed .clock {
    font-size: clamp(1.1rem, min(14cqi, 4.5cqh), 3.1rem);
  }
  .canvas.fixed .clock-date {
    font-size: clamp(0.62rem, min(4.6cqi, 1.8cqh), 0.92rem);
  }
  .canvas.fixed .widget-room h1 {
    font-size: clamp(1.05rem, min(4.4cqi, 4.4cqh), 3rem);
  }
  .canvas.fixed .state {
    font-size: clamp(1.5rem, min(10.5cqi, 7cqh), 4.6rem);
  }

  .canvas {
    display: grid;
    grid-template-columns: repeat(12, minmax(0, 1fr));
    grid-template-rows: repeat(var(--rows), minmax(4.5rem, 1fr));
    gap: 0.85rem;
    background:
      radial-gradient(circle at 12% 10%, rgba(27, 99, 64, 0.07), transparent 44%),
      radial-gradient(circle at 88% 88%, rgba(23, 49, 38, 0.06), transparent 48%),
      var(--display-canvas-bg, #ecefe7);
    padding: clamp(0.85rem, 1.6vw, 1.5rem);
    color: var(--ink-primary, #182c23);
    font-family: var(--font-sans, system-ui, sans-serif);
    min-height: 100vh;
    min-height: 100dvh;
  }
  .canvas.editing {
    min-height: 28rem;
    border-radius: 0.5rem;
    background-image:
      linear-gradient(to right, rgba(24, 44, 35, 0.045) 1px, transparent 1px),
      linear-gradient(to bottom, rgba(24, 44, 35, 0.045) 1px, transparent 1px);
    background-size: calc(100% / 12) calc(100% / var(--rows));
  }

  /* Base Widget Card */
  .block {
    position: relative;
    min-width: 0;
    min-height: 0;
    container-type: inline-size;
    overflow: hidden;
    background: linear-gradient(180deg, #ffffff 0%, #fbfcf9 100%);
    border: 1px solid rgba(24, 44, 35, 0.09);
    border-radius: 1rem;
    padding: clamp(0.85rem, 1.6vw, 1.45rem);
    display: flex;
    flex-direction: column;
    justify-content: center;
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.9),
      0 1px 2px rgba(20, 40, 30, 0.04),
      0 10px 24px -10px rgba(20, 40, 30, 0.09);
    transition: border-color var(--duration-fast, 140ms) var(--ease-out, ease), box-shadow var(--duration-fast, 140ms) var(--ease-out, ease);
  }

  .block.rectangle {
    box-shadow: inset 0 0 0 1px rgba(24, 44, 35, 0.06), 0 4px 14px -6px rgba(20, 40, 30, 0.08);
    border-color: transparent;
  }

  .block.text {
    background: transparent;
    border-color: transparent;
    border-radius: 0;
    pointer-events: none;
    box-shadow: none;
    white-space: pre-wrap;
    overflow-wrap: break-word;
    line-height: 1.25;
    letter-spacing: -0.01em;
  }
  .editing .block.text {
    pointer-events: auto;
    border: 1px dashed rgba(38, 63, 51, 0.28);
    border-radius: 0.45rem;
  }

  .editing .block { cursor: grab; }
  .editing .block:hover { border-color: rgba(27, 99, 64, 0.35); }
  .editing .block.is-selected {
    outline: 2.5px solid var(--focus-ring, #1d5fa3);
    outline-offset: 2px;
    z-index: 2;
  }

  .block-label {
    position: absolute;
    top: 0.42rem;
    right: 1.15rem;
    z-index: 4;
    display: inline-block;
    min-height: 0 !important;
    padding: 0.16rem 0.5rem !important;
    border: 1px solid rgba(24, 44, 35, 0.14) !important;
    border-radius: 9999px !important;
    background: rgba(255, 254, 250, 0.88) !important;
    backdrop-filter: blur(6px);
    color: var(--ink-secondary, #3b5448) !important;
    font-size: 0.68rem !important;
    font-weight: 700 !important;
    letter-spacing: 0.02em;
    line-height: 1.2;
    cursor: pointer;
    box-shadow: 0 1px 3px rgba(20, 40, 30, 0.08);
  }
  .editing .block.is-selected .block-label {
    background: var(--focus-ring, #1d5fa3) !important;
    color: var(--ink-on-accent, #ffffff) !important;
    border-color: var(--focus-ring, #1d5fa3) !important;
  }

  .drag-origin {
    z-index: 0;
    border: 2px dashed var(--ink-secondary, #486457);
    border-radius: 1rem;
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

  /* 1. Clock Widget */
  .widget-clock {
    justify-content: center;
    gap: 0.3rem;
  }
  .clock-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.5rem;
    min-width: 0;
  }
  .clock {
    font-size: clamp(1.35rem, 16cqi, 3.1rem);
    font-weight: 800;
    line-height: 0.98;
    letter-spacing: -0.04em;
    font-variant-numeric: tabular-nums;
    color: var(--ink-primary, #182c23);
  }
  .clock-zone {
    font-size: 0.68rem;
    font-weight: 600;
    padding: 0.14rem 0.45rem;
    border-radius: 9999px;
    background: var(--bg-subtle, #eff2eb);
    color: var(--ink-muted, #556e62);
    border: 1px solid var(--border-subtle, #dde4da);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 48%;
  }
  .clock-date {
    font-size: clamp(0.7rem, 4.8cqi, 0.92rem);
    font-weight: 500;
    color: var(--ink-muted, #4f665a);
    text-transform: capitalize;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  @container (max-width: 240px) {
    .clock-zone { display: none; }
  }

  /* 2. Branding Widget */
  .widget-branding {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    gap: 0.85rem;
  }
  .branding-copy {
    display: grid;
    gap: 0.14rem;
    min-width: 0;
  }
  .widget-eyebrow {
    font-size: 0.66rem;
    font-weight: 750;
    letter-spacing: 0.13em;
    color: var(--accent, #1b6340);
    text-transform: uppercase;
  }
  .widget-branding strong {
    font-size: clamp(0.95rem, 3.2cqi, 1.45rem);
    font-weight: 750;
    letter-spacing: -0.02em;
    line-height: 1.2;
    color: var(--ink-primary, #182c23);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .branding-logo-frame {
    display: grid;
    place-items: center;
    padding: 0.32rem 0.55rem;
    border-radius: 0.65rem;
    background: var(--bg-subtle, #eff2eb);
    border: 1px solid var(--border-subtle, #dde4da);
    max-width: 28%;
    flex-shrink: 0;
  }
  .branding-logo-frame img {
    max-width: 100%;
    max-height: 2.6rem;
    object-fit: contain;
    display: block;
  }
  .branding-mark {
    display: grid;
    place-items: center;
    width: 2.4rem;
    height: 2.4rem;
    border-radius: 0.65rem;
    background: var(--accent-soft, #e3f0e7);
    color: var(--accent, #1b6340);
    border: 1px solid var(--accent-border, #b8d5c2);
    flex-shrink: 0;
  }

  /* 3. Room Nameplate Widget */
  .widget-room {
    padding-left: clamp(1.15rem, 4cqi, 1.75rem);
    gap: 0.2rem;
  }
  .widget-room::before {
    content: '';
    position: absolute;
    left: 0.5rem;
    top: 0.65rem;
    bottom: 0.65rem;
    width: 4px;
    border-radius: 9999px;
    background: var(--ink-muted, #556e62);
  }
  .widget-room.available::before { background: var(--accent, #1b6340); }
  .widget-room.busy-status::before { background: var(--danger, #9e2a22); }
  .widget-room.unknown-status::before { background: #6b7b72; }
  .room-eyebrow {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.68rem;
    font-weight: 750;
    letter-spacing: 0.13em;
    color: var(--ink-muted, #556e62);
  }
  .room-eyebrow :global(.icon) {
    width: 0.95em;
    height: 0.95em;
    color: var(--accent, #1b6340);
  }
  .widget-room.busy-status .room-eyebrow :global(.icon) { color: var(--danger, #9e2a22); }
  .widget-room h1 {
    font-size: clamp(1.25rem, 4.4cqi, 3rem);
    font-weight: 800;
    letter-spacing: -0.03em;
    line-height: 1.08;
    margin: 0;
    color: var(--ink-primary, #182c23);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  /* 4. Status Hero Widget */
  .widget-status {
    justify-content: space-between;
    gap: 0.85rem;
  }
  .widget-status.available {
    background:
      radial-gradient(circle at 92% 14%, rgba(27, 99, 64, 0.16), transparent 52%),
      linear-gradient(145deg, #f3fbf5 0%, #dcf0e2 100%);
    border-color: #a9d4b7;
  }
  .widget-status.busy-status {
    background:
      radial-gradient(circle at 92% 14%, rgba(158, 42, 34, 0.15), transparent 52%),
      linear-gradient(145deg, #fff7f4 0%, #fbe2da 100%);
    border-color: #e6b7aa;
  }
  .widget-status.unknown-status {
    background:
      radial-gradient(circle at 92% 14%, rgba(63, 80, 72, 0.1), transparent 52%),
      linear-gradient(145deg, #f3f5f2 0%, #e4e8e3 100%);
    border-color: #c5cec4;
  }

  .status-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .status-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.28rem 0.7rem;
    border-radius: 9999px;
    background: rgba(255, 255, 255, 0.72);
    border: 1px solid rgba(24, 44, 35, 0.08);
    font-size: 0.72rem;
    font-weight: 750;
    letter-spacing: 0.11em;
    color: var(--ink-secondary, #3b5448);
  }
  .status-beacon {
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 50%;
    background: #6b7b72;
    flex-shrink: 0;
  }
  .widget-status.available .status-beacon {
    background: #17804b;
    box-shadow: 0 0 0 3px rgba(23, 128, 75, 0.22);
    animation: widget-pulse 2.4s ease-in-out infinite;
  }
  .widget-status.busy-status .status-beacon {
    background: #c03428;
    box-shadow: 0 0 0 3px rgba(192, 52, 40, 0.22);
    animation: widget-pulse 2.4s ease-in-out infinite;
  }
  @keyframes widget-pulse {
    0%, 100% { transform: scale(1); opacity: 1; }
    50% { transform: scale(1.18); opacity: 0.75; }
  }

  .status-main {
    display: flex;
    align-items: center;
    gap: clamp(0.6rem, 2.2cqi, 1.15rem);
    min-width: 0;
  }
  :global(.widget-status-icon) {
    width: clamp(1.8rem, 8.5cqi, 3.6rem) !important;
    height: clamp(1.8rem, 8.5cqi, 3.6rem) !important;
    flex-shrink: 0;
    color: var(--display-unknown-ink, #3f5048);
  }
  .widget-status.available :global(.widget-status-icon) { color: var(--display-avail-ink, #115833); }
  .widget-status.busy-status :global(.widget-status-icon) { color: var(--display-busy-ink, #962c1c); }

  .state {
    font-size: clamp(1.75rem, 10.5cqi, 4.6rem);
    font-weight: 850;
    letter-spacing: -0.04em;
    line-height: 1.02;
    color: var(--display-unknown-ink, #3f5048);
  }
  .state.available { color: var(--display-avail-ink, #0e522f); }
  .state.busy { color: var(--display-busy-ink, #8f2518); }

  .status-meta {
    display: grid;
    gap: 0.55rem;
    padding-top: 0.65rem;
    border-top: 1px solid rgba(24, 44, 35, 0.08);
  }
  .status-window {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.4rem 0.75rem;
  }
  .status-until {
    margin: 0;
    font-size: clamp(0.9rem, 3.6cqi, 1.25rem);
    font-weight: 700;
    color: var(--ink-primary, #182c23);
    font-variant-numeric: tabular-nums;
  }
  .status-remaining {
    padding: 0.22rem 0.65rem;
    border-radius: 9999px;
    background: rgba(255, 255, 255, 0.78);
    border: 1px solid rgba(24, 44, 35, 0.08);
    font-size: clamp(0.74rem, 2.4cqi, 0.9rem);
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    color: var(--ink-secondary, #3b5448);
  }
  .status-progress {
    height: 6px;
    border-radius: 9999px;
    background: rgba(143, 37, 24, 0.14);
    overflow: hidden;
  }
  .status-progress-fill {
    display: block;
    height: 100%;
    border-radius: 9999px;
    background: linear-gradient(90deg, #b83525, #8f2518);
    transition: width 350ms ease;
  }
  .status-note {
    margin: 0;
    padding-top: 0.55rem;
    border-top: 1px solid rgba(24, 44, 35, 0.08);
    font-size: clamp(0.82rem, 2.8cqi, 1rem);
    font-weight: 600;
    color: var(--ink-secondary, #3b5448);
  }

  /* 5. Upcoming Agenda Widget */
  .widget-upcoming {
    justify-content: flex-start;
    gap: 0.75rem;
  }
  .upcoming-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }
  .kicker {
    font-size: clamp(0.7rem, 2.6cqi, 0.82rem);
    font-weight: 750;
    letter-spacing: 0.12em;
    color: var(--ink-muted, #4f665a);
  }
  .upcoming-count {
    display: inline-grid;
    place-items: center;
    min-width: 1.5rem;
    height: 1.5rem;
    padding-inline: 0.4rem;
    border-radius: 9999px;
    background: var(--accent-soft, #e3f0e7);
    border: 1px solid var(--accent-border, #b8d5c2);
    color: var(--accent, #1b6340);
    font-size: 0.74rem;
    font-weight: 700;
  }
  .upcoming-list {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 0.55rem;
  }
  .upcoming-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.65rem;
    padding: clamp(0.55rem, 2.4cqi, 0.85rem) clamp(0.65rem, 2.6cqi, 0.95rem);
    border-radius: 0.75rem;
    background: var(--bg-subtle, #f4f7f2);
    border: 1px solid var(--border-subtle, #dde4da);
    min-width: 0;
  }
  .upcoming-item.is-next {
    background: linear-gradient(135deg, #f3faf5 0%, #e7f3eb 100%);
    border-color: var(--accent-border, #b8d5c2);
    box-shadow: 0 2px 8px -4px rgba(27, 99, 64, 0.18);
  }
  .upcoming-time-wrap {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    min-width: 0;
  }
  .upcoming-dot {
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 50%;
    background: var(--border-strong, #c0cbc0);
    flex-shrink: 0;
  }
  .upcoming-item.is-next .upcoming-dot {
    background: var(--accent, #1b6340);
    box-shadow: 0 0 0 3px rgba(27, 99, 64, 0.16);
  }
  .upcoming-range {
    display: block;
    font-size: clamp(0.88rem, 3.6cqi, 1.12rem);
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    color: var(--ink-primary, #182c23);
    line-height: 1.2;
  }
  .upcoming-relative {
    display: block;
    margin-top: 0.14rem;
    font-size: clamp(0.7rem, 2.4cqi, 0.82rem);
    font-weight: 600;
    color: var(--accent, #1b6340);
  }
  .upcoming-duration {
    padding: 0.2rem 0.55rem;
    border-radius: 9999px;
    background: rgba(255, 255, 255, 0.85);
    border: 1px solid rgba(24, 44, 35, 0.08);
    font-size: clamp(0.68rem, 2.3cqi, 0.8rem);
    font-weight: 650;
    font-variant-numeric: tabular-nums;
    color: var(--ink-secondary, #3b5448);
    white-space: nowrap;
    flex-shrink: 0;
  }
  .upcoming-empty-box {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.45rem;
    padding: 1rem;
    border: 1px dashed var(--border-strong, #c0cbc0);
    border-radius: 0.75rem;
    background: var(--bg-subtle, #f4f7f2);
    color: var(--ink-muted, #556e62);
    text-align: center;
  }
  .upcoming-empty {
    margin: 0;
    font-size: clamp(0.84rem, 2.8cqi, 0.96rem);
    font-weight: 500;
  }
  @container (max-width: 220px) {
    .upcoming-duration { display: none; }
  }

  /* 6. Footer Connection Widget */
  .widget-footer {
    background: linear-gradient(135deg, #132a20 0%, #1c3d2e 100%);
    border-color: rgba(255, 255, 255, 0.08);
    color: var(--display-footer-ink, #edf3ee);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.1),
      0 6px 18px -6px rgba(14, 32, 24, 0.35);
  }
  .footer-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    min-width: 0;
  }
  .footer-row {
    display: inline-flex;
    align-items: center;
    gap: 0.55rem;
    font-size: clamp(0.75rem, 2.5cqi, 0.92rem);
    font-weight: 600;
    min-width: 0;
  }
  .footer-dot {
    width: 0.55rem;
    height: 0.55rem;
    border-radius: 50%;
    background: var(--display-dot-ok, #5dd890);
    box-shadow: 0 0 0 3px rgba(93, 216, 144, 0.2);
    flex-shrink: 0;
  }
  .footer-dot.is-offline {
    background: var(--display-dot-warn, #e9a34c);
    box-shadow: 0 0 0 3px rgba(233, 163, 76, 0.22);
  }
  .footer-meta {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    flex-shrink: 0;
  }
  .footer-badge {
    padding: 0.18rem 0.55rem;
    border-radius: 9999px;
    background: rgba(255, 255, 255, 0.11);
    font-size: 0.72rem;
    font-weight: 650;
    letter-spacing: 0.02em;
  }
  .footer-zone {
    font-size: 0.74rem;
    opacity: 0.78;
  }
  @container (max-width: 300px) {
    .footer-meta { display: none; }
  }

  @media (max-width: 600px) {
    .canvas { padding: 0.5rem; gap: 0.4rem; grid-template-rows: repeat(var(--rows), minmax(3.2rem, 1fr)); }
    .block { padding: 0.6rem; border-radius: 0.8rem; }
  }

  @media (prefers-reduced-motion: reduce) {
    .status-beacon, .status-progress-fill, .block {
      animation: none !important;
      transition: none !important;
    }
  }
</style>
