<template>
  <section class="section page">
    <div class="section-head">
      <div>
        <p class="eyebrow">ADMINISTRATION</p>
        <h1>管理员中心</h1>
      </div>
      <span class="admin-note">内容管理</span>
    </div>

    <section class="admin-section">
      <h2 class="admin-title">游戏管理</h2>
      <article v-for="game in state.games" :key="game.id" class="admin-row">
        <div>
          <RouterLink :to="`/games/${game.id}`">
            <strong>{{ game.title }}</strong>
          </RouterLink>
          <small>{{ game.category }} · {{ game.plays.toLocaleString() }} 浏览 · {{ game.likes }} 点赞</small>
        </div>
        <button class="danger" @click="removeGameItem(game.id)">删除游戏</button>
      </article>
      <p v-if="!state.games.length" class="empty">暂无游戏</p>
    </section>

    <section class="admin-section">
      <h2 class="admin-title">论坛管理</h2>
      <article v-for="post in state.posts" :key="post.id" class="admin-row">
        <div>
          <RouterLink :to="`/forum/${post.id}`">
            <strong>{{ post.title }}</strong>
          </RouterLink>
          <small>{{ post.author }} · {{ post.comments?.length || 0 }} 回复 · {{ post.createdAt }}</small>
        </div>
        <button class="danger" @click="removePostItem(post.id)">删除帖子</button>
      </article>
      <p v-if="!state.posts.length" class="empty">暂无帖子</p>
    </section>

    <section class="admin-section">
      <div class="section-head">
        <h2 class="admin-title">Kafka 死信队列</h2>
        <select v-model="selectedTopic" @change="loadDeadLetters">
          <option v-for="topic in topics" :key="topic" :value="topic">
            {{ topic }}
          </option>
        </select>
      </div>
      <button class="secondary" @click="loadDeadLetters">刷新死信消息</button>
      <article v-for="item in deadLetters" :key="item.eventId" class="admin-row">
        <div>
          <strong>{{ item.type }}</strong>
          <small>{{ item.error }} · {{ item.eventId }}</small>
        </div>
        <button class="danger" @click="replay(item.eventId)">重放</button>
      </article>
      <p v-if="loaded && !deadLetters.length" class="empty">该主题暂无死信消息。</p>
    </section>
  </section>
</template>

<script setup>
import { ref } from 'vue'
import { deleteGame, deletePost, state } from '../stores'

const topics = ['game.play', 'search.sync', 'interaction.event', 'notification']
const selectedTopic = ref(topics[0])
const deadLetters = ref([])
const loaded = ref(false)

async function removeGameItem(id) {
  if (window.confirm('确定删除这个游戏吗？')) await deleteGame(id)
}

async function removePostItem(id) {
  if (window.confirm('确定删除这个帖子吗？')) await deletePost(id)
}

async function loadDeadLetters() {
  const response = await fetch(`/api/admin/kafka/dlq/${selectedTopic.value}`, {
    headers: { Authorization: `Bearer ${localStorage.getItem('gamehub_token')}` }
  })
  if (!response.ok) return
  const data = await response.json()
  deadLetters.value = data.items || []
  loaded.value = true
}

async function replay(eventID) {
  const response = await fetch(`/api/admin/kafka/dlq/${selectedTopic.value}/${eventID}/replay`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${localStorage.getItem('gamehub_token')}` }
  })
  if (response.ok) await loadDeadLetters()
}
</script>
