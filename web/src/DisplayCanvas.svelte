<script>
  import Icon from './Icon.svelte';
  import { displaySchedule, formatDuration, meetingProgress } from './display-schedule.js';
  import { canvasScale, gridCell, hexColorRe, moveBlock, resizeEdges } from './display-layout.js';

  let { config, data, statuses = {}, zone = 'UTC', clock = new Date(), scheduleNow = null, language = 'vi', editing = false, selected = '', zoomMode = 'width', zoom = 1, onmove = () => {}, onresize = () => {}, onselect = () => {}, oninspect = () => {} } = $props();

  const t = (vi, en) => language === 'en' ? en : vi;
  let locale = $derived(language === 'en' ? 'en-US' : 'vi-VN');
  let viewportWidth = $state(0), windowHeight = $state(0);
  let resizeStart = null;
  let dragging = $state(null);
  let dragOffset = { x: 0, y: 0 };
  let dragPreview = $state(null);

  let rowCount = $derived(config.rows || Math.max(7, ...config.blocks.map(block => block.y + block.h)));
  let scale = $derived(canvasScale(config.width, config.height, viewportWidth, editing ? Math.max(280, windowHeight * 0.7) : windowHeight, editing ? zoomMode : 'fit', editing ? zoom : 1));

  let labels = $derived(language === 'en'
    ? { clock: 'Clock & date', branding: 'Organization', room: 'Room name', status: 'Status', upcoming: 'Upcoming', footer: 'Connection', text:'Text', rectangle:'Rectangle' }
    : { clock: 'Giờ và ngày', branding: 'Tổ chức', room: 'Tên phòng', status: 'Trạng thái', upcoming: 'Lịch tiếp theo', footer: 'Kết nối', text:'Chữ', rectangle:'Hình chữ nhật' });

  const key = block => block.id || block.type;
  const hasPart = (block, part) => !block.hideParts?.includes(part);
  const entryTitle = (block, entry) => (!entry || !hasPart(block, 'title')) ? '' : (entry.private ? t('Cuộc họp riêng tư', 'Private meeting') : (entry.title || ''));
  const entryOrganizer = (block, entry) => (!entry || entry.private || !hasPart(block, 'organizer')) ? '' : (entry.organizer || '');
  const time = (seconds) => new Date(seconds * 1000).toLocaleTimeString(locale, { timeZone: zone, hour: '2-digit', minute: '2-digit' });
  const xMap = { left: 'flex-start', center: 'center', right: 'flex-end' };
  const yMap = { top: 'flex-start', center: 'center', bottom: 'flex-end' };
  function blockStyle(block) {
    const x = dragPreview?.key === key(block) ? dragPreview.x : block.x;
    const y = dragPreview?.key === key(block) ? dragPreview.y : block.y;
    const parts = [`grid-column:${x + 1} / span ${block.w}`, `grid-row:${y + 1} / span ${block.h}`];
    if (block.type === 'rectangle') {
      if (hexColorRe.test(block.color || '')) parts.push(`background:${block.color}`);
      return parts.join(';');
    }
    if (hexColorRe.test(block.bgColor || '')) parts.push(`--w-bg:${block.bgColor}`);
    if (hexColorRe.test(block.textColor || '')) parts.push(`--w-fg:${block.textColor}`);
    if (hexColorRe.test(block.accentColor || '')) parts.push(`--w-accent:${block.accentColor}`);
    if (Number.isInteger(block.letterSpacing)) parts.push(`--w-ls:${block.letterSpacing}`);
    if (block.type === 'text') {
      parts.push(`padding:${Number.isInteger(block.padding) ? block.padding : 16}px`);
      parts.push(`margin:${Number.isInteger(block.margin) ? block.margin : 0}px`);
      parts.push(`justify-content:${yMap[block.valign] || 'center'}`);
      parts.push(`align-items:${xMap[block.align] || 'flex-start'}`);
      parts.push(`text-align:${block.align || 'left'}`);
      parts.push(`font-size:${Number.isInteger(block.fontSize) && block.fontSize >= 8 ? block.fontSize : 24}px`);
      return parts.join(';');
    }
    if (Number.isInteger(block.fontScale) && block.fontScale >= 60) parts.push(`--w-font-scale:${block.fontScale / 100}`);
    if (Number.isInteger(block.padding) && block.padding > 0) parts.push(`padding:${block.padding}px`);
    if (block.align) { parts.push(`text-align:${block.align}`); parts.push(`--w-items-x:${xMap[block.align] || 'flex-start'}`); }
    if (block.valign) parts.push(`--w-items-y:${yMap[block.valign] || 'center'}`);
    return parts.join(';');
  }

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
    if (event.key === 'F2' || (event.key === 'Enter' && event.shiftKey)) {
      event.preventDefault();
      const rect = event.currentTarget.getBoundingClientRect();
      onselect(key(block));
      oninspect(key(block), { x: rect.right, y: rect.bottom });
      return;
    }
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
  function openInspect(event, block) {
    if (!editing) return;
    event.preventDefault();
    event.stopPropagation();
    const k = key(block);
    onselect(k);
    if (event.type === 'click') {
      const rect = event.currentTarget.getBoundingClientRect();
      oninspect(k, { x: rect.left, y: rect.bottom + 6 });
    } else {
      oninspect(k, { x: event.clientX, y: event.clientY });
    }
  }
</script>
<svelte:window bind:innerHeight={windowHeight} onkeydown={(event) => { if (event.key === 'Escape' && resizeStart) { event.preventDefault(); cancelResize(); } }} onblur={cancelResize} />
<div class="viewport" class:fixed={!!config.width} class:editing bind:clientWidth={viewportWidth}>
<div class="canvas-size" style={config.width ? `width:${config.width * scale}px;height:${config.height * scale}px` : ''}>
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="canvas" class:editing class:fixed={!!config.width} style={`--rows:${rowCount};${config.width ? `width:${config.width}px;height:${config.height}px;transform:scale(${scale});` : ''}`} ondragover={previewDrag} ondrop={drop}>
  {#each config.blocks as block (key(block))}
    {@const roomData = data?.rooms?.[block.roomId]}
    {@const status = statuses[block.roomId] || 'Không xác định'}
    {@const statusText = language === 'en' ? ({ 'Không xác định':'Unknown', 'Đang họp':'In meeting', 'Trống':'Available' }[status] || status) : status}
    {@const schedule = displaySchedule(roomData?.entries || [], scheduleNow ?? 0, roomData?.valid_until)}
    {@const progress = status === 'Đang họp' && scheduleNow !== null ? meetingProgress(schedule.current, scheduleNow) : 0}
    {#if editing && dragPreview?.key === key(block) && (dragPreview.x !== dragging.block.x || dragPreview.y !== dragging.block.y)}<div class="drag-origin" aria-hidden="true" style={`grid-column:${dragging.block.x + 1} / span ${block.w};grid-row:${dragging.block.y + 1} / span ${block.h}`}></div>{/if}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <section
      class="block widget-{block.type} {block.type} sublayout-{block.subLayout || 'default'}"
      class:available={(block.type === 'status' || block.type === 'room') && status === 'Trống'}
      class:busy-status={(block.type === 'status' || block.type === 'room') && status === 'Đang họp'}
      class:unknown-status={(block.type === 'status' || block.type === 'room') && status === 'Không xác định'}
      class:has-custom-bg={block.type !== 'rectangle' && hexColorRe.test(block.bgColor || '')}
      class:has-custom-fg={block.type !== 'rectangle' && hexColorRe.test(block.textColor || '')}
      class:has-custom-accent={block.type !== 'rectangle' && hexColorRe.test(block.accentColor || '')}
      class:has-custom-align={block.type !== 'text' && Boolean(block.align)}
      class:has-custom-valign={block.type !== 'text' && Boolean(block.valign)}
      class:no-accent-bar={block.type === 'room' && !hasPart(block, 'accentBar')}
      class:is-selected={editing && selected === key(block)}
      class:drag-preview={dragPreview?.key === key(block)}
      style={blockStyle(block)}
      draggable={editing}
      ondragstart={(event) => startDrag(event, block)}
      ondragend={cancelDrag}
      onpointerdown={() => { if (editing) onselect(key(block)); }}
      oncontextmenu={(event) => openInspect(event, block)}
      ondblclick={(event) => openInspect(event, block)}
    >
      {#if editing}
        <div class="block-actions">
          <button type="button" class="block-label" onkeydown={(event) => handleKeyNudge(event, block)} onclick={() => onselect(key(block))}>{labels[block.type]}</button>
          <button type="button" class="block-inspect-btn" aria-label={`${t('Thuộc tính', 'Properties')}: ${labels[block.type]}`} title={t('Nhấp phải hoặc bấm để chỉnh thuộc tính', 'Right-click or click to edit properties')} onclick={(event) => openInspect(event, block)}>⚙</button>
        </div>
      {/if}

      {#if block.type === 'clock'}
        <div class="clock-head">
          <strong class="clock">{clock.toLocaleTimeString(locale, { timeZone: zone, hour:'2-digit', minute:'2-digit' })}</strong>
          {#if hasPart(block, 'zone')}
            <span class="clock-zone mono">{zone}</span>
          {/if}
        </div>
        {#if hasPart(block, 'date')}
          <small class="clock-date">{clock.toLocaleDateString(locale, { timeZone: zone, weekday:'long', day:'numeric', month:'long', year:'numeric' })}</small>
        {/if}

      {:else if block.type === 'branding'}
        <div class="branding-copy">
          {#if hasPart(block, 'eyebrow')}
            <span class="widget-eyebrow">{t('ĐƠN VỊ', 'ORGANIZATION')}</span>
          {/if}
          <strong>{config.organization || 'Schedule Display'}</strong>
        </div>
        {#if hasPart(block, 'logo')}
          {#if config.logo}
            <div class="branding-logo-frame">
              <img src={config.logo} alt="Logo" />
            </div>
          {:else}
            <div class="branding-mark" aria-hidden="true">
              <Icon name="brand" />
            </div>
          {/if}
        {/if}

      {:else if block.type === 'room'}
        {#if hasPart(block, 'eyebrow')}
          <div class="room-eyebrow">
            <Icon name="rooms" />
            <span>{t('PHÒNG HỌP', 'MEETING ROOM')}</span>
          </div>
        {/if}
        <h1>{roomData?.room || (block.roomId ? t('Phòng không khả dụng', 'Room unavailable') : t('Chưa chọn phòng', 'No room selected'))}</h1>

      {:else if block.type === 'status'}
        {#if hasPart(block, 'pill')}
          <div class="status-top">
            <span class="status-pill">
              <span class="status-beacon" aria-hidden="true"></span>
              <span>{t('HIỆN TẠI', 'CURRENT')}</span>
            </span>
          </div>
        {/if}
        <div class="status-main">
          {#if hasPart(block, 'icon')}
            <Icon name={status === 'Trống' ? 'check-circle' : status === 'Đang họp' ? 'clock' : 'warning'} class="widget-status-icon" />
          {/if}
          <strong class="state" class:available={status === 'Trống'} class:busy={status === 'Đang họp'}>{statusText}</strong>
        </div>
        {#if status !== 'Không xác định' && scheduleNow !== null}
          {#if status === 'Đang họp' && schedule.current}
            {@const currentTitle = entryTitle(block, schedule.current)}
            {@const currentOrganizer = entryOrganizer(block, schedule.current)}
            {@const rem = formatDuration(schedule.current.ends - scheduleNow, language)}
            {#if currentTitle || currentOrganizer}
              <div class="status-booking">
                {#if currentTitle}
                  <p class="status-booking-title" class:is-private={schedule.current.private}>{currentTitle}</p>
                {/if}
                {#if currentOrganizer}
                  <p class="status-booking-organizer">
                    <Icon name="user" class="status-org-icon" />
                    <span>{currentOrganizer}</span>
                  </p>
                {/if}
              </div>
            {/if}
            {#if hasPart(block, 'until') || (hasPart(block, 'remaining') && rem) || hasPart(block, 'progress')}
              <div class="status-meta">
                {#if hasPart(block, 'until') || (hasPart(block, 'remaining') && rem)}
                  <div class="status-window">
                    {#if hasPart(block, 'until')}<p class="status-until">{t('Đến', 'Until')} {time(schedule.current.ends)}</p>{/if}
                    {#if hasPart(block, 'remaining') && rem}<span class="status-remaining">{t('Còn', 'Remaining')} {rem}</span>{/if}
                  </div>
                {/if}
                {#if hasPart(block, 'progress')}
                  <div class="status-progress" aria-hidden="true">
                    <span class="status-progress-fill" style={`width:${Math.round(progress * 100)}%`}></span>
                  </div>
                {/if}
              </div>
            {/if}
          {:else if status === 'Trống' && schedule.upcoming.length}
            {@const nextEntry = schedule.upcoming[0]}
            {@const nextTitle = entryTitle(block, nextEntry)}
            {@const nextOrganizer = entryOrganizer(block, nextEntry)}
            {@const rem = formatDuration(nextEntry.starts - scheduleNow, language)}
            {#if nextTitle || nextOrganizer}
              <div class="status-booking status-next">
                <span class="status-booking-kicker">{t('Sắp tới', 'Up next')}</span>
                {#if nextTitle}
                  <p class="status-booking-title" class:is-private={nextEntry.private}>{nextTitle}</p>
                {/if}
                {#if nextOrganizer}
                  <p class="status-booking-organizer">
                    <Icon name="user" class="status-org-icon" />
                    <span>{nextOrganizer}</span>
                  </p>
                {/if}
              </div>
            {/if}
            {#if hasPart(block, 'until') || (hasPart(block, 'remaining') && rem)}
              <div class="status-meta">
                <div class="status-window">
                  {#if hasPart(block, 'until')}<p class="status-until">{t('Trống đến', 'Free until')} {time(nextEntry.starts)}</p>{/if}
                  {#if hasPart(block, 'remaining') && rem}<span class="status-remaining">{t('Còn', 'Remaining')} {rem}</span>{/if}
                </div>
              </div>
            {/if}
          {:else if status === 'Trống' && hasPart(block, 'until')}
            <p class="status-note">{t('Không có lịch kế tiếp', 'No upcoming meetings')}</p>
          {/if}
        {:else if status === 'Không xác định' && hasPart(block, 'until')}
          <p class="status-note">{t('Đang chờ xác minh trạng thái', 'Awaiting server verification')}</p>
        {/if}

      {:else if block.type === 'upcoming'}
        <div class="upcoming-head">
          <span class="kicker">{t('LỊCH TIẾP THEO', 'UPCOMING')}</span>
          {#if hasPart(block, 'count') && status !== 'Không xác định' && scheduleNow !== null && schedule.upcoming.length}
            <span class="upcoming-count mono">{schedule.upcoming.length}</span>
          {/if}
        </div>
        {#if status !== 'Không xác định' && scheduleNow !== null && schedule.upcoming.length}
          <ul class="upcoming-list">
            {#each schedule.upcoming as entry, index}
              {@const untilStart = index === 0 ? formatDuration(entry.starts - scheduleNow, language) : ''}
              {@const slotDuration = formatDuration(entry.ends - entry.starts, language)}
              {@const itemTitle = entryTitle(block, entry)}
              {@const itemOrganizer = entryOrganizer(block, entry)}
              <li class="upcoming-item" class:is-next={index === 0}>
                <div class="upcoming-time-wrap">
                  <span class="upcoming-dot" aria-hidden="true"></span>
                  <div>
                    <strong class="upcoming-range">{time(entry.starts)} – {time(entry.ends)}</strong>
                    {#if index === 0 && hasPart(block, 'relative')}
                      <span class="upcoming-relative">{untilStart ? `${t('Kế tiếp', 'Next')} · ${t('Sau', 'In')} ${untilStart}` : t('Kế tiếp', 'Next')}</span>
                    {/if}
                  </div>
                </div>
                {#if itemTitle || itemOrganizer}
                  <div class="upcoming-booking">
                    {#if itemTitle}
                      <span class="upcoming-title" class:is-private={entry.private}>{itemTitle}</span>
                    {/if}
                    {#if itemTitle && itemOrganizer}
                      <span class="upcoming-sep" aria-hidden="true">·</span>
                    {/if}
                    {#if itemOrganizer}
                      <span class="upcoming-organizer">
                        {#if !itemTitle}<Icon name="user" class="upcoming-org-icon" />{/if}
                        <span>{itemOrganizer}</span>
                      </span>
                    {/if}
                  </div>
                {/if}
                {#if slotDuration && hasPart(block, 'duration')}<span class="upcoming-duration">{slotDuration}</span>{/if}
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
            <span class="footer-dot" class:is-offline={!data} aria-hidden="true"></span>
            <span>{!data ? t('Không thể xác minh kết nối', 'Connection cannot be verified') : t('Hệ thống đang hoạt động', 'System online')}</span>
          </small>
          {#if hasPart(block, 'badge') || hasPart(block, 'zone')}
            <span class="footer-meta">
              {#if hasPart(block, 'badge')}<span class="footer-badge">{!data ? t('Ngoại tuyến', 'Offline') : t('Trực tuyến', 'Live')}</span>{/if}
              {#if hasPart(block, 'zone')}<span class="footer-zone mono">{zone}</span>{/if}
            </span>
          {/if}
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
    font-size: calc(clamp(1.1rem, min(14cqi, 4.5cqh), 3.1rem) * var(--w-font-scale, 1));
  }
  .canvas.fixed .clock-date {
    font-size: calc(clamp(0.62rem, min(4.6cqi, 1.8cqh), 0.92rem) * var(--w-font-scale, 1));
  }
  .canvas.fixed .widget-room h1 {
    font-size: calc(clamp(1.05rem, min(4.4cqi, 4.4cqh), 3rem) * var(--w-font-scale, 1));
  }
  .canvas.fixed .state {
    font-size: calc(clamp(1.5rem, min(10.5cqi, 7cqh), 4.6rem) * var(--w-font-scale, 1));
  }
  .canvas.fixed .status-booking-title {
    font-size: calc(clamp(0.88rem, min(4.2cqi, 3.1cqh), 1.75rem) * var(--w-font-scale, 1));
  }
  .canvas.fixed .status-booking-organizer {
    font-size: calc(clamp(0.72rem, min(2.6cqi, 2cqh), 1.05rem) * var(--w-font-scale, 1));
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
    letter-spacing: calc(-0.01em + var(--w-ls, 0) * 0.01em);
  }
  .block.text.has-custom-bg {
    border-radius: 0.55rem;
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

  .block-actions {
    position: absolute;
    top: 0.42rem;
    right: 0.95rem;
    z-index: 4;
    display: inline-flex;
    align-items: center;
    gap: 0.22rem;
  }
  .block-label,
  .block-inspect-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 0 !important;
    padding: 0.16rem 0.5rem !important;
    border: 1px solid rgba(24, 44, 35, 0.14) !important;
    border-radius: 9999px !important;
    background: rgba(255, 254, 250, 0.9) !important;
    backdrop-filter: blur(6px);
    color: var(--ink-secondary, #3b5448) !important;
    font-size: 0.68rem !important;
    font-weight: 700 !important;
    letter-spacing: 0.02em;
    line-height: 1.2;
    cursor: pointer;
    box-shadow: 0 1px 3px rgba(20, 40, 30, 0.08);
  }
  .block-inspect-btn {
    padding: 0.16rem 0.42rem !important;
  }
  .block-inspect-btn:hover {
    background: #ffffff !important;
    border-color: var(--accent, #1b6340) !important;
    color: var(--accent, #1b6340) !important;
  }
  .editing .block.is-selected .block-label {
    background: var(--focus-ring, #1d5fa3) !important;
    color: var(--ink-on-accent, #ffffff) !important;
    border-color: var(--focus-ring, #1d5fa3) !important;
  }
  .editing .block.is-selected .block-inspect-btn {
    border-color: var(--focus-ring, #1d5fa3) !important;
    color: var(--focus-ring, #1d5fa3) !important;
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
    margin-block: auto;
  }
  .status-main:has(+ .status-booking) {
    margin-bottom: 0;
  }
  .status-booking {
    display: grid;
    gap: 0.28rem;
    min-width: 0;
    margin-bottom: auto;
  }
  .status-booking-kicker {
    font-size: calc(clamp(0.64rem, 2.2cqi, 0.78rem) * var(--w-font-scale, 1));
    font-weight: 750;
    letter-spacing: calc(0.11em + var(--w-ls, 0) * 0.01em);
    text-transform: uppercase;
    color: var(--accent, #1b6340);
  }
  .status-booking-title {
    margin: 0;
    font-size: calc(clamp(0.98rem, 4.4cqi, 1.75rem) * var(--w-font-scale, 1));
    font-weight: 750;
    line-height: 1.22;
    letter-spacing: calc(-0.02em + var(--w-ls, 0) * 0.01em);
    color: var(--ink-primary, #182c23);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .status-booking-title.is-private {
    font-style: italic;
    opacity: 0.85;
  }
  .status-booking-organizer {
    margin: 0;
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    min-width: 0;
    font-size: calc(clamp(0.78rem, 2.8cqi, 1.05rem) * var(--w-font-scale, 1));
    font-weight: 600;
    letter-spacing: calc(var(--w-ls, 0) * 0.01em);
    color: var(--ink-secondary, #3b5448);
  }
  .status-booking-organizer span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  :global(.status-org-icon),
  :global(.upcoming-org-icon) {
    width: 0.95em !important;
    height: 0.95em !important;
    flex-shrink: 0;
    opacity: 0.78;
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
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.3rem 0.65rem;
    padding: clamp(0.55rem, 2.4cqi, 0.85rem) clamp(0.65rem, 2.6cqi, 0.95rem);
    border-radius: 0.75rem;
    background: var(--bg-subtle, #f4f7f2);
    border: 1px solid var(--border-subtle, #dde4da);
    min-width: 0;
  }
  .upcoming-time-wrap {
    grid-column: 1;
    grid-row: 1;
  }
  .upcoming-duration {
    grid-column: 2;
    grid-row: 1;
    justify-self: end;
  }
  .upcoming-booking {
    grid-column: 1 / -1;
    grid-row: 2;
    display: flex;
    align-items: baseline;
    gap: 0.35rem;
    padding-left: 1.1rem;
    min-width: 0;
  }
  .upcoming-title {
    font-size: calc(clamp(0.78rem, 3cqi, 0.96rem) * var(--w-font-scale, 1));
    font-weight: 650;
    letter-spacing: calc(var(--w-ls, 0) * 0.01em);
    color: var(--ink-primary, #182c23);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 0 1 auto;
    min-width: 0;
  }
  .upcoming-title.is-private {
    font-style: italic;
    font-weight: 550;
    opacity: 0.82;
  }
  .upcoming-sep {
    font-size: 0.78rem;
    color: var(--ink-muted, #4f665a);
    opacity: 0.65;
    flex-shrink: 0;
  }
  .upcoming-organizer {
    display: inline-flex;
    align-items: center;
    gap: 0.28rem;
    font-size: calc(clamp(0.72rem, 2.5cqi, 0.86rem) * var(--w-font-scale, 1));
    font-weight: 500;
    letter-spacing: calc(var(--w-ls, 0) * 0.01em);
    color: var(--ink-muted, #4f665a);
    min-width: 0;
    flex: 0 1 auto;
  }
  .upcoming-organizer span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  @container (min-width: 440px) {
    .upcoming-item {
      grid-template-columns: minmax(clamp(9.5rem, 24cqi, 13rem), auto) minmax(0, 1fr) auto;
      gap: 0.5rem clamp(0.75rem, 2.2cqi, 1.35rem);
    }
    .upcoming-booking {
      grid-column: 2;
      grid-row: 1;
      padding-left: 0;
    }
    .upcoming-duration {
      grid-column: 3;
      grid-row: 1;
    }
    .upcoming-title {
      font-size: calc(clamp(0.84rem, 2.2cqi, 1.05rem) * var(--w-font-scale, 1));
    }
    .upcoming-organizer {
      font-size: calc(clamp(0.76rem, 1.8cqi, 0.92rem) * var(--w-font-scale, 1));
    }
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
    .upcoming-duration,
    .upcoming-sep,
    .upcoming-organizer { display: none; }
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

  /* Custom widget overrides (colors, font scale, letter spacing, alignment, sub-layouts) */
  .block.has-custom-bg {
    background: var(--w-bg);
    border-color: color-mix(in srgb, var(--w-fg, #182c23) 14%, transparent);
  }
  .block.has-custom-fg,
  .block.has-custom-fg .clock,
  .block.has-custom-fg.widget-branding strong,
  .block.has-custom-fg.widget-room h1,
  .block.has-custom-fg .state,
  .block.has-custom-fg .status-booking-title,
  .block.has-custom-fg .status-until,
  .block.has-custom-fg .upcoming-range,
  .block.has-custom-fg .upcoming-title,
  .block.has-custom-fg .footer-row {
    color: var(--w-fg);
  }
  .block.has-custom-fg .clock-date,
  .block.has-custom-fg .room-eyebrow,
  .block.has-custom-fg .status-booking-kicker,
  .block.has-custom-fg .status-booking-organizer,
  .block.has-custom-fg .status-note,
  .block.has-custom-fg .kicker,
  .block.has-custom-fg .upcoming-sep,
  .block.has-custom-fg .upcoming-organizer,
  .block.has-custom-fg .upcoming-empty,
  .block.has-custom-fg .upcoming-empty-box,
  .block.has-custom-fg .footer-zone {
    color: color-mix(in srgb, var(--w-fg) 76%, transparent);
  }
  .block.has-custom-fg .status-meta,
  .block.has-custom-fg .status-note {
    border-color: color-mix(in srgb, var(--w-fg) 16%, transparent);
  }
  .block.has-custom-fg .clock-zone,
  .block.has-custom-fg .status-pill,
  .block.has-custom-fg .status-remaining,
  .block.has-custom-fg .upcoming-duration,
  .block.has-custom-fg .footer-badge {
    background: color-mix(in srgb, var(--w-fg) 10%, transparent);
    border-color: color-mix(in srgb, var(--w-fg) 18%, transparent);
    color: var(--w-fg);
  }
  .block.has-custom-fg .upcoming-item,
  .block.has-custom-fg .upcoming-empty-box {
    background: color-mix(in srgb, var(--w-fg) 7%, transparent);
    border-color: color-mix(in srgb, var(--w-fg) 14%, transparent);
  }
  .block.has-custom-fg .upcoming-item.is-next {
    background: color-mix(in srgb, var(--w-accent, var(--w-fg)) 13%, transparent);
    border-color: color-mix(in srgb, var(--w-accent, var(--w-fg)) 32%, transparent);
  }

  .block.has-custom-accent .widget-eyebrow,
  .block.has-custom-accent .room-eyebrow :global(.icon),
  .block.has-custom-accent .status-booking-kicker,
  .block.has-custom-accent :global(.widget-status-icon),
  .block.has-custom-accent .upcoming-relative {
    color: var(--w-accent);
  }
  .block.has-custom-accent .branding-mark,
  .block.has-custom-accent .upcoming-count {
    background: color-mix(in srgb, var(--w-accent) 16%, transparent);
    border-color: color-mix(in srgb, var(--w-accent) 34%, transparent);
    color: var(--w-accent);
  }
  .block.has-custom-accent.widget-room::before,
  .block.has-custom-accent .status-progress-fill {
    background: var(--w-accent);
  }
  .block.has-custom-accent .status-progress {
    background: color-mix(in srgb, var(--w-accent) 18%, transparent);
  }
  .block.has-custom-accent .status-beacon,
  .block.has-custom-accent .upcoming-item.is-next .upcoming-dot,
  .block.has-custom-accent .footer-dot {
    background: var(--w-accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--w-accent) 26%, transparent);
  }

  /* Typography scale & letter spacing across widgets */
  .clock {
    font-size: calc(clamp(1.35rem, 16cqi, 3.1rem) * var(--w-font-scale, 1));
    letter-spacing: calc(-0.04em + var(--w-ls, 0) * 0.01em);
  }
  .clock-date {
    font-size: calc(clamp(0.7rem, 4.8cqi, 0.92rem) * var(--w-font-scale, 1));
    letter-spacing: calc(var(--w-ls, 0) * 0.01em);
  }
  .widget-branding strong {
    font-size: calc(clamp(0.95rem, 3.2cqi, 1.45rem) * var(--w-font-scale, 1));
    letter-spacing: calc(-0.02em + var(--w-ls, 0) * 0.01em);
  }
  .widget-eyebrow,
  .room-eyebrow,
  .kicker {
    letter-spacing: calc(0.13em + var(--w-ls, 0) * 0.01em);
  }
  .widget-room h1 {
    font-size: calc(clamp(1.25rem, 4.4cqi, 3rem) * var(--w-font-scale, 1));
    letter-spacing: calc(-0.03em + var(--w-ls, 0) * 0.01em);
  }
  .state {
    font-size: calc(clamp(1.75rem, 10.5cqi, 4.6rem) * var(--w-font-scale, 1));
    letter-spacing: calc(-0.04em + var(--w-ls, 0) * 0.01em);
  }
  .status-until {
    font-size: calc(clamp(0.9rem, 3.6cqi, 1.25rem) * var(--w-font-scale, 1));
    letter-spacing: calc(var(--w-ls, 0) * 0.01em);
  }
  .upcoming-range {
    font-size: calc(clamp(0.88rem, 3.6cqi, 1.12rem) * var(--w-font-scale, 1));
    letter-spacing: calc(var(--w-ls, 0) * 0.01em);
  }
  .footer-row {
    font-size: calc(clamp(0.75rem, 2.5cqi, 0.92rem) * var(--w-font-scale, 1));
    letter-spacing: calc(var(--w-ls, 0) * 0.01em);
  }

  /* Alignment & optional accent bar */
  .block.has-custom-align {
    align-items: var(--w-items-x, stretch);
  }
  .block.has-custom-align.widget-branding {
    justify-content: var(--w-items-x, space-between);
  }
  .block.has-custom-align .clock-head,
  .block.has-custom-align .status-top,
  .block.has-custom-align .status-main,
  .block.has-custom-align .status-window,
  .block.has-custom-align .upcoming-head {
    width: 100%;
    justify-content: var(--w-items-x, space-between);
  }
  .block.has-custom-align .status-booking {
    width: 100%;
    justify-items: var(--w-items-x, stretch);
  }
  .block.has-custom-align .status-booking-organizer {
    justify-content: var(--w-items-x, flex-start);
  }
  .block.has-custom-align .upcoming-booking {
    justify-content: var(--w-items-x, flex-start);
  }
  .block.has-custom-align .upcoming-list,
  .block.has-custom-align .status-meta,
  .block.has-custom-align .footer-bar {
    width: 100%;
  }
  .block.has-custom-valign {
    justify-content: var(--w-items-y, center);
  }
  .widget-room.no-accent-bar {
    padding-left: clamp(0.85rem, 1.6vw, 1.45rem);
  }
  .widget-room.no-accent-bar::before {
    display: none;
  }

  /* Internal sub-layouts (rearrange details inside each widget) */
  .widget-clock.sublayout-reverse .clock-date { order: -1; }
  .widget-clock.sublayout-compact {
    flex-direction: row;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.35rem 0.75rem;
  }

  .widget-branding.sublayout-reverse { flex-direction: row-reverse; }
  .widget-branding.sublayout-compact .branding-copy {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.25rem 0.6rem;
  }

  .widget-room.sublayout-reverse:not(.no-accent-bar) {
    padding-left: clamp(0.85rem, 1.6vw, 1.45rem);
    padding-right: clamp(1.15rem, 4cqi, 1.75rem);
  }
  .widget-room.sublayout-reverse::before {
    left: auto;
    right: 0.5rem;
  }
  .widget-room.sublayout-reverse h1 { order: -1; }
  .widget-room.sublayout-compact {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .widget-status.sublayout-reverse .status-meta,
  .widget-status.sublayout-reverse .status-note {
    order: -1;
    border-top: 0;
    border-bottom: 1px solid rgba(24, 44, 35, 0.08);
    padding-top: 0;
    padding-bottom: 0.55rem;
  }
  .block.has-custom-fg.widget-status.sublayout-reverse .status-meta,
  .block.has-custom-fg.widget-status.sublayout-reverse .status-note {
    border-bottom-color: color-mix(in srgb, var(--w-fg) 16%, transparent);
  }
  .widget-status.sublayout-reverse .status-top { order: 2; }
  .widget-status.sublayout-compact { gap: 0.4rem; }
  .widget-status.sublayout-compact .status-booking-title { -webkit-line-clamp: 1; }
  .widget-status.sublayout-compact .status-meta { padding-top: 0.35rem; gap: 0.35rem; }

  .widget-upcoming.sublayout-reverse .upcoming-head { order: 2; }
  .widget-upcoming.sublayout-reverse .upcoming-item { grid-template-columns: auto minmax(0, 1fr); }
  .widget-upcoming.sublayout-reverse .upcoming-duration { grid-column: 1; justify-self: start; }
  .widget-upcoming.sublayout-reverse .upcoming-time-wrap { grid-column: 2; flex-direction: row-reverse; justify-self: end; text-align: right; }
  .widget-upcoming.sublayout-reverse .upcoming-booking { padding-left: 0; padding-right: 1.1rem; justify-content: flex-end; }
  @container (min-width: 440px) {
    .widget-upcoming.sublayout-reverse .upcoming-item { grid-template-columns: auto minmax(0, 1fr) minmax(clamp(9.5rem, 24cqi, 13rem), auto); }
    .widget-upcoming.sublayout-reverse .upcoming-booking { grid-column: 2; grid-row: 1; padding-right: 0; }
    .widget-upcoming.sublayout-reverse .upcoming-time-wrap { grid-column: 3; }
  }
  .widget-upcoming.sublayout-compact .upcoming-list { gap: 0.35rem; }
  .widget-upcoming.sublayout-compact .upcoming-item { padding: 0.38rem 0.6rem; gap: 0.2rem 0.6rem; }

  .widget-footer.sublayout-reverse .footer-bar { flex-direction: row-reverse; }
  .widget-footer.sublayout-compact { padding-block: 0.45rem; }

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
