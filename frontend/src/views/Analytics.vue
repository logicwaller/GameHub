<template>
  <section class="section page analytics-page">
    <RouterLink to="/profile" class="back">← 返回个人中心</RouterLink>
    <div class="section-head"><div><p class="eyebrow">CREATOR ANALYTICS</p><h1>游戏数据</h1><p class="muted">查看你发布的游戏表现，按整体或单个游戏分析。</p></div><select v-model="selected"><option value="all">全部游戏</option><option v-for="game in publishedGames" :key="game.id" :value="String(game.id)">{{ game.title }}</option></select></div>
    <p v-if="!publishedGames.length" class="empty analytics-empty">你还没有发布游戏，先去个人中心创建一个吧。</p>
    <template v-else>
      <div class="metric-grid"><article><span>游玩量</span><strong>{{ total.plays.toLocaleString() }}</strong><small>累计访问</small></article><article><span>收藏量</span><strong>{{ total.favorites.toLocaleString() }}</strong><small>用户收藏</small></article><article><span>评论量</span><strong>{{ total.comments.toLocaleString() }}</strong><small>社区反馈</small></article><article><span>点赞量</span><strong>{{ total.likes.toLocaleString() }}</strong><small>内容喜爱</small></article></div>
      <div class="chart-grid"><article class="chart-panel"><div class="chart-title"><h2>近 7 天趋势</h2><span>每日数据</span></div><svg viewBox="0 0 700 280" role="img" aria-label="近七天游玩量趋势图"><line x1="45" y1="230" x2="680" y2="230" class="chart-axis"/><polyline :points="trendPoints" class="trend-line"/><circle v-for="point in trendCoordinates" :key="point.x" :cx="point.x" :cy="point.y" r="5" class="trend-dot"/><text v-for="(day, index) in days" :key="day" :x="50 + index * 105" y="255" class="chart-label">{{ day }}</text></svg></article><article class="chart-panel"><div class="chart-title"><h2>数据构成</h2><span>{{ selected === 'all' ? '整体' : selectedGame.title }}</span></div><div class="bar-list"><div v-for="item in bars" :key="item.label" class="bar-item"><div><span>{{ item.label }}</span><strong>{{ item.value.toLocaleString() }}</strong></div><div class="bar-track"><i :style="{ width: `${item.percent}%`, background: item.color }"></i></div></div></div></article></div>
    </template>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue'
import { state } from '../stores'

const selected = ref('all')
const publishedGames = computed(() => state.games.filter((game) => game.authorId === state.user?.username))
const selectedGame = computed(() => publishedGames.value.find((game) => String(game.id) === selected.value) || publishedGames.value[0])
const commentsFor = (game) => (state.comments[game.id] || []).length
const total = computed(() => { const games = selected.value === 'all' ? publishedGames.value : [selectedGame.value]; return games.reduce((sum, game) => ({ plays: sum.plays + game.plays, likes: sum.likes + game.likes, favorites: sum.favorites + (game.favoritesCount || 0), comments: sum.comments + commentsFor(game) }), { plays: 0, likes: 0, favorites: 0, comments: 0 }) })
const days = ['6天前', '5天前', '4天前', '3天前', '前天', '昨天', '今天']
const trend = computed(() => days.map((_, index) => Math.max(0, Math.round(total.value.plays * (0.06 + index * 0.018)))))
const trendCoordinates = computed(() => trend.value.map((value, index) => ({ x: 50 + index * 105, y: 220 - (value / Math.max(...trend.value, 1)) * 170 })))
const trendPoints = computed(() => trendCoordinates.value.map((point) => `${point.x},${point.y}`).join(' '))
const bars = computed(() => { const values = [{ label: '游玩量', value: total.value.plays, color: '#d4f34a' }, { label: '收藏量', value: total.value.favorites, color: '#79b9d3' }, { label: '评论量', value: total.value.comments, color: '#e9a66c' }, { label: '点赞量', value: total.value.likes, color: '#c58ad9' }]; const max = Math.max(...values.map((item) => item.value), 1); return values.map((item) => ({ ...item, percent: Math.max(3, item.value / max * 100) })) })
</script>
