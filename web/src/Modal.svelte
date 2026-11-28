<script>
  import Icon from './Icon.svelte';

  let {
    open = false,
    title = '',
    badge = '',
    busy = false,
    size = 'md',
    closeLabel = 'Close',
    onclose,
    children,
  } = $props();

  let dialogEl = $state(null);
  let pointerDownOnBackdrop = false;
  const titleId = `modal-title-${Math.random().toString(36).slice(2, 9)}`;

  $effect(() => {
    if (!dialogEl) return;
    if (open) {
      if (!dialogEl.open) {
        dialogEl.showModal();
        const firstField = dialogEl.querySelector('.ui-modal-body input:not([type="hidden"]):not([disabled]), .ui-modal-body select:not([disabled]), .ui-modal-body textarea:not([disabled])');
        if (firstField) firstField.focus({ preventScroll: true });
      }
    } else if (dialogEl.open) {
      dialogEl.close();
    }
  });

  function requestClose() {
    if (busy) return;
    onclose?.();
  }

  function handleCancel(event) {
    event.preventDefault();
    requestClose();
  }

  function handlePointerDown(event) {
    pointerDownOnBackdrop = event.target === dialogEl;
  }

  function handleClick(event) {
    if (pointerDownOnBackdrop && event.target === dialogEl) requestClose();
    pointerDownOnBackdrop = false;
  }
</script>

<dialog
  bind:this={dialogEl}
  class={`ui-modal ${size === 'lg' ? 'ui-modal--lg' : ''}`.trim()}
  aria-labelledby={titleId}
  oncancel={handleCancel}
  onpointerdown={handlePointerDown}
  onclick={handleClick}
>
  {#if open}
    <div class="ui-modal-surface">
      <header class="ui-modal-head">
        <div class="ui-modal-title-row">
          <h3 id={titleId}>{title}</h3>
          {#if badge}<span class="ui-pill ui-pill--warning">{badge}</span>{/if}
        </div>
        <button
          type="button"
          class="ui-modal-close"
          aria-label={closeLabel}
          disabled={busy}
          onclick={requestClose}
        >
          <Icon name="close" />
        </button>
      </header>
      <div class="ui-modal-body">
        {@render children?.()}
      </div>
    </div>
  {/if}
</dialog>
