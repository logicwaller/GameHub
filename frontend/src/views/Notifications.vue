<template>
  <section class="section page">
    <div class="section-head">
      <div>
        <p class="eyebrow">NOTIFICATION CENTER</p>
        <h1>通知</h1>
      </div>
      <button v-if="unreadCount" class="notification-read-all" @click="markAllNotificationsRead">
        <span aria-hidden="true">✓</span>
        全部标为已读
      </button>
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
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  loadNotifications,
  markAllNotificationsRead,
  markNotificationRead,
  state
} from '../stores'

const unreadCount = computed(() => state.notifications.filter((item) => !item.read).length)
const router = useRouter()
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

onMounted(loadNotifications)
</script>
