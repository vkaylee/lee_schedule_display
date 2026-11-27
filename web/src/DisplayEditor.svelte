<script>
  import { onMount } from 'svelte';
  import DisplayCanvas from './DisplayCanvas.svelte';
  import { displayBlocks, defaultDisplayLayout, validDisplayLayout } from './display-layout.js';
  let { device, csrf, onclose } = $props();
  let config = $state(null), selected = $state('room'), message = $state(''), busy = $state(false), saved = $state('');
  let width = $state(''), height = $state('');
  let zoomMode = $state('width'), zoom = $state(1);
  const labels = { clock:'Giờ và ngày', branding:'Tổ chức', room:'Tên phòng', status:'Trạng thái', upcoming:'Lịch tiếp theo', footer:'Kết nối', text:'Chữ', rectangle:'Hình chữ nhật' };
  const key = b => b.id || b.type;
  let block = $derived(config?.blocks.find(b => key(b) === selected));
  function replace(type, updates) {
    const blocks = config.blocks.map(b => key(b) === type ? { ...b, ...updates } : b);
    if (validDisplayLayout(blocks) && (!config.rows || blocks.every(b => b.y + b.h <= config.rows))) { config = { ...config, blocks }; message = ''; } else message = 'Vị trí hoặc kích thước không hợp lệ.';
  }
  function add(type) {
    if (config.blocks.length >= 32) { message = 'Đã đạt giới hạn 32 lớp.'; return; }
    const id = `layer_${Array.from(crypto.getRandomValues(new Uint8Array(12)), byte => byte.toString(16).padStart(2, '0')).join('')}`;
    const layer = { id, type, x:0, y:0, w:4, h:2, ...(type === 'text' ? { text:'Văn bản' } : { color:'#dce5db' }) };
    config = { ...config, blocks:[...config.blocks, layer] }; selected = id; message = '';
  }
  function reorder(delta) {
    const blocks = [...config.blocks], index = blocks.findIndex(b => key(b) === selected), next = index + delta;
    if (index < 0 || next < 0 || next >= blocks.length) return;
    [blocks[index], blocks[next]] = [blocks[next], blocks[index]]; config = { ...config, blocks };
  }
  function move(type, x, y) { selected = type; replace(type, { x:Math.max(0,x), y:Math.max(0,y) }); }
  function applyResolution() {
    const w = Number(width), h = Number(height);
    if (!Number.isInteger(w) || !Number.isInteger(h) || w < 1 || h < 1 || w > 8192 || h > 8192) { message = 'Nhập chiều rộng và chiều cao từ 1 đến 8192 pixel.'; return; }
    config = { ...config, width:w, height:h, rows:config.rows || Math.max(7, ...config.blocks.map(b => b.y + b.h)) };
    message = '';
  }
  function toggle(type, checked) {
    if (!checked) { config = { ...config, blocks:config.blocks.filter(b => b.type !== type) }; if (selected === type) selected = config.blocks.length ? key(config.blocks[0]) : ''; return; }
    const original = defaultDisplayLayout.find(b => b.type === type);
    for (let y = 0; y <= (config.rows || 24) - original.h; y++) for (let x = 0; x <= 12 - original.w; x++) {
      const blocks = [...config.blocks, { ...original, x, y }];
      if (validDisplayLayout(blocks)) { config = { ...config, blocks }; selected = type; message = ''; return; }
    }
    message = 'Không còn đủ chỗ cho khối này.';
  }
  let resolutionPending = $derived(config && (String(width || '') !== String(config.width || '') || String(height || '') !== String(config.height || '')));
  async function save() {
    if (resolutionPending) { message = 'Áp dụng độ phân giải trước khi lưu.'; return; }
    busy = true; message = '';
    try {
      const submitted = JSON.stringify(config);
      const response = await fetch(`/api/devices/${device.id}/display`, { method:'POST', headers:{ 'Content-Type':'application/json', 'X-CSRF-Token':csrf }, body:submitted });
      if (!response.ok) throw new Error(); saved = submitted; message = JSON.stringify(config) === submitted ? 'Đã lưu bố cục.' : 'Đã lưu phiên bản trước; còn thay đổi chưa lưu.';
    } catch { message = 'Không thể lưu. Kiểm tra thông tin và thử lại.'; } finally { busy = false; }
  }
  function close() { if (!config || (!resolutionPending && JSON.stringify(config) === saved) || window.confirm('Bỏ các thay đổi chưa lưu?')) onclose(); }
  onMount(async () => {
    try { const r = await fetch(`/api/devices/${device.id}/display`, { cache:'no-store' }); if (!r.ok) throw new Error(); config = await r.json(); width = config.width || ''; height = config.height || ''; saved = JSON.stringify(config); }
    catch { message = 'Không thể tải bố cục.'; }
  });
</script>
<section class="editor" aria-labelledby="editor-title">
  <header><div><h3 id="editor-title">Thiết kế: {device.name}</h3><p>Bản xem trước dùng dữ liệu minh họa. Kéo khối hoặc dùng các điều khiển bên dưới.</p></div><button type="button" onclick={close}>Đóng</button></header>
  {#if message}<p role="status">{message}</p>{/if}
  {#if config}
    <div class="workspace"><div class="stage"><div class="stage-toolbar"><div><strong>Khung thiết kế</strong><p>{config.width ? `${config.width} × ${config.height} px` : 'Chưa khai báo độ phân giải'}</p></div>{#if config.width}<div class="zoom-controls" role="group" aria-label="Thu phóng khung thiết kế"><button type="button" aria-pressed={zoomMode === 'width' && zoom === 1} onclick={() => { zoomMode = 'width'; zoom = 1; }}>Vừa chiều ngang</button><button type="button" aria-pressed={zoomMode === 'fit' && zoom === 1} onclick={() => { zoomMode = 'fit'; zoom = 1; }}>Vừa màn hình</button><button type="button" aria-label="Thu nhỏ" disabled={zoom <= 0.5} onclick={() => zoom = Math.max(0.5, Math.round((zoom - 0.25) * 100) / 100)}>−</button><output aria-label="Mức thu phóng">{Math.round(zoom * 100)}%</output><button type="button" aria-label="Phóng to" disabled={zoom >= 2} onclick={() => zoom = Math.min(2, Math.round((zoom + 0.25) * 100) / 100)}>+</button></div>{/if}</div>{#if config.width}<p class="stage-hint">Thu phóng không thay đổi bố cục đã lưu. Khi phóng lớn, cuộn ngang để xem toàn bộ chiều rộng.</p>{/if}<DisplayCanvas {config} {zoomMode} {zoom} data={{room:'Phòng họp', entries:[{starts:Math.floor(Date.now()/1000)+3600,ends:Math.floor(Date.now()/1000)+5400}]}} status="Trống" scheduleNow={Math.floor(Date.now()/1000)} editing onmove={move} onresize={(type,updates) => replace(type,updates)} onselect={(type) => selected = type} /></div>
      <aside>
        <fieldset><legend>Độ phân giải thiết bị (pixel)</legend><label>Chiều rộng <input type="number" min="1" max="8192" step="1" bind:value={width} /></label><label>Chiều cao <input type="number" min="1" max="8192" step="1" bind:value={height} /></label><button type="button" onclick={applyResolution}>Áp dụng độ phân giải</button>{#if config.rows}<label>Số hàng lưới <input type="number" min="7" max="24" value={config.rows} onchange={(e) => { const input = e.currentTarget, rows = Number(input.value); if (Number.isInteger(rows) && rows >= 7 && rows <= 24 && config.blocks.every(b => b.y + b.h <= rows)) { config = { ...config, rows }; message = ''; } else message = 'Số hàng phải từ 7 đến 24 và chứa toàn bộ các khối.'; input.value = String(config.rows); }} /></label>{/if}</fieldset>
        <label>Tên tổ chức <input maxlength="128" bind:value={config.organization} /></label>
        <label>Đường dẫn logo nội bộ <input maxlength="256" placeholder="/logo.png" bind:value={config.logo} /></label>
        <fieldset class="block-toggles"><legend>Khối hiển thị</legend>{#each displayBlocks as type}<label><input type="checkbox" checked={config.blocks.some(b => b.type === type)} onchange={(e) => toggle(type,e.currentTarget.checked)} /> {labels[type]}</label>{/each}</fieldset>
        <fieldset><legend>Lớp (trên cùng hiển thị trước)</legend><button type="button" onclick={() => add('text')}>Thêm chữ</button><button type="button" onclick={() => add('rectangle')}>Thêm hình chữ nhật</button><label>Chọn lớp <select bind:value={selected}>{#each [...config.blocks].reverse() as b}<option value={key(b)}>{labels[b.type]} {b.id ? `(${b.id.slice(-6)})` : ''}</option>{/each}</select></label><button type="button" onclick={() => reorder(1)}>Đưa lên trước</button><button type="button" onclick={() => reorder(-1)}>Đưa ra sau</button>{#if block?.id}<button type="button" onclick={() => { if (window.confirm('Xóa lớp này?')) { config = { ...config, blocks:config.blocks.filter(b => key(b) !== selected) }; selected = config.blocks.length ? key(config.blocks[0]) : ''; } }}>Xóa lớp</button>{/if}</fieldset>
        {#if block?.type === 'text'}<label>Nội dung (tối đa 512 byte UTF-8) <textarea value={block.text || ''} oninput={(e) => { const input = e.currentTarget; if (new TextEncoder().encode(input.value).length <= 512) replace(selected, { text:input.value }); else { input.value = block.text || ''; message = 'Nội dung vượt quá 512 byte.'; } }}></textarea></label>{/if}
        {#if block?.type === 'text'}<fieldset class="text-style"><legend>Kiểu chữ</legend><label>Cỡ chữ (px) <input type="number" min="8" max="256" value={block.fontSize ?? 24} onchange={(e) => replace(selected, { fontSize:Number(e.currentTarget.value) })} /></label><label>Padding (px) <input type="number" min="0" max="128" value={block.padding ?? 16} onchange={(e) => replace(selected, { padding:Number(e.currentTarget.value) })} /></label><label>Margin (px) <input type="number" min="0" max="128" value={block.margin ?? 0} onchange={(e) => replace(selected, { margin:Number(e.currentTarget.value) })} /></label><label>Căn ngang <select value={block.align || 'left'} onchange={(e) => replace(selected, { align:e.currentTarget.value })}><option value="left">Trái</option><option value="center">Giữa</option><option value="right">Phải</option></select></label><label>Căn dọc <select value={block.valign || 'center'} onchange={(e) => replace(selected, { valign:e.currentTarget.value })}><option value="top">Trên</option><option value="center">Giữa</option><option value="bottom">Dưới</option></select></label></fieldset>{/if}
        {#if block?.type === 'rectangle'}<label>Màu nền <input type="color" value={block.color || '#dce5db'} oninput={(e) => replace(selected, { color:e.currentTarget.value })} /></label>{/if}
        {#if block}<div class="coordinates">{#each ['x','y','w','h'] as field}<label>{({x:'Cột',y:'Hàng',w:'Rộng',h:'Cao'})[field]} <input type="number" min={field === 'x' || field === 'y' ? 0 : 1} max={field === 'x' || field === 'w' ? 12 : 24} value={block[field]} onchange={(e) => { const input = e.currentTarget; replace(selected, { [field]:Number(input.value) }); input.value = String(config.blocks.find(b => key(b) === selected)[field]); }} /></label>{/each}</div>{/if}
        <button type="button" onclick={() => { if (window.confirm('Thay bố cục hiện tại bằng mẫu mặc định?')) { config = { ...config, blocks:defaultDisplayLayout.map(b => ({ ...b })) }; selected = 'room'; message = ''; } }}>Dùng mẫu mặc định</button>
        <button type="button" disabled={busy} onclick={save}>{busy ? 'Đang lưu…' : 'Lưu thiết kế'}</button>
        <a href={`/display/${device.public_id}`} target="_blank" rel="noreferrer">Mở màn hình công khai</a>
      </aside></div>
  {/if}
</section>
<style>
  .workspace aside .block-toggles label{display:flex;align-items:center;gap:.3rem}
  .workspace{display:flex;flex-direction:column}.workspace .stage{width:100%;overflow:visible}.workspace aside{width:100%;display:flex;flex-wrap:wrap;align-items:flex-start;gap:1rem}.workspace aside>fieldset{flex:1 1 15rem;min-width:0;display:flex;flex-wrap:wrap;align-items:end;gap:.5rem}.workspace aside>fieldset legend{padding-inline:.25rem}.workspace aside>fieldset label{min-width:0}.workspace aside>fieldset:first-child label{flex:1 1 6rem}.workspace aside>fieldset:nth-of-type(2) label{white-space:nowrap}.workspace aside>label{flex:1 1 14rem}.workspace aside>.coordinates{flex:1 1 16rem;grid-template-columns:repeat(4,minmax(0,1fr))}.workspace aside>.coordinates input,.workspace aside input:not([type=checkbox]):not([type=color]),.workspace aside select,.workspace aside textarea{box-sizing:border-box;width:100%;min-width:0}.workspace aside>button,.workspace aside>a{align-self:end}.workspace aside>fieldset:nth-of-type(3) label{flex:1 1 100%}
  .stage-toolbar{display:flex;flex-wrap:wrap;justify-content:space-between;align-items:center;gap:.75rem}.stage-toolbar p,.stage-hint{margin:.25rem 0;color:#486457;font-size:.875rem}.stage-hint{margin:.5rem 0}.zoom-controls{display:flex;flex-wrap:wrap;align-items:center;gap:.35rem}.zoom-controls output{min-width:3rem;text-align:center;font-variant-numeric:tabular-nums}.zoom-controls button[aria-pressed=true]{outline:2px solid #234b37;outline-offset:2px}.editor{margin-block:1.5rem;padding:1.5rem;border:1px solid #d8e2d7;border-radius:.75rem;background:#f5f7f2}header{display:flex;align-items:start;justify-content:space-between;gap:1rem;margin-bottom:1rem}header h3{margin:0}header p{margin:.5rem 0 0}.workspace{gap:1.5rem;align-items:stretch}.workspace>div{min-width:0}label{display:grid;gap:.25rem}fieldset{border:1px solid #d8e2d7;border-radius:.5rem}fieldset label{display:grid}.coordinates{display:grid;grid-template-columns:1fr 1fr;gap:.5rem}input,select,button{font:inherit;max-width:100%}button{padding:.5rem}@media(max-width:850px){.workspace{grid-template-columns:1fr}.editor{padding:1rem}}</style>
