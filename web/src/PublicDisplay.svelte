<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { statusAt } from './kiosk-time.js';
  import DisplayCanvas from './DisplayCanvas.svelte';
  import { defaultDisplayLayout } from './display-layout.js';
  let { id } = $props();
  let data = $state(null), config = $state({ organization:'', logo:'', blocks:defaultDisplayLayout }), zone = $state('UTC'), status = $state('Không xác định'), clock = $state(new Date()), scheduleNow = $state(null), error = $state('');
  let syncClock = 0, syncWall = 0, serverNow = 0, uncertainty = 0, generation = 0, syncing = false;
  function tick() { const elapsed = performance.now() - syncClock; status = statusAt(data, elapsed, Date.now() - syncWall, serverNow, uncertainty, 30000); scheduleNow = status === 'Không xác định' ? null : serverNow + elapsed / 1000; if (scheduleNow !== null) clock = new Date(scheduleNow * 1000); }
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
<main class="public-display">
  <DisplayCanvas {config} {data} {status} {zone} {clock} {scheduleNow} />
  {#if error}
    <p class="display-alert" role="alert">
      <Icon name="alert" />
      <span>{error}</span>
    </p>
  {/if}
</main>
<style>
  .public-display {
    position: relative;
    margin: 0;
    min-height: 100dvh;
    background: var(--display-canvas-bg, #ecefe7);
  }
  .display-alert {
    position: fixed;
    right: 1.25rem;
    bottom: 1.25rem;
    z-index: 20;
    margin: 0;
    display: flex;
    align-items: center;
    gap: 0.6rem;
    max-width: min(28rem, calc(100vw - 2.5rem));
    padding: 0.85rem 1.1rem;
    border: 1px solid var(--danger-border, #eabeb8);
    border-left: 4px solid var(--danger, #9e2a22);
    border-radius: var(--radius-md, 0.65rem);
    background: var(--danger-soft, #fdf0ee);
    color: #7d201a;
    font-weight: 600;
    box-shadow: var(--shadow-md, 0 10px 28px rgba(24, 44, 35, 0.16));
  }
</style>
