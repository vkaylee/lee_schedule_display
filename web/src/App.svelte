<script>
  import { onMount } from 'svelte';

  let username = '';
  let password = '';
  let session = null;
  let loading = true;
  let busy = false;
  let error = '';

  async function loadSession() {
    loading = true;
    error = '';
    try {
      const response = await fetch('/api/session', { cache: 'no-store' });
      if (response.ok) session = await response.json();
      else if (response.status !== 401) error = 'Không thể kiểm tra phiên đăng nhập. Vui lòng thử lại.';
    } catch {
      error = 'Không thể kết nối máy chủ. Vui lòng thử lại.';
    } finally {
      loading = false;
    }
  }

  async function login(event) {
    event.preventDefault();
    busy = true;
    error = '';
    try {
      const response = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      });
      if (response.ok) {
        session = await response.json();
        password = '';
      } else if (response.status === 401) {
        error = 'Tên đăng nhập hoặc mật khẩu không đúng.';
      } else if (response.status === 429) {
        error = 'Quá nhiều lần đăng nhập. Vui lòng thử lại sau.';
      } else {
        error = 'Không thể đăng nhập. Vui lòng thử lại.';
      }
    } catch {
      error = 'Không thể kết nối máy chủ. Vui lòng thử lại.';
    } finally {
      busy = false;
    }
  }

  async function logout() {
    busy = true;
    error = '';
    try {
      const response = await fetch('/api/logout', {
        method: 'POST',
        headers: { 'X-CSRF-Token': session.csrf_token },
      });
      if (response.ok || response.status === 401) {
        session = null;
        username = '';
        password = '';
      } else {
        error = 'Không thể đăng xuất. Vui lòng thử lại.';
      }
    } catch {
      error = 'Không thể kết nối máy chủ. Vui lòng thử lại.';
    } finally {
      busy = false;
    }
  }

  onMount(loadSession);
</script>

<svelte:head><title>Quản lý lịch phòng họp</title></svelte:head>

<main>
  <div class="panel">
    <p class="eyebrow">SCHEDULE DISPLAY</p>
    {#if loading}
      <h1>Quản lý lịch phòng họp</h1>
      <p role="status">Đang kiểm tra phiên đăng nhập…</p>
    {:else if session}
      <h1>Quản lý lịch phòng họp</h1>
      <p>Đã đăng nhập: <strong>{session.username}</strong></p>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button type="button" onclick={logout} disabled={busy}>Đăng xuất</button>
    {:else}
      <h1>Đăng nhập quản trị</h1>
      <p>Đăng nhập để quản lý lịch phòng họp.</p>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <form onsubmit={login}>
        <label for="username">Tên đăng nhập</label>
        <input id="username" name="username" autocomplete="username" bind:value={username} required maxlength="128" />
        <label for="password">Mật khẩu</label>
        <input id="password" name="password" type="password" autocomplete="current-password" bind:value={password} required maxlength="1024" />
        <button type="submit" disabled={busy}>{busy ? 'Đang đăng nhập…' : 'Đăng nhập'}</button>
      </form>
      {#if error}<button class="secondary" type="button" onclick={loadSession}>Kiểm tra kết nối</button>{/if}
    {/if}
  </div>
</main>

<style>
  :global(*) { box-sizing: border-box; }
  :global(body) { margin: 0; font-family: system-ui, sans-serif; color: #17374a; background: #eef3f4; }
  main { min-height: 100vh; display: grid; place-items: center; padding: 1.5rem; }
  .panel { width: min(100%, 28rem); padding: 2rem; border: 1px solid #c9d7d9; border-top: 0.3rem solid #237a54; background: white; }
  .eyebrow { margin: 0 0 1rem; color: #476575; font-size: 0.75rem; font-weight: 700; letter-spacing: 0.1em; }
  h1 { margin: 0 0 0.75rem; font-size: 1.75rem; line-height: 1.2; }
  p { line-height: 1.5; }
  form { display: grid; gap: 0.5rem; margin-top: 1.5rem; }
  label { font-weight: 600; }
  input { width: 100%; padding: 0.75rem; border: 1px solid #879da4; border-radius: 0.25rem; font: inherit; margin-bottom: 0.5rem; }
  button { padding: 0.75rem 1rem; border: 1px solid #155c3f; border-radius: 0.25rem; background: #176b47; color: white; font: inherit; font-weight: 600; cursor: pointer; }
  button:disabled { opacity: 0.65; cursor: wait; }
  button.secondary { margin-top: 1rem; background: white; color: #155c3f; }
  :global(:focus-visible) { outline: 3px solid #28669a; outline-offset: 2px; }
  .error { padding: 0.75rem; border-left: 0.25rem solid #a63428; background: #fff1ef; color: #7d261e; }
</style>
