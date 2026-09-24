<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="card p-5">
        <p class="mb-4 text-sm text-gray-600 dark:text-gray-300">{{ t('adminKeys.hint') }}</p>
        <form class="flex flex-wrap items-end gap-3" @submit.prevent="search">
          <div class="min-w-56 flex-1">
            <label for="key-search" class="input-label">{{ t('common.search') }}</label>
            <input id="key-search" v-model="query" class="input" :placeholder="t('adminKeys.searchPlaceholder')" maxlength="100" />
          </div>
          <div>
            <label for="key-status" class="input-label">{{ t('common.status') }}</label>
            <select id="key-status" v-model="status" class="input">
              <option value="">{{ t('common.all') }}</option>
              <option value="active">{{ t('common.active') }}</option>
              <option value="inactive">{{ t('common.inactive') }}</option>
              <option value="quota_exhausted">{{ t('adminKeys.quotaExhausted') }}</option>
              <option value="expired">{{ t('adminKeys.expired') }}</option>
            </select>
          </div>
          <button class="btn btn-primary" :disabled="loading">{{ t('common.search') }}</button>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="load">{{ t('common.refresh') }}</button>
          <RouterLink to="/admin/audit-logs" class="btn btn-secondary">{{ t('adminKeys.audit') }}</RouterLink>
        </form>
      </div>
      <div class="card overflow-hidden">
        <p v-if="loadError" role="alert" class="p-5 text-red-600">{{ t('adminKeys.loadFailed') }}</p>
        <p v-else-if="loading" class="p-5 text-gray-500">{{ t('common.loading') }}</p>
        <div v-else class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="bg-gray-50 text-gray-600 dark:bg-dark-700 dark:text-gray-300">
              <tr><th class="p-4">{{ t('common.name') }}</th><th class="p-4">{{ t('adminKeys.owner') }}</th><th class="p-4">{{ t('adminKeys.key') }}</th><th class="p-4">{{ t('adminKeys.group') }}</th><th class="p-4">{{ t('common.status') }}</th><th class="p-4">{{ t('adminKeys.quota') }}</th></tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="item in items" :key="item.id" class="align-top">
                <td class="p-4"><p class="font-medium">{{ item.name }}</p><p class="text-xs text-gray-500">#{{ item.id }}</p></td>
                <td class="p-4"><p>{{ item.user?.username || item.user?.email || `#${item.user_id}` }}</p><p class="text-xs text-gray-500">{{ item.user?.email }}</p></td>
                <td class="min-w-64 p-4"><AdminKeySecret :api-key="item" /></td>
                <td class="p-4">{{ item.group?.name || t('common.none') }}</td>
                <td class="p-4">{{ statusLabel(item.status) }}</td>
                <td class="whitespace-nowrap p-4">${{ Number(item.quota_used || 0).toFixed(2) }} / {{ item.quota > 0 ? `$${Number(item.quota).toFixed(2)}` : t('adminKeys.unlimited') }}</td>
              </tr>
              <tr v-if="items.length === 0"><td colspan="6" class="p-10 text-center text-gray-500">{{ t('common.noData') }}</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :page="page" :page-size="pageSize" :total="total" @update:page="changePage" @update:pageSize="changePageSize" />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { AppLayout } from '@/components/layout'
import Pagination from '@/components/common/Pagination.vue'
import AdminKeySecret from '@/components/admin/AdminKeySecret.vue'
import { listApiKeys, type AdminApiKey } from '@/api/admin/apiKeys'
const { t } = useI18n()
const query = ref(''), status = ref('')
const page = ref(1), pageSize = ref(20), total = ref(0)
const items = ref<AdminApiKey[]>([])
const loading = ref(false), loadError = ref(false)
let requestID = 0
let filters = { search: '', status: '' }
async function load() {
  const id = ++requestID
  loading.value = true
  loadError.value = false
  items.value = []
  try {
    const result = await listApiKeys({ page: page.value, page_size: pageSize.value, ...filters })
    if (id !== requestID) return
    items.value = result.items
    total.value = result.total
  } catch {
    if (id === requestID) { loadError.value = true; total.value = 0 }
  } finally { if (id === requestID) loading.value = false }
}
function search() { filters = { search: query.value.trim(), status: status.value }; page.value = 1; void load() }
function changePage(value: number) { page.value = value; void load() }
function changePageSize(value: number) { pageSize.value = value; page.value = 1; void load() }
function statusLabel(value: string) {
  const labels: Record<string, string> = { active: 'common.active', inactive: 'common.inactive', disabled: 'common.inactive', expired: 'adminKeys.expired', quota_exhausted: 'adminKeys.quotaExhausted' }
  return labels[value] ? t(labels[value]) : value
}
onMounted(load)
onBeforeUnmount(() => { requestID++ })
</script>
