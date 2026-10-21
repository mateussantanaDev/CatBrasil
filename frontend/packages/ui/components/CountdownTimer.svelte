<script>
  import { onMount, onDestroy } from 'svelte';

  let {
    seconds = 0,
    ontick,
    oncomplete,
    prefix = '',
    class: className = ''
  } = $props();

  let remaining = $state(0);
  let timerId = null;

  $effect(() => {
    remaining = seconds;
  });

  onMount(() => {
    remaining = seconds;
    timerId = setInterval(() => {
      if (remaining > 0) {
        remaining -= 1;
        if (ontick) ontick(remaining);
        if (remaining === 0 && oncomplete) {
          oncomplete();
        }
      }
    }, 1000);
  });

  onDestroy(() => {
    if (timerId) clearInterval(timerId);
  });

  function formatTime(totalSeconds) {
    if (totalSeconds <= 0) return '00:00:00';
    const hrs = Math.floor(totalSeconds / 3600);
    const mins = Math.floor((totalSeconds % 3600) / 60);
    const secs = totalSeconds % 60;
    return `${String(hrs).padStart(2, '0')}:${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  }
</script>

<span class="font-mono font-bold {className}">
  {prefix}{formatTime(remaining)}
</span>
