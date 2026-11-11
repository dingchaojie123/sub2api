// 「接入文档」页的文档注册表。
//
// 文档源文件在仓库根目录的 docs/ 下，构建时用 ?raw 打进前端（与法律文档页同一做法）。
// 只显式导入下面这些对外文档，不用通配符：docs/internal/ 是内部手册，绝不能进入前端包。
import videoEntry from '../../../../docs/PP_VIDEO_GENERATION_API.md?raw'
import qwenTts from '../../../../docs/QWEN_TTS_API.md?raw'
import advanced from '../../../../docs/video-api/advanced.md?raw'
import doubaoSeedance from '../../../../docs/video-api/doubao-seedance.md?raw'
import generalModels from '../../../../docs/video-api/general-models.md?raw'
import minimaxH3 from '../../../../docs/video-api/minimax-h3.md?raw'
import pixverse from '../../../../docs/video-api/pixverse.md?raw'
import seedanceHub from '../../../../docs/video-api/seedance-hub.md?raw'
import seedanceSeries from '../../../../docs/video-api/seedance-series.md?raw'
import unified from '../../../../docs/video-api/unified.md?raw'
import wan from '../../../../docs/video-api/wan.md?raw'

export type ApiDocGroupId = 'video' | 'video-v1' | 'audio'

export interface ApiDocPage {
  /** 路由参数，/api-docs/:slug */
  slug: string
  /** 相对 docs/ 目录的路径，用来解析文档里的相对链接 */
  path: string
  /** 左侧文档列表里的名称 */
  title: string
  group: ApiDocGroupId
  markdown: string
}

export const API_DOC_DEFAULT_SLUG = 'video'

export const apiDocGroups: { id: ApiDocGroupId; label: string }[] = [
  { id: 'video', label: '视频生成' },
  { id: 'video-v1', label: '各模型原生写法（v1）' },
  { id: 'audio', label: '语音合成' },
]

export const apiDocPages: ApiDocPage[] = [
  { slug: 'video', path: 'PP_VIDEO_GENERATION_API.md', title: '快速开始', group: 'video', markdown: videoEntry },
  { slug: 'unified', path: 'video-api/unified.md', title: '统一请求格式（v2）', group: 'video', markdown: unified },
  { slug: 'seedance-hub', path: 'video-api/seedance-hub.md', title: 'Seedance 综合分组', group: 'video', markdown: seedanceHub },
  { slug: 'doubao-seedance', path: 'video-api/doubao-seedance.md', title: 'Seedance（doubao 系列）', group: 'video-v1', markdown: doubaoSeedance },
  { slug: 'seedance-series', path: 'video-api/seedance-series.md', title: 'Seedance（其他型号、Seedance2.5-480全参）', group: 'video-v1', markdown: seedanceSeries },
  { slug: 'wan', path: 'video-api/wan.md', title: 'Wan3.0', group: 'video-v1', markdown: wan },
  { slug: 'general-models', path: 'video-api/general-models.md', title: '综合视频模型', group: 'video-v1', markdown: generalModels },
  { slug: 'minimax-h3', path: 'video-api/minimax-h3.md', title: 'MiniMax-H3', group: 'video-v1', markdown: minimaxH3 },
  { slug: 'pixverse', path: 'video-api/pixverse.md', title: 'Pixverse-V6', group: 'video-v1', markdown: pixverse },
  { slug: 'advanced', path: 'video-api/advanced.md', title: '进阶说明', group: 'video-v1', markdown: advanced },
  { slug: 'qwen-tts', path: 'QWEN_TTS_API.md', title: 'Qwen TTS 语音合成', group: 'audio', markdown: qwenTts },
]

export function findApiDocPage(slug: string): ApiDocPage | undefined {
  return apiDocPages.find((page) => page.slug === slug)
}

export function apiDocRoute(slug: string): string {
  return `/api-docs/${slug}`
}

/**
 * 把文档里的相对链接（如 `video-api/wan.md`、`../PP_VIDEO_GENERATION_API.md#xxx`）解析成页面 slug。
 * 解析不到（指向未注册的文件、或越出 docs/ 目录）返回 null。
 */
export function resolveApiDocSlug(currentPath: string, href: string): string | null {
  const target = href.split('#')[0].split('?')[0]
  if (!target) return null
  const stack = currentPath.split('/').slice(0, -1)
  for (const part of target.split('/')) {
    if (part === '' || part === '.') continue
    if (part === '..') {
      if (stack.length === 0) return null
      stack.pop()
    } else {
      stack.push(part)
    }
  }
  const resolved = stack.join('/')
  return apiDocPages.find((page) => page.path === resolved)?.slug ?? null
}
