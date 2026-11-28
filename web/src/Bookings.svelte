<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import Banner from './Banner.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import Modal from './Modal.svelte';
  let confirmDialog = $state();
  import { localTime, localTimeSeconds, instant, addMinutesLocal, diffMinutesLocal, defaultBookingRange, formatDuration } from './timezone.js';
  let { csrf, language = 'vi' } = $props();
  const t = (vi, en) => language === 'en' ? en : vi;
  const DURATION_PRESETS = [
    { minutes: 15, vi: '15p', en: '15m' },
    { minutes: 30, vi: '30p', en: '30m' },
    { minutes: 45, vi: '45p', en: '45m' },
    { minutes: 60, vi: '1g', en: '1h' },
    { minutes: 90, vi: '1g30', en: '1h30' },
    { minutes: 120, vi: '2g', en: '2h' },
  ];
  let rooms = $state([]), bookings = $state([]), series = $state([]), loading = $state(true), error = $state(''), notice = $state(''), busy = $state(false), zone = $state('');
  let bookingModalOpen = $state(false), bookingError = $state(''), seriesModalOpen = $state(false), seriesError = $state('');
  let alerts = $state([]), scanStatus = $state(null);
  const seriesEnabled = true;
  let repeatFrequency = $state(1), repeatInterval = $state(1), repeatCount = $state(2), repeatUntil = $state(''), repeatEnd = $state('count');
  let repeatWeekdays = $state([]), repeatMonthMode = $state('day'), repeatMonthDay = $state(1), repeatNth = $state(1), repeatNthWeekday = $state(1);
  let originalStart = 0, originalEnd = 0, originalStartText = '', originalEndText = '';
  let occurrenceTarget = $state(null);
  let seriesEdit = $state(null);
  let seriesDuration = 0, seriesStartText = '', seriesEndText = '';
  let id = $state(null), version = $state(0), roomId = $state(''), title = $state(''), organizer = $state(''), isPrivate = $state(false);
  let startDate = $state(''), startTime = $state(''), endDate = $state(''), endTime = $state(''), multiDay = $state(false), rememberedDuration = $state(30);
  const effectiveEndDate = $derived(multiDay ? endDate : startDate);
  const starts = $derived(startDate && startTime ? `${startDate}T${startTime}` : '');
  const ends = $derived(effectiveEndDate && endTime ? `${effectiveEndDate}T${endTime}` : '');
  const durationMinutes = $derived(diffMinutesLocal(startDate, startTime, effectiveEndDate, endTime));
  async function load() {
    try {
      const [r, b, z, s, a, status] = await Promise.all([fetch('/api/rooms'), fetch('/api/bookings'), fetch('/api/timezone'), seriesEnabled ? fetch('/api/series') : Promise.resolve(null), fetch('/api/alerts'), fetch('/api/alerts/status')]);
      if (!r.ok || !b.ok || !z.ok || !a.ok || !status.ok || (s && !s.ok)) throw new Error();
      rooms = await r.json(); bookings = await b.json(); zone = (await z.json()).timezone; series = s ? await s.json() : []; alerts = await a.json(); scanStatus = await status.json();
    } catch { scanStatus = null; error = ['Không thể tải lịch hoặc trạng thái kiểm tra.', 'Could not load bookings or scan status.']; }
    finally { loading = false; }
  }
  async function acknowledge(a) {
    busy = true;
    try { const response = await fetch(`/api/alerts/${a.id}/acknowledge`, {method:'POST',headers:{'X-CSRF-Token':csrf}}); if (!response.ok) throw new Error(); await load(); }
    catch { error = ['Không thể xác nhận cảnh báo.', 'Could not acknowledge alert.']; }
    finally { busy = false; }
  }
  function clear() {
    occurrenceTarget = null; seriesEdit = null; id = null; roomId = ''; title = ''; organizer = ''; isPrivate = false;
    startDate = ''; startTime = ''; endDate = ''; endTime = ''; multiDay = false; rememberedDuration = 30;
    repeatFrequency = 1; repeatInterval = 1; repeatCount = 2; repeatUntil = ''; repeatEnd = 'count';
    repeatWeekdays = []; repeatMonthMode = 'day'; repeatMonthDay = 1; repeatNth = 1; repeatNthWeekday = 1;
    seriesDuration = 0; seriesStartText = ''; seriesEndText = '';
  }
  function applyDefaultRange() {
    const def = defaultBookingRange(zone);
    startDate = def.startDate;
    startTime = def.startTime;
    endDate = def.endDate;
    endTime = def.endTime;
    multiDay = def.multiDay;
    rememberedDuration = 30;
  }
  function setRangeFromLocalStrings(startLocal, endLocal) {
    startDate = startLocal.slice(0, 10);
    startTime = startLocal.slice(11, 16);
    endDate = endLocal.slice(0, 10);
    endTime = endLocal.slice(11, 16);
    multiDay = endDate !== startDate;
    const diff = diffMinutesLocal(startDate, startTime, endDate, endTime);
    rememberedDuration = diff && diff > 0 ? diff : 30;
  }
  function onStartChanged() {
    if (!startDate || !startTime) return;
    const shifted = addMinutesLocal(startDate, startTime, rememberedDuration > 0 ? rememberedDuration : 30);
    if (!shifted) return;
    endDate = shifted.date;
    endTime = shifted.time;
    multiDay = shifted.date !== startDate;
  }
  function onEndChanged() {
    const targetEndDate = multiDay ? endDate : startDate;
    if (!startDate || !startTime || !targetEndDate || !endTime) return;
    const diff = diffMinutesLocal(startDate, startTime, targetEndDate, endTime);
    if (diff !== null && diff > 0) rememberedDuration = diff;
  }
  function applyDurationPreset(mins) {
    rememberedDuration = mins;
    if (!startDate || !startTime) return;
    const next = addMinutesLocal(startDate, startTime, mins);
    if (!next) return;
    endDate = next.date;
    endTime = next.time;
    multiDay = next.date !== startDate;
  }
  function toggleMultiDay() {
    if (!multiDay) {
      multiDay = true;
      if (!endDate || endDate === startDate) {
        if (startDate && startTime && endTime && endTime <= startTime) {
          const nextDay = addMinutesLocal(startDate, '00:00', 1440);
          if (nextDay) endDate = nextDay.date;
        } else {
          endDate = startDate;
        }
      }
    } else {
      multiDay = false;
      endDate = startDate;
    }
    onEndChanged();
  }
  function moveEndToNextDay() {
    if (!startDate) return;
    const nextDay = addMinutesLocal(startDate, '00:00', 1440);
    if (!nextDay) return;
    multiDay = true;
    endDate = nextDay.date;
    onEndChanged();
  }
  function openCreateBooking() {
    clear();
    applyDefaultRange();
    bookingError = '';
    notice = '';
    bookingModalOpen = true;
  }
  function closeBookingModal() {
    if (busy) return;
    bookingModalOpen = false;
    bookingError = '';
    clear();
  }
  function openCreateSeries() {
    clear();
    applyDefaultRange();
    seriesError = '';
    notice = '';
    seriesModalOpen = true;
  }
  function closeSeriesModal() {
    if (busy) return;
    seriesModalOpen = false;
    seriesError = '';
    clear();
  }
  function edit(b) {
    bookingError = ''; notice = '';
    occurrenceTarget = null; seriesEdit = null; id = b.id; version = b.version; roomId = String(b.room_id); title = b.title; organizer = b.organizer; isPrivate = b.private;
    const startText = localTimeSeconds(b.starts, zone);
    const endText = localTimeSeconds(b.ends, zone);
    setRangeFromLocalStrings(startText, endText);
    originalStart = b.starts; originalEnd = b.ends; originalStartText = startText; originalEndText = endText;
    bookingModalOpen = true;
  }
  function editSeries(s, scope, occurrence = null) {
    seriesError = ''; notice = ''; occurrenceTarget = null; id = null;
    const rule = s.rule.rule;
    const durSec = Math.max(1, Math.round(rule.Duration / 1e9));
    const baseStart = scope === 'from' && occurrence ? occurrence.original_starts : Math.round(Date.parse(rule.Start) / 1000);
    roomId = String(s.room_id); title = s.title; organizer = s.organizer; isPrivate = s.private;
    const sStart = localTimeSeconds(baseStart, zone);
    const sEnd = localTimeSeconds(baseStart + durSec, zone);
    setRangeFromLocalStrings(sStart, sEnd);
    seriesDuration = durSec; seriesStartText = sStart; seriesEndText = sEnd;
    repeatFrequency = rule.Frequency; repeatInterval = rule.Interval;
    repeatWeekdays = rule.Weekdays ?? [];
    if (rule.Frequency === 3) {
      repeatMonthMode = rule.MonthDay ? 'day' : 'nth';
      repeatMonthDay = rule.MonthDay || 1;
      repeatNth = rule.Nth || 1;
      repeatNthWeekday = rule.NthWeekday ?? 1;
    }
    if (rule.Count > 0) {
      repeatEnd = 'count';
      if (scope === 'from' && occurrence) {
        const remaining = s.occurrences.filter(o => o.original_starts >= occurrence.original_starts).length;
        repeatCount = Math.max(1, remaining);
      } else {
        repeatCount = rule.Count;
      }
      repeatUntil = '';
    } else if (!rule.Until.startsWith('0001-01-01')) {
      repeatEnd = 'until'; repeatUntil = rule.Until.slice(0, 10);
    } else {
      repeatEnd = 'infinite'; repeatUntil = '';
    }
    seriesEdit = { id: s.id, version: s.version, scope, originalStarts: occurrence ? occurrence.original_starts : null };
    seriesModalOpen = true;
  }
  async function cancelSeries(s, scope, originalStarts = null) {
    const prompt = scope === 'all'
      ? t('Hủy toàn bộ chuỗi này?', 'Cancel this entire series?')
      : t('Hủy chuỗi từ buổi này trở đi?', 'Cancel this and all following occurrences?');
    if (!await confirmDialog.ask(prompt)) return;
    busy = true; error = ''; notice = '';
    try {
      const body = scope === 'all' ? { scope: 'all' } : { scope: 'from', original_starts: originalStarts };
      const response = await fetch(`/api/series/${s.id}/cancel`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf, 'If-Match': String(s.version) },
        body: JSON.stringify(body),
      });
      if (response.status === 409) { error = ['Chuỗi đã thay đổi hoặc đã bị hủy. Tải lại và thử lại.', 'Series changed or was already canceled. Reload and try again.']; return; }
      if (!response.ok) throw new Error();
      if (seriesEdit?.id === s.id) { seriesModalOpen = false; seriesError = ''; clear(); }
      await load();
      notice = scope === 'all' ? ['Đã hủy toàn chuỗi.', 'Entire series canceled.'] : ['Đã hủy từ buổi đã chọn.', 'Series canceled from selected occurrence.'];
    } catch { error = ['Không thể hủy chuỗi; hãy tải lại.', 'Could not cancel series; reload and retry.']; }
    finally { busy = false; }
  }
  function editOccurrence(s, o) {
    edit({...o, id: s.id});
    occurrenceTarget = { series: s.id, original: o.original_starts };
  }
  async function cancelOccurrence(s, o) {
    if (!await confirmDialog.ask(t('Hủy buổi này?', 'Cancel this occurrence?'))) return;
    busy = true; error = ''; notice = '';
    try {
      const response = await fetch(`/api/series/${s.id}/occurrences/${o.original_starts}/cancel`, {method:'POST', headers:{'X-CSRF-Token':csrf, 'If-Match':String(o.version)}});
      if (!response.ok) throw new Error();
      await load(); notice = ['Đã hủy buổi.', 'Occurrence canceled.'];
    } catch { error = ['Không thể hủy buổi; hãy tải lại.', 'Could not cancel occurrence; reload and retry.']; }
    finally { busy = false; }
  }
  async function save(event) {
    event.preventDefault(); bookingError = ''; notice = ''; busy = true;
    try {
      if (!zone) { bookingError = ['Chưa tải được múi giờ hệ thống. Vui lòng tải lại trang.', 'System time zone has not loaded. Please reload the page.']; return; }
      const start = id && starts === originalStartText.slice(0, 16) ? originalStart : instant(starts, zone);
      const end = id && ends === originalEndText.slice(0, 16) ? originalEnd : instant(ends, zone);
      if (start === null || end === null) { bookingError = ['Giờ không tồn tại hoặc mơ hồ do chuyển giờ. Hãy chọn giờ khác.', 'Time is invalid or ambiguous due to a clock change. Choose another time.']; return; }
      if (end <= start) { bookingError = ['Giờ kết thúc phải sau giờ bắt đầu.', 'End time must be after start time.']; return; }
      const response = await fetch(occurrenceTarget ? `/api/series/${occurrenceTarget.series}/occurrences/${occurrenceTarget.original}` : id ? `/api/bookings/${id}` : '/api/bookings', {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: JSON.stringify({room_id: Number(roomId), title: title.trim(), organizer: organizer.trim(), private: isPrivate, starts: start, ends: end, version: id ? version : 0}),
      });
      if (response.status === 409) { bookingError = ['Lịch đã thay đổi hoặc phòng có lịch trùng. Tải lại và thử lại.', 'Booking changed or overlaps another booking. Reload and try again.']; return; }
      if (!response.ok) throw new Error();
      bookingModalOpen = false; clear(); await load(); notice = ['Đã lưu lịch.', 'Booking saved.'];
    } catch { bookingError = ['Không thể lưu lịch. Kiểm tra thông tin và thử lại.', 'Could not save booking. Check the details and try again.']; }
    finally { busy = false; }
  }
  async function saveSeries(event) {
    event.preventDefault(); seriesError = ''; notice = ''; busy = true;
    const editingSeries = seriesEdit;
    try {
      if (!zone) { seriesError = ['Chưa tải được múi giờ hệ thống. Vui lòng tải lại trang.', 'System time zone has not loaded. Please reload the page.']; return; }
      const start = editingSeries && starts === seriesStartText.slice(0, 16) ? instant(seriesStartText, zone) : instant(starts, zone);
      const end = editingSeries && starts === seriesStartText.slice(0, 16) && ends === seriesEndText.slice(0, 16) && start !== null ? start + seriesDuration : instant(ends, zone);
      if (start === null || end === null || end <= start) { seriesError = ['Giờ không hợp lệ hoặc mơ hồ.', 'Invalid or ambiguous time.']; return; }
      const frequency = Number(repeatFrequency);
      const rule = {frequency, interval:Number(repeatInterval), count:repeatEnd === 'count' ? Number(repeatCount) : 0, until:repeatEnd === 'until' ? repeatUntil : ''};
      if (frequency === 2) rule.weekdays = repeatWeekdays.map(Number);
      if (frequency === 3 && repeatMonthMode === 'day') rule.month_day = Number(repeatMonthDay);
      if (frequency === 3 && repeatMonthMode === 'nth') { rule.nth = Number(repeatNth); rule.nth_weekday = Number(repeatNthWeekday); }
      const payload = {room_id:Number(roomId),title:title.trim(),organizer:organizer.trim(),private:isPrivate,starts:start,duration_seconds:end-start,...rule};
      let url = '/api/series';
      if (editingSeries) {
        url = `/api/series/${editingSeries.id}`;
        payload.scope = editingSeries.scope;
        payload.version = editingSeries.version;
        if (editingSeries.scope === 'from') payload.original_starts = editingSeries.originalStarts;
      }
      const response = await fetch(url, { method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':csrf}, body:JSON.stringify(payload) });
      if (response.status === 409) {
        const message = await response.text();
        seriesError = message.includes('exception_conflict')
          ? ['Không thể sửa hoặc tách chuỗi vì một ngoại lệ sẽ mất giờ gốc. Hãy sửa hoặc hủy ngoại lệ đó trước.', 'Cannot modify or split series because an exception would lose its original start. Resolve that exception first.']
          : ['Chuỗi đã thay đổi hoặc có lịch trùng.', 'Series changed or booking conflict.'];
        return;
      }
      if (!response.ok) { seriesError = ['Quy tắc hoặc giờ không hợp lệ, hoặc giờ bị mơ hồ do DST.', 'Invalid rule or time, or ambiguous DST time.']; return; }
      seriesModalOpen = false; clear(); await load();
      notice = editingSeries ? ['Đã cập nhật chuỗi.', 'Series updated.'] : ['Đã tạo chuỗi.', 'Series created.'];
    } catch { seriesError = editingSeries ? ['Không thể cập nhật chuỗi.', 'Could not update series.'] : ['Không thể tạo chuỗi.', 'Could not create series.']; }
    finally { busy = false; }
  }
  async function cancel(b) {
    if (!await confirmDialog.ask(t('Hủy lịch này?', 'Cancel this booking?'))) return;
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
    const timer = setInterval(load, 60000);
    return () => { clearInterval(timer); window.removeEventListener('rooms-changed', load); };
  });
</script>

<ConfirmDialog bind:this={confirmDialog} {language} />

{#snippet bookingCoreFields(prefix)}
  <label for={`${prefix}-room`}>{t('Phòng', 'Room')}</label>
  <select id={`${prefix}-room`} bind:value={roomId} required>
    <option value="">{t('Chọn phòng', 'Select room')}</option>
    {#each rooms.filter(r => r.active) as room}
      <option value={String(room.id)}>{room.name}</option>
    {/each}
  </select>

  <label for={`${prefix}-title`}>{t('Tiêu đề', 'Title')}</label>
  <input id={`${prefix}-title`} bind:value={title} maxlength="256" required placeholder={t('Ví dụ: Họp giao ban tuần', 'e.g. Weekly sync')} />

  <label for={`${prefix}-organizer`}>{t('Người tổ chức', 'Organizer')}</label>
  <input id={`${prefix}-organizer`} bind:value={organizer} maxlength="128" required placeholder={t('Ví dụ: Nguyễn Văn A', 'e.g. Alex Rivera')} />

  <label class="checkbox-card">
    <input type="checkbox" bind:checked={isPrivate} />
    <span>{t('Riêng tư', 'Private')}</span>
  </label>

  <fieldset class="time-section">
    <legend class="sr-only">{t('Thời gian họp', 'Meeting time')}</legend>

    <div class="time-date-row" class:has-end-date={multiDay}>
      <div class="time-field">
        <label for={`${prefix}-date`}>{multiDay ? t('Ngày bắt đầu', 'Start date') : t('Ngày họp', 'Date')}</label>
        <input id={`${prefix}-date`} type="date" bind:value={startDate} oninput={() => queueMicrotask(onStartChanged)} required />
      </div>
      {#if multiDay}
        <div class="time-field">
          <label for={`${prefix}-end-date`}>{t('Ngày kết thúc', 'End date')}</label>
          <input id={`${prefix}-end-date`} type="date" bind:value={endDate} min={startDate || undefined} oninput={() => queueMicrotask(onEndChanged)} required />
        </div>
      {/if}
    </div>

    <div class="form-grid-2">
      <div class="time-field">
        <label for={`${prefix}-start`}>{t('Bắt đầu', 'Start')}</label>
        <input id={`${prefix}-start`} type="time" step="60" bind:value={startTime} oninput={() => queueMicrotask(onStartChanged)} required />
      </div>
      <div class="time-field">
        <label for={`${prefix}-end`}>{t('Kết thúc', 'End')}</label>
        <input id={`${prefix}-end`} type="time" step="60" bind:value={endTime} oninput={() => queueMicrotask(onEndChanged)} required />
      </div>
    </div>

    <div class="time-meta-bar">
      <div class="duration-presets" role="group" aria-label={t('Thời lượng nhanh', 'Quick duration')}>
        {#each DURATION_PRESETS as preset (preset.minutes)}
          <button
            type="button"
            class="preset-chip"
            class:is-active={durationMinutes === preset.minutes}
            aria-pressed={durationMinutes === preset.minutes}
            onclick={() => applyDurationPreset(preset.minutes)}
          >
            {language === 'en' ? preset.en : preset.vi}
          </button>
        {/each}
      </div>
      <button type="button" class="link-toggle" aria-pressed={multiDay} onclick={toggleMultiDay}>
        {multiDay ? t('Gộp về 1 ngày', 'Same-day only') : t('+ Kết thúc ngày khác', '+ Different end date')}
      </button>
    </div>

    {#if durationMinutes !== null && durationMinutes > 0}
      <p class="field-hint duration-summary">
        {t('Thời lượng:', 'Duration:')} <strong>{formatDuration(durationMinutes, language)}</strong>
        {#if multiDay && startDate && endDate && endDate !== startDate}
          <span class="mono">({startDate} → {endDate})</span>
        {/if}
      </p>
    {:else if durationMinutes !== null && durationMinutes <= 0 && startTime && endTime}
      <div class="time-error" role="alert">
        <span>{t('Giờ kết thúc phải sau giờ bắt đầu.', 'End time must be after start time.')}</span>
        {#if !multiDay}
          <button type="button" class="link-toggle" onclick={moveEndToNextDay}>
            {t('Đẩy sang ngày hôm sau (+1 ngày)', 'Move end to next day (+1 day)')}
          </button>
        {/if}
      </div>
    {/if}
  </fieldset>
{/snippet}

<section aria-labelledby="bookings-heading">
  <div class="section-head">
    <div>
      <p class="section-kicker">{t('ĐIỀU PHỐI LỊCH HỌP', 'SCHEDULING & CONFLICTS')}</p>
      <h2 id="bookings-heading">{t('Lịch phòng', 'Room bookings')}</h2>
    </div>
    <div class="section-actions">
      <span class="ui-pill ui-pill--neutral tz-pill">
        <Icon name="clock" />
        <span>{t('Múi giờ hệ thống:', 'System time zone:')} {zone || t('đang tải', 'loading')}</span>
      </span>
      <button type="button" disabled={busy} onclick={openCreateBooking}>
        <Icon name="plus" />
        <span>{t('Tạo lịch', 'Create booking')}</span>
      </button>
      {#if seriesEnabled}
        <button class="secondary" type="button" disabled={busy} onclick={openCreateSeries}>
          <Icon name="plus" />
          <span>{t('Tạo chuỗi', 'Create series')}</span>
        </button>
      {/if}
    </div>
  </div>

  {#if error}<Banner kind="error" text={t(...error)} />{/if}
  {#if notice}<Banner kind="success" text={t(...notice)} />{/if}

  {#if seriesEnabled}
    <div class="health-panel">
      <div class="health-header">
        <h3>{t('Cảnh báo xung đột', 'Conflict alerts')}</h3>
        {#if alerts.filter(a => !a.acknowledged).length > 0}
          <span class="ui-pill ui-pill--warning">{alerts.filter(a => !a.acknowledged).length} {t('cần xử lý', 'pending')}</span>
        {/if}
      </div>

      {#if !scanStatus || scanStatus.failed || scanStatus.stale}
        <Banner kind="error" text={t('Kiểm tra xung đột đang lỗi hoặc quá hạn; không thể tin cảnh báo đã đầy đủ.', 'Conflict scan failed or is overdue; alerts may be incomplete.')} />
      {:else}
        <p class="ui-banner ui-banner--info">
          <Icon name="shield" />
          <span>
            <strong>{t('Lần quét gần nhất', 'Last scan')}:</strong> <span class="mono">{localTimeSeconds(scanStatus.last_success, zone)}</span> ·
            <strong>{t('Cửa sổ quét toàn hệ thống', 'System-wide scan window')}:</strong> <span class="mono">[{localTimeSeconds(scanStatus.checked_from, zone)}, {localTimeSeconds(scanStatus.checked_through, zone)})</span>.
            {t('Không chứng minh khoảng trước đó đã được quét; xung đột vẫn có thể tồn tại.', 'Earlier periods may not have been scanned; conflicts may still exist.')}
          </span>
        </p>
      {/if}

      {#if (scanStatus?.rooms ?? []).length > 0}
        <details class="scan-rooms-details">
          <summary>{t('Trạng thái quét theo phòng', 'Per-room scan status')} ({scanStatus.rooms.length})</summary>
          <ul class="scan-rooms-grid" aria-label={t('Trạng thái quét theo phòng', 'Per-room scan status')}>
            {#each scanStatus?.rooms ?? [] as room (room.room_id)}
              <li class="scan-room-chip">
                <div class="scan-room-top">
                  <strong>{room.room}</strong>
                  {#if room.failed || room.stale}
                    <span class="ui-pill ui-pill--danger" role="alert">{room.failed ? t('Quét lỗi', 'Scan failed') : t('Quá hạn', 'Overdue')}</span>
                  {:else}
                    <span class="ui-pill ui-pill--success">{t('Quét xong', 'Scanned')}</span>
                  {/if}
                </div>
                {#if room.last_success}
                  <small class="mono">{localTimeSeconds(room.last_success, zone)} [{localTimeSeconds(room.checked_from, zone)}, {localTimeSeconds(room.checked_through, zone)})</small>
                {/if}
              </li>
            {/each}
          </ul>
        </details>
      {/if}

      {#if alerts.length > 0}
        <p class="alert-note">{t('Xác nhận không giải quyết xung đột; hãy sửa hoặc hủy buổi liên quan.', 'Acknowledging does not resolve a conflict; edit or cancel an affected occurrence.')}</p>
        <ul class="alerts-list">
          {#each alerts as a (a.id)}
            <li class="alert-card" class:is-ack={a.acknowledged}>
              <div class="alert-info">
                <span class="ui-pill" class:ui-pill--warning={!a.acknowledged} class:ui-pill--neutral={a.acknowledged}>
                  {a.acknowledged ? t('Đã xác nhận', 'Acknowledged') : t('Xung đột', 'Conflict')}
                </span>
                <div>
                  <strong>{t('Chuỗi', 'Series')} #{a.series_a}</strong> <span class="mono">({localTimeSeconds(a.original_a, zone)})</span>
                  <span class="sep">vs</span>
                  <strong>{t('chuỗi', 'series')} #{a.series_b}</strong> <span class="mono">({localTimeSeconds(a.original_b, zone)})</span>
                </div>
              </div>
              {#if !a.acknowledged}
                <button class="secondary" type="button" disabled={busy} onclick={() => acknowledge(a)}>{t('Xác nhận', 'Acknowledge')}</button>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}

  <div class="ui-view-body bookings-main">
    <div>
      <div class="block-header">
        <h3>{t('Danh sách lịch phòng', 'Bookings list')}</h3>
        <span class="ui-pill ui-pill--neutral">{bookings.length}</span>
      </div>
      {#if loading}
        <p class="ui-loading" role="status">
          <span class="ui-spinner" aria-hidden="true"></span>
          <span>{t('Đang tải lịch phòng…', 'Loading bookings…')}</span>
        </p>
      {:else if bookings.length === 0}
        <div class="ui-empty">
          <strong>{t('Chưa có lịch.', 'No bookings yet.')}</strong>
          <p>{t('Bấm "Tạo lịch" để tạo cuộc họp mới.', 'Click "Create booking" to add a new meeting.')}</p>
        </div>
      {:else}
        <ul class="ui-card-list">
          {#each bookings as b (b.id)}
            <li class="ui-card booking-card" class:is-canceled={b.canceled} class:is-editing={bookingModalOpen && id === b.id && !occurrenceTarget}>
              <div class="card-body">
                <div class="card-badges">
                  <span class="room-badge">{rooms.find(r => r.id === b.room_id)?.name ?? t('Phòng không tồn tại', 'Room not found')}</span>
                  {#if b.private}<span class="ui-pill ui-pill--neutral">{t('Riêng tư', 'Private')}</span>{/if}
                  {#if b.canceled}<span class="ui-pill ui-pill--danger">{t('(Đã hủy)', '(Canceled)')}</span>{/if}
                </div>
                <strong class="card-title">{b.title}</strong>
                <div class="time-range">
                  <Icon name="clock" />
                  <span class="mono">{zone ? localTime(b.starts, zone) : ''}</span>
                  <span class="to-label">{t('đến', 'to')}</span>
                  <span class="mono">{zone ? localTime(b.ends, zone) : ''}</span>
                </div>
                <div class="utc-caption mono">
                  UTC: {new Date(b.starts * 1000).toISOString()} – {new Date(b.ends * 1000).toISOString()}
                </div>
              </div>
              {#if !b.canceled}
                <div class="card-actions">
                  <button class="secondary" type="button" disabled={busy} aria-label={`${t('Sửa lịch', 'Edit booking')} ${b.title}`} onclick={() => edit(b)}>{t('Sửa', 'Edit')}</button>
                  <button class="danger" type="button" disabled={busy} aria-label={`${t('Hủy lịch', 'Cancel booking')} ${b.title}`} onclick={() => cancel(b)}>{t('Hủy lịch', 'Cancel booking')}</button>
                </div>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    {#if seriesEnabled}
      <div>
        <div class="block-header">
          <h3>{t('Chuỗi lịch', 'Recurring series')}</h3>
          <span class="ui-pill ui-pill--neutral">{series.length}</span>
        </div>
        <p class="policy-note">
          {t('Lịch đơn được kiểm tra toàn bộ; xung đột giữa các chuỗi chỉ được kiểm tra trong 12 tháng. Xung đột ngoài cửa sổ có thể được lưu và chưa được cảnh báo cho đến khi vào cửa sổ quét. Cảnh báo không được bảo đảm nếu worker lỗi hoặc quá hạn; hãy kiểm tra thủ công lịch xa. Có thể sửa hoặc hủy từng buổi, từ buổi này trở đi hoặc toàn chuỗi; thay đổi làm mất giờ gốc của ngoại lệ sẽ bị từ chối.', 'Single bookings are checked throughout; conflicts between series are checked only within 12 months. Conflicts outside this window can be saved and are not alerted until they enter the scan window. Alerts are not guaranteed if the worker fails or is overdue; manually check distant schedules. Individual occurrences, this-and-following, or entire series can be edited or canceled; changes that would drop an exception’s original start are rejected.')}
        </p>
        {#if loading}
          <p class="ui-loading" role="status">
            <span class="ui-spinner" aria-hidden="true"></span>
            <span>{t('Đang tải chuỗi lịch…', 'Loading series…')}</span>
          </p>
        {:else if series.length === 0}
          <div class="ui-empty">
            <p>{t('Chưa có chuỗi lịch lặp nào.', 'No recurring series yet.')}</p>
          </div>
        {:else}
          <ul class="ui-card-list">
            {#each series as s (s.id)}
              <li class="ui-card" class:is-canceled={s.canceled} class:is-editing={seriesModalOpen && seriesEdit?.id === s.id}>
                <div class="card-badges">
                  <span class="room-badge">{rooms.find(r => r.id === s.room_id)?.name ?? s.room_id}</span>
                  {#if s.private}<span class="ui-pill ui-pill--neutral">{t('(Riêng tư)', '(Private)')}</span>{/if}
                  {#if s.canceled}<span class="ui-pill ui-pill--danger">{t('(Đã hủy)', '(Canceled)')}</span>{/if}
                  <span class="ui-pill ui-pill--success">
                    {t('Lặp mỗi', 'Repeat every')} {s.rule.rule.Interval} {s.rule.rule.Frequency === 1 ? t('ngày', 'days') : s.rule.rule.Frequency === 2 ? t('tuần', 'weeks') : t('tháng', 'months')}
                  </span>
                </div>
                <strong class="card-title">{s.title}</strong>
                <p class="series-organizer">{t('Người tổ chức', 'Organizer')}: {s.organizer}</p>
                {#if !s.canceled}
                  <div class="card-actions">
                    <button class="secondary" type="button" disabled={busy} onclick={() => editSeries(s, 'all')}>{t('Sửa chuỗi', 'Edit series')}</button>
                    <button class="danger" type="button" disabled={busy} onclick={() => cancelSeries(s, 'all')}>{t('Hủy chuỗi', 'Cancel series')}</button>
                  </div>
                {/if}

                <div class="series-meta-grid">
                  <div><span>{t('Múi giờ chuỗi', 'Series time zone')}</span><strong class="mono">{s.rule.location}</strong></div>
                  <div><span>{t('Bắt đầu', 'Start')}</span><strong class="mono">{localTimeSeconds(Date.parse(s.rule.rule.Start) / 1000, s.rule.location)}</strong></div>
                  <div>
                    <span>{t('Quy tắc', 'Pattern')}</span>
                    <strong>
                      {#if s.rule.rule.Frequency === 2}{t('thứ', 'weekdays')}: {(s.rule.rule.Weekdays ?? []).join(', ')}{/if}
                      {#if s.rule.rule.Frequency === 3}{s.rule.rule.MonthDay ? `${t('ngày', 'day')} ${s.rule.rule.MonthDay}` : `${t('thứ tự', 'position')} ${s.rule.rule.Nth}, ${t('thứ', 'weekday')} ${s.rule.rule.NthWeekday}`}{/if}
                      {#if s.rule.rule.Frequency === 1}{t('Hàng ngày', 'Daily')}{/if}
                      ·
                      {#if s.rule.rule.Count}{s.rule.rule.Count} {t('buổi', 'occurrences')}{:else if s.rule.rule.Until.startsWith('0001-01-01')}{t('không có ngày kết thúc', 'no end date')}{:else}{t('đến hết ngày', 'through date')} {s.rule.rule.Until.slice(0, 10)}{/if}
                    </strong>
                  </div>
                  <div><span>{t('Mốc bảo vệ khi tạo chuỗi (không biểu thị cửa sổ quét hiện tại)', 'Creation-time conflict protection horizon (not the current scan window)')}</span><strong class="mono">{localTimeSeconds(s.checked_through, s.rule.location)} ({s.rule.location})</strong></div>
                </div>

                <details class="occurrences-details">
                  <summary>{t('Xem các buổi', 'View occurrences')} ({s.occurrences.length})</summary>
                  <ul class="occurrences-list">
                    {#each s.occurrences as occurrence (occurrence.original_starts)}
                      <li class="occurrence-row" class:is-canceled={occurrence.canceled}>
                        <div class="occ-info">
                          <span class="mono">{localTimeSeconds(occurrence.starts, zone)} — {localTimeSeconds(occurrence.ends, zone)}</span>
                          <span>{occurrence.title} · {rooms.find(r => r.id === occurrence.room_id)?.name ?? occurrence.room_id}</span>
                          {#if occurrence.canceled}<span class="ui-pill ui-pill--danger">{t('(Đã hủy)', '(Canceled)')}</span>{/if}
                        </div>
                        {#if !s.canceled && !occurrence.canceled}
                          <div class="card-actions">
                            <button class="secondary" type="button" disabled={busy} onclick={() => editOccurrence(s, occurrence)}>{t('Sửa buổi', 'Edit occurrence')}</button>
                            <button class="danger" type="button" disabled={busy} onclick={() => cancelOccurrence(s, occurrence)}>{t('Hủy buổi', 'Cancel occurrence')}</button>
                            <button class="secondary" type="button" disabled={busy} onclick={() => editSeries(s, 'from', occurrence)}>{t('Sửa từ buổi này', 'Edit from here')}</button>
                            <button class="danger" type="button" disabled={busy} onclick={() => cancelSeries(s, 'from', occurrence.original_starts)}>{t('Hủy từ buổi này', 'Cancel from here')}</button>
                          </div>
                        {/if}
                      </li>
                    {/each}
                  </ul>
                </details>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    {/if}
  </div>

  <Modal
    open={bookingModalOpen}
    title={id ? t('Sửa lịch', 'Edit booking') : t('Tạo lịch', 'Create booking')}
    badge={occurrenceTarget ? t('Chỉnh 1 buổi', 'Single occurrence') : ''}
    {busy}
    closeLabel={t('Đóng', 'Close')}
    onclose={closeBookingModal}
  >
    {#if bookingError}<Banner kind="error" text={t(...bookingError)} />{/if}
    <form onsubmit={save}>
      {@render bookingCoreFields('booking')}
      <div class="ui-form-actions">
        <button type="submit" disabled={busy}>{t('Lưu lịch', 'Save booking')}</button>
        <button class="secondary" type="button" onclick={closeBookingModal} disabled={busy}>{t('Hủy', 'Cancel')}</button>
      </div>
    </form>
  </Modal>

  {#if seriesEnabled}
    <Modal
      open={seriesModalOpen}
      title={seriesEdit ? (seriesEdit.scope === 'all' ? t('Sửa toàn chuỗi', 'Edit entire series') : t('Sửa từ buổi đã chọn', 'Edit from selected occurrence')) : t('Tạo chuỗi', 'Create series')}
      badge={seriesEdit ? (seriesEdit.scope === 'all' ? t('Toàn chuỗi', 'Entire series') : t('Từ buổi này', 'From occurrence')) : ''}
      size="lg"
      {busy}
      closeLabel={t('Đóng', 'Close')}
      onclose={closeSeriesModal}
    >
      {#if seriesError}<Banner kind="error" text={t(...seriesError)} />{/if}
      <form onsubmit={saveSeries}>
        {@render bookingCoreFields('series')}

        <div class="form-grid-2">
          <div>
            <label for="repeat-frequency">{t('Tần suất', 'Frequency')}</label>
            <select id="repeat-frequency" bind:value={repeatFrequency}>
              <option value={1}>{t('Ngày', 'Daily')}</option>
              <option value={2}>{t('Tuần', 'Weekly')}</option>
              <option value={3}>{t('Tháng', 'Monthly')}</option>
            </select>
          </div>
          <div>
            <label for="repeat-interval">{t('Khoảng lặp', 'Interval')}</label>
            <input id="repeat-interval" type="number" min="1" max="366" bind:value={repeatInterval} required />
          </div>
        </div>

        {#if Number(repeatFrequency) === 2}
          <fieldset class="weekdays-fieldset">
            <legend>{t('Thứ trong tuần', 'Weekdays')}</legend>
            <div class="weekdays-grid">
              {#each [t('Chủ nhật','Sunday'),t('Thứ hai','Monday'),t('Thứ ba','Tuesday'),t('Thứ tư','Wednesday'),t('Thứ năm','Thursday'),t('Thứ sáu','Friday'),t('Thứ bảy','Saturday')] as day, i}
                <label class="weekday-chip">
                  <input type="checkbox" value={i} bind:group={repeatWeekdays} />
                  <span>{day}</span>
                </label>
              {/each}
            </div>
          </fieldset>
        {/if}

        {#if Number(repeatFrequency) === 3}
          <label for="repeat-month-mode">{t('Quy tắc tháng', 'Monthly rule')}</label>
          <select id="repeat-month-mode" bind:value={repeatMonthMode}>
            <option value="day">{t('Ngày trong tháng', 'Day of month')}</option>
            <option value="nth">{t('Thứ tự thứ trong tháng', 'Nth weekday')}</option>
          </select>
          {#if repeatMonthMode === 'day'}
            <label for="repeat-month-day">{t('Ngày trong tháng', 'Day of month')}</label>
            <input id="repeat-month-day" type="number" min="1" max="31" bind:value={repeatMonthDay} required />
            <p class="field-hint">{t('Tháng không có ngày này sẽ được bỏ qua.', 'Months without this day are skipped.')}</p>
          {:else}
            <div class="form-grid-2">
              <div>
                <label for="repeat-nth">{t('Thứ tự', 'Position')}</label>
                <select id="repeat-nth" bind:value={repeatNth}>
                  <option value={1}>1</option><option value={2}>2</option><option value={3}>3</option><option value={4}>4</option><option value={5}>5</option><option value={-1}>{t('Cuối', 'Last')}</option>
                </select>
              </div>
              <div>
                <label for="repeat-weekday">{t('Thứ', 'Weekday')}</label>
                <select id="repeat-weekday" bind:value={repeatNthWeekday}>
                  {#each [t('Chủ nhật','Sunday'),t('Thứ hai','Monday'),t('Thứ ba','Tuesday'),t('Thứ tư','Wednesday'),t('Thứ năm','Thursday'),t('Thứ sáu','Friday'),t('Thứ bảy','Saturday')] as day, i}
                    <option value={i}>{day}</option>
                  {/each}
                </select>
              </div>
            </div>
          {/if}
        {/if}

        <label for="repeat-end">{t('Kết thúc theo', 'End by')}</label>
        <select id="repeat-end" bind:value={repeatEnd}>
          <option value="count">{t('Số buổi', 'Occurrence count')}</option>
          <option value="until">{t('Ngày', 'Date')}</option>
          <option value="infinite">{t('Không có ngày kết thúc', 'No end date')}</option>
        </select>

        {#if repeatEnd === 'count'}
          <label for="repeat-count">{t('Số buổi', 'Number of occurrences')}</label>
          <input id="repeat-count" type="number" min="1" max="10000" bind:value={repeatCount} required />
        {:else if repeatEnd === 'until'}
          <label for="repeat-until">{t('Ngày cuối (bao gồm)', 'Last date (inclusive)')}</label>
          <input id="repeat-until" type="date" bind:value={repeatUntil} required />
        {:else}
          <Banner kind="warning" text={t('Chuỗi không có ngày kết thúc chỉ được quét từng cửa sổ 12 tháng và chỉ hỗ trợ đến năm 2400. Giờ không tồn tại hoặc mơ hồ do DST có thể làm worker ngừng quét; khi worker lỗi hoặc quá hạn, cảnh báo có thể thiếu và màn hình có thể chuyển Không xác định.', 'A series without an end date is scanned in 12-month windows and supported only through year 2400. A nonexistent or ambiguous DST time can stop the worker scan; when the worker fails or is overdue, alerts may be incomplete and displays may become Unknown.')} />
        {/if}

        <div class="ui-form-actions">
          <button type="submit" disabled={busy || !roomId || !title || !organizer || !starts || !ends || (durationMinutes !== null && durationMinutes <= 0)}>{seriesEdit ? t('Lưu thay đổi chuỗi', 'Save series changes') : t('Tạo chuỗi', 'Create series')}</button>
          <button class="secondary" type="button" onclick={closeSeriesModal} disabled={busy}>{t('Hủy', 'Cancel')}</button>
        </div>
      </form>
    </Modal>
  {/if}
</section>

<style>
  .tz-pill {
    font-size: 0.84rem;
    padding: 0.35rem 0.8rem;
  }

  .health-panel {
    margin-top: 1.25rem;
    padding: 1.1rem 1.2rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    background: var(--bg-subtle);
  }

  .health-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .health-header h3 {
    margin: 0;
  }

  .scan-rooms-details summary,
  .occurrences-details summary {
    cursor: pointer;
    font-weight: 650;
    font-size: 0.9rem;
    color: var(--accent);
    padding: 0.35rem 0;
  }

  .scan-rooms-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(15rem, 1fr));
    gap: 0.65rem;
    margin-top: 0.65rem !important;
  }

  .scan-room-chip {
    padding: 0.7rem 0.85rem;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border-subtle);
    background: var(--bg-surface);
  }

  .scan-room-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    margin-bottom: 0.25rem;
  }

  .scan-room-chip small {
    display: block;
    color: var(--ink-muted);
    font-size: 0.76rem;
    line-height: 1.35;
  }

  .alert-note {
    margin: 0.75rem 0 0.5rem;
    font-size: 0.86rem;
    color: var(--ink-muted);
  }

  .alerts-list {
    display: grid;
    gap: 0.6rem;
  }

  .alert-card {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.85rem 1rem;
    border: 1px solid var(--warning-border);
    border-radius: var(--radius-sm);
    background: var(--bg-surface);
  }

  .alert-card.is-ack {
    border-color: var(--border-subtle);
    opacity: 0.85;
  }

  .alert-info {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.6rem;
  }

  .sep {
    margin-inline: 0.35rem;
    color: var(--ink-muted);
  }

  .bookings-main {
    display: grid;
    gap: 1.75rem;
    min-width: 0;
  }

  .block-header {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    margin-bottom: 0.75rem;
  }

  .block-header h3 {
    margin: 0;
  }

  .booking-card {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.85rem;
  }

  .booking-card.is-canceled {
    opacity: 0.72;
    background: var(--bg-subtle);
  }

  .card-body {
    min-width: 0;
    flex: 1 1 16rem;
    display: grid;
    gap: 0.35rem;
  }

  .card-badges {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
  }

  .card-title {
    font-size: 1.04rem;
    color: var(--ink-primary);
  }

  .time-range {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
    color: var(--ink-secondary);
    font-weight: 550;
  }

  .to-label {
    color: var(--ink-muted);
    font-size: 0.85rem;
  }

  .utc-caption {
    font-size: 0.75rem;
    color: var(--ink-muted);
  }

  .card-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .policy-note {
    margin: 0 0 0.85rem;
    padding: 0.75rem 0.95rem;
    border-radius: var(--radius-sm);
    background: var(--bg-subtle);
    border: 1px solid var(--border-subtle);
    font-size: 0.84rem;
    line-height: 1.45;
    color: var(--ink-muted);
  }

  .series-organizer {
    margin: 0.25rem 0 0.65rem;
    font-size: 0.88rem;
    color: var(--ink-muted);
  }

  .series-meta-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
    gap: 0.6rem;
    padding: 0.75rem;
    border-radius: var(--radius-sm);
    background: var(--bg-subtle);
    margin-bottom: 0.65rem;
  }

  .series-meta-grid > div {
    display: grid;
    gap: 0.15rem;
  }

  .series-meta-grid span {
    font-size: 0.75rem;
    color: var(--ink-muted);
  }

  .series-meta-grid strong {
    font-size: 0.84rem;
    color: var(--ink-primary);
  }

  .occurrences-list {
    display: grid;
    gap: 0.5rem;
    margin-top: 0.5rem !important;
  }

  .occurrence-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.65rem;
    padding: 0.65rem 0.8rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--bg-subtle);
  }

  .occurrence-row.is-canceled {
    opacity: 0.7;
  }

  .occ-info {
    display: grid;
    gap: 0.2rem;
    font-size: 0.86rem;
  }

  .checkbox-card {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    padding: 0.55rem 0.75rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--bg-surface);
    cursor: pointer;
  }

  .form-grid-2 {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.6rem;
  }

  .form-grid-2 > div {
    display: grid;
    gap: 0.3rem;
    min-width: 0;
  }

  .weekdays-fieldset {
    margin: 0;
    padding: 0.65rem 0.75rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--bg-surface);
  }

  .weekdays-fieldset legend {
    padding-inline: 0.25rem;
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--ink-secondary);
  }

  .weekdays-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .weekday-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.35rem 0.6rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-pill);
    background: var(--bg-surface);
    font-size: 0.82rem !important;
    cursor: pointer;
    transition: background var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
  }

  .weekday-chip:has(input:checked) {
    background: var(--accent-soft);
    border-color: var(--accent);
    color: #134e31;
  }

  .field-hint {
    margin: 0;
    font-size: 0.8rem;
    color: var(--ink-muted);
  }

  .time-section {
    display: grid;
    gap: 0.6rem;
    margin: 0;
    padding: 0.75rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--bg-subtle);
  }

  .time-date-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 0.6rem;
  }

  .time-date-row.has-end-date {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .time-field {
    display: grid;
    gap: 0.3rem;
    min-width: 0;
  }

  .time-meta-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }

  .duration-presets {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .preset-chip {
    min-height: 2rem !important;
    padding: 0.25rem 0.65rem !important;
    border: 1px solid var(--border-strong) !important;
    border-radius: var(--radius-pill) !important;
    background: var(--bg-surface) !important;
    color: var(--ink-secondary) !important;
    font-size: 0.8rem !important;
    font-weight: 600 !important;
  }

  .preset-chip:hover:not(:disabled) {
    border-color: var(--accent) !important;
    color: var(--accent) !important;
    background: var(--bg-surface) !important;
  }

  .preset-chip.is-active {
    background: var(--accent-soft) !important;
    border-color: var(--accent) !important;
    color: #134e31 !important;
  }

  .link-toggle {
    min-height: auto !important;
    padding: 0.2rem 0.4rem !important;
    border: 0 !important;
    border-radius: var(--radius-sm) !important;
    background: transparent !important;
    color: var(--accent) !important;
    font-size: 0.8rem !important;
    font-weight: 600 !important;
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .link-toggle:hover:not(:disabled) {
    background: var(--accent-soft) !important;
  }

  .duration-summary strong {
    color: var(--ink-primary);
  }

  .time-error {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.4rem;
    padding: 0.45rem 0.6rem;
    border-radius: var(--radius-sm);
    background: var(--danger-soft);
    border: 1px solid var(--danger-border);
    color: #7d201a;
    font-size: 0.82rem;
    font-weight: 550;
  }

  @media (max-width: 560px) {
    .form-grid-2,
    .time-date-row.has-end-date {
      grid-template-columns: 1fr;
    }
  }
</style>
