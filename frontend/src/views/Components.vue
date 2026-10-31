<script setup>
import { computed, onMounted } from 'vue'
import { store, Api, loadCatalog } from '../store'
import Icon from '../components/Icon.vue'

onMounted(() => loadCatalog())

const meta = {
  apache: { icon: 'server', desc: 'Varsayılan web sunucusu; .htaccess tam destekli (önerilen).', key: 'apache' },
  nginx: { icon: 'server', desc: 'İsteğe bağlı, hafif alternatif. .htaccess desteklemez.', key: 'nginx' },
  mariadb: { icon: 'database', desc: 'MySQL uyumlu veritabanı (LTS). Veriler data/ klasöründe kalır.', key: 'mariadb' },
  phpmyadmin: { icon: 'globe', desc: 'Veritabanı yönetim arayüzü.', key: 'phpmyadmin' },
  composer: { icon: 'package', desc: 'PHP paket yöneticisi. Terminalde hazır gelir.', key: 'composer' },
}
const nameMap = { nginx: 'Nginx', apache: 'Apache', mariadb: 'MariaDB', phpmyadmin: 'phpMyAdmin', composer: 'Composer' }

const items = computed(() => store.state.components.map(c => {
  const latest = store.catalog?.[meta[c.id].key]
  return {
    ...c, ...meta[c.id], latest: latest?.version,
    update: c.installed && latest && c.version && latest.version !== c.version,
    busy: Object.values(store.tasks).some(t => !t.done && t.label.startsWith(nameMap[c.id] + ' ')),
  }
}))
const webServer = computed(() => store.state.config.webServer)
</script>

<template>
  <div>
    <div class="page-head">
      <div class="grow">
        <h1>Bileşenler</h1>
        <p class="muted">Resmi kaynaklardan indirilir, <span class="mono">bin</span> klasörüne kurulur. Güncellemede ayarların korunur.</p>
      </div>
      <button class="btn" :disabled="store.catalogLoading" @click="loadCatalog(true)">
        <Icon name="refresh" :size="15" :class="{ spin: store.catalogLoading }" /> Güncellemeleri denetle
      </button>
    </div>

    <div class="alert error" v-if="store.catalog?.errors?.length">
      <Icon name="alert" />
      <span class="small selectable">{{ store.catalog.errors.join(' · ') }}</span>
    </div>

    <div class="card">
      <div v-for="c in items" :key="c.id" class="row comp">
        <div class="comp-icon" :class="{ on: c.installed }"><Icon :name="c.icon" /></div>
        <div class="grow">
          <div class="row" style="gap:8px">
            <strong>{{ c.name }}</strong>
            <span v-if="c.id === webServer" class="badge accent">Aktif sunucu</span>
            <span v-if="c.update" class="badge amber">{{ c.latest }} mevcut</span>
          </div>
          <p class="muted small">{{ c.desc }}</p>
        </div>
        <div class="ver-col">
          <span v-if="c.installed" class="mono">{{ c.version || 'kurulu' }}</span>
          <span v-else class="muted small">kurulu değil</span>
          <span v-if="!c.installed && c.latest" class="muted small"> · {{ c.latest }}</span>
        </div>
        <button v-if="!c.installed" class="btn sm primary" :disabled="c.busy || !c.latest" @click="Api.Install(c.id, '')">
          <Icon :name="c.busy ? 'refresh' : 'download'" :size="14" :class="{ spin: c.busy }" /> {{ c.busy ? 'Kuruluyor' : 'Kur' }}
        </button>
        <button v-else-if="c.update" class="btn sm" :disabled="c.busy" @click="Api.Install(c.id, '')">
          <Icon :name="c.busy ? 'refresh' : 'download'" :size="14" :class="{ spin: c.busy }" /> Güncelle
        </button>
        <span v-else class="badge green"><Icon name="check" :size="12" /> Güncel</span>
      </div>
    </div>

    <p class="muted small" style="margin-top:14px">
      Apache ↔ Nginx geçişini <strong>Ayarlar</strong> sayfasından yapabilirsin; ikisi de aynı siteleri ve PHP havuzlarını kullanır.
    </p>
  </div>
</template>

<style scoped>
.comp { padding: 16px 18px; border-bottom: 1px solid var(--border); gap: 14px; }
.comp:last-child { border-bottom: none; }
.comp-icon { width: 40px; height: 40px; border-radius: 11px; display: grid; place-items: center; background: var(--panel-2); color: var(--muted); flex: none; }
.comp-icon.on { background: var(--accent-soft); color: var(--accent-text); }
.ver-col { min-width: 110px; text-align: right; }
</style>
