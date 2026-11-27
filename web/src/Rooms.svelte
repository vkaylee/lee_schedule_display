<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import Banner from './Banner.svelte';
  let { csrf, language = 'vi' } = $props();
  const t = (vi, en) => language === 'en' ? en : vi;
  let rooms = $state([]);
  let loading = $state(true);
  let busy = $state(false);
  let error = $state('');
  let notice = $state('');
  let name = $state('');
  let editing = $state(null);
  let showTitle = $state(false);
  let showOrganizer = $state(false);
  let active = $state(true);

  async function load() {
    loading = true;
    error = '';
    try {
      const response = await fetch('/api/rooms', { cache: 'no-store' });
      if (!response.ok) throw new Error();
      rooms = await response.json();
    } catch { error = ['Không thể tải danh sách phòng.', 'Could not load rooms.']; }
    finally { loading = false; }
  }
  function edit(room) {
    editing = room.id;
    name = room.name;
    active = room.active;
    showTitle = room.show_title;
    showOrganizer = room.show_organizer;
    notice = '';
  }
  function clear() { editing = null; name = ''; active = true; showTitle = false; showOrganizer = false; }
  async function save(event) {
    event.preventDefault();
    if (new TextEncoder().encode(name.trim()).length > 128) { error = ['Tên phòng không được vượt quá 128 byte UTF-8.', 'Room name must not exceed 128 UTF-8 bytes.']; return; }
    if (editing && !active && !window.confirm(t('Vô hiệu hóa phòng này? Dữ liệu lịch sẽ được giữ lại.', 'Disable this room? Booking data will be retained.'))) return;
    busy = true; error = ''; notice = '';
    try {
      const response = await fetch(editing ? `/api/rooms/${editing}` : '/api/rooms', {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: JSON.stringify({ name: name.trim(), active, show_title: showTitle, show_organizer: showOrganizer }),
      });
      if (response.status === 409) { error = ['Tên phòng đã tồn tại.', 'Room name already exists.']; return; }
      if (!response.ok) throw new Error();
      clear(); await load(); window.dispatchEvent(new Event('rooms-changed')); notice = ['Đã lưu phòng.', 'Room saved.'];
    } catch { error = ['Không thể lưu phòng. Vui lòng thử lại.', 'Could not save room. Please try again.']; }
    finally { busy = false; }
  }
  onMount(load);
</script>

<section aria-labelledby="rooms-heading">
  <div class="section-head">
    <div>
      <p class="section-kicker">{t('DANH MỤC PHÒNG HỌP', 'MEETING SPACES')}</p>
      <h2 id="rooms-heading">{t('Quản lý phòng', 'Room management')}</h2>
      <p>{t('Thiết lập phòng họp và chính sách hiển thị thông tin trên màn hình.', 'Configure meeting rooms and public display privacy policies.')}</p>
    </div>
  </div>

  {#if error}<Banner kind="error" text={t(...error)} />{/if}
  {#if notice}<Banner kind="success" text={t(...notice)} />{/if}

  <div class="ui-split">
    <div>
      {#if loading}
        <p class="ui-loading" role="status">
          <span class="ui-spinner" aria-hidden="true"></span>
          <span>{t('Đang tải phòng…', 'Loading rooms…')}</span>
        </p>
      {:else if rooms.length === 0}
        <div class="ui-empty">
          <strong>{t('Chưa có phòng.', 'No rooms yet.')}</strong>
          <p>{t('Sử dụng biểu mẫu bên cạnh để tạo phòng họp đầu tiên.', 'Use the form to create your first meeting room.')}</p>
        </div>
      {:else}
        <ul class="ui-card-list">
          {#each rooms as room (room.id)}
            <li class="ui-card room-card" class:is-editing={editing === room.id}>
              <div class="room-main">
                <div class="room-title-row">
                  <strong>{room.name}</strong>
                  <span class={room.active ? 'ui-pill ui-pill--success' : 'ui-pill ui-pill--neutral'}>
                    {room.active ? t('Đang hoạt động', 'Active') : t('Đã vô hiệu hóa', 'Disabled')}
                  </span>
                </div>
                <div class="room-meta">
                  <span class="meta-tag" class:meta-tag--on={room.show_title}>
                    {room.show_title ? t('Hiện tiêu đề', 'Title visible') : t('Ẩn tiêu đề', 'Title hidden')}
                  </span>
                  <span class="meta-tag" class:meta-tag--on={room.show_organizer}>
                    {room.show_organizer ? t('Hiện người tổ chức', 'Organizer visible') : t('Ẩn người tổ chức', 'Organizer hidden')}
                  </span>
                </div>
              </div>
              <button class="secondary" type="button" disabled={busy} aria-label={`${t('Sửa phòng', 'Edit room')} ${room.name}`} onclick={() => edit(room)}>
                <Icon name="edit" />
                <span>{t('Sửa', 'Edit')}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <div class="ui-sticky-pane">
      <form class:ui-form-editing={editing !== null} onsubmit={save}>
        <h3>{editing ? t('Sửa phòng', 'Edit room') : t('Tạo phòng', 'Create room')}</h3>
        <label for="room-name">{t('Tên phòng', 'Room name')}</label>
        <input id="room-name" bind:value={name} maxlength="128" required placeholder={t('Ví dụ: Phòng họp Tầng 3', 'e.g. Boardroom 3A')} />

        {#if editing}
          <label class="toggle-row">
            <input type="checkbox" bind:checked={active} />
            <span>
              <strong>{t('Đang hoạt động', 'Active')}</strong>
              <small>{t('Bỏ chọn để ngừng đặt lịch mới nhưng vẫn giữ lịch cũ.', 'Uncheck to stop new bookings while preserving history.')}</small>
            </span>
          </label>
        {/if}

        <label class="toggle-row">
          <input type="checkbox" bind:checked={showTitle} />
          <span>
            <strong>{t('Hiển thị tiêu đề trên màn hình', 'Show title on display')}</strong>
            <small>{t('Cho phép hiện tên cuộc họp trên kiosk và màn hình công khai.', 'Allow meeting titles to appear on kiosks and public displays.')}</small>
          </span>
        </label>

        <label class="toggle-row">
          <input type="checkbox" bind:checked={showOrganizer} />
          <span>
            <strong>{t('Hiển thị người tổ chức trên màn hình', 'Show organizer on display')}</strong>
            <small>{t('Cho phép hiện tên người đặt lịch trên kiosk.', 'Allow organizer names to appear on room kiosks.')}</small>
          </span>
        </label>

        <div class="ui-form-actions">
          <button type="submit" disabled={busy}>{t('Lưu phòng', 'Save room')}</button>
          {#if editing}
            <button class="secondary" type="button" disabled={busy} onclick={clear}>{t('Hủy', 'Cancel')}</button>
          {/if}
        </div>
      </form>
    </div>
  </div>
</section>

<style>
  .room-card {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.9rem;
  }

  .room-main {
    min-width: 0;
    flex: 1 1 14rem;
  }

  .room-title-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.55rem;
    margin-bottom: 0.45rem;
  }

  .room-title-row strong {
    font-size: 1.02rem;
    color: var(--ink-primary);
  }

  .room-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .meta-tag {
    display: inline-flex;
    align-items: center;
    padding: 0.15rem 0.55rem;
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
    font-weight: 500;
    background: var(--bg-subtle);
    color: var(--ink-muted);
    border: 1px solid var(--border-subtle);
  }

  .meta-tag--on {
    background: #eef6f0;
    color: #195637;
    border-color: #cde2d3;
  }

  .toggle-row {
    display: flex;
    align-items: flex-start;
    gap: 0.65rem;
    padding: 0.65rem 0.75rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--bg-surface);
    cursor: pointer;
  }

  .toggle-row input[type='checkbox'] {
    margin-top: 0.2rem;
  }

  .toggle-row span {
    display: grid;
    gap: 0.1rem;
  }

  .toggle-row strong {
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--ink-primary);
  }

  .toggle-row small {
    font-size: 0.8rem;
    font-weight: 400;
    color: var(--ink-muted);
    line-height: 1.35;
  }
</style>
