<script>
  import Icon from './Icon.svelte';
  let { kind = 'info', text = '', role = undefined, class: extraClass = '' } = $props();
  const iconName = $derived(
    kind === 'success' ? 'check-circle' : kind === 'warning' ? 'warning' : kind === 'info' ? 'shield' : 'alert'
  );
  const resolvedRole = $derived(role ?? (kind === 'error' ? 'alert' : 'status'));
</script>

<p class={`ui-banner ui-banner--${kind} ${extraClass}`.trim()} role={resolvedRole}>
  <Icon name={iconName} />
  <span>{text}</span>
</p>
