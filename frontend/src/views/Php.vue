<script setup>
import { computed, onMounted, ref } from 'vue'
import { store, call, Api, loadCatalog } from '../store'
import Icon from '../components/Icon.vue'

const s = computed(() => store.state)
const openExt = ref('')
const extensions = ref([])
const extFilter = ref('')

onMounted(() => loadCatalog())

const releases = computed(() => store.catalog?.php || [])
const installedSet = computed(() => new Set(s.value.php.map(p => p.branch)))
const available = computed(() => releases.value.filter(r => !installedSet.value.has(r.branch)))
const latestFor = b => releases.value.find(r => r.branch === b)
const installing = b => Object.values(store.tasks).some(t => !t.done && t.label.startsWith('PHP ' + b + ' '))

const filteredExt = computed(() => extensions.value.filter(e => e.name.includes(extFilter.value.toLowerCase())))

async function toggleExtensions(branch) {
  if (openExt.value === branch) { openExt.value = ''; return }
  openExt.value = branch
  extFilter.value = ''
  extensions.value = await Api.GetExtensions(branch)
}

async function setExt(branch, ext, on) {
  ext.enabled = on
  try {
    await call('ext', () => Api.SetExtension(branch, ext.name, on), `${ext.name} ${on ? 'açıldı' : 'kapatıldı'}`)
  } finally {
    extensions.value = await Api.GetExtensions(branch)
  }
}

function remove(p) {
  if (!confirm(`PHP ${p.branch} kaldırılsın mı? Bu sürümü kullanan siteler varsayılana döner.`)) return
  call('rm:' + p.branch, () => Api.RemovePHP(p.branch), `PHP ${p.branch} kaldırıldı`)
}
</script>

<template>
  <div>
    <div class="page-head">
      <div class="grow">
        <h1>PHP</h1>
        <p class="muted">Her sürüm kendi FastCGI havuzunda çalışır. Sadece sitelerin kullandığı sürümler başlatılır.</p>
      </div>
    </div>

    <div class="card">
      <div v-if="!s.php.length" class="empty">
        <div class="icon-wrap"><Icon name="code" :size="22" /></div>
        <h2>Henüz PHP kurulu değil</h2>
        <p>Aşağıdan bir sürüm seç.</p>
      </div>
      <div v-for="p in s.php" :key="p.branch" class="php-item">
        <div class="row php-row">
          <div class="ver" :class="{ on: p.running }">{{ p.branch }}</div>
          <div class="grow">
            <div class="row" style="gap:8px">
              <strong>PHP {{ p.version || p.branch }}</strong>
              <span v-if="p.default" class="badge accent"><Icon name="star" :size="11" /> Varsayılan</span>
              <span v-if="p.running" class="badge green">Çalışıyor</span>
              <span v-if="latestFor(p.branch) && p.version && latestFor(p.branch).version !== p.version" class="badge amber">
                {{ latestFor(p.branch).version }} mevcut
              </span>
            </div>
            <p class="muted small">
              {{ p.sites }} site · port <span class="mono">{{ p.port }}–{{ p.port + s.config.phpWorkers - 1 }}</span>
            </p>
          </div>
          <button v-if="latestFor(p.branch) && p.version && latestFor(p.branch).version !== p.version"
                  class="btn sm" :disabled="installing(p.branch)" @click="Api.Install('php', p.branch)">
            <Icon name="download" :size="14" /> Güncelle
          </button>
          <button v-if="!p.default" class="btn sm" @click="call('def', () => Api.SetDefaultPHP(p.branch), `Varsayılan PHP ${p.branch}`)">
            Varsayılan yap
          </button>
          <button class="btn sm" :class="{ active: openExt === p.branch }" @click="toggleExtensions(p.branch)">Eklentiler</button>
          <button class="btn sm icon ghost" title="php.ini düzenle" @click="Api.OpenPHPIni(p.branch)"><Icon name="file" :size="15" /></button>
          <button class="btn sm icon ghost" title="Yeniden başlat" :disabled="!p.running"
                  @click="call('php', () => Api.RestartPHP(p.branch), `PHP ${p.branch} yeniden başlatıldı`)"><Icon name="refresh" :size="15" /></button>
          <button class="btn sm icon ghost danger" title="Kaldır" :disabled="store.busy['rm:' + p.branch]" @click="remove(p)"><Icon name="trash" :size="15" /></button>
        </div>

        <div v-if="openExt === p.branch" class="ext-panel">
          <div class="row" style="margin-bottom:12px">
            <input class="input sm" v-model="extFilter" placeholder="Eklenti ara…" style="width:220px" />
            <span class="muted small">{{ extensions.filter(e => e.enabled).length }} / {{ extensions.length }} açık · değişiklik anında uygulanır</span>
          </div>
          <div class="ext-grid">
            <label v-for="e in filteredExt" :key="e.name" class="ext" :class="{ on: e.enabled }">
              <span class="mono grow">{{ e.name }}</span>
              <span class="switch">
                <input type="checkbox" :checked="e.enabled" :disabled="store.busy.ext" @change="ev => setExt(p.branch, e, ev.target.checked)" />
                <span></span>
              </span>
            </label>
          </div>
        </div>
      </div>
    </div>

    <div class="row section-head">
      <h2 class="grow">Kurulabilir sürümler</h2>
      <button class="btn sm ghost" :disabled="store.catalogLoading" @click="loadCatalog(true)">
        <Icon name="refresh" :size="14" :class="{ spin: store.catalogLoading }" /> Yenile
      </button>
    </div>
    <div class="avail" v-if="releases.length">
      <div v-for="r in available" :key="r.branch" class="card avail-item">
        <div class="ver lg">{{ r.branch }}</div>
        <div class="grow">
          <strong>PHP {{ r.version }}</strong>
          <p class="muted small">NTS · x64</p>
        </div>
        <button class="btn sm primary" :disabled="installing(r.branch)" @click="Api.Install('php', r.branch)">
          <Icon :name="installing(r.branch) ? 'refresh' : 'download'" :size="14" :class="{ spin: installing(r.branch) }" />
          {{ installing(r.branch) ? 'Kuruluyor' : 'Kur' }}
        </button>
      </div>
      <p v-if="!available.length" class="muted small">Tüm sürümler kurulu 🎉</p>
    </div>
    <p v-else-if="store.catalogLoading" class="muted small">Sürüm listesi alınıyor…</p>
    <p v-else class="muted small">Sürüm listesi alınamadı. İnternet bağlantını kontrol et.</p>
  </div>
</template>

<style scoped>
.php-item { border-bottom: 1px solid var(--border); }
.php-item:last-child { border-bottom: none; }
.php-row { padding: 14px 18px; gap: 8px; }
.ver { width: 44px; height: 44px; border-radius: 11px; display: grid; place-items: center; font-weight: 700; font-size: 14px;
  background: var(--panel-2); color: var(--muted); flex: none; margin-right: 6px; }
.ver.on { background: var(--accent-soft); color: var(--accent-text); }
.ver.lg { background: var(--accent-soft); color: var(--accent-text); }
.btn.active { background: var(--accent-soft); color: var(--accent-text); border-color: transparent; }
.ext-panel { padding: 4px 18px 18px 76px; }
.ext-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(170px, 1fr)); gap: 6px; }
.ext { display: flex; align-items: center; gap: 8px; padding: 8px 10px; border-radius: 8px; background: var(--panel-2); cursor: pointer; }
.ext.on { background: var(--accent-soft); }
.section-head { margin: 28px 0 12px; }
.avail { display: grid; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); gap: 12px; }
.avail-item { display: flex; align-items: center; gap: 12px; padding: 14px; }
</style>
