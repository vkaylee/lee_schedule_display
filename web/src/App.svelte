<script>
  import { onMount } from 'svelte';
  import Rooms from './Rooms.svelte';
  import Bookings from './Bookings.svelte';
  import Devices from './Devices.svelte';
  import Kiosk from './Kiosk.svelte';

  let t;
  $: t = session?.language === 'en' ? (_vi, en) => en : (vi, _en) => vi;
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
      if (session === currentSession) usersError = t('Không thể tải danh sách tài khoản. Vui lòng thử lại.', 'Could not load accounts. Please try again.');
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
        usersError = t('Không thể vô hiệu hóa tài khoản hoạt động cuối cùng.', 'Cannot disable the last active account.');
        return false;
      }
      if (response.status !== 201 && response.status !== 204) throw new Error();
      return true;
    } catch {
      usersError = t('Không thể thực hiện thao tác. Vui lòng thử lại.', 'Could not complete the operation. Please try again.');
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
      notice = t('Đã tạo tài khoản.', 'Account created.');
      await loadUsers();
    }
  }

  async function disableUser(user) {
    if (!window.confirm(t(`Vô hiệu hóa tài khoản "${user.username}"? Tài khoản này sẽ không thể đăng nhập.`, `Disable account "${user.username}"? This account will no longer be able to sign in.`))) return;
    if (await updateUser(`/api/users/${encodeURIComponent(user.id)}/disable`)) {
      if (user.username === session.username) {
        await logout();
        return;
      }
      if (resetUser?.id === user.id) cancelReset();
      notice = t('Đã vô hiệu hóa tài khoản.', 'Account disabled.');
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
      notice = t('Đã đặt lại mật khẩu.', 'Password reset.');
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
      else if (response.status !== 401) error = t('Không thể kiểm tra phiên đăng nhập. Vui lòng thử lại.', 'Could not check your session. Please try again.');
    } catch {
      error = t('Không thể kết nối máy chủ. Vui lòng thử lại.', 'Could not connect to the server. Please try again.');
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
        error = t('Tên đăng nhập hoặc mật khẩu không đúng.', 'Invalid username or password.');
      } else if (response.status === 429) {
        error = t('Quá nhiều lần đăng nhập. Vui lòng thử lại sau.', 'Too many login attempts. Please try again later.');
      } else {
        error = t('Không thể đăng nhập. Vui lòng thử lại.', 'Could not log in. Please try again.');
      }
    } catch {
      error = t('Không thể kết nối máy chủ. Vui lòng thử lại.', 'Could not connect to the server. Please try again.');
    } finally {
      busy = false;
    }
  }

  async function changeLanguage(event) {
    const select = event.currentTarget;
    const language = select.value;
    busy = true;
    error = '';
    try {
      const response = await fetch('/api/session/language', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': session.csrf_token },
        body: JSON.stringify({ language }),
      });
      if (!response.ok) throw new Error();
      session = { ...session, language };
    } catch {
      select.value = session.language;
      error = session.language === 'en' ? 'Could not save language. Please try again.' : 'Không thể lưu ngôn ngữ. Vui lòng thử lại.';
    } finally { busy = false; }
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
        error = t('Không thể đăng xuất. Vui lòng thử lại.', 'Could not log out. Please try again.');
      }
    } catch {
      error = t('Không thể kết nối máy chủ. Vui lòng thử lại.', 'Could not connect to the server. Please try again.');
    } finally {
      busy = false;
    }
  }

  if (window.location.pathname !== '/kiosk') onMount(loadSession);
</script>

<svelte:head><title>{t('Quản lý lịch phòng họp', 'Meeting room schedule management')}</title></svelte:head>

{#if window.location.pathname === '/kiosk'}
  <Kiosk />
{:else}
<main>
  <div class="panel">
    <p class="eyebrow">SCHEDULE DISPLAY</p>
    {#if loading}
      <h1>{t('Quản lý lịch phòng họp', 'Meeting room schedule management')}</h1>
      <p role="status">{t('Đang kiểm tra phiên đăng nhập…', 'Checking session…')}</p>
    {:else if session}
      <h1>{t('Quản lý lịch phòng họp', 'Meeting room schedule management')}</h1>
      <p>{t('Đã đăng nhập:', 'Signed in:')} <strong>{session.username}</strong></p>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <label for="account-language">Ngôn ngữ / Language</label>
      <select id="account-language" value={session.language} onchange={changeLanguage} disabled={busy}>
        <option value="vi">Tiếng Việt</option><option value="en">English</option>
      </select>
      <button type="button" onclick={logout} disabled={busy}>{t('Đăng xuất', 'Log out')}</button>
      <Rooms csrf={session.csrf_token} language={session.language} />
      <Bookings csrf={session.csrf_token} language={session.language} />
      <Devices csrf={session.csrf_token} language={session.language} />
      <section aria-labelledby="users-heading">
        <h2 id="users-heading">{t('Quản lý tài khoản', 'Account management')}</h2>
        <p>{t('Quản lý quyền truy cập và mật khẩu đăng nhập.', 'Manage access and login passwords.')}</p>
        {#if usersError}<p class="error" role="alert">{usersError}</p>{/if}
        {#if notice}<p class="notice" role="status">{notice}</p>{/if}
        <h3>{t('Danh sách tài khoản', 'Accounts')}</h3>
        {#if usersLoading}
          <p role="status">{t('Đang tải danh sách tài khoản…', 'Loading accounts…')}</p>
        {:else if usersError && users.length === 0}
          <button class="secondary" type="button" onclick={loadUsers}>{t('Thử tải lại', 'Retry')}</button>
        {:else if users.length === 0}
          <p>{t('Chưa có tài khoản nào.', 'No accounts yet.')}</p>
        {:else}
          <ul class="users">
            {#each users as user (user.id)}
              <li>
                <div><strong>{user.username}</strong><br /><span class="state">{user.active ? t('Đang hoạt động', 'Active') : t('Đã vô hiệu hóa', 'Disabled')}</span></div>
                {#if user.active}
                  <div class="actions">
                    <button class="secondary" type="button" disabled={busy} aria-label={`${t('Đặt lại mật khẩu cho', 'Reset password for')} ${user.username}`} onclick={() => { cancelReset(); resetUser = user; }}>{t('Đặt lại mật khẩu', 'Reset password')}</button>
                    <button class="danger" type="button" disabled={busy} aria-label={`${t('Vô hiệu hóa', 'Disable')} ${user.username}`} onclick={() => disableUser(user)}>{t('Vô hiệu hóa', 'Disable')}</button>
                  </div>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
        {#if resetUser}
          <form onsubmit={changePassword}>
            <h3>{t('Đặt lại mật khẩu cho', 'Reset password for')} {resetUser.username}</h3>
            <label for="reset-password">{t('Mật khẩu mới', 'New password')}</label>
            <input id="reset-password" name="new-password" type="password" autocomplete="new-password" bind:value={resetPassword} required maxlength="1024" />
            <div class="actions">
              <button type="submit" disabled={busy}>{t('Lưu mật khẩu', 'Save password')}</button>
              <button class="secondary" type="button" disabled={busy} onclick={cancelReset}>{t('Hủy', 'Cancel')}</button>
            </div>
          </form>
        {/if}
        <form onsubmit={createUser}>
          <h3>{t('Tạo tài khoản', 'Create account')}</h3>
          <label for="new-username">{t('Tên đăng nhập', 'Username')}</label>
          <input id="new-username" name="new-username" autocomplete="off" bind:value={newUsername} required maxlength="128" />
          <label for="new-password">{t('Mật khẩu', 'Password')}</label>
          <input id="new-password" name="new-password" type="password" autocomplete="new-password" bind:value={newPassword} required maxlength="1024" />
          <button type="submit" disabled={busy}>{t('Tạo tài khoản', 'Create account')}</button>
        </form>
      </section>
    {:else}
      <h1>{t('Đăng nhập quản trị', 'Administrator login')}</h1>
      <p>{t('Đăng nhập để quản lý lịch phòng họp.', 'Sign in to manage room schedules.')}</p>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <form onsubmit={login}>
        <label for="username">{t('Tên đăng nhập', 'Username')}</label>
        <input id="username" name="username" autocomplete="username" bind:value={username} required maxlength="128" />
        <label for="password">{t('Mật khẩu', 'Password')}</label>
        <input id="password" name="password" type="password" autocomplete="current-password" bind:value={password} required maxlength="1024" />
        <button type="submit" disabled={busy}>{busy ? t('Đang đăng nhập…', 'Signing in…') : t('Đăng nhập', 'Sign in')}</button>
      </form>
      {#if error}<button class="secondary" type="button" onclick={loadSession}>{t('Kiểm tra kết nối', 'Check connection')}</button>{/if}
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
