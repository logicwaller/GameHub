<template>
  <section class="section page">
    <div v-if="state.user" class="profile-head">
      <span class="avatar avatar-large">{{ state.user.username[0] }}</span>
      <div><p class="eyebrow">YOUR GAMEHUB PROFILE</p><h1>{{ state.user.username }}</h1><p>{{ state.user.email }}</p></div>
      <RouterLink v-if="isAdmin" to="/admin" class="secondary">管理员中心 →</RouterLink>
    </div>
    <div class="profile-grid">
      <div>
        <div class="section-head"><div><p class="eyebrow">YOUR COLLECTION</p><h2>收藏的游戏</h2></div><span class="count">{{ favorites.length }} 款</span></div>
        <div class="game-grid"><RouterLink v-for="game in favorites" :key="game.id" :to="`/games/${game.id}`" class="game-card"><div class="game-cover" :class="game.color"><span>{{ game.icon }}</span></div><div class="game-info"><h3>{{ game.title }}</h3><p>{{ game.description }}</p></div></RouterLink></div>
        <p v-if="!favorites.length" class="empty">还没有收藏游戏，去发现游戏看看吧。</p>
      </div>
      <div class="profile-secondary">
        <div class="profile-section"><p class="eyebrow">YOUR ACTIVITY</p><h2>近期点赞</h2><RouterLink v-for="game in likedGames" :key="game.id" :to="`/games/${game.id}`" class="activity-link">{{ game.title }} <span>♥</span></RouterLink><p v-if="!likedGames.length" class="empty">暂无点赞记录</p></div>
        <RouterLink class="secondary analytics-link" to="/profile/games">查看已发布游戏数据 →</RouterLink>
        <button class="primary full" @click="showCreate = true">创建游戏 <span>＋</span></button>
      </div>
    </div>
  </section>
  <div v-if="showCreate" class="modal-backdrop" @click.self="showCreate = false">
      <form class="modal create-form" @submit.prevent="submit"><button type="button" class="modal-close" @click="showCreate = false">×</button><p class="eyebrow">CREATOR STUDIO</p><h2>创建游戏</h2><input v-model="form.title" placeholder="游戏标题" required><textarea v-model="form.description" placeholder="游戏简介" required></textarea><select v-model="form.category" required><option disabled value="">选择分类</option><option>动作</option><option>策略</option><option>休闲</option><option>解谜</option></select><input v-model="form.playTime" placeholder="预计游玩时间" required><input v-model="form.url" type="url" placeholder="网页链接" required><label class="cover-upload">游戏封面<input type="file" accept="image/*" @change="uploadCover"></label><img v-if="form.cover" class="cover-preview" :src="form.cover" alt="游戏封面预览"><p v-if="error" class="error">{{ error }}</p><button class="primary full" :disabled="submitting">{{ submitting ? '提交中...' : '提交游戏' }} <span>→</span></button></form>
  </div>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { addGame, isAdmin, state } from '../stores'
const favorites = computed(() => state.games.filter((game) => state.favorites.includes(game.id)))
const likedGames = computed(() => state.games.filter((game) => state.liked.includes(game.id)))
const showCreate = ref(false)
const submitting = ref(false)
const error = ref('')
const form = reactive({ title: '', description: '', category: '', playTime: '', url: '', cover: '' })
function uploadCover(event) { const file = event.target.files?.[0]; if (!file) return; const reader = new FileReader(); reader.onload = () => { form.cover = reader.result }; reader.readAsDataURL(file) }
async function submit() { submitting.value = true; error.value = ''; try { await addGame(form); Object.assign(form, { title: '', description: '', category: '', playTime: '', url: '', cover: '' }); showCreate.value = false } catch (err) { error.value = err.message || '创建游戏失败' } finally { submitting.value = false } }
</script>
