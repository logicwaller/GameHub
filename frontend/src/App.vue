<template>
  <div class="app-shell">
    <header class="topbar">
      <RouterLink class="brand" to="/"><span class="brand-mark">G</span><span>GameHub</span></RouterLink>
      <nav><RouterLink to="/">首页</RouterLink><RouterLink to="/games">发现游戏</RouterLink></nav>
      <div class="account"><template v-if="auth.user"><span class="welcome">你好，{{ auth.user.username }}</span><button class="text-button" @click="auth.logout">退出</button></template><template v-else><RouterLink to="/login">登录</RouterLink><RouterLink class="signup" to="/register">注册</RouterLink></template></div>
    </header>
    <main><RouterView /></main>
    <footer>GameHub · 发现下一个让你沉浸其中的游戏</footer>
  </div>
</template>
<script setup>
import { reactive } from 'vue'
const auth = reactive({ user: JSON.parse(localStorage.getItem('gamehub_user') || 'null'), logout() { localStorage.removeItem('gamehub_user'); localStorage.removeItem('gamehub_token'); localStorage.removeItem('gamehub_refresh_token'); this.user = null } })
</script>
