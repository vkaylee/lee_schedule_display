<script>
  import { onMount } from 'svelte';
  import DisplayCanvas from './DisplayCanvas.svelte';
  import { displayBlocks, defaultDisplayLayout, validDisplayLayout } from './display-layout.js';
  let { device, csrf, onclose } = $props();
  let config = $state(null), selected = $state('room'), message = $state(''), busy = $state(false), saved = $state('');
  const labels = { clock:'Giờ và ngày', branding:'Tổ chức', room:'Tên phòng', status:'Trạng thái', upcoming:'Lịch tiếp theo', footer:'Kết nối' };
  let block = $derived(config?.blocks.find(b => b.type === selected));
  function replace(type, updates) {
    const blocks = config.blocks.map(b => b.type === type ? { ...b, ...updates } : b);
    if (validDisplayLayout(blocks)) { config = { ...config, blocks }; message = ''; } else message = 'Vị trí hoặc kích thước không hợp lệ, hoặc chồng khối khác.';
  }
  function move(type, x, y) { selected = type; replace(type, { x:Math.max(0,x), y:Math.max(0,y) }); }
  function toggle(type, checked) {
    if (!checked) { config = { ...config, blocks:config.blocks.filter(b => b.type !== type) }; return; }
    const original = defaultDisplayLayout.find(b => b.type === type);
    for (let y = 0; y < 24; y++) for (let x = 0; x <= 12 - original.w; x++) {
      const blocks = [...config.blocks, { ...original, x, y }];
      if (validDisplayLayout(blocks)) { config = { ...config, blocks }; selected = type; message = ''; return; }
    }
    message = 'Không còn đủ chỗ cho khối này.';
  }
  async function save() {
    busy = true; message = '';
    try {
      const response = await fetch(`/api/devices/${device.id}/display`, { method:'POST', headers:{ 'Content-Type':'application/json', 'X-CSRF-Token':csrf }, body:JSON.stringify(config) });
      if (!response.ok) throw new Error(); saved = JSON.stringify(config); message = 'Đã lưu bố cục.';
    } catch { message = 'Không thể lưu. Kiểm tra thông tin và thử lại.'; } finally { busy = false; }
  }
  function close() { if (!config || JSON.stringify(config) === saved || window.confirm('Bỏ các thay đổi chưa lưu?')) onclose(); }
  onMount(async () => {
    try { const r = await fetch(`/api/devices/${device.id}/display`, { cache:'no-store' }); if (!r.ok) throw new Error(); config = await r.json(); saved = JSON.stringify(config); }
    catch { message = 'Không thể tải bố cục.'; }
  });
</script>
<section class="editor" aria-labelledby="editor-title">
  <header><div><h3 id="editor-title">Thiết kế: {device.name}</h3><p>Kéo khối trên bản xem trước hoặc dùng điều khiển vị trí bên dưới.</p></div><button type="button" onclick={close}>Đóng</button></header>
  {#if message}<p role="status">{message}</p>{/if}
  {#if config}
    <div class="workspace"><div><DisplayCanvas {config} data={{room:'Phòng họp', entries:[{starts:Math.floor(Date.now()/1000)+3600,ends:Math.floor(Date.now()/1000)+5400}]}} status="Trống" editing onmove={move} /></div>
      <aside>
        <label>Tên tổ chức <input maxlength="128" bind:value={config.organization} /></label>
        <label>Đường dẫn logo nội bộ <input maxlength="256" placeholder="/logo.png" bind:value={config.logo} /></label>
        <fieldset><legend>Khối hiển thị</legend>{#each displayBlocks as type}<label><input type="checkbox" checked={config.blocks.some(b => b.type === type)} onchange={(e) => toggle(type,e.currentTarget.checked)} /> {labels[type]}</label>{/each}</fieldset>
        <label>Chọn khối <select bind:value={selected}>{#each config.blocks as b}<option value={b.type}>{labels[b.type]}</option>{/each}</select></label>
        {#if block}<div class="coordinates">{#each ['x','y','w','h'] as key}<label>{({x:'Cột',y:'Hàng',w:'Rộng',h:'Cao'})[key]} <input type="number" min={key === 'x' || key === 'y' ? 0 : 1} max={key === 'x' || key === 'w' ? 12 : 24} value={block[key]} onchange={(e) => replace(selected, { [key]:Number(e.currentTarget.value) })} /></label>{/each}</div>{/if}
        <button type="button" onclick={() => config = { ...config, blocks:defaultDisplayLayout.map(b => ({ ...b })) }}>Dùng mẫu mặc định</button>
        <button type="button" disabled={busy} onclick={save}>{busy ? 'Đang lưu…' : 'Lưu thiết kế'}</button>
        <a href={`/display/${device.id}`} target="_blank" rel="noreferrer">Mở màn hình công khai</a>
      </aside></div>
  {/if}
</section>
<style>.editor{margin-block:1.5rem;padding:1rem;border:1px solid #bfd2d9;border-radius:.5rem}header{display:flex;justify-content:space-between;gap:1rem}header h3{margin:0}.workspace{display:grid;grid-template-columns:minmax(0,3fr) minmax(15rem,1fr);gap:1rem}aside{display:grid;align-content:start;gap:.8rem}label{display:grid;gap:.25rem}fieldset label{display:block}.coordinates{display:grid;grid-template-columns:1fr 1fr;gap:.5rem}input,select,button{font:inherit;max-width:100%}button{padding:.5rem}@media(max-width:850px){.workspace{grid-template-columns:1fr}}</style>
