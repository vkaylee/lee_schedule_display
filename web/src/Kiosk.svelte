<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import Banner from './Banner.svelte';
  import { statusAt } from './kiosk-time.js';
  let token = $state(''), input = $state(''), showToken = $state(false), data = $state(null), status = $state('Không xác định'), error = $state('');
  let lastSync = $state(null), zone = $state('UTC'), clock = $state(''), language = $state('vi');
  const messages = {
    vi: { pairTitle:'Ghép màn hình phòng', pairHelp:'Nhập mã do quản trị viên cấp.', deviceToken:'Mã thiết bị', showCode:'Hiện mã', hideCode:'Ẩn mã', pair:'Ghép thiết bị', room:'Màn hình phòng', lastSync:'Lần đồng bộ cuối', none:'Chưa có', upcoming:'Lịch tới', noUpcoming:'Không có lịch sắp tới', unavailable:'Không thể xác minh lịch khi mất kết nối.', revoked:'Thiết bị đã bị thu hồi hoặc không hợp lệ.', offline:'Mất kết nối; không thể xác minh lịch.', 'Không xác định':'Không xác định', 'Đang họp':'Đang họp', 'Trống':'Trống' },
    en: { pairTitle:'Pair room display', pairHelp:'Enter the code provided by an administrator.', deviceToken:'Device code', showCode:'Show code', hideCode:'Hide code', pair:'Pair device', room:'Room display', lastSync:'Last synchronized', none:'Never', upcoming:'Upcoming schedule', noUpcoming:'No upcoming meetings', unavailable:'Schedule cannot be verified while offline.', revoked:'Device revoked or invalid.', offline:'Connection lost; schedule cannot be verified.', 'Không xác định':'Unknown', 'Đang họp':'In meeting', 'Trống':'Available' }
  };
  let text = $derived(messages[language]);
  let locale = $derived(language === 'en' ? 'en-US' : 'vi-VN');
  // ponytail: reserve one polling interval around schedule boundaries; use boundary timers for finer display.
  const refreshDelay = 30000;
  let timer, syncClock = 0, syncWall = 0, serverNow = 0, uncertainty = 0, syncing = false, generation = 0;
  function refreshStatus() {
    status = statusAt(data, performance.now() - syncClock, Date.now() - syncWall, serverNow, uncertainty, refreshDelay);
    clock = new Date().toLocaleString(locale, {timeZone: zone});
  }
  async function sync() {
    if (!token || syncing || document.hidden) return;
    syncing = true;
    const requestGeneration = generation;
    const requestClock = performance.now(), requestWall = Date.now();
    try {
      const response = await fetch('/api/kiosk', {headers:{Authorization:`Bearer ${token}`},cache:'no-store',signal:AbortSignal.timeout(10000)});
      if (requestGeneration !== generation) return;
      if (response.status === 401) { token=''; sessionStorage.removeItem('kiosk-token'); data=null; status='Không xác định'; error='revoked'; return; }
      if (!response.ok) throw new Error();
      const result = await response.json();
      if (requestGeneration !== generation) return;
      const receivedClock = performance.now(), receivedWall = Date.now();
      const elapsedClock = receivedClock - requestClock, elapsedWall = receivedWall - requestWall;
      if (document.hidden || elapsedClock < 0 || elapsedWall < 0 || Math.abs(elapsedClock - elapsedWall) > 1000) throw new Error();
      language = result.language === 'en' ? 'en' : 'vi'; data = result; syncClock = receivedClock; syncWall = receivedWall; uncertainty = Math.max(elapsedClock, elapsedWall); serverNow = data.now; lastSync = new Date(); error = ''; refreshStatus();
    } catch { if (requestGeneration === generation) { data = null; status = 'Không xác định'; error = 'offline'; } }
    finally { syncing = false; if (requestGeneration !== generation) sync(); }
  }
  function pair(event) { event.preventDefault(); token = input.trim(); input = ''; showToken = false; sessionStorage.setItem('kiosk-token', token); sync(); }
  onMount(() => {
    token = sessionStorage.getItem('kiosk-token') || '';
    fetch('/api/timezone').then(r => r.ok ? r.json() : Promise.reject()).then(value => { zone = value.timezone; refreshStatus(); }).catch(() => {});
    sync(); timer = setInterval(() => { refreshStatus(); sync(); }, refreshDelay);
    const invalidate = () => { generation++; data = null; status = 'Không xác định'; };
    const visible = () => { invalidate(); if (!document.hidden) sync(); };
    const offline = () => { invalidate(); error = 'offline'; };
    const restored = () => { invalidate(); if (!document.hidden) sync(); };
    window.addEventListener('offline', offline);
    window.addEventListener('pagehide', invalidate);
    window.addEventListener('pageshow', restored);
    document.addEventListener('freeze', invalidate);
    document.addEventListener('visibilitychange', visible);
    return () => { clearInterval(timer); document.removeEventListener('visibilitychange', visible); document.removeEventListener('freeze', invalidate); window.removeEventListener('pageshow', restored); window.removeEventListener('pagehide', invalidate); window.removeEventListener('offline', offline); };
  });
</script>

{#if !token}
  <main class="ui-auth-page">
    <div class="ui-auth-card">
      <div class="ui-auth-top">
        <Icon name="brand" class="brand-mark" />
        <span>SCHEDULE DISPLAY · KIOSK</span>
      </div>
      <div class="ui-auth-body">
        <h1>{text.pairTitle}</h1>
        <p class="ui-auth-subtitle">{text.pairHelp}</p>
        {#if error}<Banner kind="error" text={text[error]} />{/if}
        <form onsubmit={pair}>
          <label for="device-token">{text.deviceToken}</label>
          <div class="ui-pw-wrap">
            <input id="device-token" class="mono" type={showToken ? 'text' : 'password'} autocomplete="off" bind:value={input} required placeholder="••••••••••••" />
            <button type="button" class="ui-pw-toggle" aria-label={showToken ? text.hideCode : text.showCode} aria-pressed={showToken} onclick={() => showToken = !showToken}>
              <Icon name={showToken ? 'eye-off' : 'eye'} />
            </button>
          </div>
          <button type="submit">{text.pair}</button>
        </form>
      </div>
    </div>
  </main>
{:else}
  <main class="kiosk-board">
    <section
      class="kiosk-hero"
      class:is-available={status === 'Trống'}
      class:is-busy={status === 'Đang họp'}
      class:is-unknown={status === 'Không xác định'}
    >
      <header class="hero-top">
        <div>
          <span class="kiosk-kicker">SCHEDULE DISPLAY</span>
          <h1>{data?.room || text.room}</h1>
        </div>
        <div class="clock-box">
          <strong class="clock-value">{clock}</strong>
          <span class="zone-pill">{zone}</span>
        </div>
      </header>

      <div class="hero-center">
        <div class="status-badge-row">
          <Icon name={status === 'Trống' ? 'check-circle' : status === 'Đang họp' ? 'clock' : 'warning'} class="status-icon" />
          <p class="status" role="status">{text[status]}</p>
        </div>

        {#if error}
          <p class="kiosk-alert" role="alert">
            <Icon name="alert" />
            <span>{text[error]}</span>
          </p>
        {/if}
      </div>

      <footer class="hero-footer">
        <span>{text.lastSync}: {lastSync ? lastSync.toLocaleString(locale, {timeZone:zone}) : text.none}</span>
      </footer>
    </section>

    <aside class="kiosk-schedule">
      <h2>{text.upcoming}</h2>
      {#if data && status !== 'Không xác định'}
        {#if data.entries.length === 0}
          <div class="ui-empty">
            <p>{text.noUpcoming}</p>
          </div>
        {:else}
          <ul class="schedule-list">
            {#each data.entries as entry}
              <li class="schedule-item">
                <div class="entry-time mono">
                  {new Date(entry.starts*1000).toLocaleString(locale, {timeZone:zone})} – {new Date(entry.ends*1000).toLocaleString(locale, {timeZone:zone})}
                </div>
                {#if entry.title}
                  <strong class="entry-title">{entry.title}</strong>
                {/if}
                {#if entry.organizer}
                  <span class="entry-organizer">{entry.organizer}</span>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
      {:else}
        <div class="ui-empty">
          <p>{text.unavailable}</p>
        </div>
      {/if}
    </aside>
  </main>
{/if}

<style>
  :global(.brand-mark) {
    width: 1.25rem;
    height: 1.25rem;
    color: #82d4a5;
  }

  .kiosk-board {
    min-height: 100dvh;
    display: grid;
    grid-template-columns: minmax(0, 1.25fr) minmax(20rem, 0.95fr);
    background: var(--bg-canvas);
  }

  .kiosk-hero {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 2rem;
    padding: clamp(1.5rem, 3.5vw, 3rem);
    color: #f4f7f5;
    background: #2c3632;
    transition: background var(--duration-base) var(--ease-out);
  }

  .kiosk-hero.is-available {
    background: radial-gradient(circle at 80% 18%, #1e7149, #12442c 68%);
  }

  .kiosk-hero.is-busy {
    background: radial-gradient(circle at 80% 18%, #902e24, #631b15 68%);
  }

  .kiosk-hero.is-unknown {
    background: radial-gradient(circle at 80% 18%, #3c4843, #27302c 68%);
  }

  .hero-top {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
  }

  .kiosk-kicker {
    display: block;
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.14em;
    opacity: 0.78;
    margin-bottom: 0.45rem;
  }

  .hero-top h1 {
    margin: 0;
    font-size: clamp(1.8rem, 3.6vw, 3.1rem);
    letter-spacing: -0.03em;
    line-height: 1.1;
  }

  .clock-box {
    display: grid;
    justify-items: end;
    gap: 0.35rem;
  }

  .clock-value {
    font-size: clamp(1.1rem, 2vw, 1.45rem);
    font-variant-numeric: tabular-nums;
    font-weight: 650;
  }

  .zone-pill {
    padding: 0.2rem 0.65rem;
    border-radius: var(--radius-pill);
    background: rgba(255, 255, 255, 0.14);
    font-size: 0.8rem;
    font-weight: 600;
  }

  .hero-center {
    display: grid;
    gap: 1.25rem;
    margin-block: auto;
  }

  .status-badge-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 1rem;
  }

  :global(.status-icon) {
    width: clamp(2.8rem, 5vw, 4.2rem) !important;
    height: clamp(2.8rem, 5vw, 4.2rem) !important;
    flex-shrink: 0;
  }

  .status {
    margin: 0;
    font-size: clamp(2.75rem, 6.2vw, 5.2rem);
    font-weight: 800;
    letter-spacing: -0.035em;
    line-height: 1.05;
  }

  .kiosk-alert {
    margin: 0;
    display: inline-flex;
    align-items: center;
    gap: 0.55rem;
    padding: 0.85rem 1.1rem;
    border-radius: var(--radius-md);
    background: rgba(0, 0, 0, 0.28);
    border: 1px solid rgba(255, 255, 255, 0.28);
    color: #fff3f1;
    font-weight: 600;
    max-width: 34rem;
  }

  .hero-footer {
    font-size: 0.88rem;
    opacity: 0.82;
    font-variant-numeric: tabular-nums;
  }

  .kiosk-schedule {
    display: flex;
    flex-direction: column;
    padding: clamp(1.5rem, 3vw, 2.5rem);
    background: var(--bg-canvas);
    border-left: 1px solid var(--border-subtle);
  }

  .kiosk-schedule h2 {
    margin: 0 0 1.25rem;
    font-size: 1.35rem;
    letter-spacing: -0.02em;
    color: var(--ink-primary);
  }

  .schedule-list {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 0.85rem;
  }

  .schedule-item {
    display: grid;
    gap: 0.3rem;
    padding: 1rem 1.15rem;
    border: 1px solid var(--border-subtle);
    border-left: 4px solid var(--accent);
    border-radius: var(--radius-md);
    background: var(--bg-surface);
    box-shadow: var(--shadow-xs);
  }

  .entry-time {
    font-weight: 650;
    color: var(--accent);
  }

  .entry-title {
    font-size: 1.05rem;
    color: var(--ink-primary);
  }

  .entry-organizer {
    font-size: 0.88rem;
    color: var(--ink-muted);
  }

  @media (max-width: 900px) {
    .kiosk-board {
      grid-template-columns: 1fr;
    }
    .clock-box {
      justify-items: start;
    }
    .kiosk-schedule {
      border-left: 0;
      border-top: 1px solid var(--border-subtle);
    }
  }
</style>
