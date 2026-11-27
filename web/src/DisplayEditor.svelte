<script>
  import { onMount } from 'svelte';
  import DisplayCanvas from './DisplayCanvas.svelte';
  import { displayBlocks, defaultDisplayLayout, validDisplayLayout } from './display-layout.js';
  let { device, csrf, onclose } = $props();
  let config = $state(null), selected = $state('room'), message = $state(''), busy = $state(false), saved = $state('');
  let width = $state(''), height = $state('');
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
  <header><div><h3 id="editor-title">Thiết kế: {device.name}</h3><p>Bản xem trước dùng dữ liệu minh họa. Kéo khối hoặc dùng điều khiển vị trí bên dưới.</p></div><button type="button" onclick={close}>Đóng</button></header>
  {#if message}<p role="status">{message}</p>{/if}
  {#if config}
    <div class="workspace"><div><p>{config.width ? `${config.width} × ${config.height} px · Vừa khung` : 'Chưa khai báo độ phân giải; xem trước theo bố cục cũ.'}</p><DisplayCanvas {config} data={{room:'Phòng họp', entries:[{starts:Math.floor(Date.now()/1000)+3600,ends:Math.floor(Date.now()/1000)+5400}]}} status="Trống" scheduleNow={Math.floor(Date.now()/1000)} editing onmove={move} onresize={(type,updates) => replace(type,updates)} onselect={(type) => selected = type} /></div>
      <aside>
        <fieldset><legend>Độ phân giải thiết bị (pixel)</legend><label>Chiều rộng <input type="number" min="1" max="8192" step="1" bind:value={width} /></label><label>Chiều cao <input type="number" min="1" max="8192" step="1" bind:value={height} /></label><button type="button" onclick={applyResolution}>Áp dụng độ phân giải</button>{#if config.rows}<label>Số hàng lưới <input type="number" min="7" max="24" value={config.rows} onchange={(e) => { const input = e.currentTarget, rows = Number(input.value); if (Number.isInteger(rows) && rows >= 7 && rows <= 24 && config.blocks.every(b => b.y + b.h <= rows)) { config = { ...config, rows }; message = ''; } else message = 'Số hàng phải từ 7 đến 24 và chứa toàn bộ các khối.'; input.value = String(config.rows); }} /></label>{/if}</fieldset>
        <label>Tên tổ chức <input maxlength="128" bind:value={config.organization} /></label>
        <label>Đường dẫn logo nội bộ <input maxlength="256" placeholder="/logo.png" bind:value={config.logo} /></label>
        <fieldset><legend>Khối hiển thị</legend>{#each displayBlocks as type}<label><input type="checkbox" checked={config.blocks.some(b => b.type === type)} onchange={(e) => toggle(type,e.currentTarget.checked)} /> {labels[type]}</label>{/each}</fieldset>
        <fieldset><legend>Lớp (trên cùng hiển thị trước)</legend><button type="button" onclick={() => add('text')}>Thêm chữ</button><button type="button" onclick={() => add('rectangle')}>Thêm hình chữ nhật</button><label>Chọn lớp <select bind:value={selected}>{#each [...config.blocks].reverse() as b}<option value={key(b)}>{labels[b.type]} {b.id ? `(${b.id.slice(-6)})` : ''}</option>{/each}</select></label><button type="button" onclick={() => reorder(1)}>Đưa lên trước</button><button type="button" onclick={() => reorder(-1)}>Đưa ra sau</button>{#if block?.id}<button type="button" onclick={() => { if (window.confirm('Xóa lớp này?')) { config = { ...config, blocks:config.blocks.filter(b => key(b) !== selected) }; selected = config.blocks.length ? key(config.blocks[0]) : ''; } }}>Xóa lớp</button>{/if}</fieldset>
        {#if block?.type === 'text'}<label>Nội dung (tối đa 512 byte UTF-8) <textarea value={block.text || ''} oninput={(e) => { const input = e.currentTarget; if (new TextEncoder().encode(input.value).length <= 512) replace(selected, { text:input.value }); else { input.value = block.text || ''; message = 'Nội dung vượt quá 512 byte.'; } }}></textarea></label>{/if}
        {#if block?.type === 'rectangle'}<label>Màu nền <input type="color" value={block.color || '#dce5db'} oninput={(e) => replace(selected, { color:e.currentTarget.value })} /></label>{/if}
        {#if block}<div class="coordinates">{#each ['x','y','w','h'] as field}<label>{({x:'Cột',y:'Hàng',w:'Rộng',h:'Cao'})[field]} <input type="number" min={field === 'x' || field === 'y' ? 0 : 1} max={field === 'x' || field === 'w' ? 12 : 24} value={block[field]} onchange={(e) => { const input = e.currentTarget; replace(selected, { [field]:Number(input.value) }); input.value = String(config.blocks.find(b => key(b) === selected)[field]); }} /></label>{/each}</div>{/if}
        <button type="button" onclick={() => { if (window.confirm('Thay bố cục hiện tại bằng mẫu mặc định?')) { config = { ...config, blocks:defaultDisplayLayout.map(b => ({ ...b })) }; selected = 'room'; message = ''; } }}>Dùng mẫu mặc định</button>
        <button type="button" disabled={busy} onclick={save}>{busy ? 'Đang lưu…' : 'Lưu thiết kế'}</button>
        <a href={`/display/${device.public_id}`} target="_blank" rel="noreferrer">Mở màn hình công khai</a>
      </aside></div>
  {/if}
</section>
<style>.editor{margin-block:1.5rem;padding:1.5rem;border:1px solid #d8e2d7;border-radius:.75rem;background:#f5f7f2}header{display:flex;align-items:start;justify-content:space-between;gap:1rem;margin-bottom:1rem}header h3{margin:0}header p{margin:.5rem 0 0}.workspace{display:grid;grid-template-columns:minmax(0,3fr) minmax(15rem,1fr);gap:1.5rem;align-items:start}.workspace>div{min-width:0;overflow:auto}aside{display:grid;align-content:start;gap:.8rem}label{display:grid;gap:.25rem}fieldset{border:1px solid #d8e2d7;border-radius:.5rem}fieldset label{display:block}.coordinates{display:grid;grid-template-columns:1fr 1fr;gap:.5rem}input,select,button{font:inherit;max-width:100%}button{padding:.5rem}@media(max-width:850px){.workspace{grid-template-columns:1fr}.editor{padding:1rem}}</style>
