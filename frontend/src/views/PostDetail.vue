<template>
  <section class="section page post-detail">
    <RouterLink to="/forum" class="back">← 返回论坛</RouterLink>
    <div v-if="post">
      <p class="eyebrow">COMMUNITY DISCUSSION</p><h1>{{ post.title }}</h1>
      <p class="post-meta"><RouterLink :to="`/users/${post.authorId}`" class="avatar-link"><span class="avatar">{{ post.authorAvatar || post.author[0] }}</span></RouterLink><RouterLink :to="`/users/${post.authorId}`">{{ post.author }}</RouterLink> · {{ post.createdAt }}</p>
      <div class="post-content">{{ post.body }}</div>
      <h2>讨论回复 <span class="reply-count">{{ post.comments?.length || 0 }}</span></h2>
      <form class="comment-form" @submit.prevent="submitReply"><input v-model="replyText" placeholder="写下你的回复..." required><button class="primary">回复</button></form>
      <p v-if="!post.comments?.length" class="empty">还没有回复，来参与讨论吧。</p>
      <article v-for="comment in post.comments" :key="comment.id" class="comment"><RouterLink :to="`/users/${comment.authorId}`" class="avatar-link"><span class="avatar">{{ comment.authorAvatar || comment.author[0] }}</span></RouterLink><div><strong><RouterLink :to="`/users/${comment.authorId}`">{{ comment.author }}</RouterLink></strong><small>{{ comment.createdAt }}</small><p>{{ comment.text }}</p></div></article>
    </div>
    <p v-else class="empty">帖子不存在</p>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { addPostReply, state } from '../stores'

const route = useRoute()
const post = computed(() => state.posts.find((item) => item.id === Number(route.params.id)))
const replyText = ref('')
function submitReply() { if (!post.value || !replyText.value.trim()) return; addPostReply(post.value.id, replyText.value.trim()); replyText.value = '' }
</script>
