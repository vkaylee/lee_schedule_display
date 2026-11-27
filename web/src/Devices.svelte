<script>
  import { onMount } from 'svelte';
  let { csrf } = $props();
  let devices = $state([]), rooms = $state([]), name = $state(''), roomId = $state(''), token = $state(''), error = $state(''), busy = $state(false);
  async function load() {
    try {
      const [d, r] = await Promise.all([fetch('/api/devices'), fetch('/api/rooms')]);
      if (!d.ok || !r.ok) throw new Error();
      devices = await d.json(); rooms = await r.json();
    } catch { error = 'Không thể tải thiết bị.'; }
  }
  async function create(event) {
    event.preventDefault(); busy = true; error = ''; token = '';
    try {
      const response = await fetch('/api/devices', {method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':csrf}, body:JSON.stringify({name:name.trim(),room_id:Number(roomId)})});
      if (!response.ok) throw new Error();
      token = (await response.json()).token; name = ''; roomId = ''; await load();
    } catch { error = 'Không thể tạo thiết bị.'; }
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
    } catch { select.value = device.language; error = 'Không thể cập nhật ngôn ngữ thiết bị.'; }
    finally { busy = false; }
  }
  async function revoke(device) {
    if (!window.confirm(`Thu hồi thiết bị ${device.name}?`)) return;
    busy = true; error = '';
    try {
      const response = await fetch(`/api/devices/${device.id}/revoke`, {method:'POST',headers:{'X-CSRF-Token':csrf}});
      if (!response.ok) throw new Error();
      await load();
    } catch { error = 'Không thể thu hồi thiết bị.'; }
    finally { busy = false; }
  }
  onMount(load);
</script>
<section aria-labelledby="device-heading">
  <h2 id="device-heading">Thiết bị màn hình</h2>
  {#if error}<p role="alert">{error}</p>{/if}
  {#if token}<p role="status">Mã thiết bị chỉ hiển thị một lần. Nhập mã trên màn hình phòng:</p><output aria-label="Mã thiết bị">{token}</output><button type="button" onclick={() => token = ''}>Ẩn mã</button>{/if}
  <ul>{#each devices as device (device.id)}<li>{device.name} — {rooms.find(r => r.id === device.room_id)?.name} — {device.revoked ? 'Đã thu hồi' : 'Hoạt động'} — {device.last_seen ? new Date(device.last_seen * 1000).toLocaleString() : 'Chưa kết nối'}{#if !device.revoked}<label>Ngôn ngữ {device.name} <select value={device.language} disabled={busy} onchange={(event) => setLanguage(device, event)}><option value="vi">Tiếng Việt</option><option value="en">English</option></select></label><button type="button" disabled={busy} aria-label={`Thu hồi ${device.name}`} onclick={() => revoke(device)}>Thu hồi</button>{/if}</li>{/each}</ul>
  <form onsubmit={create}><h3>Đăng ký thiết bị</h3><label for="device-name">Tên thiết bị</label><input id="device-name" bind:value={name} maxlength="128" required /><label for="device-room">Phòng thiết bị</label><select id="device-room" bind:value={roomId} required><option value="">Chọn phòng</option>{#each rooms.filter(r => r.active) as room}<option value={String(room.id)}>{room.name}</option>{/each}</select><button type="submit" disabled={busy}>Tạo thiết bị</button></form>
</section>
<style>section { margin-top:2rem; padding-top:1.5rem; border-top:1px solid #c9d7d9 } form { display:grid; gap:.5rem } input, select, button { font:inherit } li, output { overflow-wrap:anywhere } [role='alert'] { color:#7d261e }</style>
