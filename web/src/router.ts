import { createRouter, createWebHistory } from 'vue-router'
import AboutView from './views/AboutView.vue'
import DashboardView from './views/DashboardView.vue'
import DeviceView from './views/DeviceView.vue'
import DiagnosticsView from './views/DiagnosticsView.vue'
import LoginView from './views/LoginView.vue'
import LogsView from './views/LogsView.vue'
import NotificationsView from './views/NotificationsView.vue'
import SettingsView from './views/SettingsView.vue'
import SignalView from './views/SignalView.vue'
import SIMView from './views/SIMView.vue'
import SMSView from './views/SMSView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/sms' },
    { path: '/dashboard', name: 'dashboard', component: DashboardView },
    { path: '/login', name: 'login', component: LoginView },
    { path: '/devices', name: 'devices', component: DeviceView },
    { path: '/sim', name: 'sim', component: SIMView },
    { path: '/signal', name: 'signal', component: SignalView },
    { path: '/sms', name: 'sms', component: SMSView },
    { path: '/notifications', name: 'notifications', component: NotificationsView },
    { path: '/logs', name: 'logs', component: LogsView },
    { path: '/diagnostics', name: 'diagnostics', component: DiagnosticsView },
    { path: '/settings', name: 'settings', component: SettingsView },
    { path: '/about', name: 'about', component: AboutView },
    { path: '/:pathMatch(.*)*', redirect: '/' }
  ]
})

export default router
