<script>
  import { onMount } from 'svelte';
  import DisplayEditor from './DisplayEditor.svelte';
  let editing = $state(null);
  let { csrf, language = 'vi' } = $props();
  const t = (vi, en) => language === 'en' ? en : vi;
  let devices = $state([]), rooms = $state([]), name = $state(''), roomId = $state(''), token = $state(''), error = $state(''), busy = $state(false);
  async function load() {
    try {
      const [d, r] = await Promise.all([fetch('/api/devices'), fetch('/api/rooms')]);
      if (!d.ok || !r.ok) throw new Error();
      devices = await d.json(); rooms = await r.json();
    } catch { error = ['Không thể tải thiết bị.', 'Could not load devices.']; }
  }
  async function create(event) {
    event.preventDefault(); busy = true; error = ''; token = '';
    try {
      const response = await fetch('/api/devices', {method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':csrf}, body:JSON.stringify({name:name.trim(),room_id:Number(roomId)})});
      if (!response.ok) throw new Error();
      token = (await response.json()).token; name = ''; roomId = ''; await load();
    } catch { error = ['Không thể tạo thiết bị.', 'Could not create device.']; }
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
    if (!window.confirm(t(`Thu hồi thiết bị ${device.name}?`, `Revoke device ${device.name}?`))) return;
    busy = true; error = '';
    try {
      const response = await fetch(`/api/devices/${device.id}/revoke`, {method:'POST',headers:{'X-CSRF-Token':csrf}});
      if (!response.ok) throw new Error();
      await load();
    } catch { error = ['Không thể thu hồi thiết bị.', 'Could not revoke device.']; }
    finally { busy = false; }
  }
  onMount(() => {
    load();
    window.addEventListener('rooms-changed', load);
    return () => window.removeEventListener('rooms-changed', load);
  });
</script>
<section aria-labelledby="device-heading">
  <h2 id="device-heading">{t('Thiết bị màn hình', 'Display devices')}</h2>
  {#if error}<p role="alert">{t(...error)}</p>{/if}
  {#if token}<p role="status">{t('Mã thiết bị chỉ hiển thị một lần. Nhập mã trên màn hình phòng:', 'The device token is shown only once. Enter it on the room display:')}</p><output aria-label={t('Mã thiết bị', 'Device token')}>{token}</output><button type="button" onclick={() => token = ''}>{t('Ẩn mã', 'Hide token')}</button>{/if}
  <ul>{#each devices as device (device.id)}<li>{device.name} — {rooms.find(r => r.id === device.room_id)?.name} — {device.revoked ? t('Đã thu hồi', 'Revoked') : t('Hoạt động', 'Active')} — {device.last_seen ? new Date(device.last_seen * 1000).toLocaleString(language === 'en' ? 'en-US' : 'vi-VN') : t('Chưa kết nối', 'Not connected')}{#if !device.revoked}<button type="button" disabled={busy || editing !== null} onclick={() => editing = device}>{t('Thiết kế màn hình', 'Design display')}</button><label>{t('Ngôn ngữ', 'Language')} {device.name} <select value={device.language} disabled={busy} onchange={(event) => setLanguage(device, event)}><option value="vi">Tiếng Việt</option><option value="en">English</option></select></label><button type="button" disabled={busy} aria-label={`${t('Thu hồi', 'Revoke')} ${device.name}`} onclick={() => revoke(device)}>{t('Thu hồi', 'Revoke')}</button>{/if}</li>{/each}</ul>
  {#if editing}<DisplayEditor device={editing} {csrf} onclose={() => editing = null} />{/if}
  <form onsubmit={create}><h3>{t('Đăng ký thiết bị', 'Register device')}</h3><label for="device-name">{t('Tên thiết bị', 'Device name')}</label><input id="device-name" bind:value={name} maxlength="128" required /><label for="device-room">{t('Phòng thiết bị', 'Device room')}</label><select id="device-room" bind:value={roomId} required><option value="">{t('Chọn phòng', 'Select room')}</option>{#each rooms.filter(r => r.active) as room}<option value={String(room.id)}>{room.name}</option>{/each}</select><button type="submit" disabled={busy}>{t('Tạo thiết bị', 'Create device')}</button></form>
</section>
<style>section { margin-top:2rem; padding-top:1.5rem; border-top:1px solid #c9d7d9 } form { display:grid; gap:.5rem } input, select, button { font:inherit } li, output { overflow-wrap:anywhere } [role='alert'] { color:#7d261e }</style>
