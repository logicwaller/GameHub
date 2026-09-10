<template>
  <section class="section page">
    <div class="section-head">
      <div>
        <p class="eyebrow">NOTIFICATION CENTER</p>
        <h1>通知</h1>
      </div>
      <div class="notification-actions">
        <button
          v-if="unreadCount"
          class="notification-read-all"
          @click="markAllNotificationsRead"
        >
          <span aria-hidden="true">✓</span>
          全部标为已读
        </button>
        <button
          v-if="state.notifications.length"
          class="notification-delete-all"
          :disabled="deleting"
          @click="deleteAll"
        >
          <span aria-hidden="true">⌫</span>
          {{ deleting ? '删除中...' : '删除全部通知' }}
        </button>
      </div>
    </div>

    <div v-if="state.notifications.length" class="notification-list">
      <section v-for="group in groupedNotifications" :key="group.date" class="notification-group">
        <h2>{{ group.label }}</h2>
        <button
          v-for="item in group.items"
          :key="item.id"
          class="notification-item"
          :class="{ unread: !item.read }"
          @click="openNotification(item)"
        >
          <span class="notification-dot"></span>
          <span>
            <strong>{{ label(item.type) }}</strong>
            <span>{{ item.content }}</span>
            <small>{{ item.createdAt }}</small>
          </span>
        </button>
      </section>
    </div>
    <p v-else class="empty">暂时没有通知。</p>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  loadNotifications,
  markAllNotificationsRead,
  markNotificationRead,
  removeAllNotifications,
  state
} from '../stores'

const unreadCount = computed(() => state.notifications.filter((item) => !item.read).length)
const router = useRouter()
const deleting = ref(false)
const groupedNotifications = computed(() => {
  const groups = new Map()
  state.notifications.forEach((item) => {
    const date = item.createdAt?.slice(0, 10) || '未知日期'
    if (!groups.has(date)) groups.set(date, [])
    groups.get(date).push(item)
  })
  return [...groups.entries()].map(([date, items]) => ({ date, label: formatDate(date), items }))
})

function label(type) {
  return {
    welcome: '欢迎加入',
    game_comment: '游戏评论',
    post_reply: '帖子回复'
  }[type] || '系统通知'
}

function formatDate(date) {
  if (date === '未知日期') return date
  const today = new Date().toISOString().slice(0, 10)
  const yesterday = new Date(Date.now() - 86400000).toISOString().slice(0, 10)
  if (date === today) return '今天'
  if (date === yesterday) return '昨天'
  return date.replace(/-/g, '/')
}

async function openNotification(item) {
  if (!item.read) {
    void markNotificationRead(item.id).catch(() => {})
  }

  if (item.targetType === 'game' && item.targetId) {
    await router.push(`/games/${item.targetId}`)
  } else if (item.targetType === 'post' && item.targetId) {
    await router.push(`/forum/${item.targetId}`)
  } else {
    await router.push('/profile')
  }
}

async function deleteAll() {
  if (deleting.value || !state.notifications.length) return
  if (!window.confirm('确定删除全部通知吗？')) return
  deleting.value = true
  try {
    await removeAllNotifications()
  } catch (error) {
    console.error('删除通知失败:', error)
  } finally {
    deleting.value = false
  }
}

onMounted(loadNotifications)
</script>

<style scoped>
.notification-actions {
  display: flex;
  align-items: center;
  gap: 9px;
}

.notification-read-all,
.notification-delete-all {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: 1px solid #353a3b;
  border-radius: 4px;
  padding: 9px 12px;
  cursor: pointer;
  font: 12px Manrope, Arial, sans-serif;
}

.notification-read-all {
  background: #d4f34a;
  border-color: #d4f34a;
  color: #121500;
}

.notification-delete-all {
  background: #1b1f20;
  color: #aeb5b6;
}

.notification-read-all:hover {
  background: #e1fa69;
}

.notification-delete-all:hover {
  border-color: #e07a70;
  color: #f0a098;
}

.notification-actions button:disabled {
  cursor: not-allowed;
  opacity: .55;
}

@media (max-width: 700px) {
  .notification-actions {
    width: 100%;
    flex-wrap: wrap;
  }
}
</style>
