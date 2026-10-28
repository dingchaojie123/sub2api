# 平台模型价格梳理

> 生成日期：2026-10-28  
> 参考页面：https://ct.corgitoken.vip/model-pricing  
> 页面报价来源：`frontend/src/views/user/ModelListView.vue`；后台基础价格来源：`backend/resources/model-pricing/model_prices_and_context_window.json` 与 `backend/internal/service/billing_service.go`。

## 计费口径

- 文本 Token 的后台扣费优先级：渠道自定义价格 > 动态模型价格目录 > Go 代码 fallback。
- Token 基础价格单位为 USD / 1M tokens；最终用户扣费通常还会乘分组倍率、用户专属倍率及高峰倍率。
- 页面“平台价”按人民币符号展示，本节仅记录页面数值，不把它直接当作后台 USD 基价。
- 图片、视频模型可能走按张、按秒或渠道分层价格，不一定使用 Token 基价。
- 数据库中的渠道自定义价格属于运行时数据，不在本仓库静态清单内。

## 后台基础价格（22 个唯一模型）

| 模型 | Provider | 模式 | 输入 / 1M | 输出 / 1M | 缓存读取 / 1M | 当前来源 |
|---|---|---:|---:|---:|---:|---|
| codex-auto-review | openai | chat | 5 | 30 | 0.5 | model pricing catalog |
| gemini-2.0-flash | vertex_ai-language-models | chat | 0.1 | 0.4 | 0.025 | model pricing catalog |
| gemini-2.5-flash | vertex_ai-language-models | chat | 0.3 | 2.5 | 0.03 | model pricing catalog |
| gemini-2.5-pro | vertex_ai-language-models | chat | 1.25 | 10 | 0.125 | model pricing catalog |
| gemini-3-flash-preview | vertex_ai-language-models | chat | 0.5 | 3 | 0.05 | model pricing catalog |
| gemini-3.1-flash-image | vertex_ai-language-models | image_generation | 0.5 | 3 | - | model pricing catalog |
| gemini-3.5-flash | vertex_ai-language-models | chat | 1.5 | 9 | 0.15 | model pricing catalog |
| glm-5-turbo | z.ai | chat | 1.2 | 4 | 0.24 | billing_service.go fallback |
| glm-5.1 | z.ai | chat | 1.4 | 4.4 | 0.26 | billing_service.go fallback |
| glm-5.2 | z.ai | chat | 1 | 3.2 | 0.2 | glm-5 fallback (current behavior) |
| gpt-5.3-codex-spark | openai | responses | 1.75 | 14 | 0.175 | model pricing catalog |
| gpt-5.4 | openai | chat | 2.5 | 15 | 0.25 | model pricing catalog |
| gpt-5.4-mini | openai | chat | 0.75 | 4.5 | 0.075 | model pricing catalog |
| gpt-5.5 | openai | chat | 25 | 150 | 2.5 | model pricing catalog |
| gpt-5.6-luna | openai | chat | 1 | 6 | 0.1 | model pricing catalog |
| gpt-5.6-sol | openai | chat | 20 | 100 | 2 | model pricing catalog |
| gpt-5.6-terra | openai | chat | 10 | 60 | 1 | model pricing catalog |
| gpt-6-astra | openai | chat | 50 | 250 | 5 | model pricing catalog |
| gpt-6-luna | openai | chat | 0.5 | 2.5 | 0.05 | model pricing catalog |
| gpt-6-sol | openai | chat | 10 | 50 | 1 | model pricing catalog |
| gpt-6.1-sol | openai | chat | 10 | 50 | 0.5 | model pricing catalog |
| gpt-image-2 | openai | image_generation | 5 | 10 | 1.25 | model pricing catalog |

## 参考页面分组报价（60 行）

### ChatGPT【特惠分组】（0.2x）

| 模型 | 平台输入 / 1M | 平台输出 / 1M | 页面官方输入 / 1M | 页面官方输出 / 1M |
|---|---:|---:|---:|---:|
| codex-auto-review | 0.04 | 0.24 | 0.2 | 1.2 |
| gpt-5.4 | 0.5 | 3 | 2.5 | 15 |
| gpt-5.5 | 5 | 30 | 5 | 30 |
| gpt-5.6-sol | 4 | 20 | 4 | 20 |
| gpt-5.6-terra | 2 | 12 | 2 | 12 |
| gpt-5.6-luna | 0.2 | 1.2 | 0.2 | 1.2 |
| gpt-6-astra | 10 | 50 | 10 | 50 |
| gpt-6-luna | 0.1 | 0.5 | 0.1 | 0.5 |
| gpt-6-sol | 2 | 10 | 2 | 10 |
| gpt-6.1-sol | 2 | 10 | 2 | 10 |
| gpt-image-2 | 1 | 2 | 5 | 10 |

### ChatGPT【便宜分组】（0.3x）

| 模型 | 平台输入 / 1M | 平台输出 / 1M | 页面官方输入 / 1M | 页面官方输出 / 1M |
|---|---:|---:|---:|---:|
| codex-auto-review | 0.06 | 0.36 | 0.2 | 1.2 |
| gpt-5.3-codex-spark | 0.525 | 4.2 | 1.75 | 14 |
| gpt-5.4 | 0.75 | 4.5 | 2.5 | 15 |
| gpt-5.5 | 7.5 | 45 | 5 | 30 |
| gpt-5.6-sol | 6 | 30 | 4 | 20 |
| gpt-5.6-terra | 3 | 18 | 2 | 12 |
| gpt-5.6-luna | 0.3 | 1.8 | 0.2 | 1.2 |
| gpt-6-astra | 15 | 75 | 10 | 50 |
| gpt-6-luna | 0.15 | 0.75 | 0.1 | 0.5 |
| gpt-6-sol | 3 | 15 | 2 | 10 |
| gpt-6.1-sol | 3 | 15 | 2 | 10 |
| gpt-image-2 | 1.5 | 3 | 5 | 10 |

### ChatGPT【稳定分组】（0.5x）

| 模型 | 平台输入 / 1M | 平台输出 / 1M | 页面官方输入 / 1M | 页面官方输出 / 1M |
|---|---:|---:|---:|---:|
| codex-auto-review | 0.1 | 0.6 | 0.2 | 1.2 |
| gpt-5.3-codex-spark | 0.875 | 7 | 1.75 | 14 |
| gpt-5.4 | 1.25 | 7.5 | 2.5 | 15 |
| gpt-5.5 | 12.5 | 75 | 5 | 30 |
| gpt-5.6-sol | 10 | 50 | 4 | 20 |
| gpt-5.6-terra | 5 | 30 | 2 | 12 |
| gpt-5.6-luna | 0.5 | 3 | 0.2 | 1.2 |
| gpt-6-astra | 25 | 125 | 10 | 50 |
| gpt-6-luna | 0.25 | 1.25 | 0.1 | 0.5 |
| gpt-6-sol | 5 | 25 | 2 | 10 |
| gpt-6.1-sol | 5 | 25 | 2 | 10 |
| gpt-image-2 | 2.5 | 5 | 5 | 10 |

### ChatGPT【企业分组】（1.4x）

| 模型 | 平台输入 / 1M | 平台输出 / 1M | 页面官方输入 / 1M | 页面官方输出 / 1M |
|---|---:|---:|---:|---:|
| codex-auto-review | 0.2 | 1.2 | 0.28 | 1.68 |
| gpt-5.3-codex-spark | 1.75 | 14 | 2.44 | 19.6 |
| gpt-5.4 | 2.5 | 15 | 3.5 | 21 |
| gpt-5.4-mini | 0.75 | 4.5 | 1.05 | 6.3 |
| gpt-5.5 | 35 | 210 | 5 | 30 |
| gpt-5.6-sol | 28 | 140 | 4 | 20 |
| gpt-5.6-terra | 14 | 84 | 2 | 12 |
| gpt-5.6-luna | 1.4 | 8.4 | 0.2 | 1.2 |
| gpt-6-astra | 70 | 350 | 10 | 50 |
| gpt-6-luna | 0.7 | 3.5 | 0.1 | 0.5 |
| gpt-6-sol | 14 | 70 | 2 | 10 |
| gpt-6.1-sol | 14 | 70 | 2 | 10 |
| gpt-image-2 | 5 | 10 | 7 | 14 |

### gemini（1.0x）

| 模型 | 平台输入 / 1M | 平台输出 / 1M | 页面官方输入 / 1M | 页面官方输出 / 1M |
|---|---:|---:|---:|---:|
| gemini-2.0-flash | 0.1 | 0.4 | 0.1 | 0.4 |
| gemini-2.5-flash | 0.3 | 2.5 | 0.3 | 2.5 |
| gemini-2.5-pro | 1.25 | 10 | 1.25 | 10 |
| gemini-3-flash-preview | 0.5 | 3 | 0.5 | 3 |
| gemini-3.1-flash-image | 0.5 | 3 | 0.5 | 3 |
| gemini-3.5-flash | 1.5 | 9 | 1.5 | 9 |

### glm - 官key openai协议（0.9x）

| 模型 | 平台输入 / 1M | 平台输出 / 1M | 页面官方输入 / 1M | 页面官方输出 / 1M |
|---|---:|---:|---:|---:|
| glm-5-turbo | 1.08 | 3.6 | 1.2 | 4 |
| glm-5.1 | 1.26 | 3.96 | 1.4 | 4.4 |
| glm-5.2 | 1.26 | 3.96 | 1.4 | 4.4 |

### glm - 官key claude协议（0.9x）

| 模型 | 平台输入 / 1M | 平台输出 / 1M | 页面官方输入 / 1M | 页面官方输出 / 1M |
|---|---:|---:|---:|---:|
| glm-5-turbo | 1.08 | 3.6 | 1.2 | 4 |
| glm-5.1 | 1.26 | 3.96 | 1.4 | 4.4 |
| glm-5.2 | 1.26 | 3.96 | 1.4 | 4.4 |

## 页面与后台口径差异

下表按“后台基础价 × 页面标注倍率”推算，仅用于发现静态配置差异；实际渠道自定义价和数据库中的真实分组倍率仍可能改变最终扣费。

| 分组 | 模型 | 页面平台输入/输出 | 后台基价 × 标注倍率 | 说明 |
|---|---|---:|---:|---|
| ChatGPT【特惠分组】 | codex-auto-review | 0.04 / 0.24 | 1 / 6 | 后台目录基价为 5/30，页面采用的基准数值为 0.2/1.2 |
| ChatGPT【便宜分组】 | codex-auto-review | 0.06 / 0.36 | 1.5 / 9 | 后台目录基价为 5/30，页面采用的基准数值为 0.2/1.2 |
| ChatGPT【稳定分组】 | codex-auto-review | 0.1 / 0.6 | 2.5 / 15 | 后台目录基价为 5/30，页面采用的基准数值为 0.2/1.2 |
| ChatGPT【企业分组】 | codex-auto-review | 0.2 / 1.2 | 7 / 42 | 后台目录基价为 5/30，页面采用的基准数值为 0.2/1.2 |
| ChatGPT【企业分组】 | gpt-5.3-codex-spark | 1.75 / 14 | 2.45 / 19.6 | 页面平台价与后台基础价乘标注倍率不一致 |
| ChatGPT【企业分组】 | gpt-5.4 | 2.5 / 15 | 3.5 / 21 | 页面平台价与后台基础价乘标注倍率不一致 |
| ChatGPT【企业分组】 | gpt-5.4-mini | 0.75 / 4.5 | 1.05 / 6.3 | 页面平台价与后台基础价乘标注倍率不一致 |
| ChatGPT【企业分组】 | gpt-image-2 | 5 / 10 | 7 / 14 | 页面平台价与后台基础价乘标注倍率不一致 |
| glm - 官key openai协议 | glm-5.2 | 1.26 / 3.96 | 0.9 / 2.88 | 当前代码会回退到 glm-5，而页面按 glm-5.1 价格展示 |
| glm - 官key claude协议 | glm-5.2 | 1.26 / 3.96 | 0.9 / 2.88 | 当前代码会回退到 glm-5，而页面按 glm-5.1 价格展示 |

## 维护位置

- 页面默认报价：`frontend/src/views/user/ModelListView.vue` 中的 `defaultPricingGroups`。
- 后台动态/本地模型目录：`backend/resources/model-pricing/model_prices_and_context_window.json`。
- 代码 fallback：`backend/internal/service/billing_service.go` 中的 `initFallbackPricing` 和 `getFallbackPricing`。
- 渠道覆盖价：数据库表 `channel_model_pricing` 与 `channel_model_pricing_intervals`。
- 分组倍率：分组的 `rate_multiplier`、`image_rate_multiplier`、`video_rate_multiplier` 等字段。
