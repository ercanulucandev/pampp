import { reactive } from 'vue'
import * as Api from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'

export const store = reactive({
  state: null,
  catalog: null,
  catalogLoading: false,
  tasks: {},
  toasts: [],
  busy: {},
})

let toastId = 0
export function toast(message, kind = 'info') {
  const id = ++toastId
  store.toasts.push({ id, message, kind })
  setTimeout(() => {
    store.toasts = store.toasts.filter(t => t.id !== id)
  }, kind === 'error' ? 7000 : 3500)
}

// call runs a backend method, tracks a busy key and shows errors as toasts.
export async function call(key, fn, success) {
  store.busy[key] = true
  try {
    const res = await fn()
    if (success) toast(success, 'success')
    return res
  } catch (e) {
    toast(String(e), 'error')
    throw e
  } finally {
    delete store.busy[key]
    await refresh()
  }
}

export async function refresh() {
  store.state = await Api.GetState()
  for (const t of store.state.tasks || []) store.tasks[t.id] = t
  for (const id of Object.keys(store.tasks)) {
    if (!(store.state.tasks || []).some(t => t.id === id)) delete store.tasks[id]
  }
}

export async function loadCatalog(force = false) {
  if (store.catalogLoading) return
  store.catalogLoading = true
  try {
    store.catalog = await Api.GetCatalog(force)
  } catch (e) {
    toast(String(e), 'error')
  } finally {
    store.catalogLoading = false
  }
}

export function init() {
  EventsOn('state', s => {
    store.state = s
  })
  EventsOn('task', t => {
    const prev = store.tasks[t.id]
    store.tasks[t.id] = t
    if (t.done && !(prev && prev.done)) {
      t.error ? toast(t.error, 'error') : toast(t.label, 'success')
      // First-run setup finished: greet the user with the start page.
      if (!t.error && t.label === 'Kurulum tamamlandı') {
        refresh().then(() => store.state?.startUrl && Api.OpenURL(store.state.startUrl))
      }
    }
  })
  refresh()
  // Light polling catches processes that die on their own.
  setInterval(refresh, 4000)
}

export { Api }
