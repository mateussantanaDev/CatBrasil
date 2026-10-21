<script>
  let {
    variant = 'yellow', // 'yellow' | 'green' | 'red' | 'reinvest'
    children,
    class: className = '',
    ...restProps
  } = $props();

  const variantClass = $derived({
    yellow: 'badge-cat',
    green: 'badge-cat badge-cat-green',
    red: 'badge-cat badge-cat-red',
    reinvest: 'badge-cat badge-cat-reinvest'
  }[variant] || 'badge-cat');
</script>

<span class="{variantClass} {className}" {...restProps}>
  {@render children?.()}
</span>
