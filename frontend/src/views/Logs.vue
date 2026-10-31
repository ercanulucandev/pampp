<script setup>
import { nextTick, onMounted, onUnmounted, ref } from 'vue'
import { call, Api } from '../store'
import Icon from '../components/Icon.vue'

const files = ref([])
const active = ref('')
const content = ref('')
const follow = ref(true)
const box = ref(null)
let timer

async function loadFiles() {
  files.value = (await Api.GetLogs()) || []
  if (!active.value && files.value.length) open(files.value[0].name)
}

async function open(name) {
  active.value = name
  await load()
}

async function load() {
  if (!active.value) return
  try {
    const text = await Api.ReadLog(active.value)
    if (text === content.value) return
    content.value = text
    if (follow.value) {
      await nextTick()
      if (box.value) box.value.scrollTop = box.value.scrollHeight
    }
  } catch { /* file rotated or removed */ }
}

async function clear() {
  await call('clear', () => Api.ClearLog(active.value))
  content.value = ''
  loadFiles()
}

const size = n => n > 1048576 ? (n / 1048576).toFixed(1) + ' MB' : n > 1024 ? (n / 1024).toFixed(0) + ' KB' : n + ' B'

onMounted(() => {
  loadFiles()
  timer = setInterval(() => { load(); loadFiles() }, 2000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="logs-page">
    <div class="page-head">
      <div class="grow">
        <h1>Loglar</h1>
        <p class="muted">Canlı takip, son 128 KB gösterilir.</p>
      </div>
      <label class="row small" style="gap:8px; cursor:pointer">
        <span class="switch"><input type="checkbox" v-model="follow" /><span></span></span> Otomatik kaydır
      </label>
      <button class="btn" :disabled="!active" @click="clear"><Icon name="trash" :size="15" /> Temizle</button>
      <button class="btn icon" title="Klasörü aç" @click="Api.OpenFolder('logs')"><Icon name="folder" :size="16" /></button>
    </div>

    <div class="layout">
      <div class="card files">
        <button v-for="f in files" :key="f.name" class="file" :class="{ active: f.name === active }" @click="open(f.name)">
          <span class="grow mono">{{ f.name }}</span>
          <span class="muted small">{{ size(f.size) }}</span>
        </button>
        <p v-if="!files.length" class="muted small" style="padding:14px">Henüz log yok.</p>
      </div>
      <pre ref="box" class="card viewer selectable">{{ content || (active ? '(boş)' : '') }}</pre>
    </div>
  </div>
</template>

<style scoped>
.layout { display: grid; grid-template-columns: 230px 1fr; gap: 14px; height: calc(100vh - 190px); min-height: 320px; }
.files { padding: 6px; overflow-y: auto; }
.file { display: flex; width: 100%; align-items: center; gap: 8px; padding: 9px 10px; border: none; border-radius: 8px;
  background: transparent; color: var(--text); font: inherit; cursor: pointer; text-align: left; }
.file:hover { background: var(--panel-2); }
.file.active { background: var(--accent-soft); color: var(--accent-text); }
.file .mono { overflow: hidden; text-overflow: ellipsis; }
.viewer { margin: 0; padding: 16px; overflow: auto; white-space: pre-wrap; word-break: break-all;
  font-family: "Cascadia Code", Consolas, monospace; font-size: 12px; line-height: 1.55; }
</style>
