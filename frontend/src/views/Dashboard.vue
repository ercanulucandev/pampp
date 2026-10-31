<script setup>
import { computed } from 'vue'
import { store, call, Api } from '../store'
import Icon from '../components/Icon.vue'
import Logo from '../components/Logo.vue'

const emit = defineEmits(['navigate'])
const s = computed(() => store.state)
const web = computed(() => s.value.services.find(x => x.id === 'web'))
const db = computed(() => s.value.services.find(x => x.id === 'db'))
const phpRunning = computed(() => s.value.php.filter(p => p.running))
const fresh = computed(() => !s.value.components.some(c => c.installed) && s.value.php.length === 0)
const pma = computed(() => s.value.sites.find(x => x.name === 'phpmyadmin'))
const sites = computed(() => s.value.sites.filter(x => !x.system).slice(0, 6))
const setupRunning = computed(() => Object.values(store.tasks).some(t => !t.done && t.label === 'Hızlı kurulum'))
const localhost = computed(() => s.value.startUrl)

function toggle(svc) {
  if (svc.running) call(svc.id, () => Api.StopService(svc.id), `${svc.name} durduruldu`)
  else call(svc.id, () => Api.StartService(svc.id), `${svc.name} başlatıldı`)
}
function restart(svc) {
  call(svc.id, () => Api.RestartService(svc.id), `${svc.name} yeniden başlatıldı`)
}
</script>

<template>
  <div>
    <div class="page-head">
      <div class="grow">
        <h1>Panel</h1>
        <p class="muted small mono selectable">{{ s.root }}</p>
      </div>
      <button class="btn" @click="Api.OpenFolder('www')"><Icon name="folder" :size="16" /> www</button>
      <button class="btn" @click="Api.OpenTerminal('')"><Icon name="terminal" :size="16" /> Terminal</button>
    </div>

    <div class="alert" v-if="!s.vcRedist">
      <Icon name="alert" />
      <span class="grow">Visual C++ Redistributable bulunamadı. PHP ve Apache'nin çalışması için gerekli.</span>
      <button class="btn sm" @click="Api.OpenURL('https://aka.ms/vs/17/release/vc_redist.x64.exe')">İndir</button>
    </div>

    <div class="alert" v-if="s.ssl.enabled && s.ssl.caExists && !s.ssl.trusted">
      <Icon name="lock" />
      <span class="grow">SSL sertifikası henüz güvenilir değil. Tarayıcıda uyarı görmemek için bir kez onayla.</span>
      <button class="btn sm" :disabled="store.busy.trust" @click="call('trust', () => Api.TrustCA(), 'Sertifika artık güvenilir 🔒')">Güvenilir yap</button>
    </div>

    <div class="card onboarding" v-if="fresh">
      <div class="ob-head">
        <Logo :size="64" style="flex:none" />
        <div class="grow">
          <h2 class="ob-title">PAMPP'a hoş geldin 👋</h2>
          <p class="muted">Windows için hafif ve hızlı PHP geliştirme ortamı. Her şey bu klasörün içinde kalır, sistemine hiçbir şey kurulmaz.</p>
        </div>
      </div>
      <div class="ob-grid">
        <div class="ob-item"><Icon name="globe" /><b>Otomatik siteler</b><span><span class="mono">www/blog</span> klasörü anında <span class="mono">blog.{{ s.config.tld }}</span> olur.</span></div>
        <div class="ob-item"><Icon name="code" /><b>Site başına PHP</b><span>7.4'ten 8.5'e kadar her sürüm, tek tıkla geçiş.</span></div>
        <div class="ob-item"><Icon name="lock" /><b>Yerel SSL</b><span>Tüm siteler <span class="mono">https://</span> ile kilit simgesiyle açılır.</span></div>
        <div class="ob-item"><Icon name="database" /><b>MariaDB + phpMyAdmin</b><span>Veritabanı ve yönetim paneli hazır gelir.</span></div>
      </div>
      <div class="ob-foot">
        <p class="muted small grow">Hızlı kurulum: Apache, en güncel iki PHP sürümü, MariaDB, phpMyAdmin, Composer ve Node.js LTS (~280 MB).</p>
        <button class="btn primary lg" :disabled="setupRunning" @click="call('setup', () => Api.QuickSetup())">
          <Icon :name="setupRunning ? 'refresh' : 'sparkle'" :size="16" :class="{ spin: setupRunning }" />
          {{ setupRunning ? 'Kuruluyor…' : 'Hızlı kurulumu başlat' }}
        </button>
      </div>
    </div>

    <div class="grid grid-3">
      <div v-for="svc in [web, db]" :key="svc.id" class="card card-pad svc">
        <div class="row">
          <div class="svc-icon" :class="{ on: svc.running }"><Icon :name="svc.id === 'web' ? 'server' : 'database'" /></div>
          <div class="grow">
            <h2>{{ svc.name }}</h2>
            <p class="muted small">{{ svc.installed ? (svc.version || 'kurulu') : 'kurulu değil' }}</p>
          </div>
          <span class="badge" :class="svc.running ? 'green' : svc.error ? 'red' : ''">
            {{ svc.running ? 'Çalışıyor' : svc.error ? 'Hata' : 'Durdu' }}
          </span>
        </div>
        <p class="err small selectable" v-if="svc.error && !svc.running">{{ svc.error }}</p>
        <div class="row svc-foot">
          <span class="muted small">Port <strong class="mono">{{ svc.port }}</strong></span>
          <span class="spacer"></span>
          <template v-if="svc.installed">
            <button class="btn sm icon ghost" v-if="svc.running" :disabled="store.busy[svc.id]" title="Yeniden başlat" @click="restart(svc)">
              <Icon name="refresh" :size="15" />
            </button>
            <button class="btn sm" :class="svc.running ? '' : 'primary'" :disabled="store.busy[svc.id]" @click="toggle(svc)">
              <Icon :name="store.busy[svc.id] ? 'refresh' : svc.running ? 'stop' : 'play'" :size="14" :class="{ spin: store.busy[svc.id] }" />
              {{ svc.running ? 'Durdur' : 'Başlat' }}
            </button>
          </template>
          <button v-else class="btn sm" @click="emit('navigate', 'components')"><Icon name="download" :size="14" /> Kur</button>
        </div>
      </div>

      <div class="card card-pad svc">
        <div class="row">
          <div class="svc-icon" :class="{ on: phpRunning.length }"><Icon name="code" /></div>
          <div class="grow">
            <h2>PHP</h2>
            <p class="muted small">{{ s.php.length }} sürüm kurulu</p>
          </div>
          <span class="badge" :class="phpRunning.length ? 'green' : ''">{{ phpRunning.length ? phpRunning.length + ' aktif' : 'Pasif' }}</span>
        </div>
        <div class="chips">
          <span v-for="p in s.php" :key="p.branch" class="badge" :class="p.running ? 'accent' : ''">
            {{ p.branch }}<Icon v-if="p.default" name="star" :size="11" />
          </span>
          <span v-if="!s.php.length" class="muted small">Henüz PHP kurulmadı</span>
        </div>
        <div class="row svc-foot">
          <span class="spacer"></span>
          <button class="btn sm" @click="emit('navigate', 'php')">Yönet</button>
        </div>
      </div>
    </div>

    <div class="grid grid-2" style="margin-top:14px">
      <div class="card">
        <div class="row card-head">
          <h2 class="grow">Siteler</h2>
          <button class="btn sm ghost" @click="emit('navigate', 'sites')">Tümü</button>
        </div>
        <div v-if="!sites.length" class="empty small" style="padding:26px">
          <p><span class="mono">www</span> klasörüne bir klasör ekle, otomatik site olsun.</p>
        </div>
        <div v-for="site in sites" :key="site.name" class="site-row row">
          <span class="dot" :class="{ on: web.running }"></span>
          <a class="grow" @click="Api.OpenURL(site.url)">{{ site.domain }}</a>
          <span class="badge">PHP {{ site.php || '—' }}</span>
        </div>
      </div>

      <div class="card">
        <div class="row card-head"><h2>Kısayollar</h2></div>
        <div class="links">
          <button class="link" @click="Api.OpenURL(localhost)">
            <Icon name="home" /><span>Başlangıç sayfası</span><Icon name="external" :size="14" />
          </button>
          <button class="link" :disabled="!pma" @click="pma && Api.OpenURL(pma.url)">
            <Icon name="database" /><span>phpMyAdmin</span><Icon name="external" :size="14" />
          </button>
          <button class="link" @click="Api.OpenFolder('logs')">
            <Icon name="logs" /><span>Log klasörü</span><Icon name="external" :size="14" />
          </button>
          <button class="link" @click="Api.OpenFolder('etc')">
            <Icon name="file" /><span>Config dosyaları</span><Icon name="external" :size="14" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.onboarding { padding: 26px; margin-bottom: 14px; background: linear-gradient(135deg, var(--accent-soft), var(--panel) 65%); }
.ob-head { display: flex; align-items: center; gap: 18px; }
.ob-title { font-size: 20px; margin-bottom: 4px; }
.ob-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin: 22px 0; }
.ob-item { display: flex; flex-direction: column; gap: 6px; padding: 14px; border-radius: 12px; background: var(--panel); border: 1px solid var(--border); }
.ob-item svg { color: var(--accent-text); }
.ob-item span { color: var(--muted); font-size: 12.5px; line-height: 1.45; }
.ob-foot { display: flex; align-items: center; gap: 16px; }
.btn.lg { height: 40px; padding: 0 18px; font-size: 14.5px; }
.svc { display: flex; flex-direction: column; gap: 14px; }
.svc-icon { width: 40px; height: 40px; border-radius: 11px; display: grid; place-items: center; background: var(--panel-2); color: var(--muted); }
.svc-icon.on { background: var(--green-soft); color: var(--green); }
.svc-foot { margin-top: auto; }
.err { color: var(--red); line-height: 1.45; }
.chips { display: flex; flex-wrap: wrap; gap: 6px; }
.card-head { padding: 14px 18px; border-bottom: 1px solid var(--border); min-height: 52px; }
.site-row { padding: 11px 18px; border-bottom: 1px solid var(--border); }
.site-row:last-child { border-bottom: none; }
.links { padding: 8px; display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
.link { display: flex; align-items: center; gap: 10px; padding: 12px; border-radius: 9px; border: none; background: transparent;
  color: var(--text); font: inherit; font-weight: 550; cursor: pointer; text-align: left; }
.link span { flex: 1; }
.link:hover:not(:disabled) { background: var(--panel-2); }
.link:disabled { opacity: .45; cursor: default; }
.link svg:first-child { color: var(--accent-text); }
.link svg:last-child { color: var(--muted); }
</style>
