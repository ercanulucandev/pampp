<script setup>
import { computed, reactive, watch } from 'vue'
import { store, call, Api } from '../store'
import Icon from '../components/Icon.vue'

const s = computed(() => store.state)
const form = reactive({})
const reset = () => Object.assign(form, JSON.parse(JSON.stringify(s.value.config)))
reset()
watch(() => s.value.config, (next, prev) => {
  if (JSON.stringify(next) !== JSON.stringify(prev)) reset()
})

const dirty = computed(() => ['webServer', 'httpPort', 'ssl', 'httpsPort', 'dbPort', 'tld', 'phpWorkers', 'autoStart', 'closeToTray']
  .some(k => form[k] !== s.value.config[k]))
const installed = id => s.value.components.find(c => c.id === id)?.installed

function save() {
  call('settings', () => Api.SaveSettings({
    ...s.value.config, ...form,
    httpPort: Number(form.httpPort), httpsPort: Number(form.httpsPort), dbPort: Number(form.dbPort), phpWorkers: Number(form.phpWorkers),
  }), 'Ayarlar kaydedildi')
}
</script>

<template>
  <div>
    <div class="page-head">
      <div class="grow">
        <h1>Ayarlar</h1>
        <p class="muted">Değişiklikler kaydedildiğinde çalışan servisler otomatik yeniden yüklenir.</p>
      </div>
      <button class="btn ghost" v-if="dirty" @click="reset">Geri al</button>
      <button class="btn primary" :disabled="!dirty || store.busy.settings" @click="save">
        <Icon name="check" :size="15" /> Kaydet
      </button>
    </div>

    <div class="card">
      <div class="setting">
        <div class="grow">
          <strong>Web sunucusu</strong>
          <p class="muted small">Tüm siteler ve PHP havuzları ikisiyle de çalışır.</p>
        </div>
        <div class="segmented">
          <button v-for="w in ['nginx', 'apache']" :key="w" :class="{ active: form.webServer === w }" @click="form.webServer = w">
            {{ w === 'nginx' ? 'Nginx' : 'Apache' }}
            <span v-if="!installed(w)" class="muted small">(kurulu değil)</span>
          </button>
        </div>
      </div>

      <div class="setting">
        <div class="grow">
          <strong>HTTP portu</strong>
          <p class="muted small">Genelde 80. Başka bir program kullanıyorsa 8080 deneyebilirsin.</p>
        </div>
        <input class="input" type="number" v-model.number="form.httpPort" style="width:110px" />
      </div>

      <div class="setting">
        <div class="grow">
          <strong>MariaDB portu</strong>
          <p class="muted small">Varsayılan 3306. Kullanıcı <span class="mono">root</span>, şifre boş.</p>
        </div>
        <input class="input" type="number" v-model.number="form.dbPort" style="width:110px" />
      </div>

      <div class="setting">
        <div class="grow">
          <strong>Alan adı uzantısı</strong>
          <p class="muted small">
            <span class="mono">test</span> için hosts dosyası otomatik güncellenir (yönetici onayı ister).
            <span class="mono">localhost</span> seçersen hosts'a hiç dokunulmaz.
          </p>
        </div>
        <div class="row" style="gap:6px">
          <span class="muted mono">site.</span>
          <input class="input" v-model="form.tld" style="width:110px" />
        </div>
      </div>

      <div class="setting">
        <div class="grow">
          <strong>PHP işçi sayısı</strong>
          <p class="muted small">Her PHP sürümü için aynı anda çalışan php-cgi süreci (1–9).</p>
        </div>
        <input class="input" type="number" min="1" max="9" v-model.number="form.phpWorkers" style="width:110px" />
      </div>

      <div class="setting">
        <div class="grow">
          <strong>Açılışta servisleri başlat</strong>
          <p class="muted small">PAMPP açılınca web sunucusu ve MariaDB otomatik başlar.</p>
        </div>
        <label class="switch"><input type="checkbox" v-model="form.autoStart" /><span></span></label>
      </div>

      <div class="setting">
        <div class="grow">
          <strong>Kapatınca sistem tepsisine küçült</strong>
          <p class="muted small">Pencere kapanır, servisler çalışmaya devam eder. Çıkmak için tepsi menüsünü kullan.</p>
        </div>
        <label class="switch"><input type="checkbox" v-model="form.closeToTray" /><span></span></label>
      </div>
    </div>

    <div class="row section-title">
      <Icon name="lock" :size="16" /><h2>SSL / HTTPS</h2>
    </div>
    <div class="card">
      <div class="setting">
        <div class="grow">
          <strong>HTTPS'i etkinleştir</strong>
          <p class="muted small">Tüm siteler <span class="mono">https://</span> ile de açılır. Sertifikalar PAMPP'ın yerel otoritesi tarafından otomatik üretilir ve yenilenir.</p>
        </div>
        <label class="switch"><input type="checkbox" v-model="form.ssl" /><span></span></label>
      </div>
      <div class="setting" v-if="form.ssl">
        <div class="grow">
          <strong>HTTPS portu</strong>
          <p class="muted small">Varsayılan 443.</p>
        </div>
        <input class="input" type="number" v-model.number="form.httpsPort" style="width:110px" />
      </div>
      <div class="setting">
        <div class="ca-icon" :class="s.ssl.trusted ? 'ok' : 'warn'"><Icon :name="s.ssl.trusted ? 'check' : 'alert'" :size="18" /></div>
        <div class="grow">
          <strong>Yerel sertifika otoritesi</strong>
          <p class="muted small" v-if="!s.ssl.caExists">Web sunucusu ilk başladığında ya da aşağıdaki butona basınca oluşturulur.</p>
          <p class="muted small" v-else-if="s.ssl.trusted">Windows tarafından güveniliyor. Chrome, Edge ve Firefox'ta kilit simgesi görünür. Geçerlilik: {{ s.ssl.expires }}</p>
          <p class="muted small" v-else>Henüz güvenilir değil, tarayıcı uyarı gösterir. "Güvenilir yap"a tıklayınca Windows bir onay penceresi açar; yönetici yetkisi gerekmez.</p>
        </div>
        <button v-if="!s.ssl.trusted" class="btn primary" :disabled="store.busy.trust"
                @click="call('trust', () => Api.TrustCA(), 'Sertifika artık güvenilir 🔒')">
          <Icon name="lock" :size="15" /> Güvenilir yap
        </button>
        <button v-else class="btn ghost danger" :disabled="store.busy.trust"
                @click="call('trust', () => Api.UntrustCA(), 'Sertifika güven listesinden kaldırıldı')">Kaldır</button>
      </div>
    </div>

    <div class="card card-pad about row">
      <div class="grow">
        <strong>Kurulum klasörü</strong>
        <p class="muted small mono selectable">{{ s.root }}</p>
      </div>
      <button class="btn" @click="Api.OpenFolder('')"><Icon name="folder" :size="15" /> Aç</button>
    </div>
  </div>
</template>

<style scoped>
.setting { display: flex; align-items: center; gap: 20px; padding: 16px 18px; border-bottom: 1px solid var(--border); }
.setting:last-child { border-bottom: none; }
.setting p { margin-top: 3px; max-width: 520px; }
.segmented { display: flex; background: var(--panel-2); padding: 3px; border-radius: 10px; gap: 2px; }
.segmented button { height: 30px; padding: 0 14px; border: none; border-radius: 8px; background: transparent; color: var(--muted);
  font: inherit; font-weight: 550; cursor: pointer; display: flex; gap: 5px; align-items: center; }
.segmented button.active { background: var(--panel); color: var(--text); box-shadow: var(--shadow); }
.about { margin-top: 14px; }
.section-title { margin: 26px 0 12px; gap: 8px; color: var(--accent-text); }
.section-title h2 { color: var(--text); }
.ca-icon { width: 36px; height: 36px; border-radius: 10px; display: grid; place-items: center; flex: none; }
.ca-icon.ok { background: var(--green-soft); color: var(--green); }
.ca-icon.warn { background: var(--amber-soft); color: var(--amber); }
</style>
