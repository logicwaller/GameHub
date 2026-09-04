<template>
  <section class="section page analytics-page">
    <RouterLink to="/profile" class="back">← 返回个人中心</RouterLink>
    <div class="section-head">
      <div>
        <p class="eyebrow">CREATOR ANALYTICS</p>
        <h1>游戏数据</h1>
        <p class="muted">查看已发布游戏的实时累计数据与近七天真实趋势。</p>
      </div>
      <select v-model="selected">
        <option value="all">全部游戏</option>
        <option v-for="game in games" :key="game.id" :value="String(game.id)">
          {{ game.title }}
        </option>
      </select>
    </div>

    <p v-if="loading" class="empty analytics-empty">正在加载数据...</p>
    <p v-else-if="!games.length" class="empty analytics-empty">
      你还没有发布游戏，先去个人中心创建一个吧。
    </p>

    <template v-else>
      <div class="metric-grid">
        <article>
          <span>游玩量</span>
          <strong>{{ total.plays.toLocaleString() }}</strong>
          <small>累计访问</small>
        </article>
        <article>
          <span>收藏量</span>
          <strong>{{ total.favorites.toLocaleString() }}</strong>
          <small>用户收藏</small>
        </article>
        <article>
          <span>评论量</span>
          <strong>{{ total.comments.toLocaleString() }}</strong>
          <small>社区反馈</small>
        </article>
        <article>
          <span>点赞量</span>
          <strong>{{ total.likes.toLocaleString() }}</strong>
          <small>内容喜爱</small>
        </article>
      </div>

      <div class="chart-grid">
        <article class="chart-panel">
          <div class="chart-title">
            <h2>近 7 天游玩趋势</h2>
            <span>Kafka 事件聚合</span>
          </div>
          <svg viewBox="0 0 700 280" role="img" aria-label="近七天游玩量趋势图">
            <line x1="45" y1="230" x2="680" y2="230" class="chart-axis" />
            <polyline :points="trendPoints" class="trend-line" />
            <circle
              v-for="point in trendCoordinates"
              :key="point.label"
              :cx="point.x"
              :cy="point.y"
              r="5"
              class="trend-dot"
            />
            <text
              v-for="point in trendCoordinates"
              :key="`${point.label}-text`"
              :x="point.x"
              y="255"
              class="chart-label"
            >
              {{ point.label }}
            </text>
          </svg>
        </article>

        <article class="chart-panel">
          <div class="chart-title">
            <h2>数据构成</h2>
            <span>{{ selected === 'all' ? '整体' : selectedGame?.title }}</span>
          </div>
          <div class="bar-list">
            <div v-for="item in bars" :key="item.label" class="bar-item">
              <div>
                <span>{{ item.label }}</span>
                <strong>{{ item.value.toLocaleString() }}</strong>
              </div>
              <div class="bar-track">
                <i :style="{ width: `${item.percent}%`, background: item.color }"></i>
              </div>
            </div>
          </div>
        </article>
      </div>
    </template>
  </section>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'

const selected = ref('all')
const games = ref([])
const trend = ref([])
const loading = ref(true)

const selectedGame = computed(() => games.value.find((game) => String(game.id) === selected.value))
const total = computed(() => {
  const source = selected.value === 'all' ? games.value : [selectedGame.value].filter(Boolean)
  return source.reduce((sum, game) => ({
    plays: sum.plays + (game.plays || 0),
    likes: sum.likes + (game.likes || 0),
    favorites: sum.favorites + (game.favorites || 0),
    comments: sum.comments + (game.comments || 0)
  }), { plays: 0, likes: 0, favorites: 0, comments: 0 })
})

const trendCoordinates = computed(() => {
  const values = trend.value.length ? trend.value : [{ date: '今天', plays: 0 }]
  const max = Math.max(...values.map((item) => item.plays || 0), 1)
  const spacing = values.length > 1 ? 630 / (values.length - 1) : 0
  return values.map((item, index) => ({
    x: 50 + index * spacing,
    y: 220 - ((item.plays || 0) / max) * 170,
    label: item.date?.slice(5) || '今天'
  }))
})

const trendPoints = computed(() => trendCoordinates.value.map((point) => `${point.x},${point.y}`).join(' '))
const bars = computed(() => {
  const values = [
    { label: '游玩量', value: total.value.plays, color: '#d4f34a' },
    { label: '收藏量', value: total.value.favorites, color: '#79b9d3' },
    { label: '评论量', value: total.value.comments, color: '#e9a66c' },
    { label: '点赞量', value: total.value.likes, color: '#c58ad9' }
  ]
  const max = Math.max(...values.map((item) => item.value), 1)
  return values.map((item) => ({ ...item, percent: Math.max(3, item.value / max * 100) }))
})

async function loadAnalytics() {
  loading.value = true
  try {
    const params = new URLSearchParams({ days: '7' })
    if (selected.value !== 'all') params.set('game_id', selected.value)
    const response = await fetch(`/api/games/analytics?${params}`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('gamehub_token')}` }
    })
    if (!response.ok) throw new Error('读取数据失败')
    const data = await response.json()
    games.value = data.items || []
    trend.value = data.trend || []
  } finally {
    loading.value = false
  }
}

onMounted(loadAnalytics)
watch(selected, loadAnalytics)
</script>
