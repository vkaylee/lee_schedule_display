<script>
  import { onMount } from 'svelte';
  import { localTime, localTimeSeconds, instant } from './timezone.js';
  let { csrf, language = 'vi' } = $props();
  const t = (vi, en) => language === 'en' ? en : vi;
  let rooms = $state([]), bookings = $state([]), series = $state([]), error = $state(''), notice = $state(''), busy = $state(false), zone = $state('');
  const seriesEnabled = true;
  let repeatFrequency = $state(1), repeatInterval = $state(1), repeatCount = $state(2), repeatUntil = $state(''), repeatEnd = $state('count');
  let repeatWeekdays = $state([]), repeatMonthMode = $state('day'), repeatMonthDay = $state(1), repeatNth = $state(1), repeatNthWeekday = $state(1);
  let originalStart = 0, originalEnd = 0, originalStartText = '', originalEndText = '';
  let id = $state(null), version = $state(0), roomId = $state(''), title = $state(''), organizer = $state(''), isPrivate = $state(false), starts = $state(''), ends = $state('');
  async function load() {
    try {
      const [r, b, z, s] = await Promise.all([fetch('/api/rooms'), fetch('/api/bookings'), fetch('/api/timezone'), seriesEnabled ? fetch('/api/series') : Promise.resolve(null)]);
      if (!r.ok || !b.ok || !z.ok || (s && !s.ok)) throw new Error();
      rooms = await r.json(); bookings = await b.json(); zone = (await z.json()).timezone; series = s ? await s.json() : [];
    } catch { error = ['Không thể tải lịch.', 'Could not load bookings.']; }
  }
  function clear() { id = null; roomId = ''; title = ''; organizer = ''; isPrivate = false; starts = ''; ends = ''; }
  function edit(b) {
    if (b.starts % 60 !== 0 || b.ends % 60 !== 0) { error = ['Lịch có độ chính xác đến giây không thể sửa bằng giao diện này.', 'Bookings with second-level precision cannot be edited here.']; return; }
    id = b.id; version = b.version; roomId = String(b.room_id); title = b.title; organizer = b.organizer; isPrivate = b.private;
    starts = localTime(b.starts, zone);
    ends = localTime(b.ends, zone);
    originalStart = b.starts; originalEnd = b.ends; originalStartText = starts; originalEndText = ends;
  }
  async function save(event) {
    event.preventDefault(); error = ''; notice = ''; busy = true;
    try {
      if (!zone) { error = ['Chưa tải được múi giờ hệ thống. Vui lòng tải lại trang.', 'System time zone has not loaded. Please reload the page.']; return; }
      const start = id && starts === originalStartText ? originalStart : instant(starts, zone);
      const end = id && ends === originalEndText ? originalEnd : instant(ends, zone);
      if (start === null || end === null) { error = ['Giờ không tồn tại hoặc mơ hồ do chuyển giờ. Hãy chọn giờ khác.', 'Time is invalid or ambiguous due to a clock change. Choose another time.']; return; }
      const response = await fetch(id ? `/api/bookings/${id}` : '/api/bookings', {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: JSON.stringify({room_id: Number(roomId), title: title.trim(), organizer: organizer.trim(), private: isPrivate, starts: start, ends: end, version: id ? version : 0}),
      });
      if (response.status === 409) { error = ['Lịch đã thay đổi hoặc phòng có lịch trùng. Tải lại và thử lại.', 'Booking changed or overlaps another booking. Reload and try again.']; return; }
      if (!response.ok) throw new Error();
      clear(); await load(); notice = ['Đã lưu lịch.', 'Booking saved.'];
    } catch { error = ['Không thể lưu lịch. Kiểm tra thông tin và thử lại.', 'Could not save booking. Check the details and try again.']; }
    finally { busy = false; }
  }
  async function saveSeries(event) {
    event.preventDefault(); error = ''; notice = ''; busy = true;
    try {
      const start = instant(starts, zone), end = instant(ends, zone);
      if (start === null || end === null || end <= start) { error = ['Giờ không hợp lệ hoặc mơ hồ.', 'Invalid or ambiguous time.']; return; }
      const frequency = Number(repeatFrequency);
      const rule = {frequency, interval:Number(repeatInterval), count:repeatEnd === 'count' ? Number(repeatCount) : 0, until:repeatEnd === 'until' ? repeatUntil : ''};
      if (frequency === 2) rule.weekdays = repeatWeekdays.map(Number);
      if (frequency === 3 && repeatMonthMode === 'day') rule.month_day = Number(repeatMonthDay);
      if (frequency === 3 && repeatMonthMode === 'nth') { rule.nth = Number(repeatNth); rule.nth_weekday = Number(repeatNthWeekday); }
      const response = await fetch('/api/series', { method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':csrf}, body:JSON.stringify({room_id:Number(roomId),title:title.trim(),organizer:organizer.trim(),private:isPrivate,starts:start,duration_seconds:end-start,...rule}) });
      if (response.status === 409) { error = ['Lịch trùng.', 'Booking conflict.']; return; }
      if (!response.ok) { const message = await response.text(); error = message.includes('12-month') ? ['Chuỗi phải kết thúc trong giới hạn 12 tháng.', 'The series must end within the 12-month limit.'] : ['Quy tắc hoặc giờ không hợp lệ, hoặc giờ bị mơ hồ do DST.', 'Invalid rule or time, or ambiguous DST time.']; return; }
      clear(); await load(); notice = ['Đã tạo chuỗi.', 'Series created.'];
    } catch { error = ['Không thể tạo chuỗi.', 'Could not create series.']; }
    finally { busy = false; }
  }
  async function cancel(b) {
    if (!window.confirm(t('Hủy lịch này?', 'Cancel this booking?'))) return;
    busy = true; error = ''; notice = '';
    try {
      const response = await fetch(`/api/bookings/${b.id}/cancel`, { method:'POST', headers:{'X-CSRF-Token':csrf, 'If-Match': String(b.version)} });
      if (!response.ok) throw new Error();
      await load(); notice = ['Đã hủy lịch.', 'Booking canceled.'];
    } catch { error = ['Không thể hủy lịch.', 'Could not cancel booking.']; }
    finally { busy = false; }
  }
  onMount(() => {
    load();
    window.addEventListener('rooms-changed', load);
    return () => window.removeEventListener('rooms-changed', load);
  });
</script>
<section aria-labelledby="bookings-heading">
  <h2 id="bookings-heading">{t('Lịch phòng', 'Room bookings')}</h2>
  <p>{t('Múi giờ hệ thống:', 'System time zone:')} {zone || t('đang tải', 'loading')}.</p>
  {#if error}<p role="alert">{t(...error)}</p>{/if}
  {#if notice}<p role="status">{t(...notice)}</p>{/if}
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
  {#if seriesEnabled}
  <h3>{t('Chuỗi lịch', 'Recurring series')}</h3>
  <p>{t('Chỉ nhận chuỗi kết thúc trong 12 tháng tính từ ngày bắt đầu hoặc hiện tại, mốc nào muộn hơn. Không thể sửa hoặc hủy chuỗi bằng giao diện này.', 'Only series ending within 12 months of the later of now or the start are accepted. This interface cannot edit or cancel a series.')}</p>
  <ul>{#each series as s (s.id)}<li>
    <strong>{rooms.find(r => r.id === s.room_id)?.name ?? s.room_id} — {s.title}</strong> — {s.organizer} {s.private ? t('(Riêng tư)', '(Private)') : ''}
    <p>{t('Múi giờ chuỗi', 'Series time zone')}: {s.rule.location}; {t('bắt đầu', 'start')}: {localTimeSeconds(Date.parse(s.rule.rule.Start) / 1000, s.rule.location)}.</p>
    <p>{t('Lặp mỗi', 'Repeat every')} {s.rule.rule.Interval} {s.rule.rule.Frequency === 1 ? t('ngày', 'days') : s.rule.rule.Frequency === 2 ? t('tuần', 'weeks') : t('tháng', 'months')}
    {#if s.rule.rule.Frequency === 2} — {t('thứ', 'weekdays')}: {s.rule.rule.Weekdays.join(', ')}{/if}
    {#if s.rule.rule.Frequency === 3} — {s.rule.rule.MonthDay ? `${t('ngày', 'day')} ${s.rule.rule.MonthDay}` : `${t('thứ tự', 'position')} ${s.rule.rule.Nth}, ${t('thứ', 'weekday')} ${s.rule.rule.NthWeekday}`}{/if};
    {#if s.rule.rule.Count} {s.rule.rule.Count} {t('buổi', 'occurrences')}{:else} {t('đến hết ngày', 'through date')} {s.rule.rule.Until.slice(0, 10)}{/if}.</p>
    <p>{t('Đã kiểm tra toàn bộ buổi trong chuỗi đến thời điểm kết thúc buổi cuối', 'All occurrences checked through the end of the last occurrence')}: {localTimeSeconds(s.checked_through, s.rule.location)} ({s.rule.location}).</p>
    <details><summary>{t('Xem các buổi', 'View occurrences')} ({s.occurrences.length})</summary><ul>{#each s.occurrences as occurrence (occurrence.starts)}<li>{localTimeSeconds(occurrence.starts, s.rule.location)} — {localTimeSeconds(occurrence.ends, s.rule.location)}</li>{/each}</ul></details>
  </li>{/each}</ul>
  <form onsubmit={saveSeries}>
    <h3>{t('Tạo chuỗi', 'Create series')}</h3>
    <p>{t('Dùng phòng, tiêu đề, người tổ chức, thời gian và riêng tư từ biểu mẫu lịch ở trên.', 'Uses room, title, organizer, time and privacy from the booking form above.')}</p>
    <label for="repeat-frequency">{t('Tần suất', 'Frequency')}</label><select id="repeat-frequency" bind:value={repeatFrequency}><option value={1}>{t('Ngày', 'Daily')}</option><option value={2}>{t('Tuần', 'Weekly')}</option><option value={3}>{t('Tháng', 'Monthly')}</option></select>
    <label for="repeat-interval">{t('Khoảng lặp', 'Interval')}</label><input id="repeat-interval" type="number" min="1" max="366" bind:value={repeatInterval} required />
    {#if Number(repeatFrequency) === 2}<fieldset><legend>{t('Thứ trong tuần', 'Weekdays')}</legend>{#each [t('Chủ nhật','Sunday'),t('Thứ hai','Monday'),t('Thứ ba','Tuesday'),t('Thứ tư','Wednesday'),t('Thứ năm','Thursday'),t('Thứ sáu','Friday'),t('Thứ bảy','Saturday')] as day, i}<label><input type="checkbox" value={i} bind:group={repeatWeekdays} />{day}</label>{/each}</fieldset>{/if}
    {#if Number(repeatFrequency) === 3}
      <label for="repeat-month-mode">{t('Quy tắc tháng', 'Monthly rule')}</label><select id="repeat-month-mode" bind:value={repeatMonthMode}><option value="day">{t('Ngày trong tháng', 'Day of month')}</option><option value="nth">{t('Thứ tự thứ trong tháng', 'Nth weekday')}</option></select>
      {#if repeatMonthMode === 'day'}<label for="repeat-month-day">{t('Ngày trong tháng', 'Day of month')}</label><input id="repeat-month-day" type="number" min="1" max="31" bind:value={repeatMonthDay} required /><p>{t('Tháng không có ngày này sẽ được bỏ qua.', 'Months without this day are skipped.')}</p>
      {:else}<label for="repeat-nth">{t('Thứ tự', 'Position')}</label><select id="repeat-nth" bind:value={repeatNth}><option value={1}>1</option><option value={2}>2</option><option value={3}>3</option><option value={4}>4</option><option value={5}>5</option><option value={-1}>{t('Cuối', 'Last')}</option></select><label for="repeat-weekday">{t('Thứ', 'Weekday')}</label><select id="repeat-weekday" bind:value={repeatNthWeekday}>{#each [t('Chủ nhật','Sunday'),t('Thứ hai','Monday'),t('Thứ ba','Tuesday'),t('Thứ tư','Wednesday'),t('Thứ năm','Thursday'),t('Thứ sáu','Friday'),t('Thứ bảy','Saturday')] as day, i}<option value={i}>{day}</option>{/each}</select>{/if}
    {/if}
    <label for="repeat-end">{t('Kết thúc theo', 'End by')}</label><select id="repeat-end" bind:value={repeatEnd}><option value="count">{t('Số buổi', 'Occurrence count')}</option><option value="until">{t('Ngày', 'Date')}</option></select>
    {#if repeatEnd === 'count'}<label for="repeat-count">{t('Số buổi', 'Number of occurrences')}</label><input id="repeat-count" type="number" min="1" max="10000" bind:value={repeatCount} required />{:else}<label for="repeat-until">{t('Ngày cuối (bao gồm)', 'Last date (inclusive)')}</label><input id="repeat-until" type="date" bind:value={repeatUntil} required />{/if}
    <button type="submit" disabled={busy || !roomId || !title || !organizer || !starts || !ends}>{t('Tạo chuỗi', 'Create series')}</button>
  </form>
  {/if}
</section>
<style>
  section { margin-top: 2rem; padding-top: 1.5rem; border-top: 1px solid #c9d7d9; }
  form { display: grid; gap: .5rem; }
  input, select, button { font: inherit; }
  li { overflow-wrap: anywhere; margin-bottom: .5rem; }
  [role='alert'] { color: #7d261e; }
</style>
