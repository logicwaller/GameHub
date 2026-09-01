<template>
  <section class="hero"><div class="hero-copy"><p class="eyebrow">GAME DISCOVERY PLATFORM</p><h1>找到你的<br><em>下一场冒险</em></h1><p class="lead">精选网页游戏，随时打开即玩。探索不同世界，收藏属于你的游戏清单。</p><div class="hero-actions"><RouterLink class="primary" to="/games">开始探索 <span>→</span></RouterLink><RouterLink class="secondary" to="/games">浏览全部游戏</RouterLink></div></div><div class="hero-art"><div class="orbit orbit-one"></div><div class="orbit orbit-two"></div><div class="planet">✦</div><div class="art-label">CURATED<br><strong>PLAYLIST</strong></div></div></section>
  <section class="section"><div class="section-head"><div><p class="eyebrow">HANDPICKED FOR YOU</p><h2>本周精选</h2></div><RouterLink to="/games" class="view-all">查看全部 <span>→</span></RouterLink></div><div class="game-grid"><RouterLink v-for="game in featured" :key="game.id" :to="`/games/${game.id}`" class="game-card"><div class="game-cover" :class="game.color"><span>{{ game.icon }}</span><small>{{ game.type }}</small></div><div class="game-info"><h3>{{ game.title }}</h3><p>{{ game.description }}</p><div><span class="tag">{{ game.category }}</span><span class="play-count">{{ game.plays }} 次游玩</span></div></div></RouterLink></div></section>
</template>
<script setup>
const featured = [
  { id: 1, title: '星际拓荒者', description: '在无尽宇宙中建立你的新家园。', category: '策略', type: 'STRATEGY', icon: '✦', color: 'cover-purple', plays: '12.4k' },
  { id: 2, title: '迷雾之城', description: '解开城市深处被隐藏的谜题。', category: '解谜', type: 'PUZZLE', icon: '◈', color: 'cover-blue', plays: '8.7k' },
  { id: 3, title: '像素冲锋', description: '快节奏的像素射击大乱斗。', category: '动作', type: 'ACTION', icon: '✹', color: 'cover-orange', plays: '6.2k' }
]
</script>
