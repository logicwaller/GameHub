import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import Home from './views/Home.vue'
import Games from './views/Games.vue'
import GameDetail from './views/GameDetail.vue'
import Login from './views/Login.vue'
import Register from './views/Register.vue'
import Forum from './views/Forum.vue'
import Profile from './views/Profile.vue'
import Admin from './views/Admin.vue'
import UserProfile from './views/UserProfile.vue'
import PostDetail from './views/PostDetail.vue'
import Analytics from './views/Analytics.vue'
import Agent from './views/Agent.vue'
import './style.css'
import './decorations.css'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Home },
    { path: '/games', component: Games },
    { path: '/games/:id', component: GameDetail },
    { path: '/login', component: Login },
    { path: '/register', component: Register },
    { path: '/forum', component: Forum },
    { path: '/forum/:id', component: PostDetail },
    { path: '/profile', component: Profile },
    { path: '/profile/games', component: Analytics },
    { path: '/users/:id', component: UserProfile },
    { path: '/admin', component: Admin },
    { path: '/agent', component: Agent }
  ]
})

createApp(App).use(router).mount('#app')
