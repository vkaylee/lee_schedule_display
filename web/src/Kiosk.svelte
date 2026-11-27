<script>
  import { onMount } from 'svelte';
  import { statusAt } from './kiosk-time.js';
  let token = $state(''), input = $state(''), data = $state(null), status = $state('Không xác định'), error = $state('');
  let lastSync = $state(null), zone = $state('UTC'), clock = $state(''), language = $state('vi');
  const messages = {
    vi: { pairTitle:'Ghép màn hình phòng', pairHelp:'Nhập mã do quản trị viên cấp.', deviceToken:'Mã thiết bị', pair:'Ghép thiết bị', room:'Màn hình phòng', lastSync:'Lần đồng bộ cuối', none:'Chưa có', upcoming:'Lịch tới', revoked:'Thiết bị đã bị thu hồi hoặc không hợp lệ.', offline:'Mất kết nối; không thể xác minh lịch.', 'Không xác định':'Không xác định', 'Đang họp':'Đang họp', 'Trống':'Trống' },
    en: { pairTitle:'Pair room display', pairHelp:'Enter the code provided by an administrator.', deviceToken:'Device code', pair:'Pair device', room:'Room display', lastSync:'Last synchronized', none:'Never', upcoming:'Upcoming schedule', revoked:'Device revoked or invalid.', offline:'Connection lost; schedule cannot be verified.', 'Không xác định':'Unknown', 'Đang họp':'In meeting', 'Trống':'Available' }
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
  function pair(event) { event.preventDefault(); token = input.trim(); input = ''; sessionStorage.setItem('kiosk-token', token); sync(); }
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
<main>
  {#if !token}
    <h1>{text.pairTitle}</h1><p>{text.pairHelp}</p>
    {#if error}<p role="alert">{text[error]}</p>{/if}
    <form onsubmit={pair}><label for="device-token">{text.deviceToken}</label><input id="device-token" type="password" autocomplete="off" bind:value={input} required /><button type="submit">{text.pair}</button></form>
  {:else}
    <h1>{data?.room || text.room}</h1>
    <p>{clock} ({zone})</p>
    <p class="status" role="status">{text[status]}</p>
    <p>{text.lastSync}: {lastSync ? lastSync.toLocaleString(locale, {timeZone:zone}) : text.none}</p>
    {#if error}<p role="alert">{text[error]}</p>{/if}
    {#if data && status !== 'Không xác định'}<h2>{text.upcoming}</h2><ul>{#each data.entries as entry}<li>{new Date(entry.starts*1000).toLocaleString(locale, {timeZone:zone})} – {new Date(entry.ends*1000).toLocaleString(locale, {timeZone:zone})} {entry.title || ''} {entry.organizer || ''}</li>{/each}</ul>{/if}
  {/if}
</main>
<style>main{font-family:system-ui,sans-serif;padding:2rem;color:#17374a}.status{font-size:3rem;font-weight:bold}form{display:grid;gap:1rem;max-width:30rem}input,button{font:inherit;padding:.75rem}</style>
