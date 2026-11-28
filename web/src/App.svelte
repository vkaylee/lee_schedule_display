<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import Banner from './Banner.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import Modal from './Modal.svelte';
  let confirmDialog;
  import Rooms from './Rooms.svelte';
  import Bookings from './Bookings.svelte';
  import Devices from './Devices.svelte';
  import PublicDisplay from './PublicDisplay.svelte';
  const displayMatch = window.location.pathname.match(/^\/display\/([0-9a-f]{32})$/);

  let t;
  $: t = session?.language === 'en' ? (_vi, en) => en : (vi, _en) => vi;
  let username = '';
  let password = '';
  let showPassword = false;
  let session = null;
  let loading = true;
  let busy = false;
  let error = '';
  let users = [];
  let usersLoading = false;
  let usersError = '';
  let notice = '';
  let createUserModalOpen = false;
  let createUserError = '';
  let newUsername = '';
  let newPassword = '';
  let showNewPassword = false;
  let resetUser = null;
  let resetPassword = '';
  let showResetPassword = false;
  let resetPasswordError = '';
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

  function clearCreateUser() {
    createUserModalOpen = false;
    createUserError = '';
    newUsername = '';
    newPassword = '';
    showNewPassword = false;
  }

  function openCreateUser() {
    clearCreateUser();
    notice = '';
    createUserModalOpen = true;
  }

  function closeCreateUserModal() {
    if (busy) return;
    clearCreateUser();
  }

  function clearReset() {
    resetUser = null;
    resetPassword = '';
    showResetPassword = false;
    resetPasswordError = '';
  }

  function openResetPassword(user) {
    clearReset();
    notice = '';
    resetUser = user;
  }

  function cancelReset() {
    if (busy) return;
    clearReset();
  }

  async function updateUser(path, body, onError = (msg) => usersError = msg) {
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
        onError(['Không thể vô hiệu hóa tài khoản hoạt động cuối cùng.', 'Cannot disable the last active account.']);
        return false;
      }
      if (response.status !== 201 && response.status !== 204) throw new Error();
      return true;
    } catch {
      onError(['Không thể thực hiện thao tác. Vui lòng thử lại.', 'Could not complete the operation. Please try again.']);
      return false;
    } finally {
      busy = false;
    }
  }

  async function createUser(event) {
    event.preventDefault();
    if (!newUsername.trim() || !newPassword) return;
    createUserError = '';
    if (await updateUser('/api/users', { username: newUsername.trim(), password: newPassword }, (msg) => createUserError = msg)) {
      clearCreateUser();
      notice = ['Đã tạo tài khoản.', 'Account created.'];
      await loadUsers();
    }
  }

  async function disableUser(user) {
    if (!await confirmDialog.ask(t(`Vô hiệu hóa tài khoản "${user.username}"? Tài khoản này sẽ không thể đăng nhập.`, `Disable account "${user.username}"? This account will no longer be able to sign in.`))) return;
    if (await updateUser(`/api/users/${encodeURIComponent(user.id)}/disable`)) {
      if (user.username === session.username) {
        await logout();
        return;
      }
      if (resetUser?.id === user.id) clearReset();
      notice = ['Đã vô hiệu hóa tài khoản.', 'Account disabled.'];
      await loadUsers();
    }
  }

  async function changePassword(event) {
    event.preventDefault();
    if (!resetUser || !resetPassword) return;
    resetPasswordError = '';
    if (await updateUser(`/api/users/${encodeURIComponent(resetUser.id)}/password`, { password: resetPassword }, (msg) => resetPasswordError = msg)) {
      if (resetUser.username === session.username) {
        await logout();
        return;
      }
      clearReset();
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
        showPassword = false;
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
        clearCreateUser();
        clearReset();
        username = '';
        password = '';
        showPassword = false;
      } else {
        error = ['Không thể đăng xuất. Vui lòng thử lại.', 'Could not log out. Please try again.'];
      }
    } catch {
      error = ['Không thể kết nối máy chủ. Vui lòng thử lại.', 'Could not connect to the server. Please try again.'];
    } finally {
      busy = false;
    }
  }

  if (!displayMatch) onMount(loadSession);
</script>

<ConfirmDialog bind:this={confirmDialog} language={session?.language || 'vi'} />

<svelte:head><title>{t('Quản lý lịch phòng họp', 'Meeting room schedule management')}</title></svelte:head>

{#if displayMatch}
  <PublicDisplay id={displayMatch[1]} />
{:else if !session}
  <main class="ui-auth-page">
    <div class="ui-auth-card">
      <div class="ui-auth-top">
        <Icon name="brand" class="brand-mark" />
        <span>SCHEDULE DISPLAY</span>
      </div>
      <div class="ui-auth-body">
        {#if loading}
          <h1>{t('Quản lý lịch phòng họp', 'Meeting room schedule management')}</h1>
          <p class="ui-loading" role="status">
            <span class="ui-spinner" aria-hidden="true"></span>
            <span>{t('Đang kiểm tra phiên đăng nhập…', 'Checking session…')}</span>
          </p>
        {:else}
          <h1>{t('Đăng nhập quản trị', 'Administrator login')}</h1>
          <p class="ui-auth-subtitle">{t('Đăng nhập để quản lý lịch phòng họp.', 'Sign in to manage room schedules.')}</p>
          {#if error}<Banner kind="error" text={t(...error)} />{/if}
          <form onsubmit={login}>
            <label for="username">{t('Tên đăng nhập', 'Username')}</label>
            <input id="username" name="username" autocomplete="username" bind:value={username} required maxlength="128" />
            <label for="password">{t('Mật khẩu', 'Password')}</label>
            <div class="ui-pw-wrap">
              <input id="password" name="password" type={showPassword ? 'text' : 'password'} autocomplete="current-password" bind:value={password} required maxlength="1024" />
              <button type="button" class="ui-pw-toggle" aria-label={showPassword ? t('Ẩn mật khẩu', 'Hide password') : t('Hiện mật khẩu', 'Show password')} aria-pressed={showPassword} onclick={() => showPassword = !showPassword}>
                <Icon name={showPassword ? 'eye-off' : 'eye'} />
              </button>
            </div>
            <button type="submit" disabled={busy}>{busy ? t('Đang đăng nhập…', 'Signing in…') : t('Đăng nhập', 'Sign in')}</button>
          </form>
          {#if error}
            <button class="ui-auth-retry" type="button" onclick={loadSession}>{t('Kiểm tra kết nối', 'Check connection')}</button>
          {/if}
        {/if}
      </div>
    </div>
  </main>
{:else}
  <main class="dashboard">
    <div class="topbar">
      <div class="topbar-inner">
        <p class="eyebrow">
          <Icon name="brand" class="brand-mark" />
          <span>SCHEDULE DISPLAY</span>
          <span class="eyebrow-line" aria-hidden="true"></span>
          <span class="eyebrow-sub">{t('KHÔNG GIAN LÀM VIỆC', 'WORKSPACE')}</span>
        </p>
        <div class="account-bar">
          <span class="account-chip">
            <Icon name="user" />
            <span class="account-name">{session.username}</span>
          </span>
          <label class="sr-only" for="account-language">Ngôn ngữ / Language</label>
          <select id="account-language" value={session.language} onchange={changeLanguage} disabled={busy} aria-label="Ngôn ngữ / Language">
            <option value="vi">Tiếng Việt</option>
            <option value="en">English</option>
          </select>
          <button class="topbar-logout" type="button" onclick={logout} disabled={busy}>
            <Icon name="logout" />
            <span>{t('Đăng xuất', 'Log out')}</span>
          </button>
        </div>
      </div>
    </div>

    <header class="workspace-header">
      <div>
        <p class="section-kicker">{t('QUẢN LÝ KHÔNG GIAN', 'SPACE MANAGEMENT')}</p>
        <h1>{t('Lịch phòng họp', 'Meeting rooms')}</h1>
        <p class="intro">{t('Mọi cuộc họp, mọi không gian — trong tầm tay.', 'Your meetings and spaces, all in one place.')}</p>
      </div>
      {#if error}<Banner kind="error" class="header-alert" text={t(...error)} />{/if}
    </header>

    <nav class="workspace-nav" aria-label={t('Điều hướng quản trị', 'Workspace navigation')}>
      <a href="#overview" aria-current={activeView === 'overview' ? 'page' : undefined}><Icon name="overview" /><span>{t('Tổng quan', 'Overview')}</span></a>
      <a href="#rooms" aria-current={activeView === 'rooms' ? 'page' : undefined}><Icon name="rooms" /><span>{t('Phòng', 'Rooms')}</span></a>
      <a href="#bookings" aria-current={activeView === 'bookings' ? 'page' : undefined}><Icon name="calendar" /><span>{t('Lịch phòng', 'Bookings')}</span></a>
      <a href="#devices" aria-current={activeView === 'devices' ? 'page' : undefined}><Icon name="monitor" /><span>{t('Thiết bị', 'Devices')}</span></a>
      <a href="#users" aria-current={activeView === 'users' ? 'page' : undefined}><Icon name="users" /><span>{t('Tài khoản', 'Accounts')}</span></a>
    </nav>

    <div class="workspace-content">
      {#if activeView === 'overview'}
        <section class="overview" aria-labelledby="overview-heading">
          <div class="section-head">
            <div>
              <p class="section-kicker">{t('KHÔNG GIAN CỦA BẠN', 'YOUR WORKSPACE')}</p>
              <h2 id="overview-heading">{t('Tổng quan', 'Overview')}</h2>
              <p>{t('Theo dõi phòng, lịch và thiết bị từ một nơi.', 'Keep track of rooms, bookings, and devices in one place.')}</p>
            </div>
            <button class="secondary" type="button" onclick={loadOverview} disabled={overviewLoading}>
              <Icon name="refresh" class={overviewLoading ? 'spin' : ''} />
              <span>{t('Làm mới', 'Refresh')}</span>
            </button>
          </div>

          {#if overviewLoading && !overview}
            <p role="status" class="ui-loading overview-status">
              <span class="ui-spinner" aria-hidden="true"></span>
              <span>{t('Đang tải tổng quan…', 'Loading overview…')}</span>
            </p>
            <div class="overview-grid" aria-hidden="true">
              <div class="skeleton-card"></div>
              <div class="skeleton-card"></div>
              <div class="skeleton-card"></div>
            </div>
          {:else if overviewLoading}
            <p role="status" class="sr-only">{t('Đang tải tổng quan…', 'Loading overview…')}</p>
          {/if}

          {#if overviewError}
            <Banner kind="error" text={t('Không thể tải tổng quan. Vui lòng thử lại.', 'Could not load the overview. Please try again.')} />
          {/if}

          {#if overview && !overviewError}
            <div class="overview-grid">
              <article class="kpi-card">
                <div class="kpi-top">
                  <p class="overview-label">{t('PHÒNG HOẠT ĐỘNG', 'ACTIVE ROOMS')}</p>
                  <span class="kpi-icon" aria-hidden="true"><Icon name="rooms" /></span>
                </div>
                <strong>{overview.rooms.filter(room => room.active).length}</strong>
                <p>{t('Phòng sẵn sàng quản lý', 'Rooms available to manage')}</p>
                <a href="#rooms">{t('Quản lý phòng', 'Manage rooms')} <span aria-hidden="true">→</span></a>
              </article>

              <article class="kpi-card">
                <div class="kpi-top">
                  <p class="overview-label">{t('LỊCH SẮP TỚI', 'UPCOMING BOOKINGS')}</p>
                  <span class="kpi-icon" aria-hidden="true"><Icon name="calendar" /></span>
                </div>
                <strong>{overview.bookings.filter(booking => !booking.canceled && booking.starts * 1000 > Date.now()).length}</strong>
                <p>{t('Lịch chưa bắt đầu', 'Bookings yet to start')}</p>
                <a href="#bookings">{t('Xem lịch phòng', 'View bookings')} <span aria-hidden="true">→</span></a>
              </article>

              <article class="kpi-card">
                <div class="kpi-top">
                  <p class="overview-label">{t('THIẾT BỊ HOẠT ĐỘNG', 'ACTIVE DEVICES')}</p>
                  <span class="kpi-icon" aria-hidden="true"><Icon name="monitor" /></span>
                </div>
                <strong>{overview.devices.filter(device => !device.revoked).length}</strong>
                <p>{t('Thiết bị chưa bị thu hồi', 'Devices not revoked')}</p>
                <a href="#devices">{t('Quản lý thiết bị', 'Manage devices')} <span aria-hidden="true">→</span></a>
              </article>
            </div>

            <div class="overview-next">
              <div>
                <h3>{t('Bắt đầu từ đâu?', 'Where to start?')}</h3>
                <p>{overview.rooms.some(room => room.active) ? t('Tạo lịch cho phòng hoặc thiết lập màn hình hiển thị.', 'Create a booking or set up a room display.') : t('Tạo phòng đầu tiên, sau đó thêm lịch và thiết bị hiển thị.', 'Create your first room, then add bookings and displays.')}</p>
              </div>
              <a class="overview-action" href={overview.rooms.some(room => room.active) ? '#bookings' : '#rooms'}>
                <span>{overview.rooms.some(room => room.active) ? t('Tạo lịch', 'Create booking') : t('Tạo phòng', 'Create room')}</span>
                <span aria-hidden="true">→</span>
              </a>
            </div>
          {/if}
        </section>
      {/if}

      <div class="view" hidden={activeView !== 'rooms'}><Rooms csrf={session.csrf_token} language={session.language} /></div>
      <div class="view" hidden={activeView !== 'bookings'}><Bookings csrf={session.csrf_token} language={session.language} /></div>
      <div class="view" hidden={activeView !== 'devices'}><Devices csrf={session.csrf_token} language={session.language} /></div>

      <section hidden={activeView !== 'users'} aria-labelledby="users-heading">
        <div class="section-head">
          <div>
            <p class="section-kicker">{t('TRUY CẬP HỆ THỐNG', 'ACCESS CONTROL')}</p>
            <h2 id="users-heading">{t('Quản lý tài khoản', 'Account management')}</h2>
            <p>{t('Quản lý quyền truy cập và mật khẩu đăng nhập.', 'Manage access and login passwords.')}</p>
          </div>
          <div class="section-actions">
            <button type="button" disabled={busy} onclick={openCreateUser}>
              <Icon name="plus" />
              <span>{t('Tạo tài khoản', 'Create account')}</span>
            </button>
          </div>
        </div>

        {#if usersError}<Banner kind="error" text={t(...usersError)} />{/if}
        {#if notice}<Banner kind="success" text={t(...notice)} />{/if}

        <div class="ui-view-body">
          <h3>{t('Danh sách tài khoản', 'Accounts')}</h3>
          {#if usersLoading}
            <p role="status" class="ui-loading">
              <span class="ui-spinner" aria-hidden="true"></span>
              <span>{t('Đang tải danh sách tài khoản…', 'Loading accounts…')}</span>
            </p>
          {:else if usersError && users.length === 0}
            <button class="secondary" type="button" onclick={loadUsers}>{t('Thử tải lại', 'Retry')}</button>
          {:else if users.length === 0}
            <div class="ui-empty"><p>{t('Chưa có tài khoản nào.', 'No accounts yet.')}</p></div>
          {:else}
            <ul class="ui-card-list">
              {#each users as user (user.id)}
                <li class="ui-card user-card" class:is-editing={resetUser?.id === user.id}>
                  <div class="user-identity">
                    <span class="user-avatar" aria-hidden="true">{user.username.slice(0, 1).toUpperCase()}</span>
                    <div>
                      <div class="user-name-row">
                        <strong>{user.username}</strong>
                        {#if user.username === session.username}
                          <span class="ui-pill ui-pill--neutral">{t('Bạn', 'You')}</span>
                        {/if}
                      </div>
                      <span class={user.active ? 'ui-pill ui-pill--success' : 'ui-pill ui-pill--neutral'}>
                        {user.active ? t('Đang hoạt động', 'Active') : t('Đã vô hiệu hóa', 'Disabled')}
                      </span>
                    </div>
                  </div>
                  {#if user.active}
                    <div class="actions">
                      <button class="secondary" type="button" disabled={busy} aria-label={`${t('Đặt lại mật khẩu cho', 'Reset password for')} ${user.username}`} onclick={() => openResetPassword(user)}>{t('Đặt lại mật khẩu', 'Reset password')}</button>
                      <button class="danger" type="button" disabled={busy} aria-label={`${t('Vô hiệu hóa', 'Disable')} ${user.username}`} onclick={() => disableUser(user)}>{t('Vô hiệu hóa', 'Disable')}</button>
                    </div>
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
        </div>

        <Modal
          open={createUserModalOpen}
          title={t('Tạo tài khoản', 'Create account')}
          {busy}
          closeLabel={t('Đóng', 'Close')}
          onclose={closeCreateUserModal}
        >
          {#if createUserError}<Banner kind="error" text={t(...createUserError)} />{/if}
          <form onsubmit={createUser}>
            <label for="new-username">{t('Tên đăng nhập', 'Username')}</label>
            <input id="new-username" name="new-username" autocomplete="off" bind:value={newUsername} required maxlength="128" />
            <label for="new-password">{t('Mật khẩu', 'Password')}</label>
            <div class="ui-pw-wrap">
              <input id="new-password" name="new-password" type={showNewPassword ? 'text' : 'password'} autocomplete="new-password" bind:value={newPassword} required maxlength="1024" />
              <button type="button" class="ui-pw-toggle" aria-label={showNewPassword ? t('Ẩn mật khẩu', 'Hide password') : t('Hiện mật khẩu', 'Show password')} aria-pressed={showNewPassword} onclick={() => showNewPassword = !showNewPassword}>
                <Icon name={showNewPassword ? 'eye-off' : 'eye'} />
              </button>
            </div>
            <div class="ui-form-actions">
              <button type="submit" disabled={busy}>{t('Tạo tài khoản', 'Create account')}</button>
              <button class="secondary" type="button" disabled={busy} onclick={closeCreateUserModal}>{t('Hủy', 'Cancel')}</button>
            </div>
          </form>
        </Modal>

        <Modal
          open={resetUser !== null}
          title={resetUser ? `${t('Đặt lại mật khẩu cho', 'Reset password for')} ${resetUser.username}` : ''}
          {busy}
          closeLabel={t('Đóng', 'Close')}
          onclose={cancelReset}
        >
          {#if resetPasswordError}<Banner kind="error" text={t(...resetPasswordError)} />{/if}
          <form onsubmit={changePassword}>
            <label for="reset-password">{t('Mật khẩu mới', 'New password')}</label>
            <div class="ui-pw-wrap">
              <input id="reset-password" name="new-password" type={showResetPassword ? 'text' : 'password'} autocomplete="new-password" bind:value={resetPassword} required maxlength="1024" />
              <button type="button" class="ui-pw-toggle" aria-label={showResetPassword ? t('Ẩn mật khẩu', 'Hide password') : t('Hiện mật khẩu', 'Show password')} aria-pressed={showResetPassword} onclick={() => showResetPassword = !showResetPassword}>
                <Icon name={showResetPassword ? 'eye-off' : 'eye'} />
              </button>
            </div>
            <div class="ui-form-actions">
              <button type="submit" disabled={busy}>{t('Lưu mật khẩu', 'Save password')}</button>
              <button class="secondary" type="button" disabled={busy} onclick={cancelReset}>{t('Hủy', 'Cancel')}</button>
            </div>
          </form>
        </Modal>
      </section>
    </div>
  </main>
{/if}

<style>
  :global(.brand-mark) {
    width: 1.25rem;
    height: 1.25rem;
    color: #82d4a5;
  }

  .dashboard {
    width: 100%;
    min-height: 100dvh;
    background: var(--bg-canvas);
  }

  .topbar {
    background: var(--bg-header);
    color: var(--ink-on-header);
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  }

  .topbar-inner {
    width: min(100% - 3rem, 78rem);
    margin-inline: auto;
    min-height: 3.75rem;
    padding-block: 0.5rem;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.85rem;
  }

  .eyebrow {
    margin: 0;
    display: flex;
    align-items: center;
    gap: 0.65rem;
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.12em;
    color: var(--ink-on-header);
  }

  .eyebrow-line {
    width: 1.25rem;
    height: 1px;
    background: rgba(237, 243, 238, 0.35);
  }

  .eyebrow-sub {
    color: rgba(237, 243, 238, 0.82);
  }

  .account-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.6rem;
    font-size: 0.85rem;
  }

  .account-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.35rem 0.75rem;
    border-radius: var(--radius-pill);
    background: rgba(255, 255, 255, 0.1);
    color: var(--ink-on-header);
    font-weight: 600;
  }

  .account-bar select {
    min-height: 2.35rem;
    padding: 0.3rem 0.65rem;
    border: 1px solid rgba(255, 255, 255, 0.24);
    border-radius: var(--radius-sm);
    background: rgba(255, 255, 255, 0.08);
    color: var(--ink-on-header);
    font: inherit;
    font-weight: 500;
    cursor: pointer;
  }

  .account-bar select option {
    background: var(--bg-surface);
    color: var(--ink-primary);
  }

  .topbar-logout {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    min-height: 2.35rem;
    padding: 0.35rem 0.8rem;
    border: 1px solid rgba(255, 255, 255, 0.26);
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--ink-on-header);
    font: inherit;
    font-weight: 600;
    cursor: pointer;
    transition: background var(--duration-fast) var(--ease-out);
  }

  .topbar-logout:hover:not(:disabled) {
    background: var(--bg-header-hover);
  }

  .workspace-header,
  .workspace-nav,
  .workspace-content {
    width: min(100% - 3rem, 78rem);
    margin-inline: auto;
  }

  .workspace-header {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    justify-content: space-between;
    gap: 1.25rem;
    padding-block: 2.25rem 1.5rem;
  }

  .workspace-header h1 {
    margin: 0;
    font-size: clamp(1.75rem, 3.2vw, 2.5rem);
    letter-spacing: -0.03em;
    line-height: 1.15;
    color: var(--ink-primary);
  }

  .intro {
    margin: 0.45rem 0 0;
    color: var(--ink-muted);
    font-size: 0.98rem;
  }

  :global(.header-alert) {
    margin: 0 !important;
    max-width: 30rem;
  }

  .workspace-nav {
    display: flex;
    gap: 0.35rem;
    overflow-x: auto;
    border-bottom: 1px solid var(--border-subtle);
  }

  .workspace-nav a {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    color: var(--ink-muted);
    text-decoration: none;
    white-space: nowrap;
    padding: 0.85rem 1.1rem;
    min-height: 2.75rem;
    font-size: 0.94rem;
    font-weight: 650;
    border-bottom: 3px solid transparent;
    border-radius: var(--radius-sm) var(--radius-sm) 0 0;
    transition: color var(--duration-fast) var(--ease-out), background var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
  }

  .workspace-nav a:hover {
    color: var(--ink-primary);
    background: rgba(27, 99, 64, 0.05);
  }

  .workspace-nav a[aria-current='page'] {
    color: var(--accent);
    border-bottom-color: var(--accent);
    background: rgba(255, 254, 250, 0.65);
  }

  .workspace-content {
    padding-block: 1.5rem 4rem;
  }

  .view[hidden],
  .workspace-content > section[hidden] {
    display: none;
  }

  .overview-status {
    margin-top: 1.25rem;
  }

  .overview-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 1rem;
    margin-block: 1.5rem;
  }

  .kpi-card {
    display: flex;
    flex-direction: column;
    padding: 1.35rem 1.4rem;
    background: var(--bg-subtle);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    transition: transform var(--duration-fast) var(--ease-out), box-shadow var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
  }

  .kpi-card:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-sm);
    border-color: var(--accent-border);
  }

  .kpi-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }

  .overview-label {
    margin: 0;
    font-size: 0.72rem;
    font-weight: 700;
    letter-spacing: 0.1em;
    color: var(--ink-muted);
  }

  .kpi-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 2.1rem;
    height: 2.1rem;
    border-radius: var(--radius-sm);
    background: var(--accent-soft);
    color: var(--accent);
  }

  .kpi-card strong {
    display: block;
    margin-top: 0.75rem;
    font-size: 2.6rem;
    font-weight: 750;
    line-height: 1.05;
    letter-spacing: -0.03em;
    color: var(--ink-primary);
    font-variant-numeric: tabular-nums;
  }

  .kpi-card p:not(.overview-label) {
    margin: 0.45rem 0 1.15rem;
    font-size: 0.9rem;
    color: var(--ink-muted);
  }

  .kpi-card a {
    margin-top: auto;
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    color: var(--accent);
    font-size: 0.92rem;
    font-weight: 700;
    text-decoration: none;
  }

  .kpi-card a:hover {
    text-decoration: underline;
  }

  .skeleton-card {
    height: 11.5rem;
    border-radius: var(--radius-md);
    background: linear-gradient(90deg, var(--bg-subtle) 25%, var(--bg-muted) 50%, var(--bg-subtle) 75%);
    background-size: 200% 100%;
    animation: shimmer 1.4s ease-in-out infinite;
  }

  @keyframes shimmer {
    0% { background-position: 200% 0; }
    100% { background-position: -200% 0; }
  }

  .overview-next {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 1.35rem 1.5rem;
    border: 1px solid var(--accent-border);
    background: var(--accent-soft);
    border-radius: var(--radius-md);
  }

  .overview-next h3,
  .overview-next p {
    margin: 0;
  }

  .overview-next p {
    margin-top: 0.25rem;
  }

  .overview-action {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    min-height: 2.75rem;
    padding: 0.6rem 1.15rem;
    border-radius: var(--radius-sm);
    background: var(--accent);
    color: var(--ink-on-accent);
    font-weight: 650;
    text-decoration: none;
    transition: background var(--duration-fast) var(--ease-out);
  }

  .overview-action:hover {
    background: var(--accent-hover);
  }

  .user-card {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.85rem;
  }

  .user-identity {
    display: flex;
    align-items: center;
    gap: 0.85rem;
    min-width: 0;
  }

  .user-avatar {
    width: 2.5rem;
    height: 2.5rem;
    border-radius: 50%;
    display: grid;
    place-items: center;
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 750;
    font-size: 1rem;
    flex-shrink: 0;
  }

  .user-name-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.45rem;
    margin-bottom: 0.25rem;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  @media (max-width: 900px) {
    .overview-grid {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 760px) {
    .topbar-inner,
    .workspace-header,
    .workspace-nav,
    .workspace-content {
      width: min(100% - 1.5rem, 78rem);
    }
    .workspace-header {
      padding-block: 1.5rem 1rem;
    }
    .workspace-nav a {
      padding: 0.7rem 0.85rem;
      font-size: 0.88rem;
    }
  }
</style>
