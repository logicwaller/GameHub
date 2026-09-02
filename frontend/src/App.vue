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
      </nav>
      <div class="account">
        <template v-if="auth.user">
          <RouterLink class="profile-link" to="/profile">
            <span class="avatar">{{ auth.user.username[0] }}</span>
            <span>{{ auth.user.username }}</span>
          </RouterLink>
          <button class="text-button" @click="auth.logout">退出</button>
        </template>
        <template v-else>
          <RouterLink to="/login">登录</RouterLink>
          <RouterLink class="signup" to="/register">注册</RouterLink>
        </template>
      </div>
    </header>
    <main><RouterView /></main>
    <footer>GameHub · 发现下一个让你沉浸其中的游戏</footer>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { loadGames, loadPosts, state, logout } from './stores'

const auth = {
  get user() {
    return state.user
  },
  logout
}

onMounted(() => { loadGames(); loadPosts() })
</script>
