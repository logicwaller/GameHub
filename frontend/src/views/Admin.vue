<template>
  <section class="section page">
    <div class="section-head">
      <div><p class="eyebrow">ADMINISTRATION</p><h1>管理员中心</h1></div>
      <span class="admin-note">内容管理</span>
    </div>

    <section class="admin-section">
      <h2 class="admin-title">游戏管理</h2>
      <article v-for="game in state.games" :key="game.id" class="admin-row">
        <div>
          <RouterLink :to="`/games/${game.id}`"><strong>{{ game.title }}</strong></RouterLink>
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
          <RouterLink :to="`/forum/${post.id}`"><strong>{{ post.title }}</strong></RouterLink>
          <small>{{ post.author }} · {{ post.comments?.length || 0 }} 回复 · {{ post.createdAt }}</small>
        </div>
        <button class="danger" @click="removePostItem(post.id)">删除帖子</button>
      </article>
      <p v-if="!state.posts.length" class="empty">暂无帖子</p>
    </section>
  </section>
</template>

<script setup>
import { deleteGame, deletePost, state } from '../stores'

async function removeGameItem(id) {
  if (window.confirm('确定删除这个游戏吗？')) await deleteGame(id)
}

async function removePostItem(id) {
  if (window.confirm('确定删除这个帖子吗？')) await deletePost(id)
}
</script>
