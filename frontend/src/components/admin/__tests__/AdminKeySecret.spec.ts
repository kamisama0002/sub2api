import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import AdminKeySecret from '../AdminKeySecret.vue'
const { reveal, copy, showError } = vi.hoisted(() => ({ reveal: vi.fn(), copy: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin/apiKeys', () => ({ revealApiKey: reveal }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: copy }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/components/auth/TotpStepUpDialog.vue', () => ({ default: {
  props: ['controller'], template: '<div v-if="controller.visible.value"><button @click="controller.onVerified()">verify</button><button @click="controller.onCancel()">cancel verification</button></div>'
} }))
const secret = 'sk-secret-not-present-until-verified'
const wrappers: ReturnType<typeof mount>[] = []
function render() {
  const w = mount(AdminKeySecret, { props: { apiKey: { id: 7, key: 'sk-mask********last' } }, global: { stubs: { teleport: true, RouterLink: { template: '<a><slot /></a>' } } } })
  wrappers.push(w)
  return w
}
async function click(w: ReturnType<typeof mount>, label: string) {
  await w.findAll('button').find(b => b.text() === label)!.trigger('click'); await flushPromises()
}
beforeEach(() => { vi.useFakeTimers(); vi.clearAllMocks(); reveal.mockResolvedValue({ id: 7, key: secret }) })
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.useRealTimers() })
describe('AdminKeySecret access boundary', () => {
  it('only fetches after user action and hides the returned secret after 30 seconds', async () => {
    const w = render()
    expect(reveal).not.toHaveBeenCalled(); expect(w.text()).not.toContain(secret)
    await click(w, 'adminKeys.reveal')
    expect(reveal).toHaveBeenCalledWith(7, 'view'); expect(w.text()).toContain(secret)
    await vi.advanceTimersByTimeAsync(30_000)
    expect(w.text()).not.toContain(secret)
  })
  it('uses a new audited copy request even if the key is already revealed', async () => {
    const w = render(); await click(w, 'adminKeys.reveal'); await click(w, 'adminKeys.copy')
    expect(reveal).toHaveBeenLastCalledWith(7, 'copy'); expect(copy).toHaveBeenCalledWith(secret)
  })
  it('prompts for verification and retries once after success', async () => {
    reveal.mockRejectedValueOnce({ code: 'STEP_UP_REQUIRED' })
    const w = render(); await click(w, 'adminKeys.reveal')
    expect(w.text()).not.toContain(secret); await click(w, 'verify')
    expect(reveal).toHaveBeenCalledTimes(2); expect(w.text()).toContain(secret)
  })
  it('does not fetch again or copy after cancelled verification', async () => {
    reveal.mockRejectedValueOnce({ code: 'STEP_UP_REQUIRED' })
    const w = render(); await click(w, 'adminKeys.copy'); await click(w, 'cancel verification')
    expect(reveal).toHaveBeenCalledTimes(1); expect(copy).not.toHaveBeenCalled(); expect(showError).not.toHaveBeenCalled()
  })
  it('directs unconfigured admins to setup without exposing the secret', async () => {
    reveal.mockRejectedValueOnce({ code: 'STEP_UP_TOTP_NOT_ENABLED' })
    const w = render(); await click(w, 'adminKeys.reveal')
    expect(w.text()).toContain('stepUp.notEnabled'); expect(w.text()).not.toContain(secret)
  })
  it('does not copy when audit or verification fails', async () => {
    reveal.mockRejectedValueOnce({ status: 503 })
    const w = render(); await click(w, 'adminKeys.copy')
    expect(copy).not.toHaveBeenCalled(); expect(showError).toHaveBeenCalled()
  })
  it('clears the secret when switching rows', async () => {
    const w = render(); await click(w, 'adminKeys.reveal')
    await w.setProps({ apiKey: { id: 8, key: 'another********mask' } })
    expect(w.text()).not.toContain(secret)
  })
})
