export default {
  videoTasks: {
    title: '客户视频任务',
    description: '查看下游客户生成的全部视频任务，可按用户、平台、状态等筛选，并查看上游账号与结算情况。',
    filters: {
      user: '用户邮箱 / 用户名',
      userId: '用户 ID',
      platform: '平台',
      apiKey: 'API Key 名称'
    },
    columns: {
      user: '用户',
      platform: '平台 / 账号',
      apiKey: 'API Key',
      billing: '结算'
    },
    userFallback: '用户 #{id}',
    held: '冻结中 {amount}',
    billing: {
      held: '已冻结',
      settling: '结算中',
      settling_none: '结算中（无扣费）',
      captured: '已扣费',
      released: '已释放',
      none: '无冻结'
    },
    detail: {
      user: '用户',
      userId: '用户 ID',
      apiKey: 'API Key',
      platform: '平台',
      account: '上游账号',
      group: '分组',
      localTaskId: '本地任务 ID',
      upstreamTaskId: '上游任务 ID',
      noUpstreamId: '尚未获得',
      billing: '结算状态',
      held: '冻结积分',
      endpoint: '调用接口',
      viewUser: '查看该用户的任务'
    }
  }
}
