<template>
  <section class="board-state" data-testid="board-hidden-state">
    <p class="board-state__eyebrow">Hidden</p>
    <h2>Leaderboard is hidden for now</h2>
    <p class="board-state__copy">该实验尚未到达公开可见时间。</p>
  </section>
</template>

<style scoped>
.board-state {
  padding: 28px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-elevated);
}

.board-state__eyebrow {
  margin: 0 0 10px;
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 0.1em;
  font-size: 0.72rem;
  font-family: var(--font-mono);
  font-weight: 600;
}

h2 {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 1.1rem;
}

.board-state__copy {
  max-width: 32rem;
  margin: 12px 0 0;
  color: var(--text-secondary);
  line-height: 1.6;
}
</style>
