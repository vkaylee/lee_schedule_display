<script>
  import { onMount } from 'svelte';
  let { csrf } = $props();
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
    } catch { error = 'Không thể tải danh sách phòng.'; }
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
    if (new TextEncoder().encode(name.trim()).length > 128) { error = 'Tên phòng không được vượt quá 128 byte UTF-8.'; return; }
    if (editing && !active && !window.confirm('Vô hiệu hóa phòng này? Dữ liệu lịch sẽ được giữ lại.')) return;
    busy = true; error = ''; notice = '';
    try {
      const response = await fetch(editing ? `/api/rooms/${editing}` : '/api/rooms', {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: JSON.stringify({ name: name.trim(), active, show_title: showTitle, show_organizer: showOrganizer }),
      });
      if (response.status === 409) { error = 'Tên phòng đã tồn tại.'; return; }
      if (!response.ok) throw new Error();
      clear(); await load(); notice = 'Đã lưu phòng.';
    } catch { error = 'Không thể lưu phòng. Vui lòng thử lại.'; }
    finally { busy = false; }
  }
  onMount(load);
</script>

<section aria-labelledby="rooms-heading">
  <h2 id="rooms-heading">Quản lý phòng</h2>
  {#if error}<p role="alert">{error}</p>{/if}
  {#if notice}<p role="status">{notice}</p>{/if}
  {#if loading}<p role="status">Đang tải phòng…</p>
  {:else if rooms.length === 0}<p>Chưa có phòng.</p>
  {:else}
    <ul>
      {#each rooms as room (room.id)}
        <li>{room.name} — {room.active ? 'Đang hoạt động' : 'Đã vô hiệu hóa'}
          <button type="button" disabled={busy} aria-label={`Sửa phòng ${room.name}`} onclick={() => edit(room)}>Sửa</button>
        </li>
      {/each}
    </ul>
  {/if}
  <form onsubmit={save}>
    <h3>{editing ? 'Sửa phòng' : 'Tạo phòng'}</h3>
    <label for="room-name">Tên phòng</label>
    <input id="room-name" bind:value={name} maxlength="128" required />
    {#if editing}<label><input type="checkbox" bind:checked={active} /> Đang hoạt động</label>{/if}
    <label><input type="checkbox" bind:checked={showTitle} /> Hiển thị tiêu đề trên màn hình</label>
    <label><input type="checkbox" bind:checked={showOrganizer} /> Hiển thị người tổ chức trên màn hình</label>
    <button type="submit" disabled={busy}>Lưu phòng</button>
    {#if editing}<button type="button" disabled={busy} onclick={clear}>Hủy</button>{/if}
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
