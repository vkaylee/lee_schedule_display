<script>
  import { onMount } from 'svelte';
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
  <h2 id="rooms-heading">{t('Quản lý phòng', 'Room management')}</h2>
  {#if error}<p role="alert">{t(...error)}</p>{/if}
  {#if notice}<p role="status">{t(...notice)}</p>{/if}
  {#if loading}<p role="status">{t('Đang tải phòng…', 'Loading rooms…')}</p>
  {:else if rooms.length === 0}<p>{t('Chưa có phòng.', 'No rooms yet.')}</p>
  {:else}
    <ul>
      {#each rooms as room (room.id)}
        <li>{room.name} — {room.active ? t('Đang hoạt động', 'Active') : t('Đã vô hiệu hóa', 'Disabled')}
          <button type="button" disabled={busy} aria-label={`${t('Sửa phòng', 'Edit room')} ${room.name}`} onclick={() => edit(room)}>{t('Sửa', 'Edit')}</button>
        </li>
      {/each}
    </ul>
  {/if}
  <form onsubmit={save}>
    <h3>{editing ? t('Sửa phòng', 'Edit room') : t('Tạo phòng', 'Create room')}</h3>
    <label for="room-name">{t('Tên phòng', 'Room name')}</label>
    <input id="room-name" bind:value={name} maxlength="128" required />
    {#if editing}<label><input type="checkbox" bind:checked={active} /> {t('Đang hoạt động', 'Active')}</label>{/if}
    <label><input type="checkbox" bind:checked={showTitle} /> {t('Hiển thị tiêu đề trên màn hình', 'Show title on display')}</label>
    <label><input type="checkbox" bind:checked={showOrganizer} /> {t('Hiển thị người tổ chức trên màn hình', 'Show organizer on display')}</label>
    <button type="submit" disabled={busy}>{t('Lưu phòng', 'Save room')}</button>
    {#if editing}<button type="button" disabled={busy} onclick={clear}>{t('Hủy', 'Cancel')}</button>{/if}
  </form>
</section>
<style>
  section { margin-top: 2rem; padding-top: 1.5rem; border-top: 1px solid #c9d7d9; }
  form { display: grid; gap: .5rem; }
  input:not([type='checkbox']) { padding: .5rem; font: inherit; }
  label { line-height: 1.5; }
  button { font: inherit; padding: .5rem; }
  li { overflow-wrap: anywhere; margin-bottom: .5rem; }
  [role='alert'] { color: #7d261e; }
</style>
