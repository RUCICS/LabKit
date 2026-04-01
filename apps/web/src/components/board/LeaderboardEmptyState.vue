<template>
  <section class="board-state" data-testid="board-empty-state">
    <p class="board-state__eyebrow">Queued</p>
    <h2>No leaderboard entries yet</h2>
    <p class="board-state__copy">首个有效提交出分后，这里会显示完整排名。</p>
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
