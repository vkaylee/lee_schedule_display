<script>
  import { onMount } from 'svelte';
  import Banner from './Banner.svelte';
  import DisplayCanvas from './DisplayCanvas.svelte';
  import { displayBlocks, defaultDisplayLayout, validDisplayLayout } from './display-layout.js';
  let { device, csrf, onclose } = $props();
  let config = $state(null), selected = $state('room'), message = $state(null), busy = $state(false), saved = $state('');
  let width = $state(''), height = $state('');
  let zoomMode = $state('width'), zoom = $state(1);
  const labels = { clock:'Giờ và ngày', branding:'Tổ chức', room:'Tên phòng', status:'Trạng thái', upcoming:'Lịch tiếp theo', footer:'Kết nối', text:'Chữ', rectangle:'Hình chữ nhật' };
  const key = b => b.id || b.type;
  let block = $derived(config?.blocks.find(b => key(b) === selected));
  let resolutionPending = $derived(config && (String(width || '') !== String(config.width || '') || String(height || '') !== String(config.height || '')));
  let dirty = $derived(config && (resolutionPending || JSON.stringify(config) !== saved));
  const setMessage = (text, kind = 'error') => { message = text ? { text, kind } : null; };
  function replace(type, updates) {
    const blocks = config.blocks.map(b => key(b) === type ? { ...b, ...updates } : b);
    if (validDisplayLayout(blocks) && (!config.rows || blocks.every(b => b.y + b.h <= config.rows))) { config = { ...config, blocks }; setMessage(''); } else setMessage('Vị trí hoặc kích thước không hợp lệ.');
  }
  function add(type) {
    if (config.blocks.length >= 32) { setMessage('Đã đạt giới hạn 32 lớp.'); return; }
    const id = `layer_${Array.from(crypto.getRandomValues(new Uint8Array(12)), byte => byte.toString(16).padStart(2, '0')).join('')}`;
    const layer = { id, type, x:0, y:0, w:4, h:2, ...(type === 'text' ? { text:'Văn bản' } : { color:'#dce5db' }) };
    config = { ...config, blocks:[...config.blocks, layer] }; selected = id; setMessage('');
  }
  function reorder(delta) {
    const blocks = [...config.blocks], index = blocks.findIndex(b => key(b) === selected), next = index + delta;
    if (index < 0 || next < 0 || next >= blocks.length) return;
    [blocks[index], blocks[next]] = [blocks[next], blocks[index]]; config = { ...config, blocks };
  }
  function move(type, x, y) { selected = type; replace(type, { x:Math.max(0,x), y:Math.max(0,y) }); }
  function applyResolution() {
    const w = Number(width), h = Number(height);
    if (!Number.isInteger(w) || !Number.isInteger(h) || w < 1 || h < 1 || w > 8192 || h > 8192) { setMessage('Nhập chiều rộng và chiều cao từ 1 đến 8192 pixel.'); return; }
    config = { ...config, width:w, height:h, rows:config.rows || Math.max(7, ...config.blocks.map(b => b.y + b.h)) };
    setMessage('');
  }
  function toggle(type, checked) {
    if (!checked) { config = { ...config, blocks:config.blocks.filter(b => b.type !== type) }; if (selected === type) selected = config.blocks.length ? key(config.blocks[0]) : ''; return; }
    const original = defaultDisplayLayout.find(b => b.type === type);
    for (let y = 0; y <= (config.rows || 24) - original.h; y++) for (let x = 0; x <= 12 - original.w; x++) {
      const blocks = [...config.blocks, { ...original, x, y }];
      if (validDisplayLayout(blocks)) { config = { ...config, blocks }; selected = type; setMessage(''); return; }
    }
    setMessage('Không còn đủ chỗ cho khối này.');
  }
  async function save() {
    if (resolutionPending) { setMessage('Áp dụng độ phân giải trước khi lưu.', 'warning'); return; }
    busy = true; setMessage('');
    try {
      const submitted = JSON.stringify(config);
      const response = await fetch(`/api/devices/${device.id}/display`, { method:'POST', headers:{ 'Content-Type':'application/json', 'X-CSRF-Token':csrf }, body:submitted });
      if (!response.ok) throw new Error();
      saved = submitted;
      if (JSON.stringify(config) === submitted) setMessage('Đã lưu bố cục.', 'success');
      else setMessage('Đã lưu phiên bản trước; còn thay đổi chưa lưu.', 'warning');
    } catch { setMessage('Không thể lưu. Kiểm tra thông tin và thử lại.'); } finally { busy = false; }
  }
  function close() { if (!config || (!resolutionPending && JSON.stringify(config) === saved) || window.confirm('Bỏ các thay đổi chưa lưu?')) onclose(); }
  onMount(async () => {
    try { const r = await fetch(`/api/devices/${device.id}/display`, { cache:'no-store' }); if (!r.ok) throw new Error(); config = await r.json(); width = config.width || ''; height = config.height || ''; saved = JSON.stringify(config); }
    catch { setMessage('Không thể tải bố cục.'); }
  });
</script>

<section class="editor" aria-labelledby="editor-title">
  <header class="editor-header">
    <div>
      <div class="editor-title-row">
        <h3 id="editor-title">Thiết kế: {device.name}</h3>
        {#if config}
          <span class={dirty ? 'ui-pill ui-pill--warning' : 'ui-pill ui-pill--success'}>
            {dirty ? 'Chưa lưu thay đổi' : 'Đã đồng bộ'}
          </span>
        {/if}
      </div>
      <p>Bản xem trước dùng dữ liệu minh họa. Kéo khối hoặc dùng các điều khiển bên dưới.</p>
    </div>
    <button class="secondary" type="button" onclick={close}>Đóng</button>
  </header>

  {#if message}<Banner kind={message.kind} text={message.text} />{/if}

  {#if !config && !message}
    <p class="ui-loading" role="status">
      <span class="ui-spinner" aria-hidden="true"></span>
      <span>Đang tải bố cục…</span>
    </p>
  {:else if config}
    <div class="workspace">
      <div class="stage">
        <div class="stage-toolbar">
          <div>
            <strong>Khung thiết kế</strong>
            <p>{config.width ? `${config.width} × ${config.height} px` : 'Chưa khai báo độ phân giải'}</p>
          </div>
          {#if config.width}
            <div class="zoom-controls" role="group" aria-label="Thu phóng khung thiết kế">
              <button class="secondary" type="button" aria-pressed={zoomMode === 'width' && zoom === 1} onclick={() => { zoomMode = 'width'; zoom = 1; }}>Vừa chiều ngang</button>
              <button class="secondary" type="button" aria-pressed={zoomMode === 'fit' && zoom === 1} onclick={() => { zoomMode = 'fit'; zoom = 1; }}>Vừa màn hình</button>
              <button class="secondary zoom-step" type="button" aria-label="Thu nhỏ" disabled={zoom <= 0.5} onclick={() => zoom = Math.max(0.5, Math.round((zoom - 0.25) * 100) / 100)}>−</button>
              <output class="mono" aria-label="Mức thu phóng">{Math.round(zoom * 100)}%</output>
              <button class="secondary zoom-step" type="button" aria-label="Phóng to" disabled={zoom >= 2} onclick={() => zoom = Math.min(2, Math.round((zoom + 0.25) * 100) / 100)}>+</button>
            </div>
          {/if}
        </div>
        {#if config.width}
          <p class="stage-hint">Thu phóng không thay đổi bố cục đã lưu. Khi phóng lớn, cuộn ngang để xem toàn bộ chiều rộng.</p>
        {/if}
        <DisplayCanvas {config} {zoomMode} {zoom} {selected} data={{room:'Phòng họp', entries:[{starts:Math.floor(Date.now()/1000)+3600,ends:Math.floor(Date.now()/1000)+5400}]}} status="Trống" scheduleNow={Math.floor(Date.now()/1000)} editing onmove={move} onresize={(type,updates) => replace(type,updates)} onselect={(type) => selected = type} />
      </div>

      <aside class="inspector">
        <fieldset>
          <legend>Độ phân giải thiết bị (pixel)</legend>
          <div class="grid-2">
            <label>Chiều rộng <input type="number" min="1" max="8192" step="1" bind:value={width} /></label>
            <label>Chiều cao <input type="number" min="1" max="8192" step="1" bind:value={height} /></label>
          </div>
          <button class="secondary" type="button" onclick={applyResolution}>Áp dụng độ phân giải</button>
          {#if config.rows}
            <label>Số hàng lưới <input type="number" min="7" max="24" value={config.rows} onchange={(e) => { const input = e.currentTarget, rows = Number(input.value); if (Number.isInteger(rows) && rows >= 7 && rows <= 24 && config.blocks.every(b => b.y + b.h <= rows)) { config = { ...config, rows }; setMessage(''); } else setMessage('Số hàng phải từ 7 đến 24 và chứa toàn bộ các khối.'); input.value = String(config.rows); }} /></label>
          {/if}
        </fieldset>

        <fieldset>
          <legend>Tổ chức &amp; Thương hiệu</legend>
          <label>Tên tổ chức <input maxlength="128" bind:value={config.organization} /></label>
          <label>Đường dẫn logo nội bộ <input maxlength="256" placeholder="/logo.png" bind:value={config.logo} /></label>
        </fieldset>

        <fieldset class="block-toggles">
          <legend>Khối hiển thị</legend>
          <div class="toggle-grid">
            {#each displayBlocks as type}
              <label class="toggle-chip"><input type="checkbox" checked={config.blocks.some(b => b.type === type)} onchange={(e) => toggle(type,e.currentTarget.checked)} /> <span>{labels[type]}</span></label>
            {/each}
          </div>
        </fieldset>

        <fieldset>
          <legend>Lớp (trên cùng hiển thị trước)</legend>
          <div class="grid-2">
            <button class="secondary" type="button" onclick={() => add('text')}>Thêm chữ</button>
            <button class="secondary" type="button" onclick={() => add('rectangle')}>Thêm hình chữ nhật</button>
          </div>
          <label>Chọn lớp
            <select bind:value={selected}>
              {#each [...config.blocks].reverse() as b}
                <option value={key(b)}>{labels[b.type]} {b.id ? `(${b.id.slice(-6)})` : ''}</option>
              {/each}
            </select>
          </label>
          <div class="grid-2">
            <button class="secondary" type="button" onclick={() => reorder(1)}>Đưa lên trước</button>
            <button class="secondary" type="button" onclick={() => reorder(-1)}>Đưa ra sau</button>
          </div>
          {#if block?.id}
            <button class="danger" type="button" onclick={() => { if (window.confirm('Xóa lớp này?')) { config = { ...config, blocks:config.blocks.filter(b => key(b) !== selected) }; selected = config.blocks.length ? key(config.blocks[0]) : ''; } }}>Xóa lớp</button>
          {/if}
        </fieldset>

        {#if block}
          <fieldset class="selected-props">
            <legend>Thuộc tính: {labels[block.type]}</legend>
            {#if block.type === 'text'}
              <label>Nội dung (tối đa 512 byte UTF-8)
                <textarea rows="3" value={block.text || ''} oninput={(e) => { const input = e.currentTarget; if (new TextEncoder().encode(input.value).length <= 512) replace(selected, { text:input.value }); else { input.value = block.text || ''; setMessage('Nội dung vượt quá 512 byte.'); } }}></textarea>
              </label>
              <div class="grid-3">
                <label>Cỡ chữ (px) <input type="number" min="8" max="256" value={block.fontSize ?? 24} onchange={(e) => replace(selected, { fontSize:Number(e.currentTarget.value) })} /></label>
                <label>Padding (px) <input type="number" min="0" max="128" value={block.padding ?? 16} onchange={(e) => replace(selected, { padding:Number(e.currentTarget.value) })} /></label>
                <label>Margin (px) <input type="number" min="0" max="128" value={block.margin ?? 0} onchange={(e) => replace(selected, { margin:Number(e.currentTarget.value) })} /></label>
              </div>
              <div class="grid-2">
                <label>Căn ngang
                  <select value={block.align || 'left'} onchange={(e) => replace(selected, { align:e.currentTarget.value })}>
                    <option value="left">Trái</option><option value="center">Giữa</option><option value="right">Phải</option>
                  </select>
                </label>
                <label>Căn dọc
                  <select value={block.valign || 'center'} onchange={(e) => replace(selected, { valign:e.currentTarget.value })}>
                    <option value="top">Trên</option><option value="center">Giữa</option><option value="bottom">Dưới</option>
                  </select>
                </label>
              </div>
            {/if}

            {#if block.type === 'rectangle'}
              <label class="color-row">Màu nền <input type="color" value={block.color || '#dce5db'} oninput={(e) => replace(selected, { color:e.currentTarget.value })} /></label>
            {/if}

            <div class="coordinates">
              {#each ['x','y','w','h'] as field}
                <label>{({x:'Cột',y:'Hàng',w:'Rộng',h:'Cao'})[field]}
                  <input type="number" min={field === 'x' || field === 'y' ? 0 : 1} max={field === 'x' || field === 'w' ? 12 : 24} value={block[field]} onchange={(e) => { const input = e.currentTarget; replace(selected, { [field]:Number(input.value) }); input.value = String(config.blocks.find(b => key(b) === selected)[field]); }} />
                </label>
              {/each}
            </div>
          </fieldset>
        {/if}

        <div class="inspector-actions">
          <button type="button" disabled={busy} onclick={save}>{busy ? 'Đang lưu…' : 'Lưu thiết kế'}</button>
          <button class="secondary" type="button" onclick={() => { if (window.confirm('Thay bố cục hiện tại bằng mẫu mặc định?')) { config = { ...config, blocks:defaultDisplayLayout.map(b => ({ ...b })) }; selected = 'room'; setMessage(''); } }}>Dùng mẫu mặc định</button>
          <a class="public-link" href={`/display/${device.public_id}`} target="_blank" rel="noreferrer">Mở màn hình công khai ↗</a>
        </div>
      </aside>
    </div>
  {/if}
</section>

<style>
  .editor {
    margin-block: 1.5rem;
    padding: 1.4rem;
    border: 1px solid var(--accent-border);
    border-radius: var(--radius-lg);
    background: var(--bg-subtle);
    box-shadow: var(--shadow-sm);
  }

  .editor-header {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 1rem;
    padding-bottom: 1rem;
    border-bottom: 1px solid var(--border-subtle);
  }

  .editor-title-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.6rem;
  }

  .editor-header h3 {
    margin: 0;
    font-size: 1.2rem;
  }

  .editor-header p {
    margin: 0.35rem 0 0;
    font-size: 0.9rem;
    color: var(--ink-muted);
  }

  .workspace {
    display: grid;
    grid-template-columns: minmax(0, 1.55fr) minmax(20rem, 0.95fr);
    gap: 1.35rem;
    align-items: start;
  }

  .stage {
    min-width: 0;
  }

  .stage-toolbar {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    margin-bottom: 0.35rem;
  }

  .stage-toolbar p,
  .stage-hint {
    margin: 0.2rem 0 0;
    color: var(--ink-muted);
    font-size: 0.85rem;
  }

  .stage-hint {
    margin-bottom: 0.65rem;
  }

  .zoom-controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.35rem;
  }

  .zoom-controls button {
    min-height: 2.35rem;
    padding: 0.35rem 0.7rem;
    font-size: 0.84rem;
  }

  .zoom-controls .zoom-step {
    min-width: 2.35rem;
    padding-inline: 0.5rem;
  }

  .zoom-controls output {
    min-width: 3.1rem;
    text-align: center;
    font-weight: 600;
  }

  .zoom-controls button[aria-pressed='true'] {
    background: var(--accent);
    color: var(--ink-on-accent);
    border-color: var(--accent);
  }

  .inspector {
    display: grid;
    gap: 0.9rem;
    min-width: 0;
  }

  fieldset {
    margin: 0;
    padding: 0.95rem 1rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    background: var(--bg-surface);
    display: grid;
    gap: 0.65rem;
    min-width: 0;
  }

  fieldset legend {
    padding-inline: 0.35rem;
    font-size: 0.82rem;
    font-weight: 700;
    letter-spacing: 0.03em;
    color: var(--ink-secondary);
  }

  label {
    display: grid;
    gap: 0.25rem;
    min-width: 0;
  }

  .grid-2 {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.5rem;
  }

  .grid-3 {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0.5rem;
  }

  .toggle-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.45rem;
  }

  .toggle-chip {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.45rem 0.6rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--bg-subtle);
    font-size: 0.85rem;
    cursor: pointer;
  }

  .toggle-chip:has(input:checked) {
    background: var(--accent-soft);
    border-color: var(--accent-border);
    color: #134e31;
  }

  .color-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .color-row input[type='color'] {
    width: 3.5rem;
    height: 2.35rem;
    padding: 0.15rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: #ffffff;
    cursor: pointer;
  }

  .coordinates {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 0.45rem;
  }

  .inspector-actions {
    display: grid;
    gap: 0.55rem;
  }

  .public-link {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 2.75rem;
    padding: 0.55rem 1rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--bg-surface);
    color: var(--accent);
    font-weight: 650;
    font-size: 0.9rem;
    text-decoration: none;
  }

  .public-link:hover {
    background: var(--bg-subtle);
    border-color: var(--accent);
  }

  @media (max-width: 1080px) {
    .workspace {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 540px) {
    .editor {
      padding: 1rem;
    }
    .grid-3, .coordinates, .toggle-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }
</style>
