<script>
  import { onDestroy, tick } from 'svelte';

  export let language = 'vi';
  let dialog;
  let cancelButton;
  let message = '';
  let resolvePending;

  export function ask(text) {
    if (resolvePending) return Promise.resolve(false);
    message = text;
    return new Promise((resolve) => {
      resolvePending = resolve;
      tick().then(() => {
        if (!resolvePending) return;
        try {
          dialog.returnValue = '';
          dialog.showModal();
        } catch (error) {
          resolvePending = null;
          resolve(false);
          console.error(error);
          return;
        }
        cancelButton.focus();
      });
    });
  }

  function finish() {
    const resolve = resolvePending;
    resolvePending = null;
    resolve?.(dialog.returnValue === 'confirm');
  }

  onDestroy(() => {
    resolvePending?.(false);
    resolvePending = null;
  });
</script>

<dialog bind:this={dialog} on:close={finish} aria-label={message}>
  <form method="dialog">
    <p>{message}</p>
    <div class="actions">
      <button bind:this={cancelButton} class="secondary" value="cancel">{language === 'en' ? 'Cancel' : 'Hủy'}</button>
      <button value="confirm">{language === 'en' ? 'Confirm' : 'Xác nhận'}</button>
    </div>
  </form>
</dialog>

<style>
  dialog { max-width: min(30rem, calc(100% - 2rem)); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 1.5rem; color: var(--ink-primary); background: var(--bg-surface); box-shadow: var(--shadow-md); }
  dialog::backdrop { background: #0008; }
  p { margin: 0 0 1.5rem; white-space: pre-wrap; }
  .actions { display: flex; justify-content: flex-end; gap: 0.75rem; }
</style>
