<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import Banner from './Banner.svelte';
  import DisplayEditor from './DisplayEditor.svelte';
  let editing = $state(null);
  let { csrf, language = 'vi' } = $props();
  const t = (vi, en) => language === 'en' ? en : vi;
  let devices = $state([]), rooms = $state([]), loading = $state(true), name = $state(''), token = $state(''), copied = $state(false), error = $state(''), busy = $state(false);
  async function load() {
    error = '';
    try {
      const [d, r] = await Promise.all([fetch('/api/devices'), fetch('/api/rooms')]);
      if (!d.ok || !r.ok) throw new Error();
      devices = await d.json(); rooms = await r.json();
    } catch { error = ['Không thể tải thiết bị.', 'Could not load devices.']; }
    finally { loading = false; }
  }
  async function create(event) {
    event.preventDefault(); busy = true; error = ''; token = ''; copied = false;
    try {
      const response = await fetch('/api/devices', {method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':csrf}, body:JSON.stringify({name:name.trim()})});
      if (!response.ok) throw new Error();
      token = (await response.json()).token; name = ''; await load();
    } catch { error = ['Không thể tạo thiết bị.', 'Could not create device.']; }
    finally { busy = false; }
  }
  async function copyToken() {
    try {
      await navigator.clipboard?.writeText(token);
      copied = true;
    } catch {
      copied = false;
    }
  }
  async function setLanguage(device, event) {
    const select = event.currentTarget, language = select.value;
    busy = true; error = '';
    try {
      const response = await fetch(`/api/devices/${device.id}/language`, {method:'PUT', headers:{'Content-Type':'application/json','X-CSRF-Token':csrf}, body:JSON.stringify({language})});
      if (!response.ok) throw new Error();
      device.language = language;
      await load();
    } catch { select.value = device.language; error = ['Không thể cập nhật ngôn ngữ thiết bị.', 'Could not update device language.']; }
    finally { busy = false; }
  }
  async function revoke(device) {
    if (!window.confirm(t(`Thu hồi thiết bị ${device.name}?`, `Revoke device ${device.name}?`))) return;
    busy = true; error = '';
    try {
      const response = await fetch(`/api/devices/${device.id}/revoke`, {method:'POST',headers:{'X-CSRF-Token':csrf}});
      if (!response.ok) throw new Error();
      await load();
    } catch { error = ['Không thể thu hồi thiết bị.', 'Could not revoke device.']; }
    finally { busy = false; }
  }
  async function reactivate(device) {
    busy = true; error = '';
    try {
      const response = await fetch(`/api/devices/${device.id}/reactivate`, {method:'POST',headers:{'X-CSRF-Token':csrf}});
      if (!response.ok) throw new Error();
      await load();
    } catch { error = ['Không thể kích hoạt lại thiết bị.', 'Could not reactivate device.']; }
    finally { busy = false; }
  }
  onMount(() => {
    load();
    window.addEventListener('rooms-changed', load);
    return () => window.removeEventListener('rooms-changed', load);
  });
</script>

<section aria-labelledby="device-heading">
  <div class="section-head">
    <div>
      <p class="section-kicker">{t('PHẦN CỨNG & HIỂN THỊ', 'HARDWARE & DISPLAYS')}</p>
      <h2 id="device-heading">{t('Thiết bị màn hình', 'Display devices')}</h2>
      <p>{t('Cấu hình ngôn ngữ và thiết kế màn hình hiển thị nhiều phòng.', 'Configure language and design displays for multiple rooms.')}</p>
    </div>
  </div>

  {#if error}<Banner kind="error" text={t(...error)} />{/if}

  {#if token}
    <div class="token-callout">
      <p class="token-title" role="status">
        <Icon name="lock" />
        <span>{t('Mã thiết bị chỉ hiển thị một lần. Nhập mã trên màn hình phòng:', 'The device token is shown only once. Enter it on the room display:')}</span>
      </p>
      <div class="token-row">
        <output class="token-output" aria-label={t('Mã thiết bị', 'Device token')}>{token}</output>
        <div class="token-actions">
          <button class="secondary" type="button" onclick={copyToken}>
            {copied ? t('Đã sao chép', 'Copied') : t('Sao chép', 'Copy')}
          </button>
          <button class="secondary" type="button" onclick={() => { token = ''; copied = false; }}>{t('Ẩn mã', 'Hide token')}</button>
        </div>
      </div>
    </div>
  {/if}

  {#if editing}
    <DisplayEditor device={editing} {rooms} {csrf} {language} onclose={() => editing = null} />
  {/if}

  <div class="ui-split">
    <div>
      {#if loading}
        <p class="ui-loading" role="status">
          <span class="ui-spinner" aria-hidden="true"></span>
          <span>{t('Đang tải thiết bị…', 'Loading devices…')}</span>
        </p>
      {:else if devices.length === 0}
        <div class="ui-empty">
          <strong>{t('Chưa có thiết bị nào.', 'No devices registered yet.')}</strong>
          <p>{t('Đăng ký thiết bị mới để lấy mã ghép cho kiosk hoặc màn hình công khai.', 'Register a device to get a pairing code or public display URL.')}</p>
        </div>
      {:else}
        <ul class="ui-card-list">
          {#each devices as device (device.id)}
            <li class="ui-card device-card" class:is-revoked={device.revoked} class:is-editing={editing?.id === device.id}>
              <div class="device-info">
                <div class="device-title-row">
                  <strong>{device.name}</strong>
                  <span class={device.revoked ? 'ui-pill ui-pill--danger' : 'ui-pill ui-pill--success'}>
                    {device.revoked ? t('Đã thu hồi', 'Revoked') : t('Hoạt động', 'Active')}
                  </span>
                </div>
                <p class="device-seen">
                  <Icon name="clock" />
                  <span>{device.last_seen ? new Date(device.last_seen * 1000).toLocaleString(language === 'en' ? 'en-US' : 'vi-VN') : t('Chưa kết nối', 'Not connected')}</span>
                </p>
              </div>

              <div class="device-controls">
                {#if !device.revoked}
                  <button class="secondary" type="button" disabled={busy || editing !== null} onclick={() => editing = device}>
                    <Icon name="layout" />
                    <span>{t('Thiết kế màn hình', 'Design display')}</span>
                  </button>

                  <label class="inline-lang">
                    <span>{t('Ngôn ngữ', 'Language')} <span class="sr-only">{device.name}</span></span>
                    <select value={device.language} disabled={busy} onchange={(event) => setLanguage(device, event)}>
                      <option value="vi">Tiếng Việt</option>
                      <option value="en">English</option>
                    </select>
                  </label>

                  <a class="display-link" href={`/display/${device.public_id}`} target="_blank" rel="noopener noreferrer">
                    <span>{t('Mở màn hình', 'Open display')}</span>
                    <Icon name="external" />
                  </a>

                  <button class="danger" type="button" disabled={busy} aria-label={`${t('Thu hồi', 'Revoke')} ${device.name}`} onclick={() => revoke(device)}>
                    {t('Thu hồi', 'Revoke')}
                  </button>
                {:else}
                  <button class="secondary" type="button" disabled={busy} aria-label={`${t('Kích hoạt lại', 'Reactivate')} ${device.name}`} onclick={() => reactivate(device)}>
                    {t('Kích hoạt lại', 'Reactivate')}
                  </button>
                {/if}
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <div class="ui-sticky-pane">
      <form onsubmit={create}>
        <h3>{t('Đăng ký thiết bị', 'Register device')}</h3>
        <label for="device-name">{t('Tên thiết bị', 'Device name')}</label>
        <input id="device-name" bind:value={name} maxlength="128" required placeholder={t('Ví dụ: iPad Cửa Phòng A', 'e.g. Lobby Tablet 01')} />
        <button type="submit" disabled={busy}>{t('Tạo thiết bị', 'Create device')}</button>
      </form>
    </div>
  </div>
</section>

<style>
  .token-callout {
    margin-top: 1.25rem;
    padding: 1.1rem 1.25rem;
    border: 1px solid var(--accent-border);
    border-left: 4px solid var(--accent);
    border-radius: var(--radius-md);
    background: var(--accent-soft);
  }

  .token-title {
    margin: 0 0 0.75rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-weight: 650;
    color: #134e31;
  }

  .token-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.75rem;
  }

  .token-output {
    flex: 1 1 16rem;
    padding: 0.7rem 0.9rem;
    border-radius: var(--radius-sm);
    border: 1px solid var(--accent-border);
    background: #ffffff;
    font-family: var(--font-mono);
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--ink-primary);
    overflow-wrap: anywhere;
    user-select: all;
  }

  .token-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .device-card {
    display: grid;
    gap: 0.9rem;
  }

  .device-card.is-revoked {
    opacity: 0.78;
    background: var(--bg-subtle);
  }

  .device-title-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem;
  }

  .device-title-row strong {
    font-size: 1.05rem;
    color: var(--ink-primary);
  }

  .device-seen {
    margin: 0.4rem 0 0;
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.85rem;
    color: var(--ink-muted);
    font-variant-numeric: tabular-nums;
  }

  .device-controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.55rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--border-subtle);
  }

  .inline-lang {
    display: inline-flex !important;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.85rem;
  }

  .inline-lang select {
    width: auto !important;
    min-width: 7.5rem;
  }

  .display-link {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    min-height: 2.75rem;
    padding: 0.55rem 0.95rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: #ffffff;
    color: var(--accent);
    font-size: 0.9rem;
    font-weight: 650;
    text-decoration: none;
    transition: background var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
  }

  .display-link:hover {
    background: var(--bg-subtle);
    border-color: var(--accent);
  }

  .device-controls .danger {
    margin-left: auto;
  }

  @media (max-width: 540px) {
    .device-controls .danger {
      margin-left: 0;
    }
  }
</style>
