import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

const apiMocks = vi.hoisted(() => ({
  getS3Config: vi.fn(),
  updateS3Config: vi.fn(),
  testS3Connection: vi.fn(),
  getImageStorageConfig: vi.fn(),
  updateImageStorageConfig: vi.fn(),
  testImageStorageConnection: vi.fn(),
  getSchedule: vi.fn(),
  updateSchedule: vi.fn(),
  listBackups: vi.fn(),
  createBackup: vi.fn(),
  getBackup: vi.fn(),
  getDownloadURL: vi.fn(),
  restoreBackup: vi.fn(),
  deleteBackup: vi.fn(),
}))
const { getSchedule, updateSchedule, deleteBackup, listBackups } = apiMocks

const storeMocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
}))

vi.mock('@/api', () => ({
  adminAPI: { backup: apiMocks },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => storeMocks,
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ run: (fn: () => unknown) => fn() }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => '',
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params?.index === undefined ? key : `${key}:${params.index}`,
    }),
  }
})

vi.mock('@/components/common/ConfirmDialog.vue', () => ({
  default: defineComponent({
    name: 'ConfirmDialog',
    props: {
      show: Boolean,
      title: String,
      message: String,
      confirmText: String,
      cancelText: String,
    },
    emits: ['confirm', 'cancel'],
    setup(props, { emit }) {
      return () => props.show
        ? h('div', { class: 'confirm-dialog-stub' }, [
            h('h3', props.title),
            h('p', props.message),
            h('button', { type: 'button', onClick: () => emit('cancel') }, props.cancelText),
            h('button', { type: 'button', onClick: () => emit('confirm') }, props.confirmText),
          ])
        : null
    },
  }),
}))

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: defineComponent({
    name: 'BaseDialog',
    props: { show: Boolean, title: String, width: String },
    emits: ['close'],
    setup(props, { slots, emit }) {
      return () => props.show
        ? h('section', { class: 'base-dialog-stub', 'data-title': props.title }, [
            h('h3', props.title),
            slots.default?.(),
            slots.footer?.({ close: () => emit('close') }),
          ])
        : null
    },
  }),
}))

vi.mock('@/components/common/Input.vue', () => ({
  default: defineComponent({
    name: 'InputStub',
    props: {
      modelValue: { type: [String, Number], default: '' },
      type: String,
      label: String,
      placeholder: String,
      autocomplete: String,
    },
    emits: ['update:modelValue', 'enter'],
    setup(props, { emit }) {
      return () => h('label', [
        props.label ? h('span', props.label) : null,
        h('input', {
          type: props.type || 'text',
          value: props.modelValue ?? '',
          placeholder: props.placeholder,
          autocomplete: props.autocomplete,
          onInput: (event: Event) =>
            emit('update:modelValue', (event.target as HTMLInputElement).value),
        }),
      ])
    },
  }),
}))

import BackupView from '../BackupView.vue'

const baseRecord = (id: string, parts?: unknown[]) => ({
  id,
  status: 'completed',
  backup_type: 'postgres',
  file_name: `${id}.sql.gz`,
  s3_key: `backups/${id}.sql.gz`,
  parts,
  size_bytes: 2048,
  triggered_by: 'manual',
  started_at: '2026-08-09T00:00:00Z',
})

async function mountLoadedView() {
  const wrapper = mount(BackupView, {
    global: { stubs: { TotpStepUpDialog: true } },
  })
  await flushPromises()
  return wrapper
}

const wrappers: ReturnType<typeof mount>[] = []
function mountBackupView() {
  const wrapper = mount(BackupView, { global: { stubs: { TotpStepUpDialog: true } } })
  wrappers.push(wrapper)
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  apiMocks.getS3Config.mockResolvedValue({
    endpoint: '',
    region: 'auto',
    bucket: '',
    access_key_id: '',
    prefix: 'backups/',
    force_path_style: false,
  })
  apiMocks.getImageStorageConfig.mockResolvedValue({ config: {}, secret_configured: false })
  apiMocks.getSchedule.mockResolvedValue({
    enabled: false,
    cron_expr: '0 2 * * *',
    retain_days: 14,
    retain_count: 10,
  })
  apiMocks.listBackups.mockResolvedValue({ items: [baseRecord('backup-1')] })
  apiMocks.restoreBackup.mockResolvedValue({
    ...baseRecord('backup-1'),
    restore_status: 'running',
  })
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
})

afterEach(() => {
  wrappers.splice(0).forEach(wrapper => wrapper.unmount())
  vi.restoreAllMocks()
  document.body.innerHTML = ''
})

describe('BackupView', () => {
  it('恢复备份使用统一密码输入弹框而不是 window.prompt', async () => {
    const promptSpy = vi.spyOn(window, 'prompt').mockReturnValue('native-password')
    const wrapper = await mountLoadedView()

    const restoreButton = wrapper.findAll('button').find(button =>
      button.text() === 'admin.backup.actions.restore',
    )
    await restoreButton!.trigger('click')

    const confirmButton = wrapper.findAll('button').find(button =>
      button.text() === 'common.confirm',
    )
    await confirmButton!.trigger('click')
    await flushPromises()

    expect(promptSpy).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.backup.actions.restorePasswordPrompt')

    await wrapper.find('input[autocomplete="current-password"]').setValue('restore-password')
    const submitButton = wrapper.findAll('button').find(button =>
      button.text() === 'common.confirm',
    )
    await submitButton!.trigger('click')
    await flushPromises()

    expect(apiMocks.restoreBackup).toHaveBeenCalledWith('backup-1', 'restore-password')
  })

  it('显示分卷数并在下载时列出每个分卷链接', async () => {
    apiMocks.listBackups.mockResolvedValue({
      items: [baseRecord('split', [{ index: 1 }, { index: 2 }, { index: 3 }])],
    })
    apiMocks.getDownloadURL.mockResolvedValue({
      parts: [
        { index: 1, size_bytes: 5, url: 'https://example.test/part-1' },
        { index: 2, size_bytes: 6, url: 'https://example.test/part-2' },
        { index: 3, size_bytes: 7, url: 'https://example.test/part-3' },
      ],
    })

    const wrapper = await mountLoadedView()
    expect(wrapper.text()).toContain('3')
    const downloadButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.backup.actions.download'),
    )
    await downloadButton!.trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('admin.backup.actions.partLabel:1')
    expect(document.body.textContent).toContain('admin.backup.actions.partLabel:3')
    expect(document.body.querySelector('a[href="https://example.test/part-2"]')).not.toBeNull()
  })

  it('旧单文件记录仍使用单个下载地址', async () => {
    apiMocks.getDownloadURL.mockResolvedValue({ url: 'https://example.test/legacy.sql.gz' })
    const wrapper = await mountLoadedView()
    const downloadButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.backup.actions.download'),
    )
    await downloadButton!.trigger('click')
    await flushPromises()

    expect(apiMocks.getDownloadURL).toHaveBeenCalledWith('backup-1')
    expect(document.body.textContent).not.toContain('admin.backup.actions.downloadParts')
  })

  it('运行中的备份不显示删除入口', async () => {
    apiMocks.listBackups.mockResolvedValue({
      items: [{ ...baseRecord('running'), status: 'running', progress: 'uploading' }],
    })
    const wrapper = await mountLoadedView()

    expect(wrapper.find('tbody tr td:nth-child(5)').text()).toBe('-')
    expect(wrapper.findAll('button').some(button => button.text() === 'common.delete')).toBe(false)
  })

  it('兼容旧配置并保留 0 值，不自动启用月度归档', async () => {
    getSchedule.mockResolvedValue({ enabled: true, cron_expr: '0 4 * * *', retain_days: 0, retain_count: 0 })
    const wrapper = mountBackupView()
    await flushPromises()
    expect((wrapper.get('[data-testid="backup-retain-days"]').element as HTMLInputElement).value).toBe('0')
    expect((wrapper.get('[data-testid="backup-retain-count"]').element as HTMLInputElement).value).toBe('0')
    expect((wrapper.get('[data-testid="archive-enabled"]').element as HTMLInputElement).checked).toBe(false)
    await wrapper.get('[data-testid="backup-schedule"] .btn-primary').trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenCalledWith(expect.objectContaining({ retain_days: 0, retain_count: 0, monthly_archive: expect.objectContaining({ enabled: false }) }))
  })

  it('多选日期，手填份数；永久保留隐藏数量并在取消后恢复', async () => {
    const wrapper = mountBackupView()
    await flushPromises()
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    await wrapper.get('#backup-archive-dates').trigger('click')
    await wrapper.get('#backup-archive-options input[value="15"]').setValue(true)
    await wrapper.get('#backup-archive-options input[value="last"]').setValue(true)
    expect(wrapper.get('#backup-archive-dates').attributes('aria-expanded')).toBe('true')
    expect(wrapper.find('[data-testid="archive-count"]').exists()).toBe(false)
    await wrapper.get('[data-testid="archive-forever"]').setValue(false)
    await wrapper.get('[data-testid="archive-count"]').setValue(17)
    await wrapper.get('[data-testid="archive-forever"]').setValue(true)
    expect(wrapper.find('[data-testid="archive-count"]').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'BackupArchiveSettings' }).text()).not.toContain('admin.backup.archive.copies')
    await wrapper.get('[data-testid="archive-forever"]').setValue(false)
    expect((wrapper.get('[data-testid="archive-count"]').element as HTMLInputElement).value).toBe('17')
    await wrapper.get('[data-testid="backup-schedule"] .btn-primary').trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: true, days: [1, 15], include_month_end: true, retain_count: 17 } }))
    await wrapper.get('[data-testid="archive-forever"]').setValue(true)
    await wrapper.get('[data-testid="backup-schedule"] .btn-primary').trigger('click')
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: expect.objectContaining({ retain_count: 0 }) }))
  })

  it('空日期和非法份数不可保存，输入 0 不会误选永久保留', async () => {
    const wrapper = mountBackupView()
    await flushPromises()
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    await wrapper.get('#backup-archive-dates').trigger('click')
    await wrapper.get('#backup-archive-options input[value="1"]').setValue(false)
    const save = wrapper.get('[data-testid="backup-schedule"] .btn-primary')
    expect(save.attributes('disabled')).toBeDefined()
    await wrapper.get('#backup-archive-options input[value="15"]').setValue(true)
    await wrapper.get('[data-testid="archive-forever"]').setValue(false)
    for (const value of ['0', '-1', '1.5', '']) {
      await wrapper.get('[data-testid="archive-count"]').setValue(value)
      expect(save.attributes('disabled')).toBeDefined()
      expect((wrapper.get('[data-testid="archive-forever"]').element as HTMLInputElement).checked).toBe(false)
    }
    await save.trigger('click')
    expect(updateSchedule).not.toHaveBeenCalled()
  })

  it('关闭月度归档后不提交隐藏的归档参数，重新启用时保留编辑值', async () => {
    getSchedule.mockResolvedValue({ enabled: true, cron_expr: '0 4 * * *', retain_days: 14, retain_count: 10,
      monthly_archive: { enabled: true, days: [1, 15], include_month_end: false, retain_count: 12 } })
    const wrapper = mountBackupView()
    await flushPromises()
    await wrapper.get('[data-testid="archive-count"]').setValue(1)
    await wrapper.get('[data-testid="archive-enabled"]').setValue(false)
    expect(wrapper.find('[data-testid="archive-count"]').exists()).toBe(false)
    const save = wrapper.get('[data-testid="backup-schedule"] .btn-primary')
    await save.trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: false, days: [1, 15], include_month_end: false, retain_count: 12 } }))
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    expect((wrapper.get('[data-testid="archive-count"]').element as HTMLInputElement).value).toBe('1')
    // A hidden invalid count neither blocks saving nor reaches the request.
    await wrapper.get('[data-testid="archive-count"]').setValue('')
    expect(save.attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="archive-enabled"]').setValue(false)
    expect(save.attributes('disabled')).toBeUndefined()
    await save.trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: false, days: [1, 15], include_month_end: false, retain_count: 12 } }))
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    await wrapper.get('[data-testid="archive-count"]').setValue(1)
    await save.trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: true, days: [1, 15], include_month_end: false, retain_count: 1 } }))
  })

  it('加载归档设置并支持键盘关闭日期选择器', async () => {
    getSchedule.mockResolvedValue({ enabled: true, cron_expr: '0 4 * * *', retain_days: 14, retain_count: 10,
      monthly_archive: { enabled: true, days: [1, 15], include_month_end: true, retain_count: 23 } })
    const wrapper = mountBackupView()
    await flushPromises()
    expect((wrapper.get('[data-testid="archive-count"]').element as HTMLInputElement).value).toBe('23')
    await wrapper.get('#backup-archive-dates').trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()
    expect(wrapper.find('#backup-archive-options').exists()).toBe(true)
    expect((wrapper.get('#backup-archive-options input[value="15"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('#backup-archive-options').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('#backup-archive-options').exists()).toBe(false)
  })

  it('归档删除需要单独确认，有限归档不会显示永不过期', async () => {
    listBackups.mockResolvedValue({ items: [{ ...baseRecord('archived'), monthly_archive: { dates: ['2026-09-01', '2026-09-15'], retain_count: 12 } }] })
    const confirm = vi.spyOn(window, 'confirm')
    const wrapper = mountBackupView()
    await flushPromises()
    expect(wrapper.text()).toContain('admin.backup.archive.badge')
    expect(wrapper.get('tbody tr td:nth-child(6)').text()).toBe('admin.backup.archive.retainLatest')
    const button = wrapper.findAll('button').find(button => button.text() === 'common.delete')!
    await button.trigger('click')
    expect(confirm).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.backup.actions.deleteConfirm')
    expect(deleteBackup).not.toHaveBeenCalled()
    const confirmButton = wrapper.find('.confirm-dialog-stub').findAll('button').find(button => button.text() === 'common.delete')!
    await confirmButton.trigger('click')
    await flushPromises()
    expect(deleteBackup).toHaveBeenCalledWith('archived', true)
  })
})
