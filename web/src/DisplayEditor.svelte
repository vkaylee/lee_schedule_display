<script>
  import { onMount, tick } from 'svelte';
  import Icon from './Icon.svelte';
  import Banner from './Banner.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  let confirmDialog = $state();
  import DisplayCanvas from './DisplayCanvas.svelte';
  import { blockKey, contrastRatio, displayBlocks, roomBlocks, defaultDisplayLayout, hasInvalidRoomAssignment, validDisplayLayout, widgetParts } from './display-layout.js';
  const widgetIcons = { clock:'clock', branding:'brand', room:'rooms', status:'check-circle', upcoming:'calendar', footer:'monitor', text:'edit', rectangle:'grid' };
  const colorPresets = [
    { id: 'default', vi: 'Mặc định', en: 'Default', bgColor: '', textColor: '', accentColor: '' },
    { id: 'dark', vi: 'Đêm sâu', en: 'Midnight', bgColor: '#13211b', textColor: '#f3f7f4', accentColor: '#4ade80' },
    { id: 'slate', vi: 'Than chì', en: 'Slate', bgColor: '#1e293b', textColor: '#f8fafc', accentColor: '#38bdf8' },
    { id: 'emerald', vi: 'Ngọc bích', en: 'Emerald', bgColor: '#e8f5ec', textColor: '#113c26', accentColor: '#1b6340' },
    { id: 'warm', vi: 'Vàng ấm', en: 'Warm', bgColor: '#fcf6ea', textColor: '#3b2c14', accentColor: '#b45309' },
    { id: 'crimson', vi: 'Đỏ trầm', en: 'Crimson', bgColor: '#fdf1ef', textColor: '#5c1811', accentColor: '#b91c1c' }
  ];
  const partLabels = {
    zone: ['Múi giờ', 'Timezone'],
    date: ['Ngày tháng', 'Date'],
    eyebrow: ['Nhãn phụ', 'Eyebrow label'],
    logo: ['Logo / biểu tượng', 'Logo / mark'],
    accentBar: ['Thanh màu bên', 'Accent bar'],
    pill: ['Nhãn HIỆN TẠI', 'CURRENT pill'],
    icon: ['Biểu tượng', 'Status icon'],
    until: ['Giờ kết thúc / tiếp theo', 'End / next time'],
    remaining: ['Thời gian còn lại', 'Remaining time'],
    progress: ['Thanh tiến độ', 'Progress bar'],
    count: ['Số lượng lịch', 'Item count'],
    relative: ['Nhãn Kế tiếp · Sau…', 'Next countdown'],
    duration: ['Thời lượng họp', 'Slot duration'],
    badge: ['Trực tuyến / Ngoại tuyến', 'Live / Offline badge']
  };
  let { device, rooms = [], csrf, language = 'vi', onclose } = $props();
  const t = (vi, en) => language === 'en' ? en : vi;
  let config = $state(null), selected = $state('room'), message = $state(null), busy = $state(false), saved = $state('');
  let width = $state(''), height = $state('');
  let zoomMode = $state('width'), zoom = $state(1);
  let popoverOpen = $state(false), popoverPos = $state({ x: 16, y: 16 }), popoverTab = $state('style');
  let popoverEl = $state(null);
  let lastTrigger = null;
  let labels = $derived(language === 'en'
    ? { clock:'Clock & date', branding:'Organization', room:'Room name', status:'Status', upcoming:'Upcoming', footer:'Connection', text:'Text', rectangle:'Rectangle' }
    : { clock:'Giờ và ngày', branding:'Tổ chức', room:'Tên phòng', status:'Trạng thái', upcoming:'Lịch tiếp theo', footer:'Kết nối', text:'Chữ', rectangle:'Hình chữ nhật' });
  const key = blockKey;
  let block = $derived(config?.blocks.find(b => key(b) === selected));
  let contrastWarning = $derived(block && block.bgColor && block.textColor ? (contrastRatio(block.bgColor, block.textColor) ?? 21) < 4.5 : false);
  let resolutionPending = $derived(config && (String(width || '') !== String(config.width || '') || String(height || '') !== String(config.height || '')));
  let dirty = $derived(config && (resolutionPending || JSON.stringify(config) !== saved));
  const setMessage = (vi, en = '', kind = 'error') => { message = vi ? { text: [vi, en || vi], kind } : null; };
  function cleanBlock(b, updates) {
    const next = { ...b, ...updates };
    for (const k of ['bgColor', 'textColor', 'accentColor', 'align', 'valign', 'subLayout']) if (!next[k]) delete next[k];
    if (!next.fontScale || next.fontScale === 100) delete next.fontScale;
    if (!next.letterSpacing) delete next.letterSpacing;
    if (!next.padding && next.type !== 'text') delete next.padding;
    if (!next.hideParts?.length) delete next.hideParts;
    return next;
  }
  function replace(type, updates) {
    const blocks = config.blocks.map(b => key(b) === type ? cleanBlock(b, updates) : b);
    if (validDisplayLayout(blocks) && (!config.rows || blocks.every(b => b.y + b.h <= config.rows))) { config = { ...config, blocks }; setMessage(''); } else setMessage('Vị trí hoặc kích thước không hợp lệ.', 'Invalid position or size.');
  }
  function togglePart(part, visible) {
    if (!block) return;
    const current = block.hideParts || [];
    const hideParts = visible ? current.filter(p => p !== part) : [...new Set([...current, part])];
    replace(selected, { hideParts });
  }
  function add(type) {
    if (config.blocks.length >= 32) { setMessage('Đã đạt giới hạn 32 lớp.', 'Maximum of 32 layers reached.'); return; }
    const id = `layer_${Array.from(crypto.getRandomValues(new Uint8Array(12)), byte => byte.toString(16).padStart(2, '0')).join('')}`;
    const template = defaultDisplayLayout.find(b => b.type === type);
    if (template && !roomBlocks.includes(type) && config.blocks.some(b => b.type === type)) { setMessage('Widget này đã có trong bố cục.', 'This widget is already in the layout.'); return; }
    const layer = template ? { ...template, ...(roomBlocks.includes(type) ? { id } : {}), h:Math.min(template.h, config.rows || 24), y:0 } : { id, type, x:0, y:0, w:4, h:2, ...(type === 'text' ? { text: t('Văn bản', 'Text') } : { color:'#dce5db' }) };
    config = { ...config, blocks:[...config.blocks, layer] }; selected = key(layer); popoverTab = 'style'; setMessage('');
  }
  function reorder(delta) {
    const blocks = [...config.blocks], index = blocks.findIndex(b => key(b) === selected), next = index + delta;
    if (index < 0 || next < 0 || next >= blocks.length) return;
    [blocks[index], blocks[next]] = [blocks[next], blocks[index]]; config = { ...config, blocks };
  }
  async function removeSelectedLayer() {
    if (!block) return;
    const target = selected;
    if (!await confirmDialog.ask(t('Xóa lớp này?', 'Delete this layer?'))) return;
    config = { ...config, blocks:config.blocks.filter(b => key(b) !== target) };
    selected = config.blocks.length ? key(config.blocks[0]) : '';
    popoverOpen = false;
  }
  async function openPopover(targetKey, point = null, triggerEl = null) {
    selected = targetKey;
    const targetBlock = config?.blocks.find(b => key(b) === targetKey);
    if (!targetBlock) return;
    if (targetBlock.type === 'text' && popoverTab === 'details') popoverTab = 'style';
    lastTrigger = triggerEl || document.activeElement;
    const vw = window.innerWidth || 1280, vh = window.innerHeight || 800;
    const pw = Math.min(380, vw - 24), ph = Math.min(480, vh - 24);
    const rawX = point?.x ?? Math.round((vw - pw) / 2);
    const rawY = point?.y ?? Math.round((vh - ph) / 2);
    popoverPos = {
      x: Math.max(12, Math.min(rawX, vw - pw - 12)),
      y: Math.max(12, Math.min(rawY, vh - ph - 12))
    };
    popoverOpen = true;
    await tick();
    if (popoverEl) {
      const rect = popoverEl.getBoundingClientRect();
      popoverPos = {
        x: Math.max(12, Math.min(popoverPos.x, vw - rect.width - 12)),
        y: Math.max(12, Math.min(popoverPos.y, vh - rect.height - 12))
      };
      popoverEl.focus({ preventScroll: true });
    }
  }
  function closePopover(restoreFocus = false) {
    if (!popoverOpen) return;
    popoverOpen = false;
    if (restoreFocus && lastTrigger && typeof lastTrigger.focus === 'function') lastTrigger.focus();
  }
  function handleWindowPointerDown(event) {
    if (!popoverOpen || !popoverEl) return;
    if (event.target.closest?.('dialog[open]') || popoverEl.contains(event.target)) return;
    if (event.target.closest?.('.block-inspect-btn, .layer-inspect-btn')) return;
    popoverOpen = false;
  }
  function handleWindowKeyDown(event) {
    if (event.key === 'Escape' && popoverOpen && !document.querySelector('dialog[open]')) {
      event.preventDefault();
      closePopover(true);
    }
  }
  function move(type, x, y) { selected = type; replace(type, { x:Math.max(0,x), y:Math.max(0,y) }); }
  function applyResolution() {
    const w = Number(width), h = Number(height);
    if (!Number.isInteger(w) || !Number.isInteger(h) || w < 1 || h < 1 || w > 8192 || h > 8192) { setMessage('Nhập chiều rộng và chiều cao từ 1 đến 8192 pixel.', 'Enter width and height between 1 and 8192 pixels.'); return; }
    config = { ...config, width:w, height:h, rows:config.rows || Math.max(7, ...config.blocks.map(b => b.y + b.h)) };
    setMessage('');
  }
  async function save() {
    if (resolutionPending) { setMessage('Áp dụng độ phân giải trước khi lưu.', 'Apply resolution before saving.', 'warning'); return; }
    if (hasInvalidRoomAssignment(config.blocks, rooms, JSON.parse(saved).blocks)) { setMessage('Chọn phòng đang hoạt động hoặc bỏ chọn phòng không khả dụng.', 'Select an active room or clear the unavailable room.', 'warning'); return; }
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
  async function close() { if (!config || (!resolutionPending && JSON.stringify(config) === saved) || await confirmDialog.ask(t('Bỏ các thay đổi chưa lưu?', 'Discard unsaved changes?'))) onclose(); }
  onMount(async () => {
    try { const r = await fetch(`/api/devices/${device.id}/display`, { cache:'no-store' }); if (!r.ok) throw new Error(); config = await r.json(); width = config.width || ''; height = config.height || ''; saved = JSON.stringify(config); }
    catch { setMessage('Không thể tải bố cục.', 'Could not load layout.'); }
  });
</script>

<ConfirmDialog bind:this={confirmDialog} {language} />

<svelte:window onpointerdown={handleWindowPointerDown} onkeydown={handleWindowKeyDown} />

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
      <p>{t('Bản xem trước dùng dữ liệu minh họa. Kéo khối để sắp xếp, nhấp chuột phải hoặc bấm ⚙ để chỉnh thuộc tính.', 'Preview uses sample data. Drag blocks to arrange, right-click or click ⚙ to edit properties.')}</p>
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
        <DisplayCanvas {config} {zoomMode} {zoom} {selected} language={device.language || language} data={{ rooms:Object.fromEntries(rooms.map(room => [room.id, { room:room.name, entries:[{starts:Math.floor(Date.now()/1000)+3600,ends:Math.floor(Date.now()/1000)+5400}], unavailable:!room.active }])) }} statuses={Object.fromEntries(rooms.map(room => [room.id, room.active ? 'Trống' : 'Không xác định']))} scheduleNow={Math.floor(Date.now()/1000)} editing onmove={move} onresize={(type,updates) => replace(type,updates)} onselect={(type) => selected = type} oninspect={(type, point) => openPopover(type, point)} />
      </div>

      <aside class="inspector">
        <fieldset class="block-toggles">
          <legend>{t('Widget hiển thị', 'Display widgets')}</legend>
          <div class="toggle-grid">
            {#each displayBlocks as type}
              <button class="secondary" type="button" disabled={config.blocks.length >= 32} onclick={() => add(type)}>
                <Icon name={widgetIcons[type]} />
                <span>+ {labels[type]}</span>
              </button>
            {/each}
          </div>
        </fieldset>

        <fieldset class="layer-panel">
          <legend>{t('Lớp (trên cùng hiển thị trước)', 'Layers (top shown first)')}</legend>
          <div class="grid-2">
            <button class="secondary" type="button" onclick={() => add('text')}>{t('+ Thêm chữ', '+ Add text')}</button>
            <button class="secondary" type="button" onclick={() => add('rectangle')}>{t('+ Hình chữ nhật', '+ Rectangle')}</button>
          </div>
          <ul class="layer-list" aria-label={t('Danh sách lớp', 'Layer stack')}>
            {#each [...config.blocks].reverse() as b (key(b))}
              {@const k = key(b)}
              <li class="layer-row" class:is-selected={selected === k}>
                <button
                  type="button"
                  class="layer-select-btn"
                  onclick={() => selected = k}
                  ondblclick={(e) => openPopover(k, { x: e.clientX - 380, y: e.clientY }, e.currentTarget)}
                >
                  <Icon name={widgetIcons[b.type]} />
                  <span class="layer-name">{labels[b.type]}{b.id ? ` (${b.id.slice(-4)})` : ''}</span>
                  {#if b.bgColor || b.textColor || b.accentColor || b.hideParts?.length || b.subLayout || b.fontScale}
                    <span class="layer-dot" title={t('Đã tùy chỉnh', 'Customized')}></span>
                  {/if}
                </button>
                <button
                  type="button"
                  class="layer-inspect-btn"
                  aria-label={`${t('Thuộc tính', 'Properties')}: ${labels[b.type]}`}
                  title={t('Chỉnh thuộc tính', 'Edit properties')}
                  onclick={(e) => { const r = e.currentTarget.getBoundingClientRect(); openPopover(k, { x: r.left - 392, y: r.top }, e.currentTarget); }}
                >⚙</button>
              </li>
            {/each}
          </ul>
          <div class="layer-bar">
            <button class="secondary" type="button" disabled={!block} onclick={() => reorder(1)}>{t('Lên trước', 'Forward')}</button>
            <button class="secondary" type="button" disabled={!block} onclick={() => reorder(-1)}>{t('Ra sau', 'Backward')}</button>
            <button class="danger" type="button" disabled={!block} onclick={removeSelectedLayer}>{t('Xóa', 'Delete')}</button>
          </div>
        </fieldset>

        <details class="sidebar-details">
          <summary>
            <span>{t('Độ phân giải màn hình', 'Screen resolution')}</span>
            <span class="summary-meta mono">{config.width ? `${config.width}×${config.height}` : t('Tự động', 'Auto')}</span>
          </summary>
          <div class="details-body">
            <div class="grid-2">
              <label>{t('Chiều rộng', 'Width')} <input type="number" min="1" max="8192" step="1" bind:value={width} /></label>
              <label>{t('Chiều cao', 'Height')} <input type="number" min="1" max="8192" step="1" bind:value={height} /></label>
            </div>
            <button class="secondary" type="button" onclick={applyResolution}>{t('Áp dụng độ phân giải', 'Apply resolution')}</button>
            {#if config.rows}
              <label>{t('Số hàng lưới', 'Grid rows')} <input type="number" min="7" max="24" value={config.rows} onchange={(e) => { const input = e.currentTarget, rows = Number(input.value); if (Number.isInteger(rows) && rows >= 7 && rows <= 24 && config.blocks.every(b => b.y + b.h <= rows)) { config = { ...config, rows }; setMessage(''); } else setMessage('Số hàng phải từ 7 đến 24 và chứa toàn bộ các khối.', 'Rows must be 7–24 and contain all blocks.'); input.value = String(config.rows); }} /></label>
            {/if}
          </div>
        </details>

        <details class="sidebar-details">
          <summary>
            <span>{t('Tổ chức & Thương hiệu', 'Organization & Brand')}</span>
            <span class="summary-meta">{config.organization || t('Mặc định', 'Default')}</span>
          </summary>
          <div class="details-body">
            <label>{t('Tên tổ chức', 'Organization name')} <input maxlength="128" bind:value={config.organization} /></label>
            <label>{t('Đường dẫn logo nội bộ', 'Internal logo path')} <input maxlength="256" placeholder="/logo.png" bind:value={config.logo} /></label>
          </div>
        </details>

        <div class="inspector-actions">
          <button type="button" disabled={busy} onclick={save}>{busy ? t('Đang lưu…', 'Saving…') : t('Lưu thiết kế', 'Save layout')}</button>
          <button class="secondary" type="button" onclick={async () => { if (await confirmDialog.ask(t('Thay bố cục hiện tại bằng mẫu mặc định? Các lựa chọn phòng sẽ bị xóa và cần chọn lại.', 'Replace current layout with default template? Room selections will be cleared and must be selected again.'))) { config = { ...config, blocks:defaultDisplayLayout.map(b => ({ ...b })) }; selected = 'room'; popoverOpen = false; setMessage(''); } }}>{t('Dùng mẫu mặc định', 'Use default layout')}</button>
          <a class="public-link" href={`/display/${device.public_id}`} target="_blank" rel="noreferrer">{t('Mở màn hình công khai', 'Open public display')} ↗</a>
        </div>
      </aside>
    </div>
  {/if}

  {#if popoverOpen && block}
    <div
      bind:this={popoverEl}
      class="props-popover"
      role="dialog"
      aria-modal="false"
      aria-label={`${t('Thuộc tính:', 'Properties:')} ${labels[block.type]}`}
      tabindex="-1"
      style={`left:${popoverPos.x}px;top:${popoverPos.y}px`}
    >
      <header class="popover-head">
        <div class="popover-title">
          <Icon name={widgetIcons[block.type]} />
          <strong>{labels[block.type]}</strong>
          {#if block.id}<span class="mono popover-id">#{block.id.slice(-4)}</span>{/if}
        </div>
        <div class="popover-head-actions">
          <button type="button" class="icon-mini-btn" title={t('Đưa lên trước', 'Bring forward')} aria-label={t('Đưa lên trước', 'Bring forward')} onclick={() => reorder(1)}>↑</button>
          <button type="button" class="icon-mini-btn" title={t('Đưa ra sau', 'Send backward')} aria-label={t('Đưa ra sau', 'Send backward')} onclick={() => reorder(-1)}>↓</button>
          <button type="button" class="icon-mini-btn is-danger" title={t('Xóa lớp', 'Delete layer')} aria-label={t('Xóa lớp', 'Delete layer')} onclick={removeSelectedLayer}>×</button>
          <button type="button" class="icon-mini-btn" aria-label={t('Đóng thuộc tính', 'Close properties')} onclick={() => closePopover(true)}>×</button>
        </div>
      </header>

      {#if displayBlocks.includes(block.type) || block.type === 'text'}
        <div class="popover-tabs" role="tablist">
          <button type="button" role="tab" aria-selected={popoverTab === 'style'} class:is-active={popoverTab === 'style'} onclick={() => popoverTab = 'style'}>
            {block.type === 'text' ? t('Nội dung & Kiểu', 'Text & Style') : t('Kiểu dáng', 'Style')}
          </button>
          {#if displayBlocks.includes(block.type)}
            <button type="button" role="tab" aria-selected={popoverTab === 'details'} class:is-active={popoverTab === 'details'} onclick={() => popoverTab = 'details'}>
              {t('Chi tiết', 'Details')}
            </button>
          {/if}
          <button type="button" role="tab" aria-selected={popoverTab === 'layout'} class:is-active={popoverTab === 'layout'} onclick={() => popoverTab = 'layout'}>
            {t('Vị trí', 'Layout')}
          </button>
        </div>
      {/if}

      <div class="popover-body">
        {#if displayBlocks.includes(block.type)}
          {#if roomBlocks.includes(block.type)}
            <label>{t('Phòng của widget', 'Widget room')}
              <select value={block.roomId || ''} onchange={(e) => replace(selected, { roomId: e.currentTarget.value ? Number(e.currentTarget.value) : undefined })}>
                <option value="">{t('Chọn phòng', 'Select room')}</option>
                {#if block.roomId && !rooms.some(room => room.id === block.roomId && room.active)}
                  <option value={String(block.roomId)} disabled>{rooms.find(room => room.id === block.roomId)?.name || `#${block.roomId}`} — {t('Không khả dụng', 'Unavailable')}</option>
                {/if}
                {#each rooms.filter(room => room.active) as room}
                  <option value={String(room.id)}>{room.name}</option>
                {/each}
              </select>
            </label>
          {/if}
          {#if popoverTab === 'style'}
            <div class="prop-group">
              <span class="prop-label">{t('Bảng màu nhanh', 'Color presets')}</span>
              <div class="preset-row" role="group" aria-label={t('Bảng màu nhanh', 'Color presets')}>
                {#each colorPresets as p}
                  <button
                    type="button"
                    class="preset-chip"
                    class:is-active={(block.bgColor || '') === p.bgColor && (block.textColor || '') === p.textColor && (block.accentColor || '') === p.accentColor}
                    onclick={() => replace(selected, { bgColor: p.bgColor, textColor: p.textColor, accentColor: p.accentColor })}
                  >
                    <span class="preset-swatch" style={`background:${p.bgColor || '#ffffff'};border-color:${p.accentColor || '#b8d5c2'};color:${p.textColor || '#182c23'}`}>Aa</span>
                    <span>{t(p.vi, p.en)}</span>
                  </button>
                {/each}
              </div>
            </div>

            <div class="grid-3">
              <label class="color-field">
                <span>{t('Màu nền', 'Background')}</span>
                <span class="color-ctrl">
                  <input type="color" value={block.bgColor || '#ffffff'} oninput={(e) => replace(selected, { bgColor: e.currentTarget.value })} />
                  {#if block.bgColor}<button type="button" class="clear-btn" aria-label={t('Xóa màu nền', 'Clear background')} onclick={() => replace(selected, { bgColor: '' })}>×</button>{/if}
                </span>
              </label>
              <label class="color-field">
                <span>{t('Màu chữ', 'Text color')}</span>
                <span class="color-ctrl">
                  <input type="color" value={block.textColor || '#182c23'} oninput={(e) => replace(selected, { textColor: e.currentTarget.value })} />
                  {#if block.textColor}<button type="button" class="clear-btn" aria-label={t('Xóa màu chữ', 'Clear text color')} onclick={() => replace(selected, { textColor: '' })}>×</button>{/if}
                </span>
              </label>
              <label class="color-field">
                <span>{t('Màu nhấn', 'Accent')}</span>
                <span class="color-ctrl">
                  <input type="color" value={block.accentColor || '#1b6340'} oninput={(e) => replace(selected, { accentColor: e.currentTarget.value })} />
                  {#if block.accentColor}<button type="button" class="clear-btn" aria-label={t('Xóa màu nhấn', 'Clear accent')} onclick={() => replace(selected, { accentColor: '' })}>×</button>{/if}
                </span>
              </label>
            </div>

            {#if contrastWarning}
              <p class="contrast-warn" role="status">{t('Độ tương phản chữ và nền hơi thấp — có thể khó đọc từ xa.', 'Low text/background contrast — may be hard to read at a distance.')}</p>
            {/if}

            <div class="grid-2">
              <label>{t('Cỡ chữ (%)', 'Font scale (%)')}
                <input type="number" min="60" max="180" step="5" value={block.fontScale || 100} onchange={(e) => replace(selected, { fontScale: Number(e.currentTarget.value) })} />
              </label>
              <label>{t('Dãn chữ', 'Letter spacing')}
                <input type="number" min="-4" max="20" step="1" value={block.letterSpacing ?? 0} onchange={(e) => replace(selected, { letterSpacing: Number(e.currentTarget.value) })} />
              </label>
            </div>
          {:else if popoverTab === 'details'}
            <label>{t('Sắp xếp chi tiết', 'Detail layout')}
              <select value={block.subLayout || ''} onchange={(e) => replace(selected, { subLayout: e.currentTarget.value })}>
                <option value="">{t('Mặc định', 'Default')}</option>
                <option value="reverse">{t('Đảo vị trí', 'Reversed')}</option>
                <option value="compact">{t('Thu gọn', 'Compact')}</option>
              </select>
            </label>

            {#if widgetParts[block.type]?.length}
              <div class="prop-group">
                <span class="prop-label">{t('Hiển thị chi tiết phụ', 'Visible details')}</span>
                <div class="toggle-grid">
                  {#each widgetParts[block.type] as part}
                    <label class="toggle-chip">
                      <input type="checkbox" checked={!block.hideParts?.includes(part)} onchange={(e) => togglePart(part, e.currentTarget.checked)} />
                      <span>{t(...(partLabels[part] || [part, part]))}</span>
                    </label>
                  {/each}
                </div>
              </div>
            {/if}

            <button type="button" class="secondary reset-widget-btn" onclick={() => replace(selected, { bgColor: '', textColor: '', accentColor: '', fontScale: 100, letterSpacing: 0, padding: 0, align: '', valign: '', subLayout: '', hideParts: [] })}>
              {t('Đặt lại kiểu mặc định cho widget này', 'Reset style for this widget')}
            </button>
          {:else}
            <div class="grid-3">
              <label>{t('Căn ngang', 'Align')}
                <select value={block.align || ''} onchange={(e) => replace(selected, { align: e.currentTarget.value })}>
                  <option value="">{t('Mặc định', 'Default')}</option>
                  <option value="left">{t('Trái', 'Left')}</option>
                  <option value="center">{t('Giữa', 'Center')}</option>
                  <option value="right">{t('Phải', 'Right')}</option>
                </select>
              </label>
              <label>{t('Căn dọc', 'Vertical')}
                <select value={block.valign || ''} onchange={(e) => replace(selected, { valign: e.currentTarget.value })}>
                  <option value="">{t('Mặc định', 'Default')}</option>
                  <option value="top">{t('Trên', 'Top')}</option>
                  <option value="center">{t('Giữa', 'Center')}</option>
                  <option value="bottom">{t('Dưới', 'Bottom')}</option>
                </select>
              </label>
              <label>Padding (px)
                <input type="number" min="0" max="128" step="2" value={block.padding ?? 0} onchange={(e) => replace(selected, { padding: Number(e.currentTarget.value) })} />
              </label>
            </div>
            <div class="coordinates">
              {#each ['x','y','w','h'] as field}
                <label>{(language === 'en' ? {x:'Col',y:'Row',w:'Width',h:'Height'} : {x:'Cột',y:'Hàng',w:'Rộng',h:'Cao'})[field]}
                  <input type="number" min={field === 'x' || field === 'y' ? 0 : 1} max={field === 'x' || field === 'w' ? 12 : 24} value={block[field]} onchange={(e) => { const input = e.currentTarget; replace(selected, { [field]:Number(input.value) }); input.value = String(config.blocks.find(b => key(b) === selected)[field]); }} />
                </label>
              {/each}
            </div>
          {/if}
        {:else if block.type === 'text'}
          {#if popoverTab === 'style'}
            <label>{t('Nội dung (tối đa 512 byte UTF-8)', 'Content (max 512 UTF-8 bytes)')}
              <textarea rows="3" value={block.text || ''} oninput={(e) => { const input = e.currentTarget; if (new TextEncoder().encode(input.value).length <= 512) replace(selected, { text:input.value }); else { input.value = block.text || ''; setMessage('Nội dung vượt quá 512 byte.', 'Content exceeds 512 bytes.'); } }}></textarea>
            </label>
            <div class="grid-2">
              <label class="color-field">
                <span>{t('Màu chữ', 'Text color')}</span>
                <span class="color-ctrl">
                  <input type="color" value={block.textColor || '#182c23'} oninput={(e) => replace(selected, { textColor: e.currentTarget.value })} />
                  {#if block.textColor}<button type="button" class="clear-btn" aria-label={t('Xóa màu chữ', 'Clear text color')} onclick={() => replace(selected, { textColor: '' })}>×</button>{/if}
                </span>
              </label>
              <label class="color-field">
                <span>{t('Màu nền', 'Background')}</span>
                <span class="color-ctrl">
                  <input type="color" value={block.bgColor || '#ffffff'} oninput={(e) => replace(selected, { bgColor: e.currentTarget.value })} />
                  {#if block.bgColor}<button type="button" class="clear-btn" aria-label={t('Xóa màu nền', 'Clear background')} onclick={() => replace(selected, { bgColor: '' })}>×</button>{/if}
                </span>
              </label>
            </div>
            <div class="grid-2">
              <label>{t('Cỡ chữ (px)', 'Font size (px)')} <input type="number" min="8" max="256" value={block.fontSize ?? 24} onchange={(e) => replace(selected, { fontSize:Number(e.currentTarget.value) })} /></label>
              <label>{t('Dãn chữ', 'Letter spacing')} <input type="number" min="-4" max="20" step="1" value={block.letterSpacing ?? 0} onchange={(e) => replace(selected, { letterSpacing:Number(e.currentTarget.value) })} /></label>
            </div>
          {:else}
            <div class="grid-2">
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
            <div class="coordinates">
              {#each ['x','y','w','h'] as field}
                <label>{(language === 'en' ? {x:'Col',y:'Row',w:'Width',h:'Height'} : {x:'Cột',y:'Hàng',w:'Rộng',h:'Cao'})[field]}
                  <input type="number" min={field === 'x' || field === 'y' ? 0 : 1} max={field === 'x' || field === 'w' ? 12 : 24} value={block[field]} onchange={(e) => { const input = e.currentTarget; replace(selected, { [field]:Number(input.value) }); input.value = String(config.blocks.find(b => key(b) === selected)[field]); }} />
                </label>
              {/each}
            </div>
          {/if}
        {:else if block.type === 'rectangle'}
          <label class="color-row">{t('Màu nền', 'Fill color')} <input type="color" value={block.color || '#dce5db'} oninput={(e) => replace(selected, { color:e.currentTarget.value })} /></label>
          <div class="coordinates">
            {#each ['x','y','w','h'] as field}
              <label>{(language === 'en' ? {x:'Col',y:'Row',w:'Width',h:'Height'} : {x:'Cột',y:'Hàng',w:'Rộng',h:'Cao'})[field]}
                <input type="number" min={field === 'x' || field === 'y' ? 0 : 1} max={field === 'x' || field === 'w' ? 12 : 24} value={block[field]} onchange={(e) => { const input = e.currentTarget; replace(selected, { [field]:Number(input.value) }); input.value = String(config.blocks.find(b => key(b) === selected)[field]); }} />
              </label>
            {/each}
          </div>
        {/if}
      </div>
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
    gap: 0.75rem;
    min-width: 0;
  }

  .layer-list {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 0.3rem;
    max-height: 17rem;
    overflow-y: auto;
  }

  .layer-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.3rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--bg-subtle);
    padding: 0.18rem;
  }

  .layer-row.is-selected {
    border-color: var(--focus-ring, #1d5fa3);
    background: #eef5fc;
  }

  .layer-select-btn {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    min-height: 2rem !important;
    padding: 0.25rem 0.45rem !important;
    border: 0 !important;
    background: transparent !important;
    color: var(--ink-primary) !important;
    font-size: 0.84rem !important;
    font-weight: 600 !important;
    text-align: left;
    cursor: pointer;
    min-width: 0;
  }

  .layer-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .layer-dot {
    width: 0.45rem;
    height: 0.45rem;
    border-radius: 9999px;
    background: var(--accent, #1b6340);
    flex-shrink: 0;
  }

  .layer-inspect-btn {
    min-width: 2rem !important;
    min-height: 2rem !important;
    padding: 0 !important;
    border: 1px solid var(--border-subtle) !important;
    border-radius: var(--radius-sm) !important;
    background: var(--bg-surface) !important;
    color: var(--ink-secondary) !important;
    font-size: 0.85rem !important;
    cursor: pointer;
  }

  .layer-inspect-btn:hover {
    border-color: var(--accent) !important;
    color: var(--accent) !important;
  }

  .layer-bar {
    display: flex;
    gap: 0.4rem;
  }

  .layer-bar button {
    flex: 1;
    min-height: 2.15rem;
    padding: 0.3rem 0.55rem;
    font-size: 0.82rem;
  }

  .sidebar-details {
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    background: var(--bg-surface);
    overflow: hidden;
  }

  .sidebar-details summary {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.72rem 0.95rem;
    font-size: 0.84rem;
    font-weight: 700;
    color: var(--ink-secondary);
    cursor: pointer;
    list-style: none;
  }

  .sidebar-details summary::-webkit-details-marker {
    display: none;
  }

  .summary-meta {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--ink-muted);
    max-width: 48%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .details-body {
    padding: 0 0.95rem 0.9rem;
    display: grid;
    gap: 0.65rem;
    border-top: 1px solid var(--border-subtle);
    padding-top: 0.75rem;
  }

  .props-popover {
    position: fixed;
    z-index: 60;
    width: min(380px, calc(100vw - 24px));
    max-height: min(80vh, 540px);
    display: flex;
    flex-direction: column;
    border: 1px solid var(--border-strong, #b8c6bb);
    border-radius: var(--radius-lg, 0.9rem);
    background: var(--bg-surface, #ffffff);
    box-shadow:
      0 18px 42px -12px rgba(18, 36, 27, 0.28),
      0 4px 12px -2px rgba(18, 36, 27, 0.12);
    overflow: hidden;
    outline: none;
  }

  .popover-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.7rem 0.85rem;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-subtle);
  }

  .popover-title {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    min-width: 0;
    font-size: 0.92rem;
    color: var(--ink-primary);
  }

  .popover-id {
    font-size: 0.74rem;
    color: var(--ink-muted);
  }

  .popover-head-actions {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
  }

  .icon-mini-btn {
    min-width: 1.95rem !important;
    min-height: 1.95rem !important;
    padding: 0 !important;
    border: 1px solid var(--border-subtle) !important;
    border-radius: var(--radius-sm) !important;
    background: var(--bg-surface) !important;
    color: var(--ink-secondary) !important;
    font-size: 0.88rem !important;
    line-height: 1;
    cursor: pointer;
  }

  .icon-mini-btn:hover {
    border-color: var(--accent) !important;
    color: var(--accent) !important;
  }

  .icon-mini-btn.is-danger:hover {
    border-color: var(--danger, #9e2a22) !important;
    color: var(--danger, #9e2a22) !important;
  }

  .popover-tabs {
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: 1fr;
    gap: 0.25rem;
    padding: 0.4rem 0.65rem;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-surface);
  }

  .popover-tabs button {
    min-height: 2rem;
    padding: 0.28rem 0.5rem;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--ink-muted);
    font-size: 0.82rem;
    font-weight: 650;
    cursor: pointer;
  }

  .popover-tabs button.is-active {
    background: var(--accent-soft);
    border-color: var(--accent-border);
    color: #134e31;
  }

  .popover-body {
    padding: 0.85rem;
    display: grid;
    gap: 0.7rem;
    overflow-y: auto;
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

  .grid-2 > label,
  .grid-3 > label {
    grid-template-rows: 1fr auto;
    align-items: end;
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

  .color-row input[type='color'],
  .color-ctrl input[type='color'] {
    width: 3.5rem;
    height: 2.35rem;
    padding: 0.15rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: #ffffff;
    cursor: pointer;
  }

  .prop-group {
    display: grid;
    gap: 0.4rem;
  }

  .prop-label {
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--ink-secondary);
  }

  .preset-row {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .preset-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    min-height: 2.15rem;
    padding: 0.28rem 0.55rem;
    border: 1px solid var(--border-subtle);
    border-radius: 9999px;
    background: var(--bg-subtle);
    color: var(--ink-primary);
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
  }

  .preset-chip.is-active {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: #134e31;
  }

  .preset-swatch {
    display: inline-grid;
    place-items: center;
    width: 1.35rem;
    height: 1.35rem;
    border-radius: 9999px;
    border: 2px solid;
    font-size: 0.65rem;
    font-weight: 800;
  }

  .color-field {
    gap: 0.3rem;
  }

  .color-ctrl {
    display: flex;
    align-items: center;
    gap: 0.3rem;
  }

  .color-ctrl input[type='color'] {
    width: 100%;
    min-width: 2.6rem;
    height: 2.2rem;
  }

  .clear-btn {
    min-width: 2rem;
    min-height: 2.2rem;
    padding: 0;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--bg-subtle);
    color: var(--ink-secondary);
    font-size: 1rem;
    line-height: 1;
    cursor: pointer;
  }

  .contrast-warn {
    margin: 0;
    padding: 0.45rem 0.65rem;
    border: 1px solid #e7c079;
    border-radius: var(--radius-sm);
    background: #fef8eb;
    color: #7a4b0b;
    font-size: 0.82rem;
    font-weight: 500;
  }

  .reset-widget-btn {
    width: 100%;
    font-size: 0.84rem;
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

  @media (max-width: 640px) {
    .props-popover {
      left: 0.75rem !important;
      right: 0.75rem !important;
      top: auto !important;
      bottom: 0.75rem !important;
      width: auto;
      max-height: 75vh;
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
