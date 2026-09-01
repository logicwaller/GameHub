<template>
  <section class="section page">
    <div class="section-head"><div><p class="eyebrow">ADMINISTRATION</p><h1>管理员中心</h1></div><span class="admin-note">内容管理</span></div>
    <h2 class="admin-title">游戏管理</h2>
    <article v-for="game in state.games" :key="game.id" class="admin-row"><div><strong>{{ game.title }}</strong><small>{{ game.category }} · {{ game.plays.toLocaleString() }} 浏览 · {{ game.likes }} 点赞</small></div><button class="danger" @click="removeGame(game.id)">删除游戏</button></article>
    <h2 class="admin-title">评论管理</h2>
    <article v-for="item in allComments" :key="item.key" class="admin-row"><div><strong><RouterLink :to="`/users/${item.comment.authorId || item.comment.author}`">{{ item.comment.author }}</RouterLink>：{{ item.comment.text }}</strong><small>来自《{{ item.game.title }}》 · {{ item.comment.createdAt }}</small></div><button class="danger" @click="removeComment(item.game.id, item.comment.id)">删除评论</button></article>
    <p v-if="!allComments.length" class="empty">暂无评论</p>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { removeComment, removeGame, state } from '../stores'

const allComments = computed(() => Object.entries(state.comments).flatMap(([id, comments]) => comments.map((comment) => ({ key: `${id}-${comment.id}`, game: state.games.find((game) => game.id === Number(id)) || { title: '已删除游戏', id: Number(id) }, comment }))))
</script>
