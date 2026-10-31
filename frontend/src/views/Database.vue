<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { store, call, Api, toast } from '../store'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'
import Icon from '../components/Icon.vue'

const emit = defineEmits(['navigate'])
const s = computed(() => store.state)
const dbSvc = computed(() => s.value.services.find(x => x.id === 'db'))
const running = computed(() => dbSvc.value.running)
const pma = computed(() => s.value.sites.find(x => x.name === 'phpmyadmin'))

const tab = ref('dbs')
const ov = ref(null)
const loading = ref(false)
const showPw = ref(false)

async function load() {
  if (!running.value) { ov.value = null; return }
  loading.value = true
  try {
    ov.value = await Api.DBOverview()
    if (selected.value && !ov.value.databases.some(d => d.name === selected.value)) selected.value = ''
    if (!selected.value) selected.value = ov.value.databases.find(d => !d.system)?.name || ''
  } catch (e) {
    toast(String(e), 'error')
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(running, load)

const size = n => n >= 1048576 ? (n / 1048576).toFixed(1) + ' MB' : n >= 1024 ? Math.round(n / 1024) + ' KB' : (n || 0) + ' B'
const copy = (text, what) => { ClipboardSetText(String(text)); toast(`${what} kopyalandı`, 'success') }
function genPassword(len = 16) {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#%^*-_'
  const buf = new Uint32Array(len)
  crypto.getRandomValues(buf)
  return [...buf].map(n => chars[n % chars.length]).join('')
}
const envSnippet = computed(() => ov.value && [
  'DB_CONNECTION=mysql', 'DB_HOST=127.0.0.1', `DB_PORT=${ov.value.port}`,
  `DB_DATABASE=${selected.value || 'veritabani'}`, 'DB_USERNAME=root', `DB_PASSWORD=${ov.value.password}`,
].join('\n'))

// ---- databases ----
const selected = ref('')
const dbSearch = ref('')
const newDb = ref('')
const tables = ref([])
const visibleDbs = computed(() => (ov.value?.databases || []).filter(d => d.name.includes(dbSearch.value.toLowerCase())))
const current = computed(() => ov.value?.databases.find(d => d.name === selected.value))

watch(selected, async name => {
  tables.value = []
  if (name) tables.value = (await Api.DBTables(name).catch(e => (toast(String(e), 'error'), []))) || []
})

async function createDb() {
  const name = newDb.value.trim()
  if (!name) return
  await call('db:create', () => Api.CreateDatabase(name), `${name} oluşturuldu`)
  newDb.value = ''
  await load()
  selected.value = name
}

async function dropDb() {
  const name = selected.value
  if (!confirm(`"${name}" veritabanı ve içindeki tüm tablolar kalıcı olarak silinsin mi?\n\nÖnce yedek almanı öneririz.`)) return
  await call('db:drop', () => Api.DropDatabase(name), `${name} silindi`)
  selected.value = ''
  load()
}

async function exportDb() {
  const path = await call('db:export', () => Api.ExportDatabase(selected.value))
  if (path) toast(`Yedek alındı: ${path.split(/[\\/]/).pop()}`, 'success')
}

async function importDb() {
  const id = await call('db:import', () => Api.ImportSQL(selected.value))
  if (id) toast('İçe aktarma başladı', 'info')
}

watch(() => Object.values(store.tasks).filter(t => t.done && t.label.endsWith('içe aktarıldı')).length, () => {
  load()
  if (selected.value) Api.DBTables(selected.value).then(t => (tables.value = t || []))
})

// ---- table preview ----
const preview = reactive({ open: false, table: '', result: null, loading: false })
async function openTable(t) {
  Object.assign(preview, { open: true, table: t.name, result: null, loading: true })
  try {
    preview.result = await Api.DBRows(selected.value, t.name)
  } catch (e) {
    toast(String(e), 'error')
  } finally {
    preview.loading = false
  }
}
async function tableAction(action) {
  const verb = action === 'drop' ? 'silinsin' : 'boşaltılsın (tüm satırlar silinir)'
  if (!confirm(`"${preview.table}" tablosu ${verb} mi?`)) return
  await call('tbl', () => Api.TableAction(selected.value, preview.table, action), action === 'drop' ? 'Tablo silindi' : 'Tablo boşaltıldı')
  preview.open = action === 'truncate'
  if (action === 'truncate') openTable({ name: preview.table })
  tables.value = (await Api.DBTables(selected.value)) || []
  load()
}

// ---- users ----
const users = ref([])
const newUser = reactive({ open: false, user: '', password: '', db: '*' })
const created = ref(null)
async function loadUsers() {
  users.value = (await Api.DBUsers().catch(e => (toast(String(e), 'error'), []))) || []
}
watch(tab, t => { if (t === 'users' && running.value) loadUsers() })
watch(running, r => { if (r && tab.value === 'users') loadUsers() })

function scopeOf(u) {
  const dbs = []
  for (const g of u.grants || []) {
    const m = g.match(/ON (\S+) TO/)
    if (!m || g.startsWith('GRANT USAGE') || g.startsWith('GRANT PROXY')) continue
    dbs.push(m[1] === '*.*' ? 'Tüm veritabanları' : m[1].replace(/`/g, '').replace('.*', ''))
  }
  return dbs.length ? dbs.join(', ') : 'Yetki yok'
}

function openNewUser() {
  Object.assign(newUser, { open: true, user: '', password: genPassword(), db: selected.value && !current.value?.system ? selected.value : '*' })
  created.value = null
}
async function createUser() {
  await call('user:create', () => Api.CreateDBUser(newUser.user.trim(), '%', newUser.password, newUser.db), 'Kullanıcı oluşturuldu')
  created.value = { ...newUser }
  newUser.open = false
  loadUsers()
}
async function changeUserPw(u) {
  const pw = prompt(`${u.user}@${u.host} için yeni şifre:`, genPassword())
  if (pw === null) return
  await call('user:pw', () => Api.SetDBUserPassword(u.user, u.host, pw), 'Şifre değiştirildi')
  if (u.root) load()
}
async function dropUser(u) {
  if (!confirm(`${u.user}@${u.host} kullanıcısı silinsin mi?`)) return
  await call('user:drop', () => Api.DropDBUser(u.user, u.host), 'Kullanıcı silindi')
  loadUsers()
}

// ---- root password ----
const rootPw = reactive({ open: false, value: '', confirm: '', reveal: false })
function openRootPw() { Object.assign(rootPw, { open: true, value: '', confirm: '', reveal: false }) }
async function saveRootPw() {
  if (rootPw.value !== rootPw.confirm) { toast('Şifreler eşleşmiyor', 'error'); return }
  await call('rootpw', () => Api.SetRootPassword(rootPw.value), rootPw.value ? 'Root şifresi değiştirildi' : 'Root şifresi kaldırıldı')
  rootPw.open = false
  load()
}

// ---- SQL console ----
const sql = reactive({ db: '', query: 'SELECT NOW(), VERSION();', result: null, error: '', running: false })
watch(selected, v => { if (!sql.db) sql.db = v })
async function runSQL() {
  sql.running = true
  sql.error = ''
  try {
    sql.result = await Api.RunSQL(sql.db, sql.query)
    if (!sql.result.isQuery) load()
  } catch (e) {
    sql.result = null
    sql.error = String(e)
  } finally {
    sql.running = false
  }
}
</script>

<template>
  <div>
    <div class="page-head">
      <div class="grow">
        <h1>Veritabanı</h1>
        <p class="muted">MariaDB {{ ov?.version?.replace('-MariaDB', '') || dbSvc.version }} · veritabanları, tablolar, kullanıcılar ve yedekler</p>
      </div>
      <button class="btn" :disabled="!pma || !running" @click="Api.OpenURL(pma.url)"><Icon name="external" :size="15" /> phpMyAdmin</button>
      <button class="btn icon" title="Yedekler klasörü" @click="Api.OpenFolder('data/backups')"><Icon name="folder" :size="16" /></button>
    </div>

    <div v-if="!dbSvc.installed" class="card empty">
      <div class="icon-wrap"><Icon name="database" :size="22" /></div>
      <h2>MariaDB kurulu değil</h2>
      <p><a @click="emit('navigate', 'components')">Bileşenler</a> sayfasından kurabilirsin.</p>
    </div>

    <div v-else-if="!running" class="card card-pad row stopped">
      <div class="big-icon"><Icon name="database" :size="22" /></div>
      <div class="grow">
        <h2>MariaDB çalışmıyor</h2>
        <p class="muted small">Veritabanlarını yönetmek için önce sunucuyu başlat.</p>
      </div>
      <button class="btn primary" :disabled="store.busy.db" @click="call('db', () => Api.StartService('db'), 'MariaDB başlatıldı')">
        <Icon :name="store.busy.db ? 'refresh' : 'play'" :size="15" :class="{ spin: store.busy.db }" /> Başlat
      </button>
    </div>

    <template v-else-if="ov">
      <!-- connection -->
      <div class="grid conn-grid">
        <div class="card card-pad">
          <div class="row" style="margin-bottom:12px">
            <h2 class="grow">Bağlantı bilgileri</h2>
            <button class="btn sm" @click="openRootPw"><Icon name="lock" :size="14" /> Root şifresini değiştir</button>
          </div>
          <div class="kv"><span>Sunucu</span><code>{{ ov.host }}</code><button class="btn ghost sm icon" @click="copy(ov.host, 'Sunucu')"><Icon name="copy" :size="13" /></button></div>
          <div class="kv"><span>Port</span><code>{{ ov.port }}</code><button class="btn ghost sm icon" @click="copy(ov.port, 'Port')"><Icon name="copy" :size="13" /></button></div>
          <div class="kv"><span>Kullanıcı</span><code>{{ ov.user }}</code><button class="btn ghost sm icon" @click="copy(ov.user, 'Kullanıcı')"><Icon name="copy" :size="13" /></button></div>
          <div class="kv">
            <span>Şifre</span>
            <code v-if="ov.password">{{ showPw ? ov.password : '••••••••' }}</code>
            <span v-else class="badge amber">boş</span>
            <button v-if="ov.password" class="btn ghost sm" @click="showPw = !showPw">{{ showPw ? 'Gizle' : 'Göster' }}</button>
            <button v-if="ov.password" class="btn ghost sm icon" @click="copy(ov.password, 'Şifre')"><Icon name="copy" :size="13" /></button>
          </div>
        </div>
        <div class="card card-pad env">
          <div class="row" style="margin-bottom:10px">
            <h2 class="grow">.env örneği</h2>
            <button class="btn ghost sm" @click="copy(envSnippet, '.env ayarları')"><Icon name="copy" :size="13" /> Kopyala</button>
          </div>
          <pre class="selectable">{{ envSnippet }}</pre>
        </div>
      </div>

      <!-- tabs -->
      <div class="segmented tabs-bar">
        <button :class="{ active: tab === 'dbs' }" @click="tab = 'dbs'"><Icon name="database" :size="14" /> Veritabanları</button>
        <button :class="{ active: tab === 'users' }" @click="tab = 'users'"><Icon name="settings" :size="14" /> Kullanıcılar</button>
        <button :class="{ active: tab === 'sql' }" @click="tab = 'sql'"><Icon name="terminal" :size="14" /> SQL konsolu</button>
      </div>

      <!-- databases -->
      <div v-if="tab === 'dbs'" class="dbs">
        <div class="card db-list">
          <form class="row new-db" @submit.prevent="createDb">
            <input class="input sm grow" v-model="newDb" placeholder="yeni_veritabani" />
            <button class="btn sm primary icon" :disabled="!newDb.trim() || store.busy['db:create']" title="Oluştur"><Icon name="plus" :size="14" /></button>
          </form>
          <input class="input sm search" v-model="dbSearch" placeholder="Ara…" />
          <button v-for="d in visibleDbs" :key="d.name" class="db-item" :class="{ active: d.name === selected, system: d.system }" @click="selected = d.name">
            <Icon name="database" :size="14" />
            <span class="grow name">{{ d.name }}</span>
            <span class="muted small">{{ d.tables }}</span>
          </button>
        </div>

        <div class="card db-detail" v-if="current">
          <div class="row detail-head">
            <div class="grow">
              <div class="row" style="gap:8px">
                <h2>{{ current.name }}</h2>
                <span v-if="current.system" class="badge">sistem</span>
              </div>
              <p class="muted small">{{ current.tables }} tablo · {{ size(current.size) }}</p>
            </div>
            <button class="btn sm" :disabled="store.busy['db:export']" @click="exportDb"><Icon :name="store.busy['db:export'] ? 'refresh' : 'download'" :size="14" :class="{ spin: store.busy['db:export'] }" /> Yedekle</button>
            <button class="btn sm" v-if="!current.system" @click="importDb"><Icon name="file" :size="14" /> İçe aktar</button>
            <button class="btn sm icon ghost danger" v-if="!current.system" title="Veritabanını sil" @click="dropDb"><Icon name="trash" :size="15" /></button>
          </div>
          <table class="table" v-if="tables.length">
            <thead><tr><th>Tablo</th><th>Satır</th><th>Boyut</th><th>Motor</th></tr></thead>
            <tbody>
              <tr v-for="t in tables" :key="t.name" class="clickable" @click="openTable(t)">
                <td><strong>{{ t.name }}</strong> <span v-if="t.kind === 'VIEW'" class="badge">view</span></td>
                <td class="mono">~{{ t.rows.toLocaleString('tr-TR') }}</td>
                <td class="muted small">{{ size(t.size) }}</td>
                <td class="muted small">{{ t.engine }}</td>
              </tr>
            </tbody>
          </table>
          <div v-else class="empty small">
            <p>Bu veritabanında tablo yok. SQL konsolundan oluşturabilir ya da bir <a @click="importDb">.sql dosyası içe aktarabilirsin</a>.</p>
          </div>
        </div>
        <div class="card empty" v-else><p>Soldan bir veritabanı seç ya da yeni bir tane oluştur.</p></div>
      </div>

      <!-- users -->
      <div v-if="tab === 'users'">
        <div class="card">
          <div class="row card-head-row">
            <h2 class="grow">Kullanıcılar</h2>
            <button class="btn sm primary" @click="openNewUser"><Icon name="plus" :size="14" /> Yeni kullanıcı</button>
          </div>
          <div class="created" v-if="created">
            <Icon name="check" :size="16" />
            <span class="grow"><b>{{ created.user }}</b> oluşturuldu · şifre: <code class="selectable">{{ created.password }}</code></span>
            <button class="btn ghost sm" @click="copy(created.password, 'Şifre')">Kopyala</button>
            <button class="btn ghost sm icon" @click="created = null"><Icon name="x" :size="13" /></button>
          </div>
          <table class="table">
            <thead><tr><th>Kullanıcı</th><th>Erişim</th><th style="width:140px"></th></tr></thead>
            <tbody>
              <tr v-for="u in users" :key="u.user + '@' + u.host">
                <td><strong>{{ u.user }}</strong><span class="muted">@{{ u.host }}</span> <span v-if="u.root" class="badge accent">yönetici</span></td>
                <td class="muted small">{{ scopeOf(u) }}</td>
                <td>
                  <div class="row" style="gap:2px; justify-content:flex-end">
                    <button class="btn ghost sm" @click="changeUserPw(u)">Şifre</button>
                    <button class="btn ghost sm icon danger" v-if="!u.root" title="Sil" @click="dropUser(u)"><Icon name="trash" :size="14" /></button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="muted small tip">Yeni kullanıcılar bu bilgisayardan bağlanabilir (MariaDB sadece 127.0.0.1'i dinler).</p>
      </div>

      <!-- SQL -->
      <div v-if="tab === 'sql'" class="card sql">
        <div class="row sql-bar">
          <select class="input sm" v-model="sql.db" style="width:200px">
            <option value="">(veritabanı yok)</option>
            <option v-for="d in ov.databases" :key="d.name" :value="d.name">{{ d.name }}</option>
          </select>
          <span class="muted small grow">Ctrl + Enter ile çalıştır · en fazla 1000 satır gösterilir</span>
          <button class="btn sm primary" :disabled="sql.running || !sql.query.trim()" @click="runSQL">
            <Icon :name="sql.running ? 'refresh' : 'play'" :size="13" :class="{ spin: sql.running }" /> Çalıştır
          </button>
        </div>
        <textarea class="editor selectable" v-model="sql.query" spellcheck="false" @keydown.ctrl.enter.prevent="runSQL"></textarea>
        <div class="sql-out">
          <p v-if="sql.error" class="sql-error selectable">{{ sql.error }}</p>
          <template v-else-if="sql.result">
            <p class="muted small sql-meta">
              <template v-if="sql.result.isQuery">{{ sql.result.rows.length }} satır{{ sql.result.truncated ? ' (ilk 1000)' : '' }}</template>
              <template v-else>{{ sql.result.affected }} satır etkilendi</template>
              · {{ sql.result.millis }} ms
            </p>
            <div class="grid-wrap" v-if="sql.result.isQuery && sql.result.columns.length">
              <table class="data">
                <thead><tr><th v-for="c in sql.result.columns" :key="c">{{ c }}</th></tr></thead>
                <tbody>
                  <tr v-for="(r, i) in sql.result.rows" :key="i">
                    <td v-for="(v, j) in r" :key="j" :class="{ null: v === null }">{{ v === null ? 'NULL' : v }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </template>
        </div>
      </div>
    </template>

    <!-- table preview -->
    <Transition name="fade">
      <div class="overlay" v-if="preview.open" @click.self="preview.open = false">
        <div class="card modal wide">
          <div class="row">
            <div class="grow">
              <h2>{{ selected }}.{{ preview.table }}</h2>
              <p class="muted small" v-if="preview.result">İlk {{ preview.result.rows.length }} satır{{ preview.result.truncated ? ' (200 ile sınırlı)' : '' }} · {{ preview.result.millis }} ms</p>
            </div>
            <template v-if="!current?.system">
              <button class="btn sm ghost danger" @click="tableAction('truncate')">Boşalt</button>
              <button class="btn sm ghost danger" @click="tableAction('drop')"><Icon name="trash" :size="14" /> Sil</button>
            </template>
            <button class="btn sm icon ghost" @click="preview.open = false"><Icon name="x" :size="15" /></button>
          </div>
          <p v-if="preview.loading" class="muted small">Yükleniyor…</p>
          <div class="grid-wrap tall" v-else-if="preview.result">
            <table class="data">
              <thead><tr><th v-for="c in preview.result.columns" :key="c">{{ c }}</th></tr></thead>
              <tbody>
                <tr v-for="(r, i) in preview.result.rows" :key="i">
                  <td v-for="(v, j) in r" :key="j" :class="{ null: v === null }">{{ v === null ? 'NULL' : v }}</td>
                </tr>
              </tbody>
            </table>
            <p v-if="!preview.result.rows.length" class="muted small" style="padding:14px">Tablo boş.</p>
          </div>
        </div>
      </div>
    </Transition>

    <!-- new user -->
    <Transition name="fade">
      <div class="overlay" v-if="newUser.open" @click.self="newUser.open = false">
        <form class="card modal" @submit.prevent="createUser">
          <h2>Yeni kullanıcı</h2>
          <label class="field"><span>Kullanıcı adı</span><input class="input" v-model="newUser.user" placeholder="uygulama" /></label>
          <label class="field">
            <span>Şifre</span>
            <div class="row" style="gap:6px">
              <input class="input grow mono" v-model="newUser.password" />
              <button type="button" class="btn sm" @click="newUser.password = genPassword()">Üret</button>
            </div>
          </label>
          <label class="field">
            <span>Erişebileceği veritabanı</span>
            <select class="input" v-model="newUser.db">
              <option value="*">Tüm veritabanları</option>
              <option v-for="d in ov.databases.filter(x => !x.system)" :key="d.name" :value="d.name">{{ d.name }}</option>
            </select>
          </label>
          <div class="row modal-foot">
            <span class="spacer"></span>
            <button type="button" class="btn ghost" @click="newUser.open = false">Vazgeç</button>
            <button class="btn primary" :disabled="!newUser.user.trim() || store.busy['user:create']">Oluştur</button>
          </div>
        </form>
      </div>
    </Transition>

    <!-- root password -->
    <Transition name="fade">
      <div class="overlay" v-if="rootPw.open" @click.self="rootPw.open = false">
        <form class="card modal" @submit.prevent="saveRootPw">
          <h2>Root şifresini değiştir</h2>
          <p class="muted small">PAMPP ve phpMyAdmin otomatik güncellenir. Projelerinin <span class="mono">.env</span> / config dosyalarındaki şifreyi kendin değiştirmen gerekir. Boş bırakırsan şifre kaldırılır.</p>
          <label class="field"><span>Yeni şifre</span><input class="input mono" :type="rootPw.reveal ? 'text' : 'password'" v-model="rootPw.value" /></label>
          <label class="field"><span>Tekrar</span><input class="input mono" :type="rootPw.reveal ? 'text' : 'password'" v-model="rootPw.confirm" /></label>
          <div class="row" style="gap:8px">
            <label class="row small" style="gap:6px; cursor:pointer"><input type="checkbox" v-model="rootPw.reveal" /> Göster</label>
            <span class="spacer"></span>
            <button type="button" class="btn sm" @click="rootPw.value = rootPw.confirm = genPassword(); rootPw.reveal = true">Güçlü şifre üret</button>
          </div>
          <div class="row modal-foot">
            <span class="spacer"></span>
            <button type="button" class="btn ghost" @click="rootPw.open = false">Vazgeç</button>
            <button class="btn primary" :disabled="rootPw.value !== rootPw.confirm || store.busy.rootpw">Kaydet</button>
          </div>
        </form>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.stopped { gap: 16px; }
.big-icon { width: 46px; height: 46px; border-radius: 12px; display: grid; place-items: center; background: var(--panel-2); color: var(--muted); flex: none; }
.conn-grid { grid-template-columns: 1.2fr 1fr; margin-bottom: 16px; }
.kv { display: flex; align-items: center; gap: 10px; padding: 7px 0; border-top: 1px solid var(--border); }
.kv:first-of-type { border-top: none; }
.kv > span:first-child { width: 80px; color: var(--muted); font-size: 13px; }
.kv code { flex: 1; font-family: "Cascadia Code", Consolas, monospace; font-size: 13px; }
.env pre { margin: 0; padding: 12px 14px; border-radius: 10px; background: var(--panel-2); font: 12.5px/1.6 "Cascadia Code", Consolas, monospace; white-space: pre-wrap; word-break: break-all; }

.tabs-bar { display: inline-flex; margin-bottom: 14px; }
.segmented { background: var(--panel-2); padding: 3px; border-radius: 10px; gap: 2px; }
.segmented button { height: 32px; padding: 0 14px; border: none; border-radius: 8px; background: transparent; color: var(--muted);
  font: inherit; font-weight: 600; cursor: pointer; display: flex; gap: 7px; align-items: center; }
.segmented button.active { background: var(--panel); color: var(--text); box-shadow: var(--shadow); }

.dbs { display: grid; grid-template-columns: 240px 1fr; gap: 14px; align-items: start; }
.db-list { padding: 8px; display: flex; flex-direction: column; gap: 2px; max-height: 560px; overflow-y: auto; }
.new-db { gap: 6px; margin-bottom: 6px; }
.search { margin-bottom: 6px; }
.db-item { display: flex; align-items: center; gap: 9px; width: 100%; padding: 8px 10px; border: none; border-radius: 8px;
  background: transparent; color: var(--text); font: inherit; cursor: pointer; text-align: left; }
.db-item svg { color: var(--accent-text); flex: none; }
.db-item .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.db-item:hover { background: var(--panel-2); }
.db-item.active { background: var(--accent-soft); color: var(--accent-text); }
.db-item.system { opacity: .65; }
.db-item.system svg { color: var(--muted); }
.detail-head { padding: 16px 18px; gap: 8px; border-bottom: 1px solid var(--border); }
.clickable { cursor: pointer; }

.card-head-row { padding: 14px 18px; border-bottom: 1px solid var(--border); }
.created { display: flex; align-items: center; gap: 10px; margin: 12px 18px 0; padding: 10px 12px; border-radius: 10px; background: var(--green-soft); color: var(--green); }
.created code { font-family: "Cascadia Code", Consolas, monospace; }
.tip { margin-top: 12px; }

.sql { overflow: hidden; }
.sql-bar { padding: 12px 14px; gap: 10px; border-bottom: 1px solid var(--border); }
.editor { display: block; width: 100%; min-height: 150px; resize: vertical; border: none; outline: none; padding: 14px 16px;
  background: #0b0d12; color: #e3e6ee; font: 13px/1.6 "Cascadia Code", Consolas, monospace; }
.sql-out { padding: 0 0 4px; }
.sql-meta { padding: 10px 16px 6px; }
.sql-error { margin: 12px 16px; padding: 10px 12px; border-radius: 9px; background: var(--red-soft); color: var(--red); font: 12.5px "Cascadia Code", Consolas, monospace; }

.grid-wrap { overflow: auto; max-height: 380px; border-top: 1px solid var(--border); }
.grid-wrap.tall { max-height: 60vh; border: 1px solid var(--border); border-radius: 10px; }
.data { border-collapse: collapse; font: 12.5px "Cascadia Code", Consolas, monospace; min-width: 100%; }
.data th { position: sticky; top: 0; background: var(--panel-2); text-align: left; padding: 7px 12px; font-weight: 600; border-bottom: 1px solid var(--border); white-space: nowrap; }
.data td { padding: 6px 12px; border-bottom: 1px solid var(--border); white-space: nowrap; max-width: 360px; overflow: hidden; text-overflow: ellipsis; user-select: text; }
.data td.null { color: var(--muted); font-style: italic; }

.overlay { position: fixed; inset: 0; z-index: 25; display: grid; place-items: center; padding: 20px;
  background: rgba(10, 12, 18, .45); backdrop-filter: blur(3px); }
.modal { width: 100%; max-width: 460px; padding: 24px; display: flex; flex-direction: column; gap: 12px;
  box-shadow: 0 24px 60px rgba(0, 0, 0, .3); animation: pop .18s ease-out; }
.modal.wide { max-width: 980px; }
.modal h2 { font-size: 18px; }
.field { display: flex; flex-direction: column; gap: 6px; font-weight: 550; font-size: 13px; }
.modal-foot { margin-top: 6px; }
@keyframes pop { from { transform: scale(.95); opacity: 0; } to { transform: scale(1); opacity: 1; } }
.fade-enter-active, .fade-leave-active { transition: opacity .15s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
