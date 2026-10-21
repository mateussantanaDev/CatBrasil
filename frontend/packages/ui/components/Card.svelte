<script>
  let {
    highlight = false,
    hazard = false,
    children,
    class: className = '',
    ...restProps
  } = $props();
</script>

<div
  class="card-cat {highlight ? 'card-cat-highlight' : ''} {className}"
  {...restProps}
>
  {#if hazard}
    <div class="hazard-stripes h-2 w-full absolute top-0 left-0"></div>
  {/if}
  {@render children?.()}
</div>
