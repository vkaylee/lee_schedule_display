<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import Banner from './Banner.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import Modal from './Modal.svelte';
  import DisplayEditor from './DisplayEditor.svelte';
  let confirmDialog = $state();
  let editing = $state(null);
  let { csrf, language = 'vi' } = $props();
  const t = (vi, en) => language === 'en' ? en : vi;
  let devices = $state([]), rooms = $state([]), loading = $state(true), name = $state(''), error = $state(''), busy = $state(false);
  let createModalOpen = $state(false), createError = $state(''), cloneSource = $state(null);
  async function load() {
    error = '';
    try {
      const [d, r] = await Promise.all([fetch('/api/devices'), fetch('/api/rooms')]);
      if (!d.ok || !r.ok) throw new Error();
      devices = await d.json(); rooms = await r.json();
    } catch { error = ['Không thể tải thiết bị.', 'Could not load devices.']; }
    finally { loading = false; }
  }
  function openCreate() {
    cloneSource = null;
    name = '';
    createError = '';
    createModalOpen = true;
  }
  function openClone(device) {
    cloneSource = device;
    const suffix = t(' (Bản sao)', ' (Copy)');
    const encoder = new TextEncoder();
    name = device.name;
    while (encoder.encode(name + suffix).length > 128) name = Array.from(name).slice(0, -1).join('');
    name = name.trimEnd() + suffix;
    createError = '';
    createModalOpen = true;
  }
  function closeCreateModal() {
    if (busy) return;
    createModalOpen = false;
    createError = '';
    name = '';
  }
  async function create(event) {
    event.preventDefault(); busy = true; createError = '';
    try {
      if (!name.trim()) {
        createError = ['Nhập tên thiết bị.', 'Enter a device name.'];
        return;
      }
      if (new TextEncoder().encode(name.trim()).length > 128) {
        createError = ['Tên thiết bị không được vượt quá 128 byte.', 'Device name must not exceed 128 bytes.'];
        return;
      }
      const response = await fetch(cloneSource ? `/api/devices/${cloneSource.id}/clone` : '/api/devices', {method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':csrf}, body:JSON.stringify({name:name.trim()})});
      if (!response.ok) throw new Error();
      createModalOpen = false; name = ''; await load();
    } catch { createError = cloneSource ? ['Không thể nhân bản thiết bị. Kiểm tra thiết bị nguồn và thử lại.', 'Could not clone device. Check the source device and try again.'] : ['Không thể tạo thiết bị.', 'Could not create device.']; }
    finally { busy = false; }
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
    if (!await confirmDialog.ask(t(`Thu hồi thiết bị ${device.name}?`, `Revoke device ${device.name}?`))) return;
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

<ConfirmDialog bind:this={confirmDialog} {language} />

<section aria-labelledby="device-heading">
  <div class="section-head">
    <div>
      <p class="section-kicker">{t('PHẦN CỨNG & HIỂN THỊ', 'HARDWARE & DISPLAYS')}</p>
      <h2 id="device-heading">{t('Thiết bị màn hình', 'Display devices')}</h2>
      <p>{t('Cấu hình ngôn ngữ và thiết kế màn hình hiển thị nhiều phòng.', 'Configure language and design displays for multiple rooms.')}</p>
    </div>
    <div class="section-actions">
      <button type="button" disabled={busy} onclick={openCreate}>
        <Icon name="plus" />
        <span>{t('Đăng ký thiết bị', 'Register device')}</span>
      </button>
    </div>
  </div>

  {#if error}<Banner kind="error" text={t(...error)} />{/if}

  {#if editing}
    <DisplayEditor device={editing} {rooms} {csrf} {language} onclose={() => editing = null} />
  {/if}

  <div class="ui-view-body">
    {#if loading}
      <p class="ui-loading" role="status">
        <span class="ui-spinner" aria-hidden="true"></span>
        <span>{t('Đang tải thiết bị…', 'Loading devices…')}</span>
      </p>
    {:else if devices.length === 0}
      <div class="ui-empty">
        <strong>{t('Chưa có thiết bị nào.', 'No devices registered yet.')}</strong>
        <p>{t('Bấm "Đăng ký thiết bị" để tạo màn hình công khai.', 'Click "Register device" to create a public display.')}</p>
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
            </div>

            <div class="device-controls">
              {#if !device.revoked}
                <button class="secondary" type="button" disabled={busy || editing !== null} onclick={() => editing = device}>
                  <Icon name="layout" />
                  <span>{t('Thiết kế màn hình', 'Design display')}</span>
                </button>

                <button class="secondary" type="button" disabled={busy || editing !== null} aria-label={`${t('Nhân bản', 'Clone')} ${device.name}`} onclick={() => openClone(device)}>
                  {t('Nhân bản', 'Clone')}
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

  <Modal
    open={createModalOpen}
    title={cloneSource ? t('Nhân bản màn hình', 'Clone display') : t('Đăng ký thiết bị', 'Register device')}
    {busy}
    closeLabel={t('Đóng', 'Close')}
    onclose={closeCreateModal}
  >
    {#if createError}<Banner kind="error" text={t(...createError)} />{/if}
    <form onsubmit={create}>
      <label for="device-name">{t('Tên thiết bị', 'Device name')}</label>
      <input id="device-name" bind:value={name} maxlength="128" required placeholder={t('Ví dụ: iPad Cửa Phòng A', 'e.g. Lobby Tablet 01')} />
      <div class="ui-form-actions">
        <button type="submit" disabled={busy}>{cloneSource ? t('Nhân bản', 'Clone') : t('Tạo thiết bị', 'Create device')}</button>
        <button class="secondary" type="button" disabled={busy} onclick={closeCreateModal}>{t('Hủy', 'Cancel')}</button>
      </div>
    </form>
  </Modal>
</section>

<style>
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
