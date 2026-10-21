<script>
  let {
    variant = 'yellow', // 'yellow' | 'dark' | 'green' | 'danger'
    size = 'md', // 'sm' | 'md' | 'lg'
    disabled = false,
    loading = false,
    onclick,
    children,
    class: className = '',
    type = 'button',
    ...restProps
  } = $props();

  const variantClass = $derived({
    yellow: 'btn-cat',
    dark: 'btn-cat btn-cat-dark',
    green: 'btn-cat btn-cat-green',
    danger: 'btn-cat btn-cat-danger'
  }[variant] || 'btn-cat');

  const sizeClass = $derived({
    sm: 'btn-cat-small',
    md: '',
    lg: 'text-lg py-4 px-6'
  }[size] || '');
</script>

<button
  {type}
  class="{variantClass} {sizeClass} {className}"
  {disabled}
  {onclick}
  {...restProps}
>
  {#if loading}
    <span class="animate-spin">⚙️</span>
  {/if}
  {@render children?.()}
</button>
