<template>
  <section class="section page">
    <div class="section-head"><div><p class="eyebrow">PLAYER COMMUNITY</p><h1>玩家论坛</h1></div><button class="primary" @click="show = !show">{{ show ? '收起发帖' : '发布新帖' }} <span>+</span></button></div>
    <form v-if="show" class="post-form" @submit.prevent="submit"><input v-model="title" placeholder="帖子标题" required><textarea v-model="body" placeholder="写下你的想法..." required></textarea><button class="primary">发布帖子</button></form>
    <article v-for="post in state.posts" :key="post.id" class="post"><div><RouterLink :to="`/users/${post.authorId}`" class="avatar-link"><span class="avatar">{{ post.authorAvatar || post.author[0] }}</span></RouterLink></div><div class="post-body"><RouterLink :to="`/forum/${post.id}`"><h3>{{ post.title }}</h3><p>{{ post.body }}</p></RouterLink><small><RouterLink :to="`/users/${post.authorId}`">{{ post.author }}</RouterLink> · {{ post.createdAt }}</small></div><strong class="replies">{{ post.comments?.length || 0 }} 回复</strong></article>
  </section>
</template>
<script setup>
import { ref } from 'vue'; import { addPost, state } from '../stores'
const show = ref(false); const title = ref(''); const body = ref(''); function submit() { addPost(title.value, body.value); title.value = ''; body.value = ''; show.value = false }
</script>
