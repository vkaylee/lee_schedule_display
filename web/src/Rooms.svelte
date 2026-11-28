<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import Banner from './Banner.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import Modal from './Modal.svelte';
  let confirmDialog = $state();
  let { csrf, language = 'vi' } = $props();
  const t = (vi, en) => language === 'en' ? en : vi;
  let rooms = $state([]);
  let loading = $state(true);
  let busy = $state(false);
  let error = $state('');
  let modalError = $state('');
  let notice = $state('');
  let modalOpen = $state(false);
  let name = $state('');
  let editing = $state(null);
  let showTitle = $state(false);
  let showOrganizer = $state(false);
  let active = $state(true);

  async function load() {
    if (rooms.length === 0) loading = true;
    error = '';
    try {
      const response = await fetch('/api/rooms', { cache: 'no-store' });
      if (!response.ok) throw new Error();
      rooms = await response.json();
    } catch { error = ['Không thể tải danh sách phòng.', 'Could not load rooms.']; }
    finally { loading = false; }
  }
  function openCreate() {
    clear();
    modalError = '';
    notice = '';
    modalOpen = true;
  }
  function edit(room) {
    editing = room.id;
    name = room.name;
    active = room.active;
    showTitle = room.show_title;
    showOrganizer = room.show_organizer;
    modalError = '';
    notice = '';
    modalOpen = true;
  }
  function clear() { editing = null; name = ''; active = true; showTitle = false; showOrganizer = false; }
  function closeModal() {
    if (busy) return;
    modalOpen = false;
    modalError = '';
    clear();
  }
  async function save(event) {
    event.preventDefault();
    if (new TextEncoder().encode(name.trim()).length > 128) { modalError = ['Tên phòng không được vượt quá 128 byte UTF-8.', 'Room name must not exceed 128 UTF-8 bytes.']; return; }
    if (editing && !active && !await confirmDialog.ask(t('Vô hiệu hóa phòng này? Dữ liệu lịch sẽ được giữ lại.', 'Disable this room? Booking data will be retained.'))) return;
    busy = true; modalError = ''; notice = '';
    try {
      const response = await fetch(editing ? `/api/rooms/${editing}` : '/api/rooms', {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: JSON.stringify({ name: name.trim(), active, show_title: showTitle, show_organizer: showOrganizer }),
      });
      if (response.status === 409) { modalError = ['Tên phòng đã tồn tại.', 'Room name already exists.']; return; }
      if (!response.ok) throw new Error();
      modalOpen = false; clear(); await load(); window.dispatchEvent(new Event('rooms-changed')); notice = ['Đã lưu phòng.', 'Room saved.'];
    } catch { modalError = ['Không thể lưu phòng. Vui lòng thử lại.', 'Could not save room. Please try again.']; }
    finally { busy = false; }
  }
  onMount(load);
</script>

<section aria-labelledby="rooms-heading">
  <div class="section-head">
    <div>
      <p class="section-kicker">{t('DANH MỤC PHÒNG HỌP', 'MEETING SPACES')}</p>
      <h2 id="rooms-heading">{t('Quản lý phòng', 'Room management')}</h2>
      <p>{t('Thiết lập và quản lý phòng họp.', 'Configure and manage meeting rooms.')}</p>
    </div>
    <div class="section-actions">
      <button type="button" disabled={busy} onclick={openCreate}>
        <Icon name="plus" />
        <span>{t('Tạo phòng', 'Create room')}</span>
      </button>
    </div>
  </div>

  {#if error}<Banner kind="error" text={t(...error)} />{/if}
  {#if notice}<Banner kind="success" text={t(...notice)} />{/if}

  <div class="ui-view-body">
    {#if loading}
      <p class="ui-loading" role="status">
        <span class="ui-spinner" aria-hidden="true"></span>
        <span>{t('Đang tải phòng…', 'Loading rooms…')}</span>
      </p>
    {:else if rooms.length === 0}
      <div class="ui-empty">
        <strong>{t('Chưa có phòng.', 'No rooms yet.')}</strong>
        <p>{t('Bấm "Tạo phòng" để thêm phòng họp đầu tiên.', 'Click "Create room" to add your first meeting room.')}</p>
      </div>
    {:else}
      <ul class="ui-card-list">
        {#each rooms as room (room.id)}
          <li class="ui-card room-card" class:is-editing={modalOpen && editing === room.id}>
            <div class="room-main">
              <div class="room-title-row">
                <strong>{room.name}</strong>
                <span class={room.active ? 'ui-pill ui-pill--success' : 'ui-pill ui-pill--neutral'}>
                  {room.active ? t('Đang hoạt động', 'Active') : t('Đã vô hiệu hóa', 'Disabled')}
                </span>
                {#if room.show_title}
                  <span class="ui-pill ui-pill--neutral">{t('Hiện tiêu đề', 'Title visible')}</span>
                {/if}
                {#if room.show_organizer}
                  <span class="ui-pill ui-pill--neutral">{t('Hiện người tổ chức', 'Organizer visible')}</span>
                {/if}
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

  <Modal
    open={modalOpen}
    title={editing ? t('Sửa phòng', 'Edit room') : t('Tạo phòng', 'Create room')}
    {busy}
    closeLabel={t('Đóng', 'Close')}
    onclose={closeModal}
  >
    <ConfirmDialog bind:this={confirmDialog} {language} />
    {#if modalError}<Banner kind="error" text={t(...modalError)} />{/if}
    <form onsubmit={save}>
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
          <strong>{t('Hiển thị tiêu đề cuộc họp trên màn hình', 'Show meeting title on displays')}</strong>
          <small>{t('Lịch được đánh dấu riêng tư vẫn luôn ẩn tiêu đề.', 'Bookings marked private still hide the title.')}</small>
        </span>
      </label>

      <label class="toggle-row">
        <input type="checkbox" bind:checked={showOrganizer} />
        <span>
          <strong>{t('Hiển thị người tổ chức trên màn hình', 'Show organizer on displays')}</strong>
          <small>{t('Lịch được đánh dấu riêng tư vẫn luôn ẩn người tổ chức.', 'Bookings marked private still hide the organizer.')}</small>
        </span>
      </label>

      <div class="ui-form-actions">
        <button type="submit" disabled={busy}>{t('Lưu phòng', 'Save room')}</button>
        <button class="secondary" type="button" disabled={busy} onclick={closeModal}>{t('Hủy', 'Cancel')}</button>
      </div>
    </form>
  </Modal>
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
