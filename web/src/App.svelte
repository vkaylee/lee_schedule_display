<script>
  import { onMount } from 'svelte';
  import Rooms from './Rooms.svelte';
  import Bookings from './Bookings.svelte';
  import Devices from './Devices.svelte';
  import Kiosk from './Kiosk.svelte';

  let username = '';
  let password = '';
  let session = null;
  let loading = true;
  let busy = false;
  let error = '';
  let users = [];
  let usersLoading = false;
  let usersError = '';
  let notice = '';
  let newUsername = '';
  let newPassword = '';
  let resetUser = null;
  let resetPassword = '';

  async function loadUsers() {
    const currentSession = session;
    usersLoading = true;
    usersError = '';
    try {
      const response = await fetch('/api/users', { cache: 'no-store' });
      if (!response.ok) throw new Error();
      const result = await response.json();
      if (session === currentSession) users = result;
    } catch {
      if (session === currentSession) usersError = 'Không thể tải danh sách tài khoản. Vui lòng thử lại.';
    } finally {
      if (session === currentSession) usersLoading = false;
    }
  }

  async function updateUser(path, body) {
    busy = true;
    usersError = '';
    notice = '';
    try {
      const response = await fetch(path, {
        method: 'POST',
        headers: {
          'X-CSRF-Token': session.csrf_token,
          ...(body ? { 'Content-Type': 'application/json' } : {}),
        },
        ...(body ? { body: JSON.stringify(body) } : {}),
      });
      if (response.status === 409 && path.endsWith('/disable')) {
        usersError = 'Không thể vô hiệu hóa tài khoản hoạt động cuối cùng.';
        return false;
      }
      if (response.status !== 201 && response.status !== 204) throw new Error();
      return true;
    } catch {
      usersError = 'Không thể thực hiện thao tác. Vui lòng thử lại.';
      return false;
    } finally {
      busy = false;
    }
  }

  async function createUser(event) {
    event.preventDefault();
    if (!newUsername.trim() || !newPassword) return;
    if (await updateUser('/api/users', { username: newUsername.trim(), password: newPassword })) {
      newUsername = '';
      newPassword = '';
      notice = 'Đã tạo tài khoản.';
      await loadUsers();
    }
  }

  async function disableUser(user) {
    if (!window.confirm(`Vô hiệu hóa tài khoản "${user.username}"? Tài khoản này sẽ không thể đăng nhập.`)) return;
    if (await updateUser(`/api/users/${encodeURIComponent(user.id)}/disable`)) {
      if (user.username === session.username) {
        await logout();
        return;
      }
      if (resetUser?.id === user.id) cancelReset();
      notice = 'Đã vô hiệu hóa tài khoản.';
      await loadUsers();
    }
  }

  function cancelReset() {
    resetUser = null;
    resetPassword = '';
  }

  async function changePassword(event) {
    event.preventDefault();
    if (!resetUser || !resetPassword) return;
    if (await updateUser(`/api/users/${encodeURIComponent(resetUser.id)}/password`, { password: resetPassword })) {
      if (resetUser.username === session.username) {
        await logout();
        return;
      }
      cancelReset();
      notice = 'Đã đặt lại mật khẩu.';
    }
  }

  async function loadSession() {
    loading = true;
    error = '';
    try {
      const response = await fetch('/api/session', { cache: 'no-store' });
      if (response.ok) {
        session = await response.json();
        await loadUsers();
      }
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
        await loadUsers();
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
        users = [];
        usersError = '';
        notice = '';
        newUsername = '';
        newPassword = '';
        cancelReset();
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

  if (window.location.pathname !== '/kiosk') onMount(loadSession);
</script>

<svelte:head><title>Quản lý lịch phòng họp</title></svelte:head>

{#if window.location.pathname === '/kiosk'}
  <Kiosk />
{:else}
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
      <Rooms csrf={session.csrf_token} />
      <Bookings csrf={session.csrf_token} />
      <Devices csrf={session.csrf_token} />
      <section aria-labelledby="users-heading">
        <h2 id="users-heading">Quản lý tài khoản</h2>
        <p>Quản lý quyền truy cập và mật khẩu đăng nhập.</p>
        {#if usersError}<p class="error" role="alert">{usersError}</p>{/if}
        {#if notice}<p class="notice" role="status">{notice}</p>{/if}
        <h3>Danh sách tài khoản</h3>
        {#if usersLoading}
          <p role="status">Đang tải danh sách tài khoản…</p>
        {:else if usersError && users.length === 0}
          <button class="secondary" type="button" onclick={loadUsers}>Thử tải lại</button>
        {:else if users.length === 0}
          <p>Chưa có tài khoản nào.</p>
        {:else}
          <ul class="users">
            {#each users as user (user.id)}
              <li>
                <div><strong>{user.username}</strong><br /><span class="state">{user.active ? 'Đang hoạt động' : 'Đã vô hiệu hóa'}</span></div>
                {#if user.active}
                  <div class="actions">
                    <button class="secondary" type="button" disabled={busy} aria-label={`Đặt lại mật khẩu cho ${user.username}`} onclick={() => { cancelReset(); resetUser = user; }}>Đặt lại mật khẩu</button>
                    <button class="danger" type="button" disabled={busy} aria-label={`Vô hiệu hóa ${user.username}`} onclick={() => disableUser(user)}>Vô hiệu hóa</button>
                  </div>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
        {#if resetUser}
          <form onsubmit={changePassword}>
            <h3>Đặt lại mật khẩu cho {resetUser.username}</h3>
            <label for="reset-password">Mật khẩu mới</label>
            <input id="reset-password" name="new-password" type="password" autocomplete="new-password" bind:value={resetPassword} required maxlength="1024" />
            <div class="actions">
              <button type="submit" disabled={busy}>Lưu mật khẩu</button>
              <button class="secondary" type="button" disabled={busy} onclick={cancelReset}>Hủy</button>
            </div>
          </form>
        {/if}
        <form onsubmit={createUser}>
          <h3>Tạo tài khoản</h3>
          <label for="new-username">Tên đăng nhập</label>
          <input id="new-username" name="new-username" autocomplete="off" bind:value={newUsername} required maxlength="128" />
          <label for="new-password">Mật khẩu</label>
          <input id="new-password" name="new-password" type="password" autocomplete="new-password" bind:value={newPassword} required maxlength="1024" />
          <button type="submit" disabled={busy}>Tạo tài khoản</button>
        </form>
      </section>
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
{/if}

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
  section { margin-top: 2rem; padding-top: 1.5rem; border-top: 1px solid #c9d7d9; }
  h2 { margin: 0; font-size: 1.4rem; }
  h3 { margin: 1rem 0 0.5rem; font-size: 1.1rem; }
  .users { list-style: none; padding: 0; margin: 0; }
  .users li { padding: 1rem 0; border-bottom: 1px solid #dbe5e5; overflow-wrap: anywhere; }
  .state { color: #476575; font-size: 0.875rem; }
  .actions { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.75rem; }
  .actions .secondary { margin-top: 0; }
  button.danger { background: white; color: #7d261e; border-color: #a63428; }
  .notice { padding: 0.75rem; border-left: 0.25rem solid #237a54; background: #edf7ef; color: #155c3f; }
</style>
