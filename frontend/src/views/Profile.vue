<template>
  <section class="section page">
    <div v-if="state.user" class="profile-head">
      <span class="avatar avatar-large">{{ state.user.username[0] }}</span>
      <div>
        <p class="eyebrow">YOUR GAMEHUB PROFILE</p>
        <h1>{{ state.user.username }}</h1>
        <p>{{ state.user.email }}</p>
      </div>
      <RouterLink v-if="isAdmin" to="/admin" class="secondary">管理员中心 →</RouterLink>
    </div>

    <div class="profile-grid">
      <div>
        <div class="section-head">
          <div>
            <p class="eyebrow">YOUR COLLECTION</p>
            <h2>收藏的游戏</h2>
          </div>
          <span class="count">{{ favorites.length }} 款</span>
        </div>
        <div class="game-grid">
          <RouterLink
            v-for="game in favorites"
            :key="game.id"
            :to="`/games/${game.id}`"
            class="game-card"
          >
            <div class="game-cover" :class="game.color" :style="coverStyle(game)">
              <img v-if="game.cover" :src="game.cover" :alt="`${game.title} 封面`">
              <span v-else>{{ game.icon }}</span>
            </div>
            <div class="game-info">
              <h3>{{ game.title }}</h3>
              <p>{{ game.description }}</p>
              <span class="tag">{{ game.primaryType }}</span>
            </div>
          </RouterLink>
        </div>
        <p v-if="!favorites.length" class="empty">还没有收藏游戏，去发现游戏看看吧。</p>
      </div>

      <div class="profile-secondary">
        <div class="profile-section">
          <p class="eyebrow">YOUR ACTIVITY</p>
          <h2>近期点赞</h2>
          <RouterLink
            v-for="game in likedGames"
            :key="game.id"
            :to="`/games/${game.id}`"
            class="activity-link"
          >
            {{ game.title }} <span>♥</span>
          </RouterLink>
          <p v-if="!likedGames.length" class="empty">暂无点赞记录</p>
        </div>
        <RouterLink class="secondary analytics-link" to="/profile/games">
          查看已发布游戏数据 →
        </RouterLink>
        <button class="primary full" @click="showCreate = true">创建游戏 <span>＋</span></button>
      </div>
    </div>
  </section>

  <div v-if="showCreate" class="modal-backdrop" @click.self="showCreate = false">
    <form class="modal create-form" @submit.prevent="submit">
      <button type="button" class="modal-close" @click="showCreate = false">×</button>
      <p class="eyebrow">CREATOR STUDIO</p>
      <h2>创建游戏</h2>
      <label class="create-field">
        游戏标题
        <input
          v-model="form.title"
          maxlength="100"
          placeholder="游戏标题"
          required
        >
        <small>{{ form.title.length }}/100</small>
      </label>
      <label class="create-field">
        游戏简介
        <textarea
          v-model="form.description"
          maxlength="2000"
          placeholder="游戏简介"
          required
        ></textarea>
        <small>{{ form.description.length }}/2000</small>
      </label>
      <select v-model="form.primaryType" required>
        <option disabled value="">选择主类型</option>
        <option v-for="type in primaryTypes" :key="type" :value="type">{{ type }}</option>
      </select>
      <label class="create-field">
        标签（可选，以逗号分隔，最多 12 个）
        <input
          v-model="form.tagsText"
          maxlength="299"
          placeholder="如：中式民俗、档案推理、轻恐"
        >
        <small>{{ form.tagsText.length }}/299</small>
      </label>
      <label class="create-field">
        预计游玩时间
        <input
          v-model="form.playTime"
          maxlength="50"
          placeholder="预计游玩时间"
          required
        >
        <small>{{ form.playTime.length }}/50</small>
      </label>
      <label class="create-field">
        网页链接
        <input
          v-model="form.url"
          maxlength="500"
          type="url"
          placeholder="网页链接"
          required
        >
        <small>{{ form.url.length }}/500</small>
      </label>
      <label
        class="cover-dropzone"
        :class="{ dragging: isDragging, 'has-cover': form.cover }"
        @dragover.prevent="isDragging = true"
        @dragleave.prevent="isDragging = false"
        @drop.prevent="dropCover"
      >
        <input
          type="file"
          accept="image/*"
          @change="uploadCover"
        >
        <template v-if="form.cover">
          <img class="cover-preview" :src="form.cover" alt="游戏封面预览">
          <span class="cover-overlay">点击或拖入图片更换封面</span>
        </template>
        <template v-else>
          <span class="upload-icon" aria-hidden="true">↑</span>
          <strong>拖动图片到这里上传封面</strong>
          <small>或点击选择图片 · 支持 JPG、PNG、GIF、WEBP</small>
        </template>
      </label>
      <button
        v-if="form.cover"
        type="button"
        class="clear-cover"
        @click="form.cover = ''"
      >
        移除封面
      </button>
      <p v-if="error" class="error">{{ error }}</p>
      <button class="primary full" :disabled="submitting">
        {{ submitting ? '提交中...' : '提交游戏' }} <span>→</span>
      </button>
    </form>
  </div>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { addGame, coverStyle, isAdmin, state } from '../stores'

const primaryTypes = ['ARG/WIG', '现实互动解谜', '网页互动游戏', '网页解谜', '互动叙事']
const favorites = computed(() => state.games.filter((game) => state.favorites.includes(game.id)))
const likedGames = computed(() => state.games.filter((game) => state.liked.includes(game.id)))
const showCreate = ref(false)
const submitting = ref(false)
const error = ref('')
const isDragging = ref(false)
const form = reactive({
  title: '',
  description: '',
  primaryType: '',
  tagsText: '',
  playTime: '',
  url: '',
  cover: ''
})

function readCover(file) {
  if (!file) return
  isDragging.value = false
  if (!file.type.startsWith('image/')) {
    error.value = '请选择图片文件'
    return
  }
  if (file.size > 5 * 1024 * 1024) {
    error.value = '封面图片不能超过 5MB'
    return
  }
  error.value = ''
  compressCover(file)
    .then((cover) => {
      if (cover.length > 3 * 1024 * 1024) {
        throw new Error('图片压缩后仍然过大')
      }
      form.cover = cover
    })
    .catch(() => {
      error.value = '图片过大或读取失败，请选择较小的图片'
    })
}

function compressCover(file) {
  return new Promise((resolve, reject) => {
    const image = new Image()
    const objectURL = URL.createObjectURL(file)
    image.onload = () => {
      URL.revokeObjectURL(objectURL)
      const maxSide = 1600
      const scale = Math.min(1, maxSide / Math.max(image.width, image.height))
      const canvas = document.createElement('canvas')
      canvas.width = Math.max(1, Math.round(image.width * scale))
      canvas.height = Math.max(1, Math.round(image.height * scale))
      const context = canvas.getContext('2d')
      if (!context) {
        reject(new Error('浏览器不支持图片处理'))
        return
      }
      context.drawImage(image, 0, 0, canvas.width, canvas.height)
      resolve(canvas.toDataURL('image/jpeg', 0.82))
    }
    image.onerror = () => {
      URL.revokeObjectURL(objectURL)
      reject(new Error('图片读取失败'))
    }
    image.src = objectURL
  })
}

function uploadCover(event) {
  readCover(event.target.files?.[0])
  event.target.value = ''
}

function dropCover(event) {
  readCover(event.dataTransfer.files?.[0])
}

function resetForm() {
  Object.assign(form, {
    title: '',
    description: '',
    primaryType: '',
    tagsText: '',
    playTime: '',
    url: '',
    cover: ''
  })
  isDragging.value = false
}

async function submit() {
  submitting.value = true
  error.value = ''
  try {
    await addGame({
      title: form.title,
      description: form.description,
      primaryType: form.primaryType,
      tags: form.tagsText.split(/[,，]/).map((tag) => tag.trim()).filter(Boolean),
      playTime: form.playTime,
      url: form.url,
      cover: form.cover
    })
    resetForm()
    showCreate.value = false
  } catch (err) {
    error.value = err.message || '创建游戏失败'
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.cover-dropzone {
  position: relative;
  display: grid;
  min-height: 190px;
  align-content: center;
  justify-items: center;
  gap: 10px;
  overflow: hidden;
  border: 1px dashed #4a5352;
  border-radius: 6px;
  background: #111315;
  color: #aeb5b6;
  text-align: center;
  cursor: pointer;
  transition: border-color .2s, background .2s;
}

.cover-dropzone:hover,
.cover-dropzone.dragging {
  border-color: #d4f34a;
  background: #1b211b;
}

.cover-dropzone input {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
  white-space: nowrap;
}

.upload-icon {
  display: grid;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 1px solid #53603b;
  border-radius: 50%;
  color: #d4f34a;
  font-size: 24px;
}

.cover-dropzone strong {
  color: #e2e7e5;
  font-size: 14px;
}

.cover-dropzone small {
  color: #737c7d;
  font-size: 11px;
}

.cover-dropzone.has-cover {
  min-height: 220px;
}

.cover-dropzone .cover-preview {
  width: 100%;
  height: 220px;
  max-height: none;
  border: 0;
  border-radius: 0;
  object-fit: fill;
}

.cover-overlay {
  position: absolute;
  inset: auto 0 0;
  padding: 12px;
  background: #080a0acc;
  color: #d4f34a;
  font-size: 11px;
  opacity: 0;
  transition: opacity .2s;
}

.cover-dropzone.has-cover:hover .cover-overlay {
  opacity: 1;
}

.clear-cover {
  justify-self: start;
  border: 0;
  background: transparent;
  color: #8d9696;
  cursor: pointer;
  font: 11px Manrope, Arial, sans-serif;
}

.clear-cover:hover {
  color: #f28c7f;
}

.create-field small {
  color: #737c7d;
  font-size: 10px;
  text-align: right;
}
</style>
