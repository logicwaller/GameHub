<template>
  <section class="hero">
    <div class="hero-copy">
      <p class="eyebrow">GAME DISCOVERY PLATFORM</p>
      <h1>找到你的<br><em>下一场冒险</em></h1>
      <p class="lead">精选网页游戏，随时打开即玩。探索不同世界，收藏属于你的游戏清单。</p>
      <div class="hero-actions"><RouterLink class="primary" to="/games">开始探索 <span>→</span></RouterLink><RouterLink class="secondary" to="/games">浏览全部游戏</RouterLink></div>
    </div>
    <div class="hero-art"><div class="orbit orbit-one"></div><div class="orbit orbit-two"></div><div class="planet">✦</div><div class="art-label">CURATED<br><strong>PLAYLIST</strong></div></div>
  </section>
  <section class="section">
    <div class="section-head"><div><p class="eyebrow">HANDPICKED FOR YOU</p><h2>本周精选</h2></div><RouterLink to="/games" class="view-all">查看全部 <span>→</span></RouterLink></div>
    <div class="game-grid">
      <RouterLink v-for="game in featured" :key="game.id" :to="`/games/${game.id}`" class="game-card">
        <div class="game-cover" :class="game.color" :style="coverStyle(game)"><img v-if="game.cover" :src="game.cover" :alt="`${game.title} 封面`"><span v-else>{{ game.icon }}</span><small>{{ game.type }}</small></div>
        <div class="game-info"><h3>{{ game.title }}</h3><p>{{ game.description }}</p><div><span class="tag">{{ game.category }}</span><span class="play-count">{{ format(game.plays) }} 次游玩</span></div></div>
      </RouterLink>
    </div>
    <p v-if="!featured.length" class="empty">数据库中暂时还没有游戏。</p>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { coverStyle, state } from '../stores'

const featured = computed(() => (state.hotGames.length ? state.hotGames : state.games).slice(0, 3))
const format = (value) => value > 999 ? `${(value / 1000).toFixed(1)}k` : value
</script>
