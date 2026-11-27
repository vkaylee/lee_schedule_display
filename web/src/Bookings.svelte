<script>
  import { onMount } from 'svelte';
  import { localTime, instant } from './timezone.js';
  let { csrf, language = 'vi' } = $props();
  const t = (vi, en) => language === 'en' ? en : vi;
  let rooms = $state([]), bookings = $state([]), error = $state(''), notice = $state(''), busy = $state(false), zone = $state('');
  let originalStart = 0, originalEnd = 0, originalStartText = '', originalEndText = '';
  let id = $state(null), version = $state(0), roomId = $state(''), title = $state(''), organizer = $state(''), isPrivate = $state(false), starts = $state(''), ends = $state('');
  async function load() {
    try {
      const [r, b, z] = await Promise.all([fetch('/api/rooms'), fetch('/api/bookings'), fetch('/api/timezone')]);
      if (!r.ok || !b.ok || !z.ok) throw new Error();
      rooms = await r.json(); bookings = await b.json(); zone = (await z.json()).timezone;
    } catch { error = t('Không thể tải lịch.', 'Could not load bookings.'); }
  }
  function clear() { id = null; roomId = ''; title = ''; organizer = ''; isPrivate = false; starts = ''; ends = ''; }
  function edit(b) {
    if (b.starts % 60 !== 0 || b.ends % 60 !== 0) { error = t('Lịch có độ chính xác đến giây không thể sửa bằng giao diện này.', 'Bookings with second-level precision cannot be edited here.'); return; }
    id = b.id; version = b.version; roomId = String(b.room_id); title = b.title; organizer = b.organizer; isPrivate = b.private;
    starts = localTime(b.starts, zone);
    ends = localTime(b.ends, zone);
    originalStart = b.starts; originalEnd = b.ends; originalStartText = starts; originalEndText = ends;
  }
  async function save(event) {
    event.preventDefault(); error = ''; notice = ''; busy = true;
    try {
      if (!zone) { error = t('Chưa tải được múi giờ hệ thống. Vui lòng tải lại trang.', 'System time zone has not loaded. Please reload the page.'); return; }
      const start = id && starts === originalStartText ? originalStart : instant(starts, zone);
      const end = id && ends === originalEndText ? originalEnd : instant(ends, zone);
      if (start === null || end === null) { error = t('Giờ không tồn tại hoặc mơ hồ do chuyển giờ. Hãy chọn giờ khác.', 'Time is invalid or ambiguous due to a clock change. Choose another time.'); return; }
      const response = await fetch(id ? `/api/bookings/${id}` : '/api/bookings', {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: JSON.stringify({room_id: Number(roomId), title: title.trim(), organizer: organizer.trim(), private: isPrivate, starts: start, ends: end, version: id ? version : 0}),
      });
      if (response.status === 409) { error = t('Lịch đã thay đổi hoặc phòng có lịch trùng. Tải lại và thử lại.', 'Booking changed or overlaps another booking. Reload and try again.'); return; }
      if (!response.ok) throw new Error();
      clear(); await load(); notice = t('Đã lưu lịch.', 'Booking saved.');
    } catch { error = t('Không thể lưu lịch. Kiểm tra thông tin và thử lại.', 'Could not save booking. Check the details and try again.'); }
    finally { busy = false; }
  }
  async function cancel(b) {
    if (!window.confirm(t('Hủy lịch này?', 'Cancel this booking?'))) return;
    busy = true; error = ''; notice = '';
    try {
      const response = await fetch(`/api/bookings/${b.id}/cancel`, { method:'POST', headers:{'X-CSRF-Token':csrf, 'If-Match': String(b.version)} });
      if (!response.ok) throw new Error();
      await load(); notice = t('Đã hủy lịch.', 'Booking canceled.');
    } catch { error = t('Không thể hủy lịch.', 'Could not cancel booking.'); }
    finally { busy = false; }
  }
  onMount(load);
</script>
<section aria-labelledby="bookings-heading">
  <h2 id="bookings-heading">{t('Lịch phòng', 'Room bookings')}</h2>
  <p>{t('Múi giờ hệ thống:', 'System time zone:')} {zone || t('đang tải', 'loading')}.</p>
  {#if error}<p role="alert">{error}</p>{/if}
  {#if notice}<p role="status">{notice}</p>{/if}
  {#if bookings.length === 0}<p>{t('Chưa có lịch.', 'No bookings yet.')}</p>{/if}
  <ul>{#each bookings as b (b.id)}<li>{rooms.find(r => r.id === b.room_id)?.name ?? t('Phòng không tồn tại', 'Room not found')} — {b.title} — {zone ? localTime(b.starts, zone) : ''} ({new Date(b.starts * 1000).toISOString()}) {t('đến', 'to')} {zone ? localTime(b.ends, zone) : ''} ({new Date(b.ends * 1000).toISOString()}) {b.canceled ? t('(Đã hủy)', '(Canceled)') : ''}
    {#if !b.canceled}<button type="button" disabled={busy} aria-label={`${t('Sửa lịch', 'Edit booking')} ${b.title}`} onclick={() => edit(b)}>{t('Sửa', 'Edit')}</button><button type="button" disabled={busy} aria-label={`${t('Hủy lịch', 'Cancel booking')} ${b.title}`} onclick={() => cancel(b)}>{t('Hủy lịch', 'Cancel booking')}</button>{/if}
  </li>{/each}</ul>
  <form onsubmit={save}>
    <h3>{id ? t('Sửa lịch', 'Edit booking') : t('Tạo lịch', 'Create booking')}</h3>
    <label for="booking-room">{t('Phòng', 'Room')}</label><select id="booking-room" bind:value={roomId} required><option value="">{t('Chọn phòng', 'Select room')}</option>{#each rooms.filter(r => r.active) as room}<option value={String(room.id)}>{room.name}</option>{/each}</select>
    <label for="booking-title">{t('Tiêu đề', 'Title')}</label><input id="booking-title" bind:value={title} maxlength="256" required />
    <label for="booking-organizer">{t('Người tổ chức', 'Organizer')}</label><input id="booking-organizer" bind:value={organizer} maxlength="128" required />
    <label><input type="checkbox" bind:checked={isPrivate} /> {t('Riêng tư', 'Private')}</label>
    <label for="booking-start">{t('Bắt đầu', 'Start')}</label><input id="booking-start" type="datetime-local" bind:value={starts} required />
    <label for="booking-end">{t('Kết thúc', 'End')}</label><input id="booking-end" type="datetime-local" bind:value={ends} required />
    <button type="submit" disabled={busy}>{t('Lưu lịch', 'Save booking')}</button>{#if id}<button type="button" onclick={clear} disabled={busy}>{t('Hủy sửa', 'Cancel editing')}</button>{/if}
  </form>
</section>
<style>
  section { margin-top: 2rem; padding-top: 1.5rem; border-top: 1px solid #c9d7d9; }
  form { display: grid; gap: .5rem; }
  input, select, button { font: inherit; }
  li { overflow-wrap: anywhere; margin-bottom: .5rem; }
  [role='alert'] { color: #7d261e; }
</style>
