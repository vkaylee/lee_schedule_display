<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import Banner from './Banner.svelte';
  import DisplayCanvas from './DisplayCanvas.svelte';
  import { displayBlocks, defaultDisplayLayout, validDisplayLayout } from './display-layout.js';
  const widgetIcons = { clock:'clock', branding:'brand', room:'rooms', status:'check-circle', upcoming:'calendar', footer:'monitor' };
  let { device, csrf, language = 'vi', onclose } = $props();
  const t = (vi, en) => language === 'en' ? en : vi;
  let config = $state(null), selected = $state('room'), message = $state(null), busy = $state(false), saved = $state('');
  let width = $state(''), height = $state('');
  let zoomMode = $state('width'), zoom = $state(1);
  let labels = $derived(language === 'en'
    ? { clock:'Clock & date', branding:'Organization', room:'Room name', status:'Status', upcoming:'Upcoming', footer:'Connection', text:'Text', rectangle:'Rectangle' }
    : { clock:'Giờ và ngày', branding:'Tổ chức', room:'Tên phòng', status:'Trạng thái', upcoming:'Lịch tiếp theo', footer:'Kết nối', text:'Chữ', rectangle:'Hình chữ nhật' });
  const key = b => b.id || b.type;
  let block = $derived(config?.blocks.find(b => key(b) === selected));
  let resolutionPending = $derived(config && (String(width || '') !== String(config.width || '') || String(height || '') !== String(config.height || '')));
  let dirty = $derived(config && (resolutionPending || JSON.stringify(config) !== saved));
  const setMessage = (vi, en = '', kind = 'error') => { message = vi ? { text: [vi, en || vi], kind } : null; };
  function replace(type, updates) {
    const blocks = config.blocks.map(b => key(b) === type ? { ...b, ...updates } : b);
    if (validDisplayLayout(blocks) && (!config.rows || blocks.every(b => b.y + b.h <= config.rows))) { config = { ...config, blocks }; setMessage(''); } else setMessage('Vị trí hoặc kích thước không hợp lệ.', 'Invalid position or size.');
  }
  function add(type) {
    if (config.blocks.length >= 32) { setMessage('Đã đạt giới hạn 32 lớp.', 'Maximum of 32 layers reached.'); return; }
    const id = `layer_${Array.from(crypto.getRandomValues(new Uint8Array(12)), byte => byte.toString(16).padStart(2, '0')).join('')}`;
    const layer = { id, type, x:0, y:0, w:4, h:2, ...(type === 'text' ? { text: t('Văn bản', 'Text') } : { color:'#dce5db' }) };
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
    if (!Number.isInteger(w) || !Number.isInteger(h) || w < 1 || h < 1 || w > 8192 || h > 8192) { setMessage('Nhập chiều rộng và chiều cao từ 1 đến 8192 pixel.', 'Enter width and height between 1 and 8192 pixels.'); return; }
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
    setMessage('Không còn đủ chỗ cho khối này.', 'Not enough space left for this block.');
  }
  async function save() {
    if (resolutionPending) { setMessage('Áp dụng độ phân giải trước khi lưu.', 'Apply resolution before saving.', 'warning'); return; }
    busy = true; setMessage('');
    try {
      const submitted = JSON.stringify(config);
      const response = await fetch(`/api/devices/${device.id}/display`, { method:'POST', headers:{ 'Content-Type':'application/json', 'X-CSRF-Token':csrf }, body:submitted });
      if (!response.ok) throw new Error();
      saved = submitted;
      if (JSON.stringify(config) === submitted) setMessage('Đã lưu bố cục.', 'Layout saved.', 'success');
      else setMessage('Đã lưu phiên bản trước; còn thay đổi chưa lưu.', 'Saved earlier revision; unsaved changes remain.', 'warning');
    } catch { setMessage('Không thể lưu. Kiểm tra thông tin và thử lại.', 'Could not save. Check the details and try again.'); } finally { busy = false; }
  }
  function close() { if (!config || (!resolutionPending && JSON.stringify(config) === saved) || window.confirm(t('Bỏ các thay đổi chưa lưu?', 'Discard unsaved changes?'))) onclose(); }
  onMount(async () => {
    try { const r = await fetch(`/api/devices/${device.id}/display`, { cache:'no-store' }); if (!r.ok) throw new Error(); config = await r.json(); width = config.width || ''; height = config.height || ''; saved = JSON.stringify(config); }
    catch { setMessage('Không thể tải bố cục.', 'Could not load layout.'); }
  });
</script>

<section class="editor" aria-labelledby="editor-title">
  <header class="editor-header">
    <div>
      <div class="editor-title-row">
        <h3 id="editor-title">{t('Thiết kế:', 'Design:')} {device.name}</h3>
        {#if config}
          <span class={dirty ? 'ui-pill ui-pill--warning' : 'ui-pill ui-pill--success'}>
            {dirty ? t('Chưa lưu thay đổi', 'Unsaved changes') : t('Đã đồng bộ', 'Synced')}
          </span>
        {/if}
      </div>
      <p>{t('Bản xem trước dùng dữ liệu minh họa. Kéo khối hoặc dùng các điều khiển bên dưới.', 'Preview uses sample data. Drag blocks or use the controls below.')}</p>
    </div>
    <button class="secondary" type="button" onclick={close}>{t('Đóng', 'Close')}</button>
  </header>

  {#if message}<Banner kind={message.kind} text={t(...message.text)} />{/if}

  {#if !config && !message}
    <p class="ui-loading" role="status">
      <span class="ui-spinner" aria-hidden="true"></span>
      <span>{t('Đang tải bố cục…', 'Loading layout…')}</span>
    </p>
  {:else if config}
    <div class="workspace">
      <div class="stage">
        <div class="stage-toolbar">
          <div>
            <strong>{t('Khung thiết kế', 'Design canvas')}</strong>
            <p>{config.width ? `${config.width} × ${config.height} px` : t('Chưa khai báo độ phân giải', 'No resolution set')}</p>
          </div>
          {#if config.width}
            <div class="zoom-controls" role="group" aria-label={t('Thu phóng khung thiết kế', 'Canvas zoom')}>
              <button class="secondary" type="button" aria-pressed={zoomMode === 'width' && zoom === 1} onclick={() => { zoomMode = 'width'; zoom = 1; }}>{t('Vừa chiều ngang', 'Fit width')}</button>
              <button class="secondary" type="button" aria-pressed={zoomMode === 'fit' && zoom === 1} onclick={() => { zoomMode = 'fit'; zoom = 1; }}>{t('Vừa màn hình', 'Fit screen')}</button>
              <button class="secondary zoom-step" type="button" aria-label={t('Thu nhỏ', 'Zoom out')} disabled={zoom <= 0.5} onclick={() => zoom = Math.max(0.5, Math.round((zoom - 0.25) * 100) / 100)}>−</button>
              <output class="mono" aria-label={t('Mức thu phóng', 'Zoom level')}>{Math.round(zoom * 100)}%</output>
              <button class="secondary zoom-step" type="button" aria-label={t('Phóng to', 'Zoom in')} disabled={zoom >= 2} onclick={() => zoom = Math.min(2, Math.round((zoom + 0.25) * 100) / 100)}>+</button>
            </div>
          {/if}
        </div>
        {#if config.width}
          <p class="stage-hint">{t('Thu phóng không thay đổi bố cục đã lưu. Khi phóng lớn, cuộn ngang để xem toàn bộ chiều rộng.', 'Zoom does not affect the saved layout. When zoomed in, scroll horizontally to view full width.')}</p>
        {/if}
        <DisplayCanvas {config} {zoomMode} {zoom} {selected} language={device.language || language} data={{room:t('Phòng họp', 'Meeting room'), entries:[{starts:Math.floor(Date.now()/1000)+3600,ends:Math.floor(Date.now()/1000)+5400}]}} status="Trống" scheduleNow={Math.floor(Date.now()/1000)} editing onmove={move} onresize={(type,updates) => replace(type,updates)} onselect={(type) => selected = type} />
      </div>

      <aside class="inspector">
        <fieldset>
          <legend>{t('Độ phân giải thiết bị (pixel)', 'Device resolution (px)')}</legend>
          <div class="grid-2">
            <label>{t('Chiều rộng', 'Width')} <input type="number" min="1" max="8192" step="1" bind:value={width} /></label>
            <label>{t('Chiều cao', 'Height')} <input type="number" min="1" max="8192" step="1" bind:value={height} /></label>
          </div>
          <button class="secondary" type="button" onclick={applyResolution}>{t('Áp dụng độ phân giải', 'Apply resolution')}</button>
          {#if config.rows}
            <label>{t('Số hàng lưới', 'Grid rows')} <input type="number" min="7" max="24" value={config.rows} onchange={(e) => { const input = e.currentTarget, rows = Number(input.value); if (Number.isInteger(rows) && rows >= 7 && rows <= 24 && config.blocks.every(b => b.y + b.h <= rows)) { config = { ...config, rows }; setMessage(''); } else setMessage('Số hàng phải từ 7 đến 24 và chứa toàn bộ các khối.', 'Rows must be 7–24 and contain all blocks.'); input.value = String(config.rows); }} /></label>
          {/if}
        </fieldset>

        <fieldset>
          <legend>{t('Tổ chức & Thương hiệu', 'Organization & Branding')}</legend>
          <label>{t('Tên tổ chức', 'Organization name')} <input maxlength="128" bind:value={config.organization} /></label>
          <label>{t('Đường dẫn logo nội bộ', 'Internal logo path')} <input maxlength="256" placeholder="/logo.png" bind:value={config.logo} /></label>
        </fieldset>

        <fieldset class="block-toggles">
          <legend>{t('Widget hiển thị', 'Display widgets')}</legend>
          <div class="toggle-grid">
            {#each displayBlocks as type}
              <label class="toggle-chip">
                <input type="checkbox" checked={config.blocks.some(b => b.type === type)} onchange={(e) => toggle(type,e.currentTarget.checked)} />
                <Icon name={widgetIcons[type]} />
                <span>{labels[type]}</span>
              </label>
            {/each}
          </div>
        </fieldset>

        <fieldset>
          <legend>{t('Lớp (trên cùng hiển thị trước)', 'Layers (top shown first)')}</legend>
          <div class="grid-2">
            <button class="secondary" type="button" onclick={() => add('text')}>{t('Thêm chữ', 'Add text')}</button>
            <button class="secondary" type="button" onclick={() => add('rectangle')}>{t('Thêm hình chữ nhật', 'Add rectangle')}</button>
          </div>
          <label>{t('Chọn lớp', 'Select layer')}
            <select bind:value={selected}>
              {#each [...config.blocks].reverse() as b}
                <option value={key(b)}>{labels[b.type]} {b.id ? `(${b.id.slice(-6)})` : ''}</option>
              {/each}
            </select>
          </label>
          <div class="grid-2">
            <button class="secondary" type="button" onclick={() => reorder(1)}>{t('Đưa lên trước', 'Bring forward')}</button>
            <button class="secondary" type="button" onclick={() => reorder(-1)}>{t('Đưa ra sau', 'Send backward')}</button>
          </div>
          {#if block?.id}
            <button class="danger" type="button" onclick={() => { if (window.confirm(t('Xóa lớp này?', 'Delete this layer?'))) { config = { ...config, blocks:config.blocks.filter(b => key(b) !== selected) }; selected = config.blocks.length ? key(config.blocks[0]) : ''; } }}>{t('Xóa lớp', 'Delete layer')}</button>
          {/if}
        </fieldset>

        {#if block}
          <fieldset class="selected-props">
            <legend>{t('Thuộc tính:', 'Properties:')} {labels[block.type]}</legend>
            {#if block.type === 'text'}
              <label>{t('Nội dung (tối đa 512 byte UTF-8)', 'Content (max 512 UTF-8 bytes)')}
                <textarea rows="3" value={block.text || ''} oninput={(e) => { const input = e.currentTarget; if (new TextEncoder().encode(input.value).length <= 512) replace(selected, { text:input.value }); else { input.value = block.text || ''; setMessage('Nội dung vượt quá 512 byte.', 'Content exceeds 512 bytes.'); } }}></textarea>
              </label>
              <div class="grid-3">
                <label>{t('Cỡ chữ (px)', 'Font size (px)')} <input type="number" min="8" max="256" value={block.fontSize ?? 24} onchange={(e) => replace(selected, { fontSize:Number(e.currentTarget.value) })} /></label>
                <label>Padding (px) <input type="number" min="0" max="128" value={block.padding ?? 16} onchange={(e) => replace(selected, { padding:Number(e.currentTarget.value) })} /></label>
                <label>Margin (px) <input type="number" min="0" max="128" value={block.margin ?? 0} onchange={(e) => replace(selected, { margin:Number(e.currentTarget.value) })} /></label>
              </div>
              <div class="grid-2">
                <label>{t('Căn ngang', 'Horizontal align')}
                  <select value={block.align || 'left'} onchange={(e) => replace(selected, { align:e.currentTarget.value })}>
                    <option value="left">{t('Trái', 'Left')}</option><option value="center">{t('Giữa', 'Center')}</option><option value="right">{t('Phải', 'Right')}</option>
                  </select>
                </label>
                <label>{t('Căn dọc', 'Vertical align')}
                  <select value={block.valign || 'center'} onchange={(e) => replace(selected, { valign:e.currentTarget.value })}>
                    <option value="top">{t('Trên', 'Top')}</option><option value="center">{t('Giữa', 'Center')}</option><option value="bottom">{t('Dưới', 'Bottom')}</option>
                  </select>
                </label>
              </div>
            {/if}

            {#if block.type === 'rectangle'}
              <label class="color-row">{t('Màu nền', 'Fill color')} <input type="color" value={block.color || '#dce5db'} oninput={(e) => replace(selected, { color:e.currentTarget.value })} /></label>
            {/if}

            <div class="coordinates">
              {#each ['x','y','w','h'] as field}
                <label>{(language === 'en' ? {x:'Col',y:'Row',w:'Width',h:'Height'} : {x:'Cột',y:'Hàng',w:'Rộng',h:'Cao'})[field]}
                  <input type="number" min={field === 'x' || field === 'y' ? 0 : 1} max={field === 'x' || field === 'w' ? 12 : 24} value={block[field]} onchange={(e) => { const input = e.currentTarget; replace(selected, { [field]:Number(input.value) }); input.value = String(config.blocks.find(b => key(b) === selected)[field]); }} />
                </label>
              {/each}
            </div>
          </fieldset>
        {/if}

        <div class="inspector-actions">
          <button type="button" disabled={busy} onclick={save}>{busy ? t('Đang lưu…', 'Saving…') : t('Lưu thiết kế', 'Save layout')}</button>
          <button class="secondary" type="button" onclick={() => { if (window.confirm(t('Thay bố cục hiện tại bằng mẫu mặc định?', 'Replace current layout with default template?'))) { config = { ...config, blocks:defaultDisplayLayout.map(b => ({ ...b })) }; selected = 'room'; setMessage(''); } }}>{t('Dùng mẫu mặc định', 'Use default layout')}</button>
          <a class="public-link" href={`/display/${device.public_id}`} target="_blank" rel="noreferrer">{t('Mở màn hình công khai', 'Open public display')} ↗</a>
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
