<script>
  import { onMount } from 'svelte';
  import { statusAt } from './kiosk-time.js';
  import DisplayCanvas from './DisplayCanvas.svelte';
  import { defaultDisplayLayout } from './display-layout.js';
  let { id } = $props();
  let data = $state(null), config = $state({ organization:'', logo:'', blocks:defaultDisplayLayout }), zone = $state('UTC'), status = $state('Không xác định'), clock = $state(new Date()), error = $state('');
  let syncClock = 0, syncWall = 0, serverNow = 0, uncertainty = 0, generation = 0, syncing = false;
  function tick() { clock = new Date(); status = statusAt(data, performance.now() - syncClock, Date.now() - syncWall, serverNow, uncertainty, 30000); }
  async function sync() {
    if (syncing || document.hidden) return;
    syncing = true; const current = generation, start = performance.now(), wall = Date.now();
    try {
      const response = await fetch(`/api/display/${encodeURIComponent(id)}`, { cache:'no-store', signal:AbortSignal.timeout(10000) });
      if (!response.ok) throw new Error();
      const result = await response.json();
      const elapsed = performance.now() - start, wallElapsed = Date.now() - wall;
      if (current !== generation || document.hidden || elapsed < 0 || wallElapsed < 0 || Math.abs(elapsed - wallElapsed) > 1000) throw new Error();
      data = result; config = result.config; syncClock = performance.now(); syncWall = Date.now(); serverNow = result.now; uncertainty = Math.max(elapsed, wallElapsed); error = ''; tick();
    } catch { if (current === generation) { data = null; status = 'Không xác định'; error = 'Không thể kết nối hoặc thiết bị không còn hoạt động.'; } }
    finally { syncing = false; if (current !== generation && !document.hidden) sync(); }
  }
  onMount(() => {
    fetch('/api/timezone').then(r => r.ok ? r.json() : Promise.reject()).then(v => zone = v.timezone).catch(() => {});
    sync(); const timer = setInterval(() => { tick(); sync(); }, 30000); const clockTimer = setInterval(tick, 1000);
    const invalidate = () => { generation++; data = null; status = 'Không xác định'; };
    const visibility = () => { invalidate(); if (!document.hidden) sync(); };
    window.addEventListener('offline', invalidate); window.addEventListener('pagehide', invalidate); window.addEventListener('pageshow', visibility); document.addEventListener('visibilitychange', visibility);
    return () => { clearInterval(timer); clearInterval(clockTimer); window.removeEventListener('offline', invalidate); window.removeEventListener('pagehide', invalidate); window.removeEventListener('pageshow', visibility); document.removeEventListener('visibilitychange', visibility); };
  });
</script>
<main><DisplayCanvas {config} {data} {status} {zone} {clock} />{#if error}<p role="alert">{error}</p>{/if}</main>
<style>main{margin:0;min-height:100vh;background:#e9f2f7}p{padding:1rem;color:#7d261e}</style>
