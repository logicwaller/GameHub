<template>
  <section class="section page">
    <div class="section-head">
      <div>
        <p class="eyebrow">EXPLORE THE COLLECTION</p>
        <h1>发现游戏</h1>
      </div>
      <div class="search">
        <span>⌕</span>
        <input v-model="query" placeholder="搜索游戏..." aria-label="搜索游戏">
      </div>
    </div>

    <div class="toolbar">
      <div class="filters">
        <button
          v-for="item in categories"
          :key="item"
          :class="{ active: category === item }"
          @click="category = item"
        >
          {{ item }}
        </button>
      </div>
      <select v-model="sort" aria-label="排序方式">
        <option value="plays">浏览次数</option>
        <option value="likes">点赞数</option>
      </select>
    </div>

    <div class="game-grid">
      <RouterLink
        v-for="game in paged"
        :key="game.id"
        :to="`/games/${game.id}`"
        class="game-card"
      >
        <div class="game-cover" :class="game.color" :style="coverStyle(game)">
          <img v-if="game.cover" :src="game.cover" :alt="`${game.title} 封面`">
          <span v-else>{{ game.icon }}</span>
          <small>{{ game.type }}</small>
        </div>
        <div class="game-info">
          <h3 v-html="highlight(game.title)"></h3>
          <p>{{ game.description }}</p>
          <div>
            <span class="tag">{{ game.category }}</span>
            <span class="play-count">
              {{ format(game[sort]) }} {{ sort === 'plays' ? '次浏览' : '个赞' }}
            </span>
          </div>
        </div>
      </RouterLink>
    </div>

    <p v-if="!filtered.length" class="empty">没有找到匹配的游戏</p>
    <div v-else class="pagination">
      <button class="secondary" :disabled="page === 1" @click="page -= 1">上一页</button>
      <span>第 {{ page }} / {{ pageCount }} 页</span>
      <button class="secondary" :disabled="page === pageCount" @click="page += 1">下一页</button>
    </div>
  </section>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { coverStyle, loadGames, state } from '../stores'

const query = ref('')
const category = ref('全部')
const sort = ref('plays')
const page = ref(1)
const pageSize = 9
const categories = ['全部', '动作', '策略', '休闲', '解谜']

const filtered = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  return [...state.games]
    .filter((game) => {
      const matchesCategory = category.value === '全部' || game.category === category.value
      const matchesKeyword = !keyword
        || game.title.toLowerCase().includes(keyword)
        || game.description.toLowerCase().includes(keyword)
      return matchesCategory && matchesKeyword
    })
    .sort((a, b) => (b[sort.value] || 0) - (a[sort.value] || 0))
})

const pageCount = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize)))
const paged = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize))
const format = (value) => (value > 999 ? `${(value / 1000).toFixed(1)}k` : value || 0)

function highlight(text) {
  const escaped = String(text || '').replace(/[&<>"']/g, (char) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  })[char])
  const keyword = query.value.trim().replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return keyword ? escaped.replace(new RegExp(`(${keyword})`, 'ig'), '<mark>$1</mark>') : escaped
}

let searchTimer
watch([query, sort, category], ([nextQuery, nextSort, nextCategory]) => {
  page.value = 1
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => loadGames({
    q: nextQuery.trim(),
    sort: nextSort,
    category: nextCategory
  }), 250)
})

watch(pageCount, (count) => {
  if (page.value > count) page.value = count
})
</script>
