<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Clipboard, Events, Updater } from '@wailsio/runtime'
import {
  TranslationService,
  UpdateService,
  type Language,
} from '../bindings/github.com/dingxyang/wails-v3-demo'

const languages = ref<Language[]>([])
const sourceLang = ref('auto')
const targetLang = ref('zh-CN')
const provider = ref('auto')
const sourceText = ref('')
const result = ref('')
const detected = ref('')
const resultProvider = ref('')
const loading = ref(false)
const version = ref('')
const update = ref<{ version: string; name?: string; notes?: string } | null>(null)
const toast = ref('')

let toastTimer: ReturnType<typeof setTimeout> | undefined

const providerNames: Record<string, string> = {
  auto: '自动选择',
  microsoft: '微软翻译',
  google: '谷歌翻译',
}

const detectedLabel = computed(() => languageName(detected.value))
const providerLabel = computed(() => providerNames[resultProvider.value] ?? resultProvider.value)

function languageName(code: string) {
  if (!code) {
    return ''
  }
  const aliases: Record<string, string> = {
    auto: '自动检测',
    'zh-Hans': '简体中文',
    'zh-Hant': '繁体中文',
  }
  return aliases[code] ?? languages.value.find((item) => item.code === code)?.name ?? code
}

function showToast(message: string) {
  toast.value = message
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (toast.value = ''), 3200)
}

function errorText(error: unknown) {
  if (error instanceof Error) {
    return error.message
  }
  if (typeof error === 'string') {
    return error
  }
  return JSON.stringify(error)
}

async function translate() {
  if (!sourceText.value.trim() || loading.value) {
    return
  }
  loading.value = true
  try {
    const translated = await TranslationService.Translate(
      sourceText.value,
      sourceLang.value,
      targetLang.value,
      provider.value,
    )
    result.value = translated.text
    detected.value = translated.detectedSource
    resultProvider.value = translated.provider
  } catch (error) {
    showToast(errorText(error))
  } finally {
    loading.value = false
  }
}

function swapLanguages() {
  if (sourceLang.value === 'auto') {
    return
  }
  const previousSource = sourceLang.value
  sourceLang.value = targetLang.value
  targetLang.value = previousSource
}

async function copyResult() {
  if (!result.value) {
    return
  }
  await Clipboard.SetText(result.value)
  showToast('译文已复制')
}

async function checkForUpdates() {
  try {
    await UpdateService.CheckForUpdates()
    update.value = null
  } catch (error) {
    showToast(errorText(error))
  }
}

onMounted(async () => {
  Events.On(Updater.Events.UpdateAvailable, (event) => {
    const release = event.data as { version?: string; name?: string; notes?: string } | null
    if (release?.version) {
      update.value = { ...release, version: release.version }
    }
  })

  try {
    const [availableLanguages, runningVersion] = await Promise.all([
      TranslationService.Languages(),
      UpdateService.Version(),
    ])
    languages.value = availableLanguages ?? []
    version.value = runningVersion
  } catch (error) {
    showToast(errorText(error))
  }
})
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div class="brand">
        <span class="brand-mark">译</span>
        <div class="brand-text">
          <h1>简译</h1>
          <p>免费翻译 · 无需 API Key</p>
        </div>
      </div>
      <button class="ghost-button" type="button" @click="checkForUpdates">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-2.64-6.36"/><polyline points="21 3 21 9 15 9"/></svg>
        <span>检查更新</span>
      </button>
    </header>

    <div v-if="update" class="update-banner" role="status">
      <span>发现新版本 <strong>v{{ update.version }}</strong></span>
      <button class="link-button" type="button" @click="checkForUpdates">立即更新</button>
      <button class="dismiss-button" type="button" aria-label="关闭更新提示" @click="update = null">×</button>
    </div>

    <main class="workspace">
      <section class="pane">
        <div class="pane-head">
          <label class="lang-select">
            <span class="sr-only">源语言</span>
            <select v-model="sourceLang">
              <option value="auto">自动检测</option>
              <option v-for="language in languages" :key="language.code" :value="language.code">{{ language.name }}</option>
            </select>
          </label>
          <span class="char-count">{{ sourceText.length.toLocaleString() }} / 50,000</span>
        </div>
        <textarea
          v-model="sourceText"
          class="text-input"
          maxlength="50000"
          placeholder="输入或粘贴要翻译的文本…"
          spellcheck="false"
          @keydown.meta.enter.prevent="translate"
          @keydown.ctrl.enter.prevent="translate"
        ></textarea>
      </section>

      <div class="pane-divider">
        <button class="swap-button" type="button" :disabled="sourceLang === 'auto'" title="交换语言" aria-label="交换语言" @click="swapLanguages">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg>
        </button>
      </div>

      <section class="pane pane-result">
        <div class="pane-head">
          <label class="lang-select">
            <span class="sr-only">目标语言</span>
            <select v-model="targetLang">
              <option v-for="language in languages" :key="language.code" :value="language.code">{{ language.name }}</option>
            </select>
          </label>
          <button class="text-button" type="button" :disabled="!result" @click="copyResult">复制</button>
        </div>
        <div v-if="loading" class="result-placeholder">翻译中…</div>
        <div v-else-if="result" class="result-text">{{ result }}</div>
        <div v-else class="result-placeholder">译文将显示在这里</div>
        <div v-if="result && !loading" class="result-meta">
          检测到 {{ detectedLabel || '未知语言' }} · 由{{ providerLabel }}提供
        </div>
      </section>
    </main>

    <div class="actions">
      <label class="provider-select">
        <span>翻译服务</span>
        <select v-model="provider">
          <option value="auto">自动（微软 → 谷歌）</option>
          <option value="microsoft">微软翻译（免费）</option>
          <option value="google">谷歌翻译（免费）</option>
        </select>
      </label>
      <button class="primary-button" type="button" :disabled="loading || !sourceText.trim()" @click="translate">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/></svg>
        <span>{{ loading ? '翻译中…' : '翻译' }}</span>
      </button>
    </div>

    <footer class="footer">
      <span class="footer-version">v{{ version || '—' }}</span>
      <span class="footer-hint">⌘ / Ctrl + Enter 快速翻译</span>
      <span class="footer-status"><span class="dot"></span>免费服务，无需密钥</span>
    </footer>

    <div class="toast" :class="{ 'is-visible': toast }" role="status">{{ toast }}</div>
  </div>
</template>
