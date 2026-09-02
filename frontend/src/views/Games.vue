<template>
  <section class="section page">
    <div class="section-head"><div><p class="eyebrow">EXPLORE THE COLLECTION</p><h1>发现游戏</h1></div><div class="search"><span>⌕</span><input v-model="query" placeholder="搜索游戏..." aria-label="搜索游戏"></div></div>
    <div class="toolbar"><div class="filters"><button v-for="item in categories" :key="item" :class="{ active: category === item }" @click="category = item">{{ item }}</button></div><select v-model="sort" aria-label="排序方式"><option value="plays">浏览次数</option><option value="likes">点赞数</option></select></div>
    <div class="game-grid">
      <RouterLink v-for="game in filtered" :key="game.id" :to="`/games/${game.id}`" class="game-card">
        <div class="game-cover" :class="game.color" :style="coverStyle(game)"><img v-if="game.cover" :src="game.cover" :alt="`${game.title} 封面`"><span v-else>{{ game.icon }}</span><small>{{ game.type }}</small></div>
        <div class="game-info"><h3>{{ game.title }}</h3><p>{{ game.description }}</p><div><span class="tag">{{ game.category }}</span><span class="play-count">{{ format(game[sort]) }} {{ sort === 'plays' ? '次浏览' : '个赞' }}</span></div></div>
      </RouterLink>
    </div>
    <p v-if="!filtered.length" class="empty">没有找到匹配的游戏</p>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue'
import { coverStyle, state } from '../stores'

const query = ref('')
const category = ref('全部')
const sort = ref('plays')
const categories = ['全部', '动作', '策略', '休闲', '解谜']
const filtered = computed(() => [...state.games].filter((game) => (category.value === '全部' || game.category === category.value) && game.title.includes(query.value)).sort((a, b) => b[sort.value] - a[sort.value]))
const format = (value) => value > 999 ? `${(value / 1000).toFixed(1)}k` : value
</script>
