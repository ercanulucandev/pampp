<script setup>
import { computed, ref } from 'vue'
import { store, call, Api } from '../store'
import Icon from '../components/Icon.vue'

const s = computed(() => store.state)
const webRunning = computed(() => s.value.services.find(x => x.id === 'web').running)
const query = ref('')
const newName = ref('')
const newPHP = ref('')
const adding = ref(false)
const created = ref(null)
// `autofocus` is ignored on inputs Vue inserts later, so focus explicitly.
const vFocus = { mounted: el => el.focus() }
const slug = computed(() => newName.value.trim().toLowerCase().replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, ''))

function openCreate() {
  newName.value = ''
  newPHP.value = s.value.php.find(p => p.default)?.branch || ''
  created.value = null
  adding.value = true
}

const sites = computed(() => {
  const q = query.value.trim().toLowerCase()
  return s.value.sites.filter(x => !q || x.name.includes(q))
})

function setPHP(site, branch) {
  call('site:' + site.name, () => Api.SetSitePHP(site.name, branch), `${site.domain} → PHP ${branch}`)
}

async function create() {
  if (!slug.value) return
  created.value = await call('create', () => Api.CreateSite(slug.value, newPHP.value))
}
</script>

<template>
  <div>
    <div class="page-head">
      <div class="grow">
        <h1>Siteler</h1>
        <p class="muted">
          <span class="mono">www</span> içindeki her klasör otomatik olarak
          <span class="mono">klasör.{{ s.config.tld }}</span> adresinde yayınlanır.
        </p>
      </div>
      <div class="search">
        <Icon name="search" :size="15" />
        <input class="input" v-model="query" placeholder="Ara…" />
      </div>
      <button class="btn primary" @click="openCreate"><Icon name="plus" :size="16" /> Yeni site</button>
    </div>

    <Transition name="fade">
      <div class="overlay" v-if="adding" @click.self="adding = false">
        <form class="card modal" v-if="!created" @submit.prevent="create">
          <div class="modal-icon"><Icon name="globe" :size="22" /></div>
          <h2>Yeni site oluştur</h2>
          <p class="muted small">Klasör <span class="mono">www</span> içinde açılır, karşılama sayfası eklenir ve hosts/SSL otomatik ayarlanır.</p>

          <label class="field">
            <span>Site adı</span>
            <input class="input" v-model="newName" placeholder="ornek-proje" v-focus />
          </label>
          <div class="preview" :class="{ empty: !slug }">
            <Icon :name="s.config.ssl ? 'lock' : 'globe'" :size="14" />
            <span class="mono">{{ s.config.ssl ? 'https' : 'http' }}://<b>{{ slug || 'ornek-proje' }}</b>.{{ s.config.tld }}</span>
          </div>

          <label class="field" v-if="s.php.length">
            <span>PHP sürümü</span>
            <select class="input" v-model="newPHP">
              <option v-for="p in s.php" :key="p.branch" :value="p.branch">PHP {{ p.version || p.branch }}{{ p.default ? ' (varsayılan)' : '' }}</option>
            </select>
          </label>

          <div class="row modal-foot">
            <span class="spacer"></span>
            <button type="button" class="btn ghost" @click="adding = false">Vazgeç</button>
            <button class="btn primary" :disabled="store.busy.create || !slug">
              <Icon :name="store.busy.create ? 'refresh' : 'plus'" :size="15" :class="{ spin: store.busy.create }" /> Oluştur
            </button>
          </div>
        </form>

        <div class="card modal success" v-else>
          <div class="burst"><Icon name="check" :size="30" /></div>
          <h2>{{ created.domain }} hazır! 🎉</h2>
          <p class="muted">Siten oluşturuldu ve yayında. Tarayıcıda açınca karşılama sayfasını göreceksin.</p>
          <div class="facts">
            <div><span>Adres</span><a @click="Api.OpenURL(created.url)">{{ created.url }}</a></div>
            <div><span>Klasör</span><code class="selectable">{{ created.path }}</code></div>
            <div><span>PHP</span><b>{{ created.php || '—' }}</b></div>
            <div><span>SSL</span><b>{{ s.config.ssl ? (s.ssl.trusted ? '🔒 Güvenilir' : '⚠️ Güven onayı bekliyor') : 'Kapalı' }}</b></div>
          </div>
          <p class="muted small" v-if="!webRunning">Web sunucusu kapalı. Siteyi görmek için Panel'den başlat.</p>
          <div class="row modal-foot">
            <button class="btn" @click="Api.OpenFolder(created.path)"><Icon name="folder" :size="15" /> Klasör</button>
            <button class="btn" @click="Api.OpenTerminal(created.path)"><Icon name="terminal" :size="15" /> Terminal</button>
            <span class="spacer"></span>
            <button class="btn ghost" @click="adding = false">Kapat</button>
            <button class="btn primary" @click="Api.OpenURL(created.url); adding = false"><Icon name="external" :size="15" /> Siteyi aç</button>
          </div>
        </div>
      </div>
    </Transition>

    <div class="card">
      <table class="table" v-if="sites.length">
        <thead>
          <tr><th>Site</th><th>Kök dizin</th><th style="width:150px">PHP</th><th style="width:120px"></th></tr>
        </thead>
        <tbody>
          <tr v-for="site in sites" :key="site.name">
            <td>
              <div class="row">
                <span class="dot" :class="{ on: webRunning }"></span>
                <div>
                  <a @click="Api.OpenURL(site.url)"><Icon v-if="s.config.ssl" name="lock" :size="12" class="lock" /><strong>{{ site.domain }}</strong></a>
                  <span v-if="site.system" class="badge" style="margin-left:8px">sistem</span>
                </div>
              </div>
            </td>
            <td class="muted small mono path selectable" :title="site.docRoot">{{ site.docRoot.replace(s.root, '') }}</td>
            <td>
              <select class="input sm" :value="site.php" :disabled="store.busy['site:' + site.name] || !s.php.length"
                      @change="e => setPHP(site, e.target.value)" style="width:120px">
                <option v-if="!s.php.length" value="">—</option>
                <option v-for="p in s.php" :key="p.branch" :value="p.branch">PHP {{ p.branch }}</option>
              </select>
            </td>
            <td>
              <div class="row" style="gap:2px; justify-content:flex-end">
                <button class="btn ghost sm icon" title="Tarayıcıda aç" @click="Api.OpenURL(site.url)"><Icon name="external" :size="15" /></button>
                <button class="btn ghost sm icon" title="Klasörü aç" @click="Api.OpenFolder(site.path)"><Icon name="folder" :size="15" /></button>
                <button class="btn ghost sm icon" title="Terminal" @click="Api.OpenTerminal(site.path)"><Icon name="terminal" :size="15" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="empty">
        <div class="icon-wrap"><Icon name="globe" :size="22" /></div>
        <h2>{{ query ? 'Sonuç yok' : 'Henüz site yok' }}</h2>
        <p>"Yeni site" ile oluştur ya da <a @click="Api.OpenFolder('www')">www klasörüne</a> bir proje kopyala.</p>
      </div>
    </div>

    <p class="muted small tip">
      <Icon name="zap" :size="13" /> <span class="mono">public</span>, <span class="mono">web</span> veya
      <span class="mono">public_html</span> klasörü varsa (Laravel, Symfony…) kök dizin olarak o kullanılır.
      PHP sürümü değişince sunucu kesinti olmadan yeniden yüklenir.
    </p>
  </div>
</template>

<style scoped>
.search { position: relative; display: flex; align-items: center; }
.search svg { position: absolute; left: 10px; color: var(--muted); }
.search .input { padding-left: 32px; width: 200px; }
.lock { margin-right: 5px; vertical-align: -1px; color: var(--green); }
.overlay { position: fixed; inset: 0; z-index: 25; display: grid; place-items: center; padding: 20px;
  background: rgba(10, 12, 18, .45); backdrop-filter: blur(3px); }
.modal { width: 100%; max-width: 480px; padding: 26px; display: flex; flex-direction: column; gap: 12px;
  box-shadow: 0 24px 60px rgba(0, 0, 0, .3); animation: pop .18s ease-out; }
.modal h2 { font-size: 18px; }
.modal-icon { width: 44px; height: 44px; border-radius: 12px; display: grid; place-items: center; background: var(--accent-soft); color: var(--accent-text); }
.field { display: flex; flex-direction: column; gap: 6px; font-weight: 550; font-size: 13px; }
.preview { display: flex; align-items: center; gap: 8px; padding: 10px 12px; border-radius: 9px; background: var(--panel-2); color: var(--muted); }
.preview b { color: var(--accent-text); }
.preview.empty b { opacity: .5; }
.modal-foot { margin-top: 8px; }
.success { align-items: center; text-align: center; }
.success .modal-foot { width: 100%; }
.burst { width: 68px; height: 68px; border-radius: 50%; display: grid; place-items: center; color: #fff;
  background: linear-gradient(135deg, #22c55e, #16a34a); box-shadow: 0 0 0 10px var(--green-soft); animation: pop .35s cubic-bezier(.2, 1.6, .4, 1); margin: 6px 0 8px; }
.facts { width: 100%; text-align: left; border: 1px solid var(--border); border-radius: 12px; overflow: hidden; margin-top: 6px; }
.facts div { display: flex; gap: 12px; padding: 10px 14px; border-bottom: 1px solid var(--border); font-size: 13px; }
.facts div:last-child { border-bottom: none; }
.facts span { width: 60px; flex: none; color: var(--muted); }
.facts code { font-size: 12px; word-break: break-all; }
@keyframes pop { from { transform: scale(.92); opacity: 0; } to { transform: scale(1); opacity: 1; } }
.fade-enter-active, .fade-leave-active { transition: opacity .15s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
.path { max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tip { margin-top: 14px; line-height: 1.6; }
.tip svg { vertical-align: -2px; }
</style>
