<template>
  <div class="space-y-2">
    <code class="block max-w-lg break-all text-xs text-gray-600 dark:text-gray-300">{{ secret || apiKey.key }}</code>
    <div class="flex flex-wrap items-center gap-2">
      <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="secret ? hide() : access('view')">{{ secret ? t('adminKeys.hide') : t('adminKeys.reveal') }}</button>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="access('copy')">{{ t('adminKeys.copy') }}</button>
      <span v-if="secret" class="text-xs text-gray-500">{{ t('adminKeys.autoHide') }}</span>
    </div>
    <p v-if="setupRequired" class="text-xs text-amber-700 dark:text-amber-400">
      {{ t('stepUp.notEnabled') }} <RouterLink to="/profile" class="underline">{{ t('adminKeys.setup') }}</RouterLink>
    </p>
    <Teleport to="body"><TotpStepUpDialog :controller="stepUp" /></Teleport>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ApiKey } from '@/types'
import { revealApiKey } from '@/api/admin/apiKeys'
import { useAppStore } from '@/stores'
import { useClipboard } from '@/composables/useClipboard'
import { useStepUp, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'

const props = defineProps<{ apiKey: Pick<ApiKey, 'id' | 'key'> }>()
const { t } = useI18n()
const app = useAppStore()
const { copyToClipboard } = useClipboard()
const stepUp = useStepUp()
const secret = ref('')
const busy = ref(false)
const setupRequired = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
let generation = 0
function hide() {
  secret.value = ''
  if (timer) clearTimeout(timer)
}
function clear() {
  generation++
  hide()
  stepUp.onCancel()
}
function onVisibilityChange() { if (document.hidden) clear() }
watch(() => props.apiKey.id, clear)
onMounted(() => document.addEventListener('visibilitychange', onVisibilityChange))
onBeforeUnmount(() => {
  clear()
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
async function access(purpose: 'view' | 'copy') {
  if (busy.value) return
  busy.value = true
  setupRequired.value = false
  const currentGeneration = generation
  const keyID = props.apiKey.id
  try {
    // Always call the audited endpoint, including when a visible key is copied.
    const result = await stepUp.run(() => revealApiKey(keyID, purpose))
    if (currentGeneration !== generation) return
    if (purpose === 'copy') {
      await copyToClipboard(result.key)
    } else {
      hide()
      secret.value = result.key
      timer = setTimeout(hide, 30_000)
    }
  } catch (error: unknown) {
    if (currentGeneration !== generation || isStepUpCancelled(error)) return
    const reason = stepUpBlockReason(error)
    if (reason === 'STEP_UP_TOTP_NOT_ENABLED') {
      setupRequired.value = true
    } else if (reason === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN') {
      app.showError(t('stepUp.adminApiKeyForbidden'))
    } else {
      app.showError(t('adminKeys.revealFailed'))
    }
  } finally {
    busy.value = false
  }
}
</script>
