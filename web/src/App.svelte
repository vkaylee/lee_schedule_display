<script>
  import { onMount } from 'svelte';
  import Rooms from './Rooms.svelte';
  import Bookings from './Bookings.svelte';
  import Devices from './Devices.svelte';
  import Kiosk from './Kiosk.svelte';
  import PublicDisplay from './PublicDisplay.svelte';
  const displayMatch = window.location.pathname.match(/^\/display\/([0-9a-f]{32})$/);

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
  const views = ['overview', 'rooms', 'bookings', 'devices', 'users'];
  let activeView = views.includes(window.location.hash.slice(1)) ? window.location.hash.slice(1) : 'overview';
  let overview = null;
  let overviewError = false;
  let overviewLoading = false;
  let overviewRequest = 0;

  async function loadOverview() {
    const request = ++overviewRequest;
    overviewLoading = true;
    overviewError = false;
    try {
      const responses = await Promise.all(['/api/rooms', '/api/bookings', '/api/devices'].map(url => fetch(url, { cache: 'no-store' })));
      if (responses.some(response => !response.ok)) throw new Error();
      const [rooms, bookings, devices] = await Promise.all(responses.map(response => response.json()));
      if (request === overviewRequest) overview = { rooms, bookings, devices };
    } catch {
      if (request === overviewRequest) overviewError = true;
    } finally {
      if (request === overviewRequest) overviewLoading = false;
    }
  }

  $: if (session && activeView === 'overview') loadOverview();

  onMount(() => {
    const syncView = () => { const view = window.location.hash.slice(1); if (views.includes(view)) activeView = view; };
    window.addEventListener('hashchange', syncView);
    return () => window.removeEventListener('hashchange', syncView);
  });

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
      if (session === currentSession) usersError = ['Không thể tải danh sách tài khoản. Vui lòng thử lại.', 'Could not load accounts. Please try again.'];
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
        usersError = ['Không thể vô hiệu hóa tài khoản hoạt động cuối cùng.', 'Cannot disable the last active account.'];
        return false;
      }
      if (response.status !== 201 && response.status !== 204) throw new Error();
      return true;
    } catch {
      usersError = ['Không thể thực hiện thao tác. Vui lòng thử lại.', 'Could not complete the operation. Please try again.'];
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
      notice = ['Đã tạo tài khoản.', 'Account created.'];
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
      notice = ['Đã vô hiệu hóa tài khoản.', 'Account disabled.'];
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
      notice = ['Đã đặt lại mật khẩu.', 'Password reset.'];
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
      else if (response.status !== 401) error = ['Không thể kiểm tra phiên đăng nhập. Vui lòng thử lại.', 'Could not check your session. Please try again.'];
    } catch {
      error = ['Không thể kết nối máy chủ. Vui lòng thử lại.', 'Could not connect to the server. Please try again.'];
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
        error = ['Tên đăng nhập hoặc mật khẩu không đúng.', 'Invalid username or password.'];
      } else if (response.status === 429) {
        error = ['Quá nhiều lần đăng nhập. Vui lòng thử lại sau.', 'Too many login attempts. Please try again later.'];
      } else {
        error = ['Không thể đăng nhập. Vui lòng thử lại.', 'Could not log in. Please try again.'];
      }
    } catch {
      error = ['Không thể kết nối máy chủ. Vui lòng thử lại.', 'Could not connect to the server. Please try again.'];
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
      error = ['Không thể lưu ngôn ngữ. Vui lòng thử lại.', 'Could not save language. Please try again.'];
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
        error = ['Không thể đăng xuất. Vui lòng thử lại.', 'Could not log out. Please try again.'];
      }
    } catch {
      error = ['Không thể kết nối máy chủ. Vui lòng thử lại.', 'Could not connect to the server. Please try again.'];
    } finally {
      busy = false;
    }
  }

  if (window.location.pathname !== '/kiosk' && !displayMatch) onMount(loadSession);
</script>

<svelte:head><title>{t('Quản lý lịch phòng họp', 'Meeting room schedule management')}</title></svelte:head>

{#if window.location.pathname === '/kiosk'}
  <Kiosk />
{:else if displayMatch}
  <PublicDisplay id={displayMatch[1]} />
{:else}
<main>
  <div class:dashboard={session} class="panel">
    <p class="eyebrow">SCHEDULE DISPLAY <span class="eyebrow-line"></span> {session ? t('KHÔNG GIAN LÀM VIỆC', 'WORKSPACE') : ''}</p>
    {#if loading}
      <h1>{t('Quản lý lịch phòng họp', 'Meeting room schedule management')}</h1>
      <p role="status">{t('Đang kiểm tra phiên đăng nhập…', 'Checking session…')}</p>
    {:else if session}
      <header class="workspace-header">
        <div><p class="section-kicker">{t('QUẢN LÝ KHÔNG GIAN', 'SPACE MANAGEMENT')}</p><h1>{t('Lịch phòng họp', 'Meeting rooms')}</h1><p class="intro">{t('Mọi cuộc họp, mọi không gian — trong tầm tay.', 'Your meetings and spaces, all in one place.')}</p></div>
        <div class="account-bar"><span class="account-name">{session.username}</span>
      {#if error}<p class="error" role="alert">{t(...error)}</p>{/if}
      <label for="account-language">Ngôn ngữ / Language</label>
      <select id="account-language" value={session.language} onchange={changeLanguage} disabled={busy}>
        <option value="vi">Tiếng Việt</option><option value="en">English</option>
      </select>
      <button class="secondary logout" type="button" onclick={logout} disabled={busy}>{t('Đăng xuất', 'Log out')}</button></div></header>
      <nav class="workspace-nav" aria-label={t('Điều hướng quản trị', 'Workspace navigation')}>
        <a href="#overview" aria-current={activeView === 'overview' ? 'page' : undefined}>{t('Tổng quan', 'Overview')}</a><a href="#rooms" aria-current={activeView === 'rooms' ? 'page' : undefined}>{t('Phòng', 'Rooms')}</a><a href="#bookings" aria-current={activeView === 'bookings' ? 'page' : undefined}>{t('Lịch phòng', 'Bookings')}</a><a href="#devices" aria-current={activeView === 'devices' ? 'page' : undefined}>{t('Thiết bị', 'Devices')}</a><a href="#users" aria-current={activeView === 'users' ? 'page' : undefined}>{t('Tài khoản', 'Accounts')}</a>
      </nav>
      <div class="workspace-content">
      {#if activeView === 'overview'}
        <section class="overview" aria-labelledby="overview-heading">
          <div class="overview-header"><div><p class="section-kicker">{t('KHÔNG GIAN CỦA BẠN', 'YOUR WORKSPACE')}</p><h2 id="overview-heading">{t('Tổng quan', 'Overview')}</h2><p>{t('Theo dõi phòng, lịch và thiết bị từ một nơi.', 'Keep track of rooms, bookings, and devices in one place.')}</p></div><button class="secondary" type="button" onclick={loadOverview} disabled={overviewLoading}>{t('Làm mới', 'Refresh')}</button></div>
          {#if overviewLoading}<p role="status">{t('Đang tải tổng quan…', 'Loading overview…')}</p>{/if}
          {#if overviewError}<p class="error" role="alert">{t('Không thể tải tổng quan. Vui lòng thử lại.', 'Could not load the overview. Please try again.')}</p>{/if}
          {#if overview && !overviewError}
            <div class="overview-grid">
              <article><p class="overview-label">{t('PHÒNG HOẠT ĐỘNG', 'ACTIVE ROOMS')}</p><strong>{overview.rooms.filter(room => room.active).length}</strong><p>{t('Phòng sẵn sàng quản lý', 'Rooms available to manage')}</p><a href="#rooms">{t('Quản lý phòng', 'Manage rooms')} <span aria-hidden="true">→</span></a></article>
              <article><p class="overview-label">{t('LỊCH SẮP TỚI', 'UPCOMING BOOKINGS')}</p><strong>{overview.bookings.filter(booking => !booking.canceled && booking.starts * 1000 > Date.now()).length}</strong><p>{t('Lịch chưa bắt đầu', 'Bookings yet to start')}</p><a href="#bookings">{t('Xem lịch phòng', 'View bookings')} <span aria-hidden="true">→</span></a></article>
              <article><p class="overview-label">{t('THIẾT BỊ HOẠT ĐỘNG', 'ACTIVE DEVICES')}</p><strong>{overview.devices.filter(device => !device.revoked).length}</strong><p>{t('Thiết bị chưa bị thu hồi', 'Devices not revoked')}</p><a href="#devices">{t('Quản lý thiết bị', 'Manage devices')} <span aria-hidden="true">→</span></a></article>
            </div>
            <div class="overview-next"><div><h3>{t('Bắt đầu từ đâu?', 'Where to start?')}</h3><p>{overview.rooms.some(room => room.active) ? t('Tạo lịch cho phòng hoặc thiết lập màn hình hiển thị.', 'Create a booking or set up a room display.') : t('Tạo phòng đầu tiên, sau đó thêm lịch và thiết bị hiển thị.', 'Create your first room, then add bookings and displays.')}</p></div><a class="overview-action" href={overview.rooms.some(room => room.active) ? '#bookings' : '#rooms'}>{overview.rooms.some(room => room.active) ? t('Tạo lịch', 'Create booking') : t('Tạo phòng', 'Create room')} <span aria-hidden="true">→</span></a></div>
          {/if}
        </section>
      {/if}
      <div class="view" hidden={activeView !== 'rooms'}><Rooms csrf={session.csrf_token} language={session.language} /></div>
      <div class="view" hidden={activeView !== 'bookings'}><Bookings csrf={session.csrf_token} language={session.language} /></div>
      <div class="view" hidden={activeView !== 'devices'}><Devices csrf={session.csrf_token} language={session.language} /></div>
      <section hidden={activeView !== 'users'} aria-labelledby="users-heading">
        <h2 id="users-heading">{t('Quản lý tài khoản', 'Account management')}</h2>
        <p>{t('Quản lý quyền truy cập và mật khẩu đăng nhập.', 'Manage access and login passwords.')}</p>
        {#if usersError}<p class="error" role="alert">{t(...usersError)}</p>{/if}
        {#if notice}<p class="notice" role="status">{t(...notice)}</p>{/if}
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
      </div>
    {:else}
      <h1>{t('Đăng nhập quản trị', 'Administrator login')}</h1>
      <p>{t('Đăng nhập để quản lý lịch phòng họp.', 'Sign in to manage room schedules.')}</p>
      {#if error}<p class="error" role="alert">{t(...error)}</p>{/if}
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

  :global(body) { color: #253a33; background: #f4f3ee; }
  main:has(.dashboard) { display: block; padding: 0; }
  .dashboard { width: 100%; min-height: 100vh; padding: 0; border: 0; background: transparent; }
  .dashboard > .eyebrow { display: flex; align-items: center; gap: 1rem; margin: 0; padding: 1.15rem max(2rem, calc((100% - 76rem) / 2)); background: #203e32; color: #e9f0e8; letter-spacing: .12em; }
  .eyebrow-line { width: 1.5rem; height: 1px; background: #92a99b; }
  .workspace-header, .workspace-nav, .workspace-content { width: min(100% - 4rem, 76rem); margin-inline: auto; }
  .workspace-header { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 1.5rem; padding-block: 3.2rem 2rem; }
  .section-kicker { margin: 0 0 .6rem; font-size: .72rem; font-weight: 700; letter-spacing: .13em; color: #557365; }
  .workspace-header h1 { font-size: clamp(2rem, 3.5vw, 3.1rem); letter-spacing: -.035em; color: #213b30; }
  .intro { margin: .6rem 0 0; color: #5a7065; }
  .account-bar { display: flex; flex-wrap: wrap; align-items: center; gap: .75rem; font-size: .85rem; }
  .account-name { font-weight: 700; color: #284a39; }
  .account-bar label { font-weight: 500; }
  .account-bar .logout { margin: 0; }
  .workspace-nav { display: flex; gap: .25rem; overflow-x: auto; border-bottom: 1px solid #d6ded4; }
  .workspace-nav a { color: #4c6558; text-decoration: none; white-space: nowrap; padding: .95rem 1.2rem; font-weight: 650; border-bottom: 3px solid transparent; }
  .workspace-nav a:hover, .workspace-nav a:focus-visible, .workspace-nav a[aria-current='page'] { color: #185739; border-bottom-color: #397758; }
  .workspace-content { padding-block: 1.5rem 4rem; }
  .view[hidden], .workspace-content > section[hidden] { display: none; }
  .overview-header, .overview-next { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 1rem; }
  .overview-header p, .overview-next p { margin: .5rem 0 0; }
  .overview-header button.secondary { margin: 0; color: #155c3f; background: transparent; }
  .overview-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 1rem; margin-block: 1.75rem; }
  .overview-grid article { padding: 1.5rem; background: #f5f7f1; border: 1px solid #e1e8dd; border-radius: .65rem; }
  .overview-grid .overview-label { margin: 0; font-size: .72rem; font-weight: 700; letter-spacing: .1em; }
  .overview-grid strong { display: block; margin-top: .8rem; font-size: 2.5rem; line-height: 1.1; color: #234b37; font-variant-numeric: tabular-nums; }
  .overview-grid article p:not(.overview-label) { margin: .5rem 0 1.25rem; }
  .overview-grid a, .overview-action { color: #155c3f; font-weight: 700; text-decoration: none; }
  .overview-grid a:hover, .overview-action:hover { text-decoration: underline; }
  .overview-next { padding: 1.4rem 1.5rem; border: 1px solid #d8e6d7; background: #edf5eb; border-radius: .65rem; }
  .overview-next h3 { margin: 0; }
  @media (max-width: 760px) { .overview-grid { grid-template-columns: 1fr; } }
  .workspace-content :global(section[aria-labelledby]) { min-width: 0; margin: 0; padding: 1.75rem; border: 1px solid #e0e5dc; border-radius: .85rem; background: #fffefa; box-shadow: 0 6px 24px #304b3b0b; scroll-margin-top: 1rem; }
  .workspace-content :global(section[aria-labelledby='device-heading']) { grid-column: 1 / -1; }
  .workspace-content :global(h2) { margin: 0 0 .75rem; color: #263f33; font-size: 1.3rem; letter-spacing: -.02em; }
  .workspace-content :global(h3) { margin: 1rem 0 .45rem; color: #30483a; font-size: 1rem; }
  .workspace-content :global(p) { color: #52695c; }
  .workspace-content :global(form) { display: grid; gap: .55rem; margin-top: 1.5rem; padding: 1.2rem; border-radius: .6rem; background: #f6f7f2; }
  .workspace-content :global(label) { line-height: 1.5; color: #425c4e; font-size: .9rem; font-weight: 600; }
  .workspace-content :global(input:not([type='checkbox'])), .workspace-content :global(select), .account-bar select { width: 100%; min-height: 2.75rem; margin: 0; padding: .6rem .75rem; border: 1px solid #c4d0c3; border-radius: .4rem; background: white; color: #263f33; font: inherit; }
  .account-bar select { width: auto; }
  .workspace-content :global(input[type='checkbox']) { accent-color: #286c49; width: 1rem; height: 1rem; vertical-align: middle; }
  .workspace-content :global(button) { min-height: 2.75rem; padding: .65rem 1rem; border: 1px solid #286c49; border-radius: .4rem; background: #286c49; color: white; font: inherit; font-weight: 650; cursor: pointer; }
  .workspace-content :global(button:hover:not(:disabled)) { background: #1e563a; }
  .workspace-content :global(button:disabled) { opacity: .55; cursor: not-allowed; }
  .workspace-content :global(ul) { padding: 0; list-style: none; }
  .workspace-content :global(li) { overflow-wrap: anywhere; padding: .8rem 0; border-bottom: 1px solid #e5eae2; }
  .workspace-content :global([role='alert']) { color: #8b3027; }
  .workspace-content :global([role='status']) { color: #225c3e; }
  .workspace-content .actions button.secondary, .workspace-content button.danger { background: transparent; }
  .workspace-content .actions button.secondary { color: #155c3f; }
  .workspace-content button.danger { color: #7d261e; border-color: #a63428; }
  .workspace-content .actions button.secondary:hover:not(:disabled) { background: #edf4ed; }
  .workspace-content button.danger:hover:not(:disabled) { background: #fff1ef; }
  .panel:not(.dashboard) { border: 1px solid #e0e5dc; border-top: .3rem solid #286c49; border-radius: .75rem; background: #fffefa; box-shadow: 0 12px 40px #304b3b12; }
  .panel:not(.dashboard) .eyebrow { color: #557365; }
  .panel:not(.dashboard) h1 { color: #213b30; letter-spacing: -.025em; }
  .panel:not(.dashboard) input { min-height: 2.75rem; border-color: #c4d0c3; border-radius: .4rem; }
  .panel:not(.dashboard) button { min-height: 2.75rem; border-radius: .4rem; }
  @media (max-width: 760px) {
    .workspace-header, .workspace-nav, .workspace-content { width: min(100% - 2rem, 76rem); }
    .dashboard > .eyebrow { padding-inline: 1rem; font-size: .65rem; gap: .65rem; }
    .workspace-nav { flex-wrap: wrap; overflow: visible; }
    .workspace-nav a { padding: .75rem .8rem; }
    .workspace-header { padding-block: 2rem 1.5rem; }
    .workspace-content { grid-template-columns: 1fr; }
    .workspace-content :global(section[aria-labelledby='device-heading']) { grid-column: auto; }
    .workspace-content :global(section[aria-labelledby]) { padding: 1.25rem; }
  }
</style>
