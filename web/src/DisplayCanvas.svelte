<script>
  import { displaySchedule } from './display-schedule.js';
  import { gridCell } from './display-layout.js';
  let { config, data, status = 'Không xác định', zone = 'UTC', clock = new Date(), scheduleNow = null, editing = false, onmove = () => {} } = $props();
  let dragging = '';
  let dragOffset = { x: 0, y: 0 };
  let rowCount = $derived(Math.max(7, ...config.blocks.map(block => block.y + block.h)));
  let schedule = $derived(displaySchedule(data?.entries || [], scheduleNow ?? 0));
  const labels = { clock: 'Giờ và ngày', branding: 'Tổ chức', room: 'Tên phòng', status: 'Trạng thái', upcoming: 'Lịch tiếp theo', footer: 'Kết nối' };
  const time = (seconds) => new Date(seconds * 1000).toLocaleTimeString('vi-VN', { timeZone: zone, hour: '2-digit', minute: '2-digit' });
  function cellAt(canvas, clientX, clientY) {
    const rect = canvas.getBoundingClientRect();
    const style = getComputedStyle(canvas);
    const left = parseFloat(style.paddingLeft), right = parseFloat(style.paddingRight);
    const top = parseFloat(style.paddingTop), bottom = parseFloat(style.paddingBottom);
    const columnGap = parseFloat(style.columnGap), rowGap = parseFloat(style.rowGap);
    return { x: gridCell(clientX, rect.left, rect.width, left, right, columnGap, 12), y: gridCell(clientY, rect.top, rect.height, top, bottom, rowGap, rowCount) };
  }
  function startDrag(event, block) {
    const cell = cellAt(event.currentTarget.parentElement, event.clientX, event.clientY);
    dragging = block.type;
    dragOffset = { x: cell.x - block.x, y: cell.y - block.y };
    event.dataTransfer.setData('text/plain', block.type);
    event.dataTransfer.effectAllowed = 'move';
  }
  function drop(event) {
    event.preventDefault();
    const cell = cellAt(event.currentTarget, event.clientX, event.clientY);
    if (dragging) onmove(dragging, cell.x - dragOffset.x, cell.y - dragOffset.y);
    dragging = '';
  }
</script>
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="canvas" class:editing style={`--rows:${rowCount}`} ondragover={(event) => { if (editing) event.preventDefault(); }} ondrop={drop}>
  {#each config.blocks as block (block.type)}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <section class="block {block.type}" style={`grid-column:${block.x + 1} / span ${block.w};grid-row:${block.y + 1} / span ${block.h}`} draggable={editing} ondragstart={(event) => startDrag(event, block)} ondragend={() => dragging = ''}>
      {#if editing}<span class="block-label">{labels[block.type]}</span>{/if}
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
      {:else}<small>{status === 'Không xác định' ? 'Không thể xác minh kết nối' : 'Hệ thống đang hoạt động'}</small>{/if}
    </section>
  {/each}
</div>
<style>
  .canvas{display:grid;grid-template-columns:repeat(12,minmax(0,1fr));grid-template-rows:repeat(var(--rows),minmax(4rem,1fr));gap:.45rem;background:#e9f2f7;padding:1rem;color:#18394d;font-family:system-ui,sans-serif;min-height:28rem}.block{min-width:0;overflow:auto;background:#f7fbfc;border:1px solid #c7dbe2;border-radius:.35rem;padding:.7rem;display:flex;flex-direction:column;justify-content:center}.editing .block{cursor:grab}.block-label{font-size:.7rem;color:#345d6a}.clock{font-size:2.2rem;line-height:1}.branding{flex-direction:row;align-items:center;justify-content:space-between;gap:1rem}.branding img{max-width:20%;max-height:3rem;object-fit:contain}.room h1{font-size:clamp(1.2rem,3vw,2.4rem);margin:0}.status{background:#e2f3ed}.state{font-size:clamp(1.7rem,4vw,3.4rem)}.available{color:#08763b}.upcoming ul{padding-left:1rem}.footer{background:#194c69;color:white}.footer small,small{font-size:.75rem}@media(max-width:600px){.canvas{padding:.4rem;gap:.2rem;grid-template-rows:repeat(var(--rows),minmax(3rem,1fr))}.block{padding:.3rem}.clock{font-size:1.2rem}}
</style>
