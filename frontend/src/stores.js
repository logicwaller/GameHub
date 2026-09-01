import { computed, reactive } from 'vue'

const seedGames = [
  { id: 1, title: '星际拓荒者', description: '在无尽宇宙中建立你的新家园。', category: '策略', type: 'STRATEGY', icon: '✦', color: 'cover-purple', plays: 12400, likes: 928, favoritesCount: 486, playTime: '约 20 分钟', url: 'https://example.com', author: 'GameHub 编辑', authorId: 'editor', authorAvatar: 'G' },
  { id: 2, title: '迷雾之城', description: '解开城市深处被隐藏的谜题。', category: '解谜', type: 'PUZZLE', icon: '◈', color: 'cover-blue', plays: 8700, likes: 711, favoritesCount: 352, playTime: '约 35 分钟', url: 'https://example.com', author: 'GameHub 编辑', authorId: 'editor', authorAvatar: 'G' },
  { id: 3, title: '像素冲锋', description: '快节奏的像素射击大乱斗。', category: '动作', type: 'ACTION', icon: '✹', color: 'cover-orange', plays: 6200, likes: 582, favoritesCount: 240, playTime: '约 10 分钟', url: 'https://example.com', author: 'GameHub 编辑', authorId: 'editor', authorAvatar: 'G' },
  { id: 4, title: '悠然小镇', description: '打造你的理想小镇，享受慢生活。', category: '休闲', type: 'CASUAL', icon: '❋', color: 'cover-green', plays: 5100, likes: 403, favoritesCount: 198, playTime: '约 15 分钟', url: 'https://example.com', author: 'GameHub 编辑', authorId: 'editor', authorAvatar: 'G' }
]

function load(key, fallback) {
  try { return JSON.parse(localStorage.getItem(key)) || fallback } catch { return fallback }
}

const storedPosts = load('gamehub_posts', [{ id: 1, title: '大家最近都在玩什么？', body: '来分享一下你最近发现的宝藏网页游戏吧。', author: 'GameHub 编辑', authorId: 'editor', authorAvatar: 'G', createdAt: '今天', comments: [] }])
storedPosts.forEach((post) => { post.comments = Array.isArray(post.comments) ? post.comments : []; post.replies = post.comments.length })

export const state = reactive({
  user: load('gamehub_user', null),
  games: load('gamehub_games', seedGames),
  favorites: load('gamehub_favorites', []),
  liked: load('gamehub_liked', []),
  comments: load('gamehub_comments', {}),
  posts: storedPosts
})

export const isLoggedIn = computed(() => !!state.user)
export const isAdmin = computed(() => state.user?.role === 'admin' || state.user?.username === 'admin')
function persist(key, value) { localStorage.setItem(key, JSON.stringify(value)) }
export function setUser(user, token, refresh) { state.user = user; if (user) localStorage.setItem('gamehub_user', JSON.stringify(user)); else localStorage.removeItem('gamehub_user'); if (token) localStorage.setItem('gamehub_token', token); if (refresh) localStorage.setItem('gamehub_refresh_token', refresh) }
export function logout() { setUser(null); localStorage.removeItem('gamehub_token'); localStorage.removeItem('gamehub_refresh_token') }
export function toggleFavorite(id) { const index = state.favorites.indexOf(id); const game = state.games.find((item) => item.id === id); index >= 0 ? state.favorites.splice(index, 1) : state.favorites.push(id); if (game) game.favoritesCount = Math.max(0, (game.favoritesCount || 0) + (index >= 0 ? -1 : 1)); persist('gamehub_favorites', state.favorites); persist('gamehub_games', state.games) }
export function toggleLike(id) { const index = state.liked.indexOf(id); const game = state.games.find((item) => item.id === id); if (index >= 0) { state.liked.splice(index, 1); if (game) game.likes-- } else { state.liked.push(id); if (game) game.likes++ }; persist('gamehub_liked', state.liked); persist('gamehub_games', state.games) }
export function addComment(id, text) { if (!state.comments[id]) state.comments[id] = []; state.comments[id].unshift({ id: Date.now(), text, author: state.user?.username || '访客', authorId: state.user?.username || 'guest', authorAvatar: state.user?.username?.[0] || '访', createdAt: '刚刚' }); persist('gamehub_comments', state.comments) }
export function addPost(title, body) { state.posts.unshift({ id: Date.now(), title, body, author: state.user?.username || '访客', authorId: state.user?.username || 'guest', authorAvatar: state.user?.username?.[0] || '访', replies: 0, createdAt: '刚刚', comments: [] }); persist('gamehub_posts', state.posts) }
export function addPostReply(postId, text) { const post = state.posts.find((item) => item.id === postId); if (!post) return; if (!post.comments) post.comments = []; post.comments.push({ id: Date.now(), text, author: state.user?.username || '访客', authorId: state.user?.username || 'guest', authorAvatar: state.user?.username?.[0] || '访', createdAt: '刚刚' }); post.replies = post.comments.length; persist('gamehub_posts', state.posts) }
export function addGame(game) { const id = Math.max(0, ...state.games.map((item) => item.id)) + 1; state.games.push({ ...game, id, plays: 0, likes: 0, favoritesCount: 0, icon: '✦', type: game.category.toUpperCase(), color: 'cover-purple', author: state.user?.username || '访客', authorId: state.user?.username || 'guest', authorAvatar: state.user?.username?.[0] || '访' }); persist('gamehub_games', state.games) }
export function removeGame(id) { const index = state.games.findIndex((game) => game.id === id); if (index >= 0) state.games.splice(index, 1); persist('gamehub_games', state.games) }
export function removeComment(gameId, commentId) { if (state.comments[gameId]) state.comments[gameId] = state.comments[gameId].filter((comment) => comment.id !== commentId); persist('gamehub_comments', state.comments) }
