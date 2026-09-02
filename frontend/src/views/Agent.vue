<template>
  <section class="section page agent-page">
    <div class="agent-layout">
      <!-- 侧边栏：历史对话 -->
      <aside class="agent-sidebar">
        <div class="sidebar-header">
          <h2><span class="brand-mark">A</span> AI 攻略助手</h2>
          <button class="new-chat-btn" @click="newChat">
            <span>+</span> 新对话
          </button>
        </div>
        <div class="history-list">
          <div
            v-for="chat in chatHistory"
            :key="chat.id"
            class="history-item"
            :class="{ active: chat.id === currentChatId }"
            @click="switchChat(chat.id)"
          >
            <span class="history-icon">💬</span>
            <div class="history-info">
              <strong>{{ chat.title }}</strong>
              <small>{{ chat.updatedAt }}</small>
            </div>
            <button class="history-delete" @click.stop="deleteChat(chat.id)">✕</button>
          </div>
          <p v-if="!chatHistory.length" class="empty-history">暂无历史对话</p>
        </div>
      </aside>

      <!-- 主区域：对话框 -->
      <main class="agent-main">
        <!-- 消息列表 -->
        <div class="message-list" ref="messageList">
          <div v-for="(msg, idx) in currentMessages" :key="idx" class="message" :class="msg.role">
            <div class="message-avatar">
              <span v-if="msg.role === 'user'">{{ userAvatar }}</span>
              <span v-else class="bot-avatar">A</span>
            </div>
            <div class="message-content">
              <div class="message-text">{{ msg.text }}</div>
            </div>
          </div>
          <p v-if="!currentMessages.length" class="empty-chat">
            开始向 AI 攻略助手提问吧！<br>
            例如：<em @click="sendQuickQuestion('这个游戏怎么快速上手？')">"这个游戏怎么快速上手？"</em>
          </p>
        </div>

        <!-- 输入框 -->
        <div class="chat-input-bar">
          <input
            v-model="inputText"
            placeholder="输入你的问题..."
            @keydown.enter.prevent="sendMessage"
            :disabled="loading"
          />
          <button class="primary" @click="sendMessage" :disabled="loading || !inputText.trim()">
            <span v-if="loading">● ● ●</span>
            <span v-else>发送</span>
          </button>
        </div>
      </main>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, nextTick } from 'vue'
import { state } from '../stores'

const inputText = ref('')
const loading = ref(false)
const messageList = ref(null)
const currentChatId = ref(null)

const userAvatar = computed(() => state.user?.username?.[0] || '访')

// 历史对话数据
const chatHistory = ref([
  { id: 1, title: '艾尔登法环入门攻略', updatedAt: '2026-09-01' },
  { id: 2, title: '星露谷物语耕种技巧', updatedAt: '2026-08-30' }
])

// 每个对话的消息
const chatMessages = ref({
  1: [
    { role: 'assistant', text: '你好！欢迎来到艾尔登法环攻略助手，有什么可以帮你的？' },
    { role: 'user', text: '开局选什么职业比较好？' },
    { role: 'assistant', text: '对于新手玩家，推荐选择**流浪骑士**（Vagabond），初始生命力高、有100%物防盾，容错率大。如果想玩法系，可以选择**观星者**（Astrologer），初始智力高、自带远程魔法。' }
  ],
  2: [
    { role: 'assistant', text: '嗨！星露谷物语攻略助手为你服务~' }
  ]
})

if (!currentChatId.value && chatHistory.value.length) {
  currentChatId.value = chatHistory.value[0].id
}

const currentMessages = computed(() => chatMessages.value[currentChatId.value] || [])

function newChat() {
  const id = Date.now()
  chatHistory.value.unshift({ id, title: '新对话', updatedAt: new Date().toLocaleDateString() })
  chatMessages.value[id] = [{ role: 'assistant', text: '你好！我是AI攻略助手，有什么游戏问题需要帮忙吗？' }]
  currentChatId.value = id
}

function switchChat(id) {
  currentChatId.value = id
}

function deleteChat(id) {
  const idx = chatHistory.value.findIndex(c => c.id === id)
  if (idx < 0) return
  chatHistory.value.splice(idx, 1)
  delete chatMessages.value[id]
  if (currentChatId.value === id) {
    currentChatId.value = chatHistory.value.length ? chatHistory.value[0].id : null
  }
}

async function sendMessage() {
  const text = inputText.value.trim()
  if (!text || loading.value) return
  inputText.value = ''

  if (!currentChatId.value) newChat()
  const chatId = currentChatId.value

  chatMessages.value[chatId].push({ role: 'user', text })
  scrollToBottom()

  loading.value = true
  // 模拟 AI 回复（后续对接真实 API）
  setTimeout(() => {
    chatMessages.value[chatId].push({
      role: 'assistant',
      text: '这是一个模拟回复。后续接入后端 AI 接口后将返回真实攻略回答。'
    })
    // 更新标题
    const chat = chatHistory.value.find(c => c.id === chatId)
    if (chat && chat.title === '新对话') {
      chat.title = text.length > 12 ? text.slice(0, 12) + '...' : text
    }
    loading.value = false
    scrollToBottom()
  }, 800)
}

function sendQuickQuestion(text) {
  inputText.value = text
  sendMessage()
}

function scrollToBottom() {
  nextTick(() => {
    if (messageList.value) {
      messageList.value.scrollTop = messageList.value.scrollHeight
    }
  })
}
</script>

<style scoped>
.agent-page {
  padding-top: 40px;
  height: calc(100vh - 140px);
}
.agent-layout {
  display: grid;
  grid-template-columns: 280px 1fr;
  height: 100%;
  border: 1px solid #2b2f30;
  border-radius: 8px;
  overflow: hidden;
  background: #191c1e;
}

/* 侧边栏 */
.agent-sidebar {
  background: #15181a;
  border-right: 1px solid #2b2f30;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.sidebar-header {
  padding: 18px 16px;
  border-bottom: 1px solid #2b2f30;
}
.sidebar-header h2 {
  font-size: 14px;
  margin: 0 0 14px;
  display: flex;
  align-items: center;
  gap: 8px;
  letter-spacing: -.02em;
}
.sidebar-header .brand-mark {
  width: 24px;
  height: 24px;
  background: #d4f34a;
  color: #101200;
  display: inline-grid;
  place-items: center;
  border-radius: 5px;
  font-size: 13px;
  font-weight: 800;
}
.new-chat-btn {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  background: #d4f34a;
  color: #121500;
  border: 0;
  border-radius: 4px;
  padding: 10px;
  font-weight: 700;
  font-size: 13px;
  cursor: pointer;
}
.new-chat-btn span {
  font-size: 16px;
}

.history-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}
.history-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 8px;
  border-radius: 5px;
  cursor: pointer;
  margin-bottom: 2px;
}
.history-item:hover,
.history-item.active {
  background: #23282a;
}
.history-icon {
  font-size: 14px;
}
.history-info {
  flex: 1;
  min-width: 0;
}
.history-info strong {
  display: block;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.history-info small {
  display: block;
  font-size: 10px;
  color: #6f7778;
  margin-top: 2px;
}
.history-delete {
  background: none;
  border: 0;
  color: #6f7778;
  cursor: pointer;
  font-size: 11px;
  padding: 4px;
  opacity: 0;
}
.history-item:hover .history-delete {
  opacity: 1;
}
.empty-history {
  color: #6f7778;
  font-size: 12px;
  text-align: center;
  padding: 30px 0;
}

/* 主区域 */
.agent-main {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.message-list {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.message {
  display: flex;
  gap: 12px;
  max-width: 85%;
}
.message.user {
  align-self: flex-end;
  flex-direction: row-reverse;
}
.message-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: #35402a;
  color: #d4f34a;
  display: grid;
  place-items: center;
  font-size: 13px;
  font-weight: 700;
  flex-shrink: 0;
}
.bot-avatar {
  background: #2a3540;
  color: #4ad3f3;
}
.message-content {
  background: #23282a;
  padding: 12px 16px;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.7;
  color: #c0c5c6;
}
.message.user .message-content {
  background: #2a3d20;
  color: #e0f0d0;
}
.message-text {
  white-space: pre-wrap;
}
.empty-chat {
  text-align: center;
  color: #6f7778;
  font-size: 14px;
  margin: auto;
  line-height: 2;
}
.empty-chat em {
  color: #d4f34a;
  cursor: pointer;
  font-style: normal;
}

/* 输入栏 */
.chat-input-bar {
  display: flex;
  gap: 10px;
  padding: 16px 20px;
  border-top: 1px solid #2b2f30;
  background: #15181a;
}
.chat-input-bar input {
  flex: 1;
  background: #111315;
  border: 1px solid #353a3b;
  border-radius: 4px;
  padding: 12px 14px;
  color: #fff;
  font: 13px Manrope;
  outline: 0;
}
.chat-input-bar input:focus {
  border-color: #d4f34a;
}
.chat-input-bar .primary {
  padding: 12px 20px;
  gap: 0;
  min-width: 70px;
}

@media (max-width: 700px) {
  .agent-layout {
    grid-template-columns: 1fr;
  }
  .agent-sidebar {
    display: none;
  }
  .agent-page {
    height: calc(100vh - 120px);
  }
}
</style>