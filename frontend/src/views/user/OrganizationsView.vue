<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">团队空间</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">企业统一额度，成员独立使用和统计</p>
        </div>
        <div class="flex gap-2">
          <button class="btn btn-secondary" @click="showAccept = true">加入团队</button>
          <button v-if="canCreateTeam" class="btn btn-primary" @click="showCreate = true">创建团队</button>
        </div>
      </div>

      <!-- 企业信息栏：企业账号显示自己的公司与认证状态；信息未补全或被驳回时提示去个人资料处理 -->
      <section v-if="enterprise" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="truncate text-base font-semibold text-gray-900 dark:text-white">{{ enterprise.company_name || t('enterprise.profile.cardTitle') }}</span>
              <EnterpriseStatusBadge :status="enterprise.verify_status" />
            </div>
            <p v-if="enterprise.contact_name" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('enterprise.fields.contactName') }}：{{ enterprise.contact_name }}
              <template v-if="enterprise.contact_phone"> · {{ enterprise.contact_phone }}</template>
            </p>
          </div>
          <router-link to="/profile" class="link-button shrink-0">{{ t('enterprise.profile.cardTitle') }}</router-link>
        </div>
        <p v-if="enterprise.verify_status === 'incomplete'" class="mt-3 rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
          {{ t('enterprise.team.incompleteBanner') }}
          <router-link to="/profile" class="ml-1 font-medium underline">{{ t('enterprise.team.goComplete') }}</router-link>
        </p>
        <p v-else-if="enterprise.verify_status === 'rejected'" class="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">
          {{ t('enterprise.profile.rejectedHint') }}
          <span v-if="enterprise.reject_reason">（{{ enterprise.reject_reason }}）</span>
          <router-link to="/profile" class="ml-1 font-medium underline">{{ t('enterprise.team.goComplete') }}</router-link>
        </p>
      </section>

      <div v-if="loading" class="py-20 text-center text-gray-500">加载中...</div>
      <div v-else-if="organizations.length === 0" class="rounded-lg border border-dashed border-gray-300 py-20 text-center dark:border-dark-600">
        <p class="text-gray-600 dark:text-gray-300">{{ canCreateTeam ? '你还没有加入团队空间' : t('enterprise.team.emptyMember') }}</p>
        <button v-if="canCreateTeam" class="btn btn-primary mt-4" @click="showCreate = true">创建第一个团队</button>
        <button v-else class="btn btn-primary mt-4" @click="showAccept = true">加入团队</button>
      </div>
      <template v-else>
        <div class="team-switcher">
          <div class="team-switcher-heading">
            <span class="team-switcher-icon"><Icon name="users" size="md" /></span>
            <div>
              <label class="block text-sm font-semibold text-gray-900 dark:text-white">当前团队</label>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">切换后查看对应团队的数据与成员</p>
            </div>
          </div>
          <div class="flex min-w-0 flex-1 items-center gap-2 sm:max-w-md">
            <Select v-model="selectedId" :options="organizationOptions" :searchable="false" class="min-w-0 flex-1" @change="loadWorkspace">
              <template #selected="{ option }">
                <div v-if="option" class="flex min-w-0 items-center gap-2 text-left">
                  <span class="truncate font-medium text-gray-900 dark:text-white">{{ option.label }}</span>
                  <span class="team-role-chip">{{ roleLabel(String(option.role)) }}</span>
                </div>
              </template>
              <template #option="{ option, selected }">
                <div class="flex w-full min-w-0 items-center gap-3">
                  <span class="team-option-icon"><Icon name="users" size="sm" /></span>
                  <div class="min-w-0 flex-1">
                    <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ option.label }}</p>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ roleLabel(String(option.role)) }} · {{ option.seatUsed }}/{{ option.seatLimit }} 席位</p>
                  </div>
                  <Icon v-if="selected" name="check" size="sm" class="shrink-0 text-primary-500" />
                </div>
              </template>
            </Select>
            <button v-if="current?.role === 'owner'" type="button" class="delete-team-button" title="删除团队" aria-label="删除当前团队" @click="showDelete = true">
              <Icon name="trash" size="md" />
            </button>
            <button v-else-if="current" type="button" class="btn btn-secondary shrink-0" @click="showLeave = true">退出团队</button>
          </div>
        </div>

        <section v-if="current" class="grid gap-4 md:grid-cols-4">
          <div class="stat-block"><span>可用额度</span><strong>{{ money(current.display_balance) }}</strong></div>
          <div class="stat-block"><span>冻结额度</span><strong>{{ money(current.frozen_display_balance) }}</strong></div>
          <div class="stat-block"><span>席位</span><strong>{{ current.seat_used }} / {{ current.seat_limit }}</strong></div>
          <div class="stat-block"><span>近 30 天消耗</span><strong>{{ money(usage?.summary.total_cost || 0) }}</strong></div>
        </section>

        <div class="flex gap-1 overflow-x-auto border-b border-gray-200 dark:border-dark-700">
          <button v-for="item in tabs" :key="item.key" class="tab-button" :class="{ active: tab === item.key }" @click="tab = item.key">{{ item.label }}</button>
        </div>

        <section v-if="tab === 'overview'" class="space-y-5">
          <div v-if="canManage" class="space-y-2">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">团队额度</h2>
            <div class="flex flex-wrap gap-3">
              <input v-model.number="fundAmount" min="0.01" step="0.01" type="number" class="input w-44" placeholder="转入额度" />
              <button class="btn btn-primary" :disabled="busy" @click="fund">从个人余额转入</button>
            </div>
            <!-- 转出：仅超级管理员，把团队可用额度转回自己的个人余额（团队不再使用时取回，或只取回一部分） -->
            <template v-if="current?.role === 'owner'">
              <div class="flex flex-wrap gap-3 pt-1">
                <input v-model.number="withdrawAmount" min="0.01" step="0.01" type="number" class="input w-44" placeholder="转出额度" />
                <button class="btn btn-secondary" :disabled="busy || !withdrawAmount" @click="withdraw">转出到个人余额</button>
                <button class="link-button" :disabled="busy || !(current?.display_balance > 0)" @click="withdrawAll">全部转出（{{ money(current?.display_balance || 0) }}）</button>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">仅超级管理员可转出，额度转入你的个人余额；冻结中的额度不可转出。</p>
            </template>
          </div>
          <div class="overflow-x-auto"><table class="data-table"><thead><tr><th>成员</th><th>请求</th><th>输入 Token</th><th>输出 Token</th><th>费用</th></tr></thead><tbody><tr v-for="row in usage?.members || []" :key="row.user_id"><td>{{ row.username || row.email }}</td><td>{{ row.total_requests }}</td><td>{{ row.input_tokens }}</td><td>{{ row.output_tokens }}</td><td>{{ money(row.total_cost) }}</td></tr></tbody></table></div>
        </section>

        <section v-else-if="tab === 'members'" class="overflow-x-auto">
          <table class="data-table"><thead><tr><th>成员</th><th>角色</th><th>状态</th><th>本月用量 / 限额</th><th v-if="canManage">操作</th></tr></thead><tbody>
            <tr v-for="member in members" :key="member.user_id"><td><div class="font-medium">{{ member.username }}</div><div class="text-xs text-gray-500">{{ member.email }}</div></td><td><div v-if="canManage && member.role !== 'owner'" class="role-select-wrap" :class="{ 'opacity-60': busy }"><Icon :name="member.role === 'admin' ? 'shield' : 'user'" size="sm" class="role-icon" /><select :value="member.role" class="role-select" aria-label="成员角色" :disabled="busy" @change="changeRole(member, $event)"><option value="member">成员</option><option v-if="current?.role === 'owner'" value="admin">管理员</option></select><Icon name="chevronDown" size="xs" class="role-chevron" /></div><span v-else class="role-badge" :class="`role-${member.role}`"><Icon :name="member.role === 'owner' || member.role === 'admin' ? 'shield' : 'user'" size="xs" />{{ roleLabel(member.role) }}</span></td><td><span class="status-dot"><span></span>正常</span></td><td>{{ money(member.monthly_used) }} / {{ member.monthly_limit > 0 ? money(member.monthly_limit) : '不限' }}</td><td v-if="canManage"><div v-if="member.role !== 'owner'" class="flex gap-3"><button class="link-button" @click="setLimit(member)">设置限额</button><button class="link-button text-red-600 dark:text-red-400" @click="removeMember(member)">移除</button></div></td></tr>
          </tbody></table>
        </section>

        <section v-else-if="tab === 'invitations'" class="space-y-4">
          <div v-if="canManage" class="flex flex-wrap gap-3"><input v-model="inviteEmail" type="email" class="input w-64" placeholder="成员邮箱（链接邀请可留空）" /><select v-model="inviteRole" class="input w-32"><option value="member">成员</option><option v-if="current?.role === 'owner'" value="admin">管理员</option></select><button class="btn btn-primary" :disabled="busy" @click="createInvitation">创建邀请</button></div>
          <div v-if="inviteLink" class="rounded-md bg-blue-50 p-4 dark:bg-blue-950/30"><p class="text-sm text-blue-700 dark:text-blue-300">邀请链接只展示一次，被邀请人打开链接即可加入团队</p><code class="mt-2 block break-all">{{ inviteLink }}</code><button class="link-button mt-2" @click="copyInvite">复制</button></div>
          <div class="overflow-x-auto"><table class="data-table"><thead><tr><th>邮箱</th><th>角色</th><th>使用次数</th><th>到期时间</th></tr></thead><tbody><tr v-for="item in invitations" :key="item.id"><td>{{ item.email || '通用链接' }}</td><td>{{ roleLabel(item.role) }}</td><td>{{ item.used_count }} / {{ item.max_uses }}</td><td>{{ dateTime(item.expires_at) }}</td></tr></tbody></table></div>
        </section>

        <!-- 资金记录：额度的转入 / 转出明细，仅超级管理员和管理员可见 -->
        <section v-else-if="tab === 'funds'" class="space-y-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">团队额度的转入与转出明细（逐请求的消耗见「模型用量」）。</p>
          <div class="overflow-x-auto">
            <table class="data-table">
              <thead><tr><th>时间</th><th>类型</th><th>金额</th><th>操作人</th><th>说明</th></tr></thead>
              <tbody>
                <tr v-if="fundLoading"><td colspan="5" class="py-10 text-center text-gray-500">加载中...</td></tr>
                <tr v-else-if="fundRecords.length === 0"><td colspan="5" class="py-10 text-center text-gray-500">暂无转入转出记录</td></tr>
                <tr v-for="row in fundRecords" :key="row.id">
                  <td class="whitespace-nowrap">{{ dateTime(row.created_at) }}</td>
                  <td><span class="role-badge" :class="row.type === 'fund' ? 'role-owner' : 'role-member'">{{ row.type === 'fund' ? '转入' : '转出' }}</span></td>
                  <td class="font-mono" :class="row.type === 'fund' ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'">{{ row.type === 'fund' ? '+' : '-' }}{{ money(row.display_amount) }}</td>
                  <td>{{ row.actor_username || row.actor_email || '-' }}</td>
                  <td class="text-gray-500 dark:text-gray-400">{{ row.type === 'fund' ? '来自操作人的个人余额' : '转入超级管理员的个人余额' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination v-if="fundTotal > 0" :page="fundPage" :page-size="fundPageSize" :total="fundTotal" @update:page="loadFundRecords" @update:pageSize="changeFundPageSize" />
        </section>

        <section v-else-if="tab === 'usage'" class="overflow-x-auto"><table class="data-table"><thead><tr><th>模型</th><th>请求</th><th>Token</th><th>费用</th></tr></thead><tbody><tr v-for="row in usage?.models || []" :key="row.model"><td>{{ row.model }}</td><td>{{ row.total_requests }}</td><td>{{ row.total_tokens }}</td><td>{{ money(row.total_cost) }}</td></tr></tbody></table></section>
        <section v-else class="space-y-2"><div v-for="item in auditLogs" :key="item.id" class="flex flex-col gap-1 border-b border-gray-100 py-3 text-sm dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between"><div><span class="font-medium">{{ item.action }}</span><span class="ml-2 text-gray-500">{{ item.target_type }} #{{ item.target_id }}</span></div><time class="text-xs text-gray-500">{{ dateTime(item.created_at) }}</time></div></section>
      </template>
    </div>

    <BaseDialog :show="showCreate" title="创建团队空间" width="narrow" @close="closeDialogs">
      <form id="create-team-form" class="space-y-5" @submit.prevent="createOrganization">
        <div>
          <label class="input-label" for="team-name">团队名称</label>
          <input id="team-name" v-model.trim="createName" class="input w-full" maxlength="120" placeholder="例如：产品研发团队" />
          <p class="input-hint">用于成员识别和切换团队空间</p>
        </div>
        <div>
          <label class="input-label" for="team-seats">团队席位</label>
          <div class="relative">
            <input id="team-seats" v-model.number="createSeats" type="number" min="1" max="10000" class="input w-full pr-12" />
            <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-gray-500">人</span>
          </div>
          <p class="input-hint">包含团队创建者，后续可邀请成员加入</p>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeDialogs">取消</button>
          <button form="create-team-form" type="submit" class="btn btn-primary" :disabled="busy || !createName.trim() || createSeats < 1">{{ busy ? '创建中...' : '创建团队' }}</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="showAccept" title="加入团队" width="narrow" @close="closeDialogs">
      <form id="join-team-form" class="space-y-2" @submit.prevent="acceptInvitation">
        <label class="input-label" for="invitation-token">邀请令牌</label>
        <input id="invitation-token" v-model.trim="acceptToken" class="input w-full" placeholder="粘贴团队邀请令牌" />
        <p class="input-hint">可粘贴邀请链接或邀请令牌；加入后，可在创建 API 密钥时选择使用团队共享额度</p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeDialogs">取消</button>
          <button form="join-team-form" type="submit" class="btn btn-primary" :disabled="busy || !acceptToken.trim()">{{ busy ? '加入中...' : '加入团队' }}</button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog :show="showLeave" title="退出团队" :message="`确定退出“${current?.name || ''}”吗？`" confirm-text="退出团队" cancel-text="取消" danger @confirm="leaveOrganization" @cancel="showLeave = false">
      <div class="rounded-md border border-amber-100 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300">
        <p class="font-medium">退出后将立即执行：</p>
        <ul class="mt-2 list-disc space-y-1 pl-5">
          <li>停用你在该团队下创建的、使用团队额度的 API 密钥</li>
          <li>不再能使用该团队的共享额度，也看不到该团队数据</li>
        </ul>
        <p class="mt-2 text-xs opacity-80">如需重新加入，需要团队管理员再次发送邀请。</p>
      </div>
    </ConfirmDialog>

    <ConfirmDialog :show="showDelete" title="删除团队空间" :message="`确定删除“${current?.name || ''}”吗？`" confirm-text="删除团队" cancel-text="取消" danger @confirm="deleteOrganization" @cancel="showDelete = false">
      <div class="rounded-md border border-red-100 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300">
        <p class="font-medium">删除后将立即执行：</p>
        <ul class="mt-2 list-disc space-y-1 pl-5">
          <li>停用所有成员的团队 API 密钥</li>
          <li>撤销尚未使用的邀请</li>
          <li>将剩余 {{ money(current?.display_balance || 0) }} 额度退回你的个人账户</li>
        </ul>
        <p class="mt-2 text-xs opacity-80">团队有未结算请求时，需要等待结算完成后再删除。</p>
      </div>
    </ConfirmDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import { Icon } from '@/components/icons'
import EnterpriseStatusBadge from '@/components/enterprise/EnterpriseStatusBadge.vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useWorkspaceAccessStore } from '@/stores/workspaceAccess'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractInviteToken } from '@/utils/organizationInvite'
import { organizationsAPI, type Organization, type OrganizationAuditLog, type OrganizationFundRecord, type OrganizationInvitation, type OrganizationMember, type OrganizationUsageReport } from '@/api/organizations'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const workspaceAccess = useWorkspaceAccessStore()
const authStore = useAuthStore()
const appStore = useAppStore()
// 只有企业账号可以创建团队；已加入团队的个人账号只能加入/使用团队（后端同样强制校验）
const canCreateTeam = computed(() => workspaceAccess.canCreateTeam)
const enterprise = computed(() => workspaceAccess.enterprise)
const organizations=ref<Organization[]>([]),selectedId=ref(0),members=ref<OrganizationMember[]>([]),invitations=ref<OrganizationInvitation[]>([]),usage=ref<OrganizationUsageReport|null>(null),auditLogs=ref<OrganizationAuditLog[]>([])
const loading=ref(true),busy=ref(false),tab=ref('overview'),showCreate=ref(false),showAccept=ref(false),showDelete=ref(false),showLeave=ref(false),createName=ref(''),createSeats=ref(5),acceptToken=ref(''),fundAmount=ref<number>(),withdrawAmount=ref<number>(),inviteEmail=ref(''),inviteRole=ref('member'),inviteLink=ref('')
// 「资金记录」「审计日志」只对超级管理员和管理员有意义；普通成员不展示该标签
const tabs=computed(()=>[{key:'overview',label:'概览'},{key:'members',label:'成员'},{key:'invitations',label:'邀请'},{key:'usage',label:'模型用量'},...(canManage.value?[{key:'funds',label:'资金记录'}]:[]),{key:'audit',label:'审计日志'}])
const fundRecords=ref<OrganizationFundRecord[]>([]),fundTotal=ref(0),fundPage=ref(1),fundPageSize=ref(20),fundLoading=ref(false)
async function loadFundRecords(page=fundPage.value){
  if(!current.value||!canManage.value)return
  fundLoading.value=true
  try{const data=await organizationsAPI.fundRecords(current.value.id,page,fundPageSize.value);fundRecords.value=data.items;fundTotal.value=data.total;fundPage.value=data.page}
  catch(error){appStore.showError((error as {message?:string})?.message||'加载资金记录失败')}
  finally{fundLoading.value=false}
}
function changeFundPageSize(size:number){fundPageSize.value=size;return loadFundRecords(1)}
const current=computed(()=>organizations.value.find(item=>item.id===selectedId.value)),canManage=computed(()=>current.value?.role==='owner'||current.value?.role==='admin')
const organizationOptions=computed(()=>organizations.value.map(item=>({value:item.id,label:item.name,role:item.role,seatUsed:item.seat_used,seatLimit:item.seat_limit})))
const money=(value:number)=>Number(value||0).toFixed(4),dateTime=(value:string)=>new Date(value).toLocaleString(),roleLabel=(role?:string)=>({owner:'超级管理员',admin:'管理员',member:'成员'}[role||'']||role||'')
async function refresh(){loading.value=true;try{organizations.value=await organizationsAPI.list();if(!selectedId.value&&organizations.value.length)selectedId.value=organizations.value[0].id;await loadWorkspace()}finally{loading.value=false}}
async function loadWorkspace(){if(!selectedId.value)return;const [org,memberRows,usageReport]=await Promise.all([organizationsAPI.get(selectedId.value),organizationsAPI.members(selectedId.value),organizationsAPI.usage(selectedId.value).catch(()=>null)]);organizations.value=organizations.value.map(item=>item.id===org.id?org:item);members.value=memberRows;usage.value=usageReport;if(canManage.value){[invitations.value,auditLogs.value]=await Promise.all([organizationsAPI.invitations(selectedId.value),organizationsAPI.auditLogs(selectedId.value)])}else{invitations.value=[];auditLogs.value=[]}}
async function createOrganization(){busy.value=true;try{const item=await organizationsAPI.create(createName.value,createSeats.value);organizations.value.push(item);selectedId.value=item.id;showCreate.value=false;createName.value='';await loadWorkspace()}finally{busy.value=false}}
async function deleteOrganization(){if(!current.value||busy.value)return;busy.value=true;try{await organizationsAPI.delete(current.value.id);showDelete.value=false;selectedId.value=0;members.value=[];invitations.value=[];usage.value=null;auditLogs.value=[];tab.value='overview';await refresh()}finally{busy.value=false}}
async function acceptInvitation(){busy.value=true;try{const item=await organizationsAPI.accept(extractInviteToken(acceptToken.value));await refresh();selectedId.value=item.id;showAccept.value=false;acceptToken.value='';await loadWorkspace();await workspaceAccess.fetch(true);if(route.query.invite)await router.replace({path:route.path,query:{}})}finally{busy.value=false}}
async function fund(){if(!current.value||!fundAmount.value)return;busy.value=true;try{const item=await organizationsAPI.fund(current.value.id,fundAmount.value);organizations.value=organizations.value.map(org=>org.id===item.id?item:org);fundAmount.value=undefined;await authStore.refreshUser().catch(()=>undefined)}finally{busy.value=false}}
// 转入 / 转出都会改变个人余额，顶部余额同步刷新
async function withdraw(){
  if(!current.value||!withdrawAmount.value||busy.value)return
  const amount=withdrawAmount.value
  if(!window.confirm(`确认将 ${money(amount)} 额度从“${current.value.name}”转出到你的个人余额？`))return
  busy.value=true
  try{const item=await organizationsAPI.withdraw(current.value.id,amount);organizations.value=organizations.value.map(org=>org.id===item.id?item:org);withdrawAmount.value=undefined;await authStore.refreshUser().catch(()=>undefined);appStore.showSuccess('已转出到个人余额')}
  catch(error){appStore.showError(withdrawErrorMessage(error))}
  finally{busy.value=false}
}
function withdrawAll(){if(!current.value)return;withdrawAmount.value=Math.floor(Number(current.value.display_balance||0)*1e8)/1e8;return withdraw()}
function withdrawErrorMessage(error:unknown){
  const err=(error||{}) as {reason?:string;message?:string}
  const known:Record<string,string>={ORGANIZATION_QUOTA_EXHAUSTED:'团队可用额度不足',ORGANIZATION_WITHDRAW_OWNER_ONLY:'仅超级管理员可以转出团队额度',INVALID_ORGANIZATION_WITHDRAW_AMOUNT:'请输入大于 0 的转出额度'}
  return (err.reason&&known[err.reason])||err.message||'转出失败'
}
async function createInvitation(){if(!current.value)return;busy.value=true;try{const item=await organizationsAPI.invite(current.value.id,{email:inviteEmail.value||undefined,role:inviteRole.value,max_uses:inviteEmail.value?1:20,expires_in_hours:168});inviteLink.value=item.token?`${window.location.origin}/organizations?invite=${encodeURIComponent(item.token)}`:'';inviteEmail.value='';invitations.value=await organizationsAPI.invitations(current.value.id)}finally{busy.value=false}}
async function copyInvite(){await navigator.clipboard.writeText(inviteLink.value)}
async function changeRole(member:OrganizationMember,event:Event){if(!current.value)return;await organizationsAPI.updateMember(current.value.id,member.user_id,{role:(event.target as HTMLSelectElement).value});await loadWorkspace()}
async function setLimit(member:OrganizationMember){if(!current.value)return;const raw=window.prompt('输入每月额度（USD），0 表示不限',String(member.monthly_limit));if(raw===null)return;const value=Number(raw);if(!Number.isFinite(value)||value<0)return;await organizationsAPI.updateMember(current.value.id,member.user_id,{monthly_limit:value});await loadWorkspace()}
async function removeMember(member:OrganizationMember){if(!current.value||!window.confirm(`确认移除 ${member.username||member.email}？其团队 Key 将立即停用。`))return;await organizationsAPI.removeMember(current.value.id,member.user_id);await loadWorkspace()}
// 退出团队：创建者不可退出（只能删除团队）。退出后若不再有任何团队访问权限（个人账号），回到个人资料页，那里有「加入团队」入口。
async function leaveOrganization(){const id=current.value?.id,uid=authStore.user?.id;if(!id||!uid||busy.value)return;busy.value=true;try{await organizationsAPI.leave(id,uid);showLeave.value=false;selectedId.value=0;members.value=[];invitations.value=[];usage.value=null;auditLogs.value=[];tab.value='overview';await workspaceAccess.fetch(true);if(!workspaceAccess.hasTeamAccess){await router.replace('/profile');return}await refresh()}finally{busy.value=false}}
function closeDialogs(){showCreate.value=false;showAccept.value=false;showDelete.value=false;showLeave.value=false}
watch(tab,async(value)=>{if(value==='audit'&&canManage.value&&current.value)auditLogs.value=await organizationsAPI.auditLogs(current.value.id);if(value==='funds')await loadFundRecords(1)})
// 在资金记录页切换团队，或角色变化导致标签消失时同步
watch(selectedId,()=>{if(tab.value==='funds')void loadFundRecords(1)})
watch(canManage,(ok)=>{if(!ok&&(tab.value==='funds'||tab.value==='audit'))tab.value='overview'})
onMounted(async()=>{
  await Promise.all([refresh(),workspaceAccess.fetch(true)])
  // 通过邀请链接进入：自动打开「加入团队」并填入令牌
  const invite=route.query.invite
  if(typeof invite==='string'&&invite){acceptToken.value=invite;showAccept.value=true}
})
</script>

<style scoped>
.stat-block { @apply rounded-md border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800; }
.stat-block span { @apply block text-xs text-gray-500 dark:text-gray-400; }
.stat-block strong { @apply mt-2 block text-xl font-semibold text-gray-900 dark:text-white; }
.tab-button { @apply whitespace-nowrap border-b-2 border-transparent px-4 py-3 text-sm text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white; }
.tab-button.active { @apply border-primary-500 font-medium text-primary-600 dark:text-primary-400; }
.input { @apply rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-primary-500 dark:border-dark-600 dark:bg-dark-800 dark:text-white; }
.data-table { @apply w-full text-left text-sm; }
.data-table th { @apply border-b border-gray-200 px-3 py-3 font-medium text-gray-500 dark:border-dark-700; }
.data-table td { @apply border-b border-gray-100 px-3 py-3 text-gray-700 dark:border-dark-700 dark:text-gray-300; }
.link-button { @apply text-sm font-medium text-primary-600 hover:underline dark:text-primary-400; }
.icon-button { @apply inline-flex h-8 w-8 shrink-0 items-center justify-center rounded text-gray-500 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-white; }
.team-switcher { @apply flex flex-col gap-4 border-b border-gray-200 pb-5 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between; }
.team-switcher-heading { @apply flex shrink-0 items-center gap-3; }
.team-switcher-icon { @apply inline-flex h-10 w-10 items-center justify-center rounded-md bg-primary-50 text-primary-600 dark:bg-primary-950/40 dark:text-primary-300; }
.team-role-chip { @apply shrink-0 rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300; }
.team-option-icon { @apply inline-flex h-8 w-8 shrink-0 items-center justify-center rounded bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-300; }
.delete-team-button { @apply inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-md border border-gray-200 bg-white text-gray-400 transition hover:border-red-200 hover:bg-red-50 hover:text-red-600 focus:outline-none focus:ring-2 focus:ring-red-500/20 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-red-900 dark:hover:bg-red-950/30 dark:hover:text-red-400; }
.role-select-wrap { @apply relative inline-flex h-9 min-w-32 items-center rounded-md border border-gray-200 bg-white shadow-sm transition focus-within:border-primary-500 focus-within:ring-2 focus-within:ring-primary-500/15 dark:border-dark-600 dark:bg-dark-800; }
.role-icon { @apply pointer-events-none absolute left-3 text-gray-500 dark:text-gray-400; }
.role-select { @apply h-full w-full appearance-none bg-transparent pl-9 pr-8 text-sm font-medium text-gray-800 outline-none dark:text-gray-100; }
.role-chevron { @apply pointer-events-none absolute right-3 text-gray-400; }
.role-badge { @apply inline-flex h-8 items-center gap-1.5 rounded-full border px-3 text-xs font-medium; }
.role-owner { @apply border-primary-200 bg-primary-50 text-primary-700 dark:border-primary-800 dark:bg-primary-950/30 dark:text-primary-300; }
.role-admin { @apply border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-950/30 dark:text-blue-300; }
.role-member { @apply border-gray-200 bg-gray-50 text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300; }
.status-dot { @apply inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300; }
.status-dot span { @apply h-2 w-2 rounded-full bg-emerald-500; }
</style>
