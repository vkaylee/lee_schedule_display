<script>
  import { onMount } from 'svelte';
  import { localTime, instant } from './timezone.js';
  let { csrf } = $props();
  let rooms = $state([]), bookings = $state([]), error = $state(''), notice = $state(''), busy = $state(false), zone = $state('');
  let originalStart = 0, originalEnd = 0, originalStartText = '', originalEndText = '';
  let id = $state(null), version = $state(0), roomId = $state(''), title = $state(''), organizer = $state(''), isPrivate = $state(false), starts = $state(''), ends = $state('');
  async function load() {
    try {
      const [r, b, z] = await Promise.all([fetch('/api/rooms'), fetch('/api/bookings'), fetch('/api/timezone')]);
      if (!r.ok || !b.ok || !z.ok) throw new Error();
      rooms = await r.json(); bookings = await b.json(); zone = (await z.json()).timezone;
    } catch { error = 'Không thể tải lịch.'; }
  }
  function clear() { id = null; roomId = ''; title = ''; organizer = ''; isPrivate = false; starts = ''; ends = ''; }
  function edit(b) {
    if (b.starts % 60 !== 0 || b.ends % 60 !== 0) { error = 'Lịch có độ chính xác đến giây không thể sửa bằng giao diện này.'; return; }
    id = b.id; version = b.version; roomId = String(b.room_id); title = b.title; organizer = b.organizer; isPrivate = b.private;
    starts = localTime(b.starts, zone);
    ends = localTime(b.ends, zone);
    originalStart = b.starts; originalEnd = b.ends; originalStartText = starts; originalEndText = ends;
  }
  async function save(event) {
    event.preventDefault(); error = ''; notice = ''; busy = true;
    try {
      const start = id && starts === originalStartText ? originalStart : instant(starts, zone);
      const end = id && ends === originalEndText ? originalEnd : instant(ends, zone);
      if (start === null || end === null) { error = 'Giờ không tồn tại hoặc mơ hồ do chuyển giờ. Hãy chọn giờ khác.'; return; }
      const response = await fetch(id ? `/api/bookings/${id}` : '/api/bookings', {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: JSON.stringify({room_id: Number(roomId), title: title.trim(), organizer: organizer.trim(), private: isPrivate, starts: start, ends: end, version: id ? version : 0}),
      });
      if (response.status === 409) { error = 'Lịch đã thay đổi hoặc phòng có lịch trùng. Tải lại và thử lại.'; return; }
      if (!response.ok) throw new Error();
      clear(); await load(); notice = 'Đã lưu lịch.';
    } catch { error = 'Không thể lưu lịch. Kiểm tra thông tin và thử lại.'; }
    finally { busy = false; }
  }
  async function cancel(b) {
    if (!window.confirm('Hủy lịch này?')) return;
    busy = true; error = ''; notice = '';
    try {
      const response = await fetch(`/api/bookings/${b.id}/cancel`, { method:'POST', headers:{'X-CSRF-Token':csrf, 'If-Match': String(b.version)} });
      if (!response.ok) throw new Error();
      await load(); notice = 'Đã hủy lịch.';
    } catch { error = 'Không thể hủy lịch.'; }
    finally { busy = false; }
  }
  onMount(load);
</script>
<section aria-labelledby="bookings-heading">
  <h2 id="bookings-heading">Lịch phòng</h2>
  <p>Múi giờ hệ thống: {zone || 'đang tải'}.</p>
  {#if error}<p role="alert">{error}</p>{/if}
  {#if notice}<p role="status">{notice}</p>{/if}
  {#if bookings.length === 0}<p>Chưa có lịch.</p>{/if}
  <ul>{#each bookings as b (b.id)}<li>{rooms.find(r => r.id === b.room_id)?.name ?? 'Phòng không tồn tại'} — {b.title} — {zone ? localTime(b.starts, zone) : ''} ({new Date(b.starts * 1000).toISOString()}) đến {zone ? localTime(b.ends, zone) : ''} ({new Date(b.ends * 1000).toISOString()}) {b.canceled ? '(Đã hủy)' : ''}
    {#if !b.canceled}<button type="button" disabled={busy} aria-label={`Sửa lịch ${b.title}`} onclick={() => edit(b)}>Sửa</button><button type="button" disabled={busy} aria-label={`Hủy lịch ${b.title}`} onclick={() => cancel(b)}>Hủy lịch</button>{/if}
  </li>{/each}</ul>
  <form onsubmit={save}>
    <h3>{id ? 'Sửa lịch' : 'Tạo lịch'}</h3>
    <label for="booking-room">Phòng</label><select id="booking-room" bind:value={roomId} required><option value="">Chọn phòng</option>{#each rooms.filter(r => r.active) as room}<option value={String(room.id)}>{room.name}</option>{/each}</select>
    <label for="booking-title">Tiêu đề</label><input id="booking-title" bind:value={title} maxlength="256" required />
    <label for="booking-organizer">Người tổ chức</label><input id="booking-organizer" bind:value={organizer} maxlength="128" required />
    <label><input type="checkbox" bind:checked={isPrivate} /> Riêng tư</label>
    <label for="booking-start">Bắt đầu</label><input id="booking-start" type="datetime-local" bind:value={starts} required />
    <label for="booking-end">Kết thúc</label><input id="booking-end" type="datetime-local" bind:value={ends} required />
    <button type="submit" disabled={busy}>Lưu lịch</button>{#if id}<button type="button" onclick={clear} disabled={busy}>Hủy sửa</button>{/if}
  </form>
</section>
<style>
  section { margin-top: 2rem; padding-top: 1.5rem; border-top: 1px solid #c9d7d9; }
  form { display: grid; gap: .5rem; }
  input, select, button { font: inherit; }
  li { overflow-wrap: anywhere; margin-bottom: .5rem; }
  [role='alert'] { color: #7d261e; }
</style>
