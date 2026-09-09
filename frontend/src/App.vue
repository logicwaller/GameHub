<template>
  <div class="app-shell">
    <header class="topbar">
      <RouterLink class="brand" to="/">
        <span class="brand-mark">G</span>
        <span>GameHub</span>
      </RouterLink>
      <nav>
        <RouterLink to="/">首页</RouterLink>
        <RouterLink to="/games">发现游戏</RouterLink>
        <RouterLink to="/forum">论坛</RouterLink>
        <RouterLink to="/agent">AI攻略</RouterLink>
      </nav>
      <div class="account">
        <template v-if="auth.user">
          <RouterLink class="profile-link" to="/profile">
            <span class="avatar">{{ auth.user.username[0] }}</span>
            <span>{{ auth.user.username }}</span>
          </RouterLink>
          <RouterLink class="notification-link" to="/notifications" aria-label="查看通知">
            ♢
            <i v-if="unreadNotifications"></i>
          </RouterLink>
          <button class="text-button" @click="logoutAndReturnHome">退出</button>
        </template>
        <template v-else>
          <RouterLink to="/login">登录</RouterLink>
          <RouterLink class="signup" to="/register">注册</RouterLink>
        </template>
      </div>
    </header>
    <main>
      <RouterView />
    </main>
    <footer>GameHub · 发现下一个让你沉浸其中的游戏</footer>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  loadNotifications,
  loadRelations,
  logout,
  refreshHomeData,
  state,
  syncUserFromStorage
} from './stores'

const router = useRouter()

const auth = {
  get user() {
    return state.user
  }
}

const unreadNotifications = computed(() => state.notifications.filter((item) => !item.read).length)

async function logoutAndReturnHome() {
  logout()
  await router.replace('/')
  void refreshHomeData()
}

function syncAuthAcrossTabs(event) {
  if (event.key === 'gamehub_user') {
    syncUserFromStorage()
  }
}

function redirectToLogin() {
  if (router.currentRoute.value.path !== '/login') {
    void router.replace('/login')
  }
}

watch(
  () => state.user?.id,
  (userID) => {
    state.notifications.splice(0, state.notifications.length)
    if (!userID) return

    void loadRelations()
    void loadNotifications()
  },
  { immediate: true }
)

onMounted(() => {
  window.addEventListener('storage', syncAuthAcrossTabs)
  window.addEventListener('gamehub:auth-expired', redirectToLogin)
  void refreshHomeData()
})

onBeforeUnmount(() => {
  window.removeEventListener('storage', syncAuthAcrossTabs)
  window.removeEventListener('gamehub:auth-expired', redirectToLogin)
})
</script>
