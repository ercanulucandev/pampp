<script setup>
import { computed, ref } from 'vue'
import { store, call, Api } from './store'
import Icon from './components/Icon.vue'
import Logo from './components/Logo.vue'
import Dashboard from './views/Dashboard.vue'
import Sites from './views/Sites.vue'
import Php from './views/Php.vue'
import Node from './views/Node.vue'
import Components from './views/Components.vue'
import Logs from './views/Logs.vue'
import Settings from './views/Settings.vue'

const view = ref('dashboard')
const nav = [
  { id: 'dashboard', label: 'Panel', icon: 'dashboard', comp: Dashboard },
  { id: 'sites', label: 'Siteler', icon: 'globe', comp: Sites },
  { id: 'php', label: 'PHP', icon: 'code', comp: Php },
  { id: 'node', label: 'Node.js', icon: 'hexagon', comp: Node },
  { id: 'components', label: 'Bileşenler', icon: 'package', comp: Components },
  { id: 'logs', label: 'Loglar', icon: 'logs', comp: Logs },
  { id: 'settings', label: 'Ayarlar', icon: 'settings', comp: Settings },
]
const current = computed(() => nav.find(n => n.id === view.value).comp)

const services = computed(() => store.state?.services || [])
const anyRunning = computed(() => services.value.some(s => s.running))
const allRunning = computed(() => services.value.filter(s => s.installed).every(s => s.running) && services.value.some(s => s.installed))
const tasks = computed(() => Object.values(store.tasks))

function toggleAll() {
  if (anyRunning.value) call('all', () => Api.StopAll(), 'Servisler durduruldu')
  else call('all', () => Api.StartAll(), 'Servisler başlatıldı')
}
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">
        <Logo :size="30" />
        <span>PAMPP</span>
      </div>

      <nav>
        <button v-for="n in nav" :key="n.id" class="nav-item" :class="{ active: view === n.id }" @click="view = n.id">
          <Icon :name="n.icon" />
          <span>{{ n.label }}</span>
          <span v-if="n.id === 'sites' && store.state" class="count">{{ store.state.sites.length }}</span>
          <span v-if="n.id === 'node' && store.state?.node.running" class="count live">{{ store.state.node.running }}</span>
        </button>
      </nav>

      <div class="side-foot">
        <div class="svc-mini" v-for="s in services" :key="s.id">
          <span class="dot" :class="{ on: s.running, err: s.error && !s.running }"></span>
          <span class="grow">{{ s.name }}</span>
          <span class="muted small">{{ s.running ? ':' + s.port : '—' }}</span>
        </div>
        <button class="btn" :class="anyRunning ? '' : 'primary'" style="width:100%; margin-top:10px"
                :disabled="store.busy.all" @click="toggleAll">
          <Icon :name="store.busy.all ? 'refresh' : anyRunning ? 'stop' : 'play'" :size="15" :class="{ spin: store.busy.all }" />
          {{ anyRunning ? (allRunning ? 'Tümünü durdur' : 'Durdur') : 'Tümünü başlat' }}
        </button>
      </div>
    </aside>

    <main class="content">
      <div class="content-inner" v-if="store.state">
        <component :is="current" @navigate="v => view = v" />
      </div>
    </main>

    <div class="dock">
      <div v-for="t in tasks" :key="t.id" class="task card" :class="{ error: t.error }">
        <div class="row">
          <Icon :name="t.error ? 'alert' : t.done ? 'check' : 'download'" :size="16" />
          <strong class="grow">{{ t.label }}</strong>
          <button class="btn ghost sm icon" @click="Api.DismissTask(t.id)"><Icon name="x" :size="14" /></button>
        </div>
        <p class="small muted" v-if="t.error">{{ t.error }}</p>
        <p class="small muted" v-else-if="t.stage">{{ t.stage }}</p>
        <div class="bar" v-if="!t.done">
          <div :class="{ indeterminate: t.progress < 0 }" :style="t.progress >= 0 ? { width: (t.progress * 100).toFixed(1) + '%' } : {}"></div>
        </div>
      </div>
    </div>

    <div class="toasts">
      <TransitionGroup name="toast">
        <div v-for="t in store.toasts" :key="t.id" class="toast" :class="t.kind">
          <Icon :name="t.kind === 'error' ? 'alert' : t.kind === 'success' ? 'check' : 'zap'" :size="16" />
          <span class="selectable">{{ t.message }}</span>
        </div>
      </TransitionGroup>
    </div>
  </div>
</template>

<style scoped>
.shell { display: flex; height: 100%; }

.sidebar {
  width: 228px; flex: none; display: flex; flex-direction: column;
  padding: 18px 12px; border-right: 1px solid var(--border); background: var(--panel);
}
.brand { display: flex; align-items: center; gap: 10px; padding: 4px 10px 22px; font-weight: 700; font-size: 16px; letter-spacing: -.01em; }
nav { display: flex; flex-direction: column; gap: 2px; }
.nav-item {
  display: flex; align-items: center; gap: 11px; height: 38px; padding: 0 12px;
  border: none; border-radius: 9px; background: transparent; color: var(--muted);
  font: inherit; font-weight: 550; cursor: pointer; text-align: left;
}
.nav-item:hover { background: var(--panel-2); color: var(--text); }
.nav-item.active { background: var(--accent-soft); color: var(--accent-text); }
.nav-item span:nth-child(2) { flex: 1; }
.count.live { background: var(--green-soft); color: var(--green); }
.count { font-size: 11.5px; padding: 1px 7px; border-radius: 999px; background: var(--panel-2); color: var(--muted); }

.side-foot { margin-top: auto; padding: 14px 6px 0; border-top: 1px solid var(--border); }
.svc-mini { display: flex; align-items: center; gap: 9px; padding: 5px 4px; font-weight: 500; }

.content { flex: 1; overflow-y: auto; }
.content-inner { max-width: 1040px; margin: 0 auto; padding: 30px 34px 60px; }

.dock { position: fixed; right: 18px; bottom: 18px; display: flex; flex-direction: column; gap: 10px; width: 320px; z-index: 20; }
.task { padding: 12px 14px; display: flex; flex-direction: column; gap: 8px; }
.task.error { border-color: var(--red); }
.bar { height: 5px; border-radius: 9px; background: var(--panel-2); overflow: hidden; }
.bar > div { height: 100%; background: var(--accent); border-radius: 9px; transition: width .2s; }
.bar > div.indeterminate { width: 35%; animation: slide 1.1s ease-in-out infinite; }
@keyframes slide { from { transform: translateX(-100%); } to { transform: translateX(300%); } }

.toasts { position: fixed; top: 16px; left: 50%; transform: translateX(-50%); display: flex; flex-direction: column; gap: 8px; z-index: 30; align-items: center; }
.toast {
  display: flex; align-items: center; gap: 10px; padding: 10px 16px; border-radius: 10px; max-width: 560px;
  background: var(--text); color: var(--bg); font-weight: 500; box-shadow: 0 8px 24px rgba(0,0,0,.18);
}
.toast.error { background: var(--red); color: #fff; }
.toast.success { background: var(--green); color: #fff; }
.toast-enter-active, .toast-leave-active { transition: all .2s; }
.toast-enter-from, .toast-leave-to { opacity: 0; transform: translateY(-8px); }
</style>
