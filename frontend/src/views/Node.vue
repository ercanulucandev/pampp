<script setup>
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { store, call, Api, loadCatalog, loadNodeLog, toast } from '../store'
import Icon from '../components/Icon.vue'

const s = computed(() => store.state)
const node = computed(() => s.value.node)
const apps = computed(() => node.value.apps)
const releases = computed(() => store.catalog?.node || [])
const recommended = computed(() => releases.value.find(r => r.lts) || releases.value[0])
const installingNode = b => Object.values(store.tasks).some(t => !t.done && t.label.startsWith('Node.js ' + b + ' '))
const showVersions = ref(false)

onMounted(() => loadCatalog())

// ---- logs ----
const open = reactive({}) // app -> selected run key ("__install", "dev", ...)
const logBox = ref({})
const runLabel = k => ({ __install: 'Paket kurulumu', __create: 'Oluşturma' }[k] || k)

function runsOf(app) {
  const keys = new Set()
  for (const k of Object.keys(store.nodeLogs)) {
    if (k.startsWith(app.name + '/')) keys.add(k.slice(app.name.length + 1))
  }
  for (const sc of app.scripts) if (sc.running || sc.exitCode !== null) keys.add(sc.name)
  if (app.installing) keys.add('__install')
  return [...keys]
}

async function showLog(app, script) {
  open[app.name] = script
  await loadNodeLog(app.name, script)
  scrollLog(app.name)
}

function scrollLog(name) {
  nextTick(() => {
    const el = logBox.value[name]
    if (el) el.scrollTop = el.scrollHeight
  })
}

watch(() => store.nodeLogs, () => {
  for (const name of Object.keys(open)) if (open[name]) scrollLog(name)
}, { deep: true })

// ---- actions ----
function run(app, sc) {
  if (sc.running) {
    call('run:' + app.name + sc.name, () => Api.StopScript(app.name, sc.name), `${sc.name} durduruldu`)
    return
  }
  store.nodeLogs[app.name + '/' + sc.name] = []
  open[app.name] = sc.name
  call('run:' + app.name + sc.name, () => Api.RunScript(app.name, sc.name))
}

function install(app) {
  store.nodeLogs[app.name + '/__install'] = []
  open[app.name] = '__install'
  call('install:' + app.name, () => Api.InstallPackages(app.name))
}

const frameworkColor = f => ({
  'Next.js': '#111827', Nuxt: '#00dc82', Vite: '#8b5cf6', Astro: '#ff5d01', SvelteKit: '#ff3e00', Remix: '#3992ff',
  Angular: '#dd0031', NestJS: '#e0234e', Express: '#4b5563', Electron: '#47848f', 'Create React App': '#61dafb',
}[f] || '#16a34a')

// ---- create ----
const templates = ref([])
const creating = ref(false)
const newName = ref('')
const newTpl = ref('vite-react')
const createdName = ref('')
const vFocus = { mounted: el => el.focus() }
const slug = computed(() => newName.value.trim().toLowerCase().replace(/[^a-z0-9._-]+/g, '-').replace(/^-+|-+$/g, ''))
const createLog = computed(() => [
  ...(store.nodeLogs[createdName.value + '/__create'] || []),
  ...(store.nodeLogs[createdName.value + '/__install'] || []),
])
const createdApp = computed(() => apps.value.find(a => a.name === createdName.value))
const createDone = computed(() => createdApp.value && createdApp.value.hasModules && !createdApp.value.installing)

async function openCreate() {
  if (!templates.value.length) templates.value = await Api.GetNodeTemplates()
  newName.value = ''
  createdName.value = ''
  creating.value = true
}

async function create() {
  if (!slug.value) return
  store.nodeLogs[slug.value + '/__create'] = []
  store.nodeLogs[slug.value + '/__install'] = []
  await call('create', () => Api.CreateNodeApp(slug.value, newTpl.value))
  createdName.value = slug.value
}

watch(createDone, done => {
  if (done) toast(`${createdName.value} hazır 🎉`, 'success')
})
</script>

<template>
  <div>
    <div class="page-head">
      <div class="grow">
        <h1>Node.js</h1>
        <p class="muted"><span class="mono">nodejs</span> klasöründeki her <span class="mono">package.json</span> projesi burada; script'leri tek tıkla çalıştır.</p>
      </div>
      <button class="btn" @click="Api.OpenFolder('nodejs')"><Icon name="folder" :size="16" /> Klasör</button>
      <button class="btn primary" :disabled="!node.installed.length" @click="openCreate"><Icon name="plus" :size="16" /> Yeni proje</button>
    </div>

    <!-- runtime -->
    <div class="card card-pad runtime" v-if="!node.installed.length">
      <div class="rt-icon"><Icon name="hexagon" :size="24" /></div>
      <div class="grow">
        <h2>Node.js kurulu değil</h2>
        <p class="muted small">Projeleri çalıştırmak için bir Node.js sürümü kur. npm ile birlikte gelir; pnpm ve yarn otomatik kullanılır.</p>
      </div>
      <button class="btn primary" :disabled="!recommended || installingNode(recommended.branch)" @click="Api.Install('node', recommended.branch)">
        <Icon :name="recommended && installingNode(recommended.branch) ? 'refresh' : 'download'" :size="15" :class="{ spin: recommended && installingNode(recommended.branch) }" />
        {{ recommended ? `Node ${recommended.version}${recommended.lts ? ' LTS' : ''} kur` : 'Sürümler yükleniyor…' }}
      </button>
    </div>

    <div class="row versions" v-else>
      <span class="muted small">Node sürümleri:</span>
      <span v-for="v in node.installed" :key="v.branch" class="badge" :class="v.default ? 'accent' : ''">
        {{ v.version }}<Icon v-if="v.default" name="star" :size="11" />
      </span>
      <button class="btn ghost sm" @click="showVersions = !showVersions">{{ showVersions ? 'Kapat' : 'Yönet' }}</button>
    </div>

    <div class="card versions-panel" v-if="showVersions && node.installed.length">
      <div v-for="r in releases" :key="r.branch" class="row ver-row">
        <div class="ver">{{ r.branch }}</div>
        <div class="grow">
          <strong>Node {{ r.version }}</strong>
          <span class="badge" :class="r.lts ? 'green' : ''" style="margin-left:8px">{{ r.lts ? 'LTS · ' + r.lts : 'Current' }}</span>
        </div>
        <template v-if="node.installed.find(v => v.branch === r.branch)">
          <span v-if="node.default === r.branch" class="badge accent"><Icon name="star" :size="11" /> Varsayılan</span>
          <button v-else class="btn sm" @click="call('ndef', () => Api.SetDefaultNode(r.branch), `Varsayılan Node ${r.branch}`)">Varsayılan yap</button>
          <button class="btn sm icon ghost danger" title="Kaldır" @click="call('nrm', () => Api.RemoveNode(r.branch), `Node ${r.branch} kaldırıldı`)"><Icon name="trash" :size="15" /></button>
        </template>
        <button v-else class="btn sm primary" :disabled="installingNode(r.branch)" @click="Api.Install('node', r.branch)">
          <Icon :name="installingNode(r.branch) ? 'refresh' : 'download'" :size="14" :class="{ spin: installingNode(r.branch) }" /> Kur
        </button>
      </div>
      <p v-if="!releases.length" class="muted small" style="padding:14px 18px">Sürüm listesi alınıyor…</p>
    </div>

    <!-- projects -->
    <div v-if="!apps.length" class="card empty">
      <div class="icon-wrap"><Icon name="hexagon" :size="22" /></div>
      <h2>Henüz Node projesi yok</h2>
      <p>"Yeni proje" ile şablondan başla ya da var olan bir projeyi <a @click="Api.OpenFolder('nodejs')">nodejs klasörüne</a> kopyala.</p>
    </div>

    <div v-for="app in apps" :key="app.name" class="card app">
      <div class="row app-head">
        <div class="fw" :style="{ background: frameworkColor(app.framework) }">{{ (app.framework || app.name)[0].toUpperCase() }}</div>
        <div class="grow">
          <div class="row" style="gap:8px; flex-wrap:wrap">
            <strong class="app-name">{{ app.name }}</strong>
            <span v-if="app.framework" class="badge">{{ app.framework }}</span>
            <span class="badge">{{ app.manager }}</span>
            <span v-if="app.version" class="muted small">v{{ app.version }}</span>
          </div>
          <p class="muted small desc" v-if="app.description">{{ app.description }}</p>
        </div>
        <select class="input sm" :value="app.node" title="Bu projeyi çalıştıran Node sürümü"
                @change="e => call('anode', () => Api.SetAppNode(app.name, e.target.value), `${app.name} → Node ${e.target.value}`)">
          <option v-for="v in node.installed" :key="v.branch" :value="v.branch">Node {{ v.branch }}</option>
        </select>
        <button class="btn ghost sm icon" title="Editörde aç" @click="Api.OpenInEditor(app.path)"><Icon name="editor" :size="15" /></button>
        <button class="btn ghost sm icon" title="Klasörü aç" @click="Api.OpenFolder(app.path)"><Icon name="folder" :size="15" /></button>
        <button class="btn ghost sm icon" title="Terminal" @click="Api.OpenTerminal(app.path)"><Icon name="terminal" :size="15" /></button>
      </div>

      <div class="alert inline" v-if="!app.hasModules && !app.installing">
        <Icon name="alert" :size="16" />
        <span class="grow">Paketler kurulu değil (<span class="mono">node_modules</span> yok).</span>
        <button class="btn sm primary" @click="install(app)"><Icon name="download" :size="14" /> Paketleri kur</button>
      </div>

      <div class="scripts">
        <button class="script install" :class="{ running: app.installing }" :disabled="app.installing" @click="install(app)"
                :title="`${app.manager} install`">
          <Icon :name="app.installing ? 'refresh' : 'download'" :size="13" :class="{ spin: app.installing }" />
          {{ app.installing ? 'Kuruluyor…' : 'Paketleri kur' }}
        </button>
        <div v-for="sc in app.scripts" :key="sc.name" class="script-wrap">
          <button class="script" :class="{ running: sc.running, failed: !sc.running && sc.exitCode > 0 }" :title="sc.command" @click="run(app, sc)">
            <Icon :name="sc.running ? 'stop' : 'play'" :size="12" />
            <span>{{ sc.name }}</span>
          </button>
          <button v-if="sc.running && sc.url" class="url" @click="Api.OpenURL(sc.url)">
            {{ sc.url.replace(/^https?:\/\//, '') }} <Icon name="external" :size="12" />
          </button>
        </div>
        <span v-if="!app.scripts.length" class="muted small">package.json'da script yok.</span>
      </div>

      <div class="logs" v-if="runsOf(app).length">
        <div class="row tabs">
          <button v-for="k in runsOf(app)" :key="k" class="tab" :class="{ active: open[app.name] === k }"
                  @click="open[app.name] === k ? (open[app.name] = '') : showLog(app, k)">
            <span class="dot" :class="{ on: k === '__install' ? app.installing : app.scripts.find(x => x.name === k)?.running }"></span>
            {{ runLabel(k) }}
          </button>
          <span class="spacer"></span>
          <button v-if="open[app.name]" class="btn ghost sm" @click="Api.ClearNodeLog(app.name, open[app.name]); store.nodeLogs[app.name + '/' + open[app.name]] = []">Temizle</button>
        </div>
        <pre v-if="open[app.name]" class="log selectable" :ref="el => (logBox[app.name] = el)">{{ (store.nodeLogs[app.name + '/' + open[app.name]] || []).join('\n') || '…' }}</pre>
      </div>
    </div>

    <!-- create dialog -->
    <Transition name="fade">
      <div class="overlay" v-if="creating" @click.self="!createdName && (creating = false)">
        <form class="card modal" v-if="!createdName" @submit.prevent="create">
          <div class="modal-icon"><Icon name="hexagon" :size="22" /></div>
          <h2>Yeni Node.js projesi</h2>
          <label class="field">
            <span>Proje adı</span>
            <input class="input" v-model="newName" placeholder="my-app" v-focus />
          </label>
          <div class="field">
            <span>Şablon</span>
            <div class="tpl-grid">
              <button v-for="t in templates" :key="t.id" type="button" class="tpl" :class="{ active: newTpl === t.id }" @click="newTpl = t.id">
                <strong>{{ t.name }}</strong><small>{{ t.desc }}</small>
              </button>
            </div>
          </div>
          <p class="muted small">Klasör: <span class="mono">nodejs/{{ slug || 'my-app' }}</span> · paketler otomatik kurulur</p>
          <div class="row modal-foot">
            <span class="spacer"></span>
            <button type="button" class="btn ghost" @click="creating = false">Vazgeç</button>
            <button class="btn primary" :disabled="store.busy.create || !slug"><Icon name="plus" :size="15" /> Oluştur</button>
          </div>
        </form>

        <div class="card modal wide" v-else>
          <div class="row">
            <div class="modal-icon" :class="{ ok: createDone }"><Icon :name="createDone ? 'check' : 'refresh'" :size="22" :class="{ spin: !createDone }" /></div>
            <div class="grow">
              <h2>{{ createDone ? `${createdName} hazır! 🎉` : `${createdName} oluşturuluyor…` }}</h2>
              <p class="muted small">{{ createDone ? 'Paketler kuruldu. Script\'leri proje kartından çalıştırabilirsin.' : 'Şablon indiriliyor ve paketler kuruluyor, bu bir-iki dakika sürebilir.' }}</p>
            </div>
          </div>
          <pre class="log selectable">{{ createLog.join('\n') || '…' }}</pre>
          <div class="row modal-foot">
            <button class="btn" v-if="createdApp" @click="Api.OpenInEditor(createdApp.path)"><Icon name="editor" :size="15" /> Editörde aç</button>
            <span class="spacer"></span>
            <button class="btn" :class="createDone ? 'primary' : ''" @click="creating = false">{{ createDone ? 'Tamam' : 'Arka planda devam et' }}</button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.runtime { display: flex; align-items: center; gap: 16px; margin-bottom: 14px; }
.rt-icon { width: 46px; height: 46px; border-radius: 12px; display: grid; place-items: center; background: var(--green-soft); color: var(--green); flex: none; }
.versions { margin-bottom: 14px; flex-wrap: wrap; gap: 8px; }
.versions-panel { margin-bottom: 14px; }
.ver-row { padding: 12px 18px; border-bottom: 1px solid var(--border); gap: 10px; }
.ver-row:last-child { border-bottom: none; }
.ver { width: 38px; height: 38px; border-radius: 10px; display: grid; place-items: center; font-weight: 700; background: var(--green-soft); color: var(--green); flex: none; }

.app { margin-bottom: 14px; overflow: hidden; }
.app-head { padding: 16px 18px 12px; gap: 8px; }
.fw { width: 40px; height: 40px; border-radius: 11px; display: grid; place-items: center; color: #fff; font-weight: 700; font-size: 17px; flex: none; margin-right: 4px;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, .14); }
.app-name { font-size: 15px; }
.desc { margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.alert.inline { margin: 0 18px 12px; padding: 9px 12px; }

.scripts { display: flex; flex-wrap: wrap; gap: 8px; padding: 0 18px 16px; align-items: center; }
.script-wrap { display: flex; align-items: center; gap: 4px; }
.script { display: inline-flex; align-items: center; gap: 7px; height: 30px; padding: 0 12px; border-radius: 8px;
  border: 1px solid var(--border); background: var(--panel-2); color: var(--text); font: inherit; font-weight: 600; font-size: 12.5px; cursor: pointer; }
.script:hover:not(:disabled) { border-color: var(--accent); }
.script svg { color: var(--accent-text); }
.script.running { background: var(--green-soft); border-color: var(--green); color: var(--green); }
.script.running svg { color: var(--green); }
.script.failed { border-color: var(--red); }
.script.install { background: transparent; }
.url { display: inline-flex; align-items: center; gap: 5px; height: 30px; padding: 0 10px; border-radius: 8px; border: none;
  background: var(--accent-soft); color: var(--accent-text); font: 600 12px "Cascadia Code", Consolas, monospace; cursor: pointer; }

.logs { border-top: 1px solid var(--border); background: var(--panel-2); }
.tabs { padding: 6px 10px; gap: 4px; flex-wrap: wrap; }
.tab { display: inline-flex; align-items: center; gap: 7px; height: 28px; padding: 0 10px; border: none; border-radius: 7px;
  background: transparent; color: var(--muted); font: inherit; font-size: 12.5px; font-weight: 600; cursor: pointer; }
.tab:hover { color: var(--text); }
.tab.active { background: var(--panel); color: var(--text); box-shadow: var(--shadow); }
.log { margin: 0; max-height: 280px; min-height: 120px; overflow: auto; padding: 12px 16px; background: #0b0d12; color: #d6dae3;
  font: 12px/1.55 "Cascadia Code", Consolas, monospace; white-space: pre-wrap; word-break: break-all; }

.overlay { position: fixed; inset: 0; z-index: 25; display: grid; place-items: center; padding: 20px;
  background: rgba(10, 12, 18, .45); backdrop-filter: blur(3px); }
.modal { width: 100%; max-width: 520px; padding: 26px; display: flex; flex-direction: column; gap: 12px;
  box-shadow: 0 24px 60px rgba(0, 0, 0, .3); animation: pop .18s ease-out; }
.modal.wide { max-width: 680px; }
.modal h2 { font-size: 18px; }
.modal .log { border-radius: 10px; max-height: 320px; }
.modal-icon { width: 44px; height: 44px; border-radius: 12px; display: grid; place-items: center; background: var(--accent-soft); color: var(--accent-text); flex: none; }
.modal-icon.ok { background: var(--green-soft); color: var(--green); }
.field { display: flex; flex-direction: column; gap: 6px; font-weight: 550; font-size: 13px; }
.tpl-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
.tpl { display: flex; flex-direction: column; gap: 2px; text-align: left; padding: 10px 12px; border-radius: 10px;
  border: 1px solid var(--border); background: var(--panel); color: var(--text); font: inherit; cursor: pointer; }
.tpl small { color: var(--muted); font-weight: 400; font-size: 11.5px; }
.tpl:hover { border-color: var(--accent); }
.tpl.active { border-color: var(--accent); background: var(--accent-soft); }
.modal-foot { margin-top: 6px; }
@keyframes pop { from { transform: scale(.94); opacity: 0; } to { transform: scale(1); opacity: 1; } }
.fade-enter-active, .fade-leave-active { transition: opacity .15s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
