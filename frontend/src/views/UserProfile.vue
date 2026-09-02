<template>
  <section class="section page public-profile">
    <RouterLink to="/games" class="back">← 返回</RouterLink>
    <div class="profile-head">
      <span class="avatar avatar-large">{{ profile.avatar }}</span>
      <div><p class="eyebrow">PLAYER PROFILE</p><h1>{{ profile.name }}</h1><p>ID: {{ profile.id }}</p></div>
    </div>
    <div class="profile-tabs">
      <section><p class="eyebrow">PUBLISHED WORKS</p><h2>发布作品</h2><div class="game-grid"><RouterLink v-for="game in published" :key="game.id" :to="`/games/${game.id}`" class="game-card"><div class="game-cover" :class="game.color" :style="coverStyle(game)"><img v-if="game.cover" :src="game.cover" :alt="`${game.title} 封面`"><span v-else>{{ game.icon }}</span></div><div class="game-info"><h3>{{ game.title }}</h3><p>{{ game.description }}</p></div></RouterLink></div><p v-if="!published.length" class="empty">暂无发布作品</p></section>
      <section><p class="eyebrow">SAVED GAMES</p><h2>收藏作品</h2><RouterLink v-for="game in saved" :key="game.id" :to="`/games/${game.id}`" class="activity-link">{{ game.title }}</RouterLink><p v-if="!saved.length" class="empty">暂无收藏作品</p></section>
      <section><p class="eyebrow">RECENT LIKES</p><h2>近期点赞</h2><RouterLink v-for="game in liked" :key="game.id" :to="`/games/${game.id}`" class="activity-link">{{ game.title }} <span>♥</span></RouterLink><p v-if="!liked.length" class="empty">暂无点赞作品</p></section>
    </div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { coverStyle, state } from '../stores'

const route = useRoute()
const id = computed(() => String(route.params.id))
const profile = computed(() => ({ id: id.value, name: id.value === 'editor' ? 'GameHub 编辑' : id.value, avatar: id.value[0]?.toUpperCase() || '访' }))
const published = computed(() => state.games.filter((game) => game.author === id.value || String(game.authorId) === id.value))
const saved = computed(() => id.value === state.user?.username ? state.games.filter((game) => state.favorites.includes(game.id)) : [])
const liked = computed(() => id.value === state.user?.username ? state.games.filter((game) => state.liked.includes(game.id)) : [])
</script>
