<script>
  import { onMount } from 'svelte';
  import { statusAt } from './kiosk-time.js';
  let token = $state(''), input = $state(''), data = $state(null), status = $state('Không xác định'), error = $state('');
  let lastSync = $state(''), zone = $state('UTC'), clock = $state('');
  let timer, syncClock = 0, syncWall = 0, serverNow = 0, uncertainty = 0, syncing = false, generation = 0;
  function refreshStatus() {
    status = statusAt(data, performance.now() - syncClock, Date.now() - syncWall, serverNow, uncertainty);
    clock = new Date().toLocaleString('vi-VN', {timeZone: zone});
  }
  async function sync() {
    if (!token || syncing) return;
    syncing = true;
    const requestGeneration = generation;
    const requestClock = performance.now(), requestWall = Date.now();
    try {
      const response = await fetch('/api/kiosk', {headers:{Authorization:`Bearer ${token}`},cache:'no-store',signal:AbortSignal.timeout(10000)});
      if (requestGeneration !== generation) return;
      if (response.status === 401) { token=''; sessionStorage.removeItem('kiosk-token'); data=null; status='Không xác định'; error='Thiết bị đã bị thu hồi hoặc không hợp lệ.'; return; }
      if (!response.ok) throw new Error();
      const result = await response.json();
      if (requestGeneration !== generation) return;
      data = result; syncClock = performance.now(); syncWall = Date.now(); uncertainty = Math.max(syncClock - requestClock, syncWall - requestWall); serverNow = data.now; lastSync = new Date().toLocaleString(); error = ''; refreshStatus();
    } catch { if (requestGeneration === generation) { data = null; status = 'Không xác định'; error = 'Mất kết nối; không thể xác minh lịch.'; } }
    finally { syncing = false; if (requestGeneration !== generation) sync(); }
  }
  function pair(event) { event.preventDefault(); token = input.trim(); input = ''; sessionStorage.setItem('kiosk-token', token); sync(); }
  onMount(() => {
    token = sessionStorage.getItem('kiosk-token') || '';
    fetch('/api/timezone').then(r => r.ok ? r.json() : Promise.reject()).then(value => { zone = value.timezone; refreshStatus(); }).catch(() => {});
    sync(); timer = setInterval(() => { refreshStatus(); sync(); }, 30000);
    const visible = () => { if (!document.hidden) { generation++; data = null; status = 'Không xác định'; sync(); } };
    const offline = () => { generation++; data = null; status = 'Không xác định'; error = 'Mất kết nối; không thể xác minh lịch.'; };
    window.addEventListener('offline', offline);
    const restored = () => { generation++; data = null; status = 'Không xác định'; sync(); };
    window.addEventListener('pageshow', restored);
    document.addEventListener('visibilitychange', visible);
    return () => { clearInterval(timer); document.removeEventListener('visibilitychange', visible); window.removeEventListener('pageshow', restored); window.removeEventListener('offline', offline); };
  });
</script>
<main>
  {#if !token}
    <h1>Ghép màn hình phòng</h1><p>Nhập mã do quản trị viên cấp.</p>
    {#if error}<p role="alert">{error}</p>{/if}
    <form onsubmit={pair}><label for="device-token">Mã thiết bị</label><input id="device-token" type="password" autocomplete="off" bind:value={input} required /><button type="submit">Ghép thiết bị</button></form>
  {:else}
    <h1>{data?.room || 'Màn hình phòng'}</h1>
    <p>{clock} ({zone})</p>
    <p class="status" role="status">{status}</p>
    <p>Lần đồng bộ cuối: {lastSync || 'Chưa có'}</p>
    {#if error}<p role="alert">{error}</p>{/if}
    {#if data && status !== 'Không xác định'}<h2>Lịch tới</h2><ul>{#each data.entries as entry}<li>{new Date(entry.starts*1000).toLocaleString('vi-VN', {timeZone:zone})} – {new Date(entry.ends*1000).toLocaleString('vi-VN', {timeZone:zone})} {entry.title || ''} {entry.organizer || ''}</li>{/each}</ul>{/if}
  {/if}
</main>
<style>main{font-family:system-ui,sans-serif;padding:2rem;color:#17374a}.status{font-size:3rem;font-weight:bold}form{display:grid;gap:1rem;max-width:30rem}input,button{font:inherit;padding:.75rem}</style>
