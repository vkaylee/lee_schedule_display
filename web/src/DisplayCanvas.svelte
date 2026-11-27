<script>
  import { displaySchedule } from './display-schedule.js';
  import { gridCell, resizeEdges } from './display-layout.js';
  let { config, data, status = 'Không xác định', zone = 'UTC', clock = new Date(), scheduleNow = null, editing = false, onmove = () => {}, onresize = () => {}, onselect = () => {} } = $props();
  let viewportWidth = $state(0), windowHeight = $state(0);
  let resizeStart = null;
  let dragging = '';
  let dragOffset = { x: 0, y: 0 };
  let rowCount = $derived(config.rows || Math.max(7, ...config.blocks.map(block => block.y + block.h)));
  let scale = $derived(config.width && viewportWidth ? Math.min(viewportWidth / config.width, (editing ? Math.max(280, windowHeight * 0.7) : windowHeight || config.height) / config.height) : 1);
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
    dragging = key(block);
    dragOffset = { x: cell.x - block.x, y: cell.y - block.y };
    event.dataTransfer.setData('text/plain', key(block));
    event.dataTransfer.effectAllowed = 'move';
  }
  function drop(event) {
    event.preventDefault();
    const cell = cellAt(event.currentTarget, event.clientX, event.clientY);
    if (dragging) onmove(dragging, cell.x - dragOffset.x, cell.y - dragOffset.y);
    dragging = '';
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
  function finishResize(event) {
    if (!resizeStart || event.pointerId !== resizeStart.pointer) return;
    const { block, canvas, edges, cell } = resizeStart;
    const end = cellAt(canvas, event.clientX, event.clientY);
    onresize(key(block), resizeEdges(block, edges, end.x - cell.x, end.y - cell.y, rowCount));
    resizeStart = null;
  }
</script>
<svelte:window bind:innerHeight={windowHeight} />
<div class="viewport" class:fixed={!!config.width} bind:clientWidth={viewportWidth} style={config.width ? `height:${config.height * scale}px` : ''}>
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="canvas" class:editing class:fixed={!!config.width} style={`--rows:${rowCount};${config.width ? `width:${config.width}px;height:${config.height}px;transform:scale(${scale});` : ''}`} ondragover={(event) => { if (editing) event.preventDefault(); }} ondrop={drop}>
  {#each config.blocks as block (key(block))}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <section class="block {block.type}" style={`grid-column:${block.x + 1} / span ${block.w};grid-row:${block.y + 1} / span ${block.h};${block.type === 'rectangle' && /^#[0-9a-f]{6}$/i.test(block.color) ? `background:${block.color}` : ''}`} draggable={editing} ondragstart={(event) => startDrag(event, block)} ondragend={() => dragging = ''}>
      {#if editing}<button type="button" class="block-label" onclick={() => onselect(key(block))}>{labels[block.type]}</button>{/if}
      {#if block.type === 'clock'}
        <strong class="clock">{clock.toLocaleTimeString('vi-VN', { timeZone: zone, hour:'2-digit', minute:'2-digit' })}</strong><small>{clock.toLocaleDateString('vi-VN', { timeZone: zone, weekday:'long', day:'numeric', month:'long', year:'numeric' })}</small>
      {:else if block.type === 'branding'}
        <strong>{config.organization}</strong>{#if config.logo}<img src={config.logo} alt="Logo" />{/if}
      {:else if block.type === 'room'}<h1>{data?.room || 'Phòng họp'}</h1>
      {:else if block.type === 'status'}
        <small>HIỆN TẠI</small><strong class="state" class:available={status === 'Trống'}>{status}</strong>
        {#if status !== 'Không xác định' && scheduleNow !== null}{#if status === 'Đang họp' && schedule.current}<p>Đến {time(schedule.current.ends)}</p>{:else if status === 'Trống' && schedule.upcoming.length}<p>Đến {time(schedule.upcoming[0].starts)}</p>{/if}{/if}
      {:else if block.type === 'upcoming'}
        <small>LỊCH TIẾP THEO</small>{#if status !== 'Không xác định' && scheduleNow !== null && schedule.upcoming.length}<ul>{#each schedule.upcoming as entry}<li>{time(entry.starts)} – {time(entry.ends)}</li>{/each}</ul>{:else}<p>{status === 'Không xác định' ? 'Không thể xác minh lịch' : 'Không có lịch sắp tới'}</p>{/if}
      {:else if block.type === 'text'}<span>{block.text}</span>
      {:else if block.type === 'footer'}<small>{status === 'Không xác định' ? 'Không thể xác minh kết nối' : 'Hệ thống đang hoạt động'}</small>{/if}
      {#if editing}{#each ['n','s','e','w','ne','nw','se','sw'] as edge}<span class="resize {edge}" aria-hidden="true" onpointerdown={(event) => startResize(event, block, edge)} onpointerup={finishResize} onpointercancel={() => resizeStart = null} onlostpointercapture={() => resizeStart = null}></span>{/each}{/if}
    </section>
  {/each}
</div>
</div>
<style>
  .block.text{background:transparent;border-color:transparent;border-radius:0;pointer-events:none}.editing .block.text{pointer-events:auto}
  .editing .resize{position:absolute;z-index:2;touch-action:none;min-width:0;min-height:0}.resize.n,.resize.s{height:12px;left:12px;right:12px;cursor:ns-resize}.resize.n{top:0;bottom:auto}.resize.s{bottom:0}.resize.e,.resize.w{width:12px;top:12px;bottom:12px;cursor:ew-resize}.resize.e{right:0}.resize.w{left:0;right:auto}.resize.ne,.resize.nw,.resize.se,.resize.sw{width:16px;height:16px}.resize.ne{top:0;right:0;bottom:auto;cursor:nesw-resize}.resize.nw{top:0;left:0;bottom:auto;right:auto;cursor:nwse-resize}.resize.se{bottom:0;right:0;cursor:nwse-resize}.resize.sw{bottom:0;left:0;right:auto;cursor:nesw-resize}
  .viewport.fixed{overflow:hidden}.canvas.fixed{box-sizing:border-box;min-height:0;transform-origin:top left;grid-template-rows:repeat(var(--rows),minmax(0,1fr));container-type:size;padding:1.5rem;gap:.7rem}.canvas.fixed .block{min-height:0;padding:1.4rem}.canvas.fixed .clock{font-size:clamp(1.5rem,3cqw,2.8rem)}.canvas.fixed .room h1{font-size:clamp(1.3rem,3.5cqw,3rem)}.canvas.fixed .state{font-size:clamp(1.8rem,5cqw,4.5rem)}.canvas.fixed small{font-size:clamp(.75rem,1cqw,.95rem)}.block{position:relative}.block-label{align-self:flex-start;border:0;background:transparent;padding:0;cursor:pointer}  .canvas{display:grid;grid-template-columns:repeat(12,minmax(0,1fr));grid-template-rows:repeat(var(--rows),minmax(4rem,1fr));gap:.7rem;background:#edeFE8;padding:clamp(.75rem,1.5vw,1.5rem);color:#263f33;font-family:system-ui,sans-serif;min-height:100vh}.block{min-width:0;overflow:auto;background:#fffefa;border:1px solid #dce5db;border-radius:.65rem;padding:clamp(.75rem,1.5vw,1.4rem);display:flex;flex-direction:column;justify-content:center}.editing{min-height:28rem;border-radius:.65rem}.editing .block{cursor:grab}.block-label{font-size:.7rem;color:#486457}.clock{font-size:clamp(1.5rem,3vw,2.8rem);line-height:1;font-variant-numeric:tabular-nums}.branding{flex-direction:row;align-items:center;justify-content:space-between;gap:1rem}.branding img{max-width:20%;max-height:3rem;object-fit:contain}.room h1{font-size:clamp(1.3rem,3.5vw,3rem);letter-spacing:-.03em;margin:0}.status{background:#e2efe4;border-color:#c9ddcb}.state{font-size:clamp(1.8rem,5vw,4.5rem);letter-spacing:-.035em;line-height:1.1}.available{color:#17653b}.upcoming ul{padding-left:1rem}.upcoming li{font-variant-numeric:tabular-nums}.footer{background:#234b37;border-color:#234b37;color:#fff}.footer small,small{font-size:clamp(.75rem,1vw,.95rem)}@media(max-width:600px){.canvas{padding:.5rem;gap:.3rem;grid-template-rows:repeat(var(--rows),minmax(3rem,1fr))}.block{padding:.4rem}.clock{font-size:1.2rem}}
</style>
