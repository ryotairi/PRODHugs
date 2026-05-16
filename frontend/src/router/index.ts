import { createRouter, createWebHistory } from 'vue-router'
import { accessToken, ensureAccessToken } from '@/lib/token'
import { authApi } from '@/api/client'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { guest: true },
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/views/RegisterView.vue'),
      meta: { guest: true },
    },
    {
      path: '/',
      redirect: '/dashboard',
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('@/views/DashboardView.vue'),
      meta: { auth: true },
    },
    {
      path: '/users',
      name: 'users',
      component: () => import('@/views/UsersView.vue'),
      meta: { auth: true },
    },
    {
      // Canonical, shareable URL. The literal "@" disambiguates from the
      // legacy UUID route below; the component reads `route.params.username`
      // and resolves it via the v2 profile endpoint.
      path: '/user/@:username',
      name: 'user-profile-by-username',
      component: () => import('@/views/UserProfileView.vue'),
      meta: { auth: true, back: true },
    },
    {
      // Legacy: profile by UUID. The component performs a router.replace to
      // the @username URL once the profile is loaded.
      path: '/user/:id',
      name: 'user-profile',
      component: () => import('@/views/UserProfileView.vue'),
      meta: { auth: true, back: true },
    },
    {
      path: '/leaderboard',
      name: 'leaderboard',
      component: () => import('@/views/LeaderboardView.vue'),
      meta: { auth: true },
    },
    {
      path: '/connections',
      name: 'connections',
      component: () => import('@/views/ConnectionsView.vue'),
      meta: { auth: true },
    },
    {
      path: '/intimacy-leaderboard',
      redirect: '/leaderboard',
    },
    {
      path: '/profile',
      redirect: '/dashboard',
    },
    {
      path: '/feed',
      name: 'feed',
      component: () => import('@/views/FeedView.vue'),
      meta: { auth: true },
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import('@/views/AdminView.vue'),
      meta: { auth: true, admin: true, back: true },
    },
  ],
})

router.beforeEach(async (to, _from, next) => {
  let token = accessToken.value

  if (!token && to.meta.auth) {
    token = await ensureAccessToken()
  }

  if (to.meta.auth && !token) {
    next('/login')
  } else if (to.meta.guest && token) {
    next('/dashboard')
  } else if (to.meta.admin) {
    // Verify admin role against the server, not just localStorage.
    try {
      const res = await authApi.me()
      const user = res.data
      localStorage.setItem('user', JSON.stringify(user))
      if (user?.role !== 'admin') {
        next('/dashboard')
      } else {
        next()
      }
    } catch {
      next('/dashboard')
    }
  } else {
    next()
  }
})

export default router
