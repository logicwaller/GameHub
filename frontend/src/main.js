import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import Home from './views/Home.vue'
import Games from './views/Games.vue'
import GameDetail from './views/GameDetail.vue'
import Login from './views/Login.vue'
import Register from './views/Register.vue'
import './style.css'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Home },
    { path: '/games', component: Games },
    { path: '/games/:id', component: GameDetail },
    { path: '/login', component: Login },
    { path: '/register', component: Register }
  ]
})

createApp(App).use(router).mount('#app')
