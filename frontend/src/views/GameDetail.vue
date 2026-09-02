<template>
  <template v-if="game">
    <section class="section page detail">
      <RouterLink to="/games" class="back">← 返回游戏库</RouterLink>
      <div class="detail-layout">
        <div class="detail-cover" :class="game.color">
          <img v-if="game.cover" :src="game.cover" :alt="`${game.title} 封面`">
          <span v-else>{{ game.icon }}</span>
          <small>{{ game.type }}</small>
        </div>
        <div>
          <p class="eyebrow">FEATURED GAME · {{ game.category }}</p>
          <h1>{{ game.title }}</h1>
          <p class="detail-lead">{{ game.description }}</p>
          <RouterLink :to="`/users/${game.author || game.authorId || 'editor'}`" class="game-author"><span class="avatar">{{ game.authorAvatar || game.author?.[0] || 'G' }}</span><span><small>作者</small><strong>{{ game.author || 'GameHub 编辑' }}</strong></span></RouterLink>
          <div class="detail-meta">
            <span>◷ {{ game.playTime }}</span>
            <span>♙ {{ game.plays.toLocaleString() }} 次浏览</span>
          </div>
          <div class="detail-actions">
            <button class="action-button" :class="{ active: liked }" @click="like">
              <span>♥</span>{{ game.likes }} 点赞
            </button>
            <button class="action-button" :class="{ active: favorite }" @click="fav">
              <span>★</span>{{ favorite ? '已收藏' : '收藏' }}
            </button>
            <a class="primary play" :href="game.url" target="_blank" rel="noreferrer">
              开始游玩 <span>↗</span>
            </a>
          </div>
          <p v-if="actionError" class="error">{{ actionError }}</p>
        </div>
      </div>

      <div class="comments">
        <div class="section-head">
          <div>
            <p class="eyebrow">COMMUNITY VOICES</p>
            <h2>评论区</h2>
          </div>
        </div>
        <form class="comment-form" @submit.prevent="submitComment">
          <input v-model="commentText" placeholder="分享你的游戏体验..." required>
          <button class="primary">发表评论</button>
        </form>
        <article v-for="comment in comments" :key="comment.id" class="comment">
          <RouterLink :to="`/users/${comment.authorId || comment.author}`" class="avatar-link"><span class="avatar">{{ comment.authorAvatar || comment.author[0] }}</span></RouterLink>
          <div>
            <strong><RouterLink :to="`/users/${comment.authorId || comment.author}`">{{ comment.author }}</RouterLink></strong>
            <small>{{ comment.createdAt }}</small>
            <p>{{ comment.text }}</p>
          </div>
        </article>
        <p v-if="!comments.length" class="empty">还没有评论，来留下第一条吧。</p>
      </div>
    </section>
  </template>
  <section v-else class="section page">
    <p>游戏不存在</p>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  addComment,
  loadGameComments,
  state,
  toggleFavorite,
  toggleLike
} from '../stores'

const route = useRoute()
const game = computed(() => state.games.find((item) => item.id === Number(route.params.id)))
const liked = computed(() => game.value && state.liked.includes(game.value.id))
const favorite = computed(() => game.value && state.favorites.includes(game.value.id))
const comments = computed(() => (game.value ? state.comments[game.value.id] || [] : []))
const commentText = ref('')
const actionError = ref('')

loadGameComments(Number(route.params.id)).catch(() => {})

async function like() {
  if (!game.value) return
  actionError.value = ''
  try { await toggleLike(game.value.id) } catch (error) { actionError.value = error.message }
}

async function fav() {
  if (!game.value) return
  actionError.value = ''
  try { await toggleFavorite(game.value.id) } catch (error) { actionError.value = error.message }
}

async function submitComment() {
  if (game.value && commentText.value.trim()) {
    actionError.value = ''
    try {
      await addComment(game.value.id, commentText.value.trim())
    } catch (error) {
      actionError.value = error.message
      return
    }
    commentText.value = ''
  }
}
</script>
