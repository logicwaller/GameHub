import { computed, reactive } from 'vue'

function load(key, fallback) {
  try {
    return JSON.parse(localStorage.getItem(key)) || fallback
  } catch {
    return fallback
  }
}

export const state = reactive({
  user: load('gamehub_user', null),
  games: [],
  favorites: load('gamehub_favorites', []),
  liked: load('gamehub_liked', []),
  comments: {},
  posts: []
})

export const isLoggedIn = computed(() => !!state.user)
export const isAdmin = computed(() => state.user?.role === 'admin' || state.user?.username === 'admin')

export function coverStyle(game) {
  if (game?.cover) return {}
  const seed = Math.abs(Number(game?.id || 1) * 37)
  const hue = seed % 360
  const angle = 25 + (seed % 100)
  const size = 24 + (seed % 28)
  const shape = seed % 4
  const backgrounds = [
    `radial-gradient(circle at 34% 28%, hsl(${(hue + 45) % 360}, 78%, 82%), hsl(${hue}, 48%, 48%) 48%, hsl(${(hue + 220) % 360}, 30%, 22%) 82%)`,
    `linear-gradient(${angle}deg, hsl(${(hue + 25) % 360}, 70%, 72%), hsl(${hue}, 52%, 38%))`,
    `conic-gradient(from ${angle}deg, hsl(${hue}, 70%, 66%), hsl(${(hue + 115) % 360}, 58%, 42%), hsl(${(hue + 220) % 360}, 48%, 30%), hsl(${hue}, 70%, 66%))`,
    `radial-gradient(circle at 65% 28%, hsl(${(hue + 75) % 360}, 72%, 72%), hsl(${hue}, 52%, 42%) 58%, hsl(${(hue + 180) % 360}, 36%, 28%))`
  ]
  const radii = ['50%', '18%', '12%', '55% 25% 48% 32%']
  const transforms = ['none', 'rotate(-12deg)', 'rotate(45deg) scale(.82)', 'rotate(14deg)']
  return {
    '--cover-orbit-angle': `${angle}deg`,
    '--cover-shape-radius': radii[shape],
    '--cover-shape-transform': transforms[shape],
    '--cover-subject-background': backgrounds[shape],
    '--cover-ring-display': shape === 0 ? 'block' : 'none',
    backgroundColor: `hsl(${hue}, 24%, 19%)`,
    backgroundImage: 'none',
    backgroundSize: `${size}px ${size}px`
  }
}

function requireLogin() {
  if (!state.user || !localStorage.getItem('gamehub_token')) {
    throw new Error('请先登录后再操作')
  }
}

async function postAuth(path, body, method = 'POST') {
  requireLogin()
  const response = await fetch(path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${localStorage.getItem('gamehub_token')}`
    },
    body: JSON.stringify(body || {})
  })
  if (response.status === 204) return {}
  const data = await response.json()
  if (!response.ok) throw new Error(data.message || '操作失败')
  return data
}

export async function loadGames() {
  try {
    const response = await fetch('/api/games')
    if (!response.ok) throw new Error('游戏接口不可用')
    const data = await response.json()
    state.games.splice(0, state.games.length, ...(data.items || []))
  } catch {
    state.games.splice(0, state.games.length)
  }
}

export async function loadPosts() {
  try {
    const response = await fetch('/api/posts')
    if (!response.ok) throw new Error('论坛接口不可用')
    const data = await response.json()
    state.posts.splice(0, state.posts.length, ...(data.items || []))
  } catch {
    state.posts.splice(0, state.posts.length)
  }
}

export async function loadGameComments(id) {
  const response = await fetch(`/api/games/${id}/comments`)
  const data = await response.json()
  if (!response.ok) throw new Error(data.message || '读取评论失败')
  state.comments[id] = data.items || []
}

export function setUser(user, token, refresh) {
  state.user = user
  if (user) localStorage.setItem('gamehub_user', JSON.stringify(user))
  else localStorage.removeItem('gamehub_user')
  if (token) localStorage.setItem('gamehub_token', token)
  if (refresh) localStorage.setItem('gamehub_refresh_token', refresh)
}

export function logout() {
  setUser(null)
  localStorage.removeItem('gamehub_token')
  localStorage.removeItem('gamehub_refresh_token')
}

export async function toggleFavorite(id) {
  const data = await postAuth(`/api/games/${id}/favorite`)
  const index = state.favorites.indexOf(id)
  if (data.favorite && index < 0) state.favorites.push(id)
  if (!data.favorite && index >= 0) state.favorites.splice(index, 1)
  const game = state.games.find((item) => item.id === id)
  if (game) game.favorites = Math.max(0, (game.favorites || 0) + (data.favorite ? 1 : -1))
  return data.favorite
}

export async function toggleLike(id) {
  const data = await postAuth(`/api/games/${id}/like`)
  const index = state.liked.indexOf(id)
  if (data.liked && index < 0) state.liked.push(id)
  if (!data.liked && index >= 0) state.liked.splice(index, 1)
  const game = state.games.find((item) => item.id === id)
  if (game) game.likes = Math.max(0, (game.likes || 0) + (data.liked ? 1 : -1))
  return data.liked
}

export async function addComment(id, text) {
  const data = await postAuth(`/api/games/${id}/comments`, { text })
  if (!state.comments[id]) state.comments[id] = []
  state.comments[id].unshift(data.comment)
  return data.comment
}

export async function addGame(game) {
  const data = await postAuth('/api/games', game)
  state.games.unshift(data.game)
  return data.game
}

export async function deleteGame(id) {
  await postAuth(`/api/admin/games/${id}`, undefined, 'DELETE')
  const index = state.games.findIndex((game) => game.id === id)
  if (index >= 0) state.games.splice(index, 1)
}

export async function deleteGameComment(gameId, commentId) {
  await postAuth(`/api/admin/games/${gameId}/comments/${commentId}`, undefined, 'DELETE')
  if (state.comments[gameId]) state.comments[gameId] = state.comments[gameId].filter((comment) => comment.id !== commentId)
}

export async function deletePost(postId) {
  await postAuth(`/api/admin/posts/${postId}`, undefined, 'DELETE')
  const index = state.posts.findIndex((post) => post.id === postId)
  if (index >= 0) state.posts.splice(index, 1)
}

export async function deletePostReply(postId, replyId) {
  await postAuth(`/api/admin/posts/${postId}/replies/${replyId}`, undefined, 'DELETE')
  const post = state.posts.find((item) => item.id === postId)
  if (post?.comments) post.comments = post.comments.filter((comment) => comment.id !== replyId)
}

export async function addPost(title, body) {
  const data = await postAuth('/api/posts', { title, body })
  state.posts.unshift(data.post)
  return data.post
}

export async function addPostReply(postId, text) {
  const data = await postAuth(`/api/posts/${postId}/replies`, { text })
  const index = state.posts.findIndex((item) => item.id === postId)
  if (index >= 0) state.posts[index] = data.post
  else state.posts.push(data.post)
  return data.post
}

export function removeGame(id) {
  const index = state.games.findIndex((game) => game.id === id)
  if (index >= 0) state.games.splice(index, 1)
}

export function removeComment(gameId, commentId) {
  if (!state.comments[gameId]) return
  state.comments[gameId] = state.comments[gameId].filter((comment) => comment.id !== commentId)
}
