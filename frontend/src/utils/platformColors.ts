/**
 * Centralized platform color definitions.
 *
 * All components that need platform-specific styling should import from here
 * instead of defining their own color mappings.
 */

export type Platform =
  | 'anthropic'
  | 'openai'
  | 'antigravity'
  | 'gemini'
  | 'grok'
  | 'jimeng'
  | 'doubao'
  | 'qwen'
  | 'kimi'
  | 'deepseek'
  | 'midjourney'
  | 'kling'
  | 'happyhourse'
  | 'seedance'
  | 'ctmo-seedance'
	| 'xingapi-seedance'
	| 'jijiu-seedance'
	| 'miaowu-seedance'
	| 'paisio-seedance'
	| 'apophis-seedance'
	| 'xsn-seedance'
	| 'fengyue-seedance'
	| 'hongpei-seedance'
	| 'vshi-seedance'
	| 'huahuo-seedance'
	| 'zexi-seedance'
	| 'xm-seedance'
	| 'naiwa-seedance'
	| 'v6k-seedance'
  | 'guangying'
  | 'opone'
  | 'mai-token-seedance'
  | 'seedance-hub'
  | 'bytedance'
  | 'wan3'
  | 'kyy-wan'
  | 'minimax-h3'
  | 'minimax-h3-compshare'
  | '88api-video'
  | 'minimax-speech'
  | 'qwen-tts'
  | 'pixverse-v6'
  | 'grok-imagine-video'
  | 'kuaishou'

// ── Badge (bg + text + border, for inline badges with border) ───────
const BADGE: Record<Platform, string> = {
  anthropic: 'bg-orange-500/10 text-orange-600 border-orange-500/30 dark:text-orange-400',
  openai: 'bg-green-500/10 text-green-600 border-green-500/30 dark:text-green-400',
  antigravity: 'bg-purple-500/10 text-purple-600 border-purple-500/30 dark:text-purple-400',
  gemini: 'bg-blue-500/10 text-blue-600 border-blue-500/30 dark:text-blue-400',
  grok: 'bg-zinc-800/10 text-zinc-800 border-zinc-800/30 dark:bg-zinc-500/10 dark:text-zinc-200 dark:border-zinc-500/30',
  jimeng: 'bg-rose-500/10 text-rose-600 border-rose-500/30 dark:text-rose-400',
  doubao: 'bg-cyan-500/10 text-cyan-700 border-cyan-500/30 dark:text-cyan-300',
  qwen: 'bg-amber-500/10 text-amber-700 border-amber-500/30 dark:text-amber-300',
  kimi: 'bg-indigo-500/10 text-indigo-700 border-indigo-500/30 dark:text-indigo-300',
  deepseek: 'bg-slate-700/10 text-slate-700 border-slate-700/30 dark:text-slate-200',
  midjourney: 'bg-pink-500/10 text-pink-700 border-pink-500/30 dark:text-pink-300',
  kling: 'bg-sky-500/10 text-sky-700 border-sky-500/30 dark:text-sky-300',
  happyhourse: 'bg-emerald-500/10 text-emerald-700 border-emerald-500/30 dark:text-emerald-300',
  seedance: 'bg-teal-500/10 text-teal-700 border-teal-500/30 dark:text-teal-300',
  'ctmo-seedance': 'bg-teal-500/10 text-teal-700 border-teal-500/30 dark:text-teal-300',
	'xingapi-seedance': 'bg-amber-500/10 text-amber-700 border-amber-500/30 dark:text-amber-300',
	'jijiu-seedance': 'bg-orange-500/10 text-orange-700 border-orange-500/30 dark:text-orange-300',
	'miaowu-seedance': 'bg-lime-500/10 text-lime-700 border-lime-500/30 dark:text-lime-300',
	'paisio-seedance': 'bg-blue-500/10 text-blue-700 border-blue-500/30 dark:text-blue-300',
	'apophis-seedance': 'bg-yellow-500/10 text-yellow-700 border-yellow-500/30 dark:text-yellow-300',
	'xsn-seedance': 'bg-red-500/10 text-red-700 border-red-500/30 dark:text-red-300',
	'fengyue-seedance': 'bg-pink-500/10 text-pink-700 border-pink-500/30 dark:text-pink-300',
	'hongpei-seedance': 'bg-violet-500/10 text-violet-700 border-violet-500/30 dark:text-violet-300',
	'vshi-seedance': 'bg-teal-500/10 text-teal-700 border-teal-500/30 dark:text-teal-300',
	'huahuo-seedance': 'bg-fuchsia-500/10 text-fuchsia-700 border-fuchsia-500/30 dark:text-fuchsia-300',
	'zexi-seedance': 'bg-stone-500/10 text-stone-700 border-stone-500/30 dark:text-stone-300',
	'xm-seedance': 'bg-sky-500/10 text-sky-700 border-sky-500/30 dark:text-sky-300',
	'naiwa-seedance': 'bg-emerald-500/10 text-emerald-700 border-emerald-500/30 dark:text-emerald-300',
	'v6k-seedance': 'bg-indigo-500/10 text-indigo-700 border-indigo-500/30 dark:text-indigo-300',
	'kyy-wan': 'bg-indigo-500/10 text-indigo-700 border-indigo-500/30 dark:text-indigo-300',
  guangying: 'bg-rose-500/10 text-rose-700 border-rose-500/30 dark:text-rose-300',
  opone: 'bg-indigo-500/10 text-indigo-700 border-indigo-500/30 dark:text-indigo-300',
  'mai-token-seedance': 'bg-cyan-500/10 text-cyan-700 border-cyan-500/30 dark:text-cyan-300',
  'seedance-hub': 'bg-purple-500/10 text-purple-700 border-purple-500/30 dark:text-purple-300',
  bytedance: 'bg-violet-500/10 text-violet-700 border-violet-500/30 dark:text-violet-300',
  wan3: 'bg-fuchsia-500/10 text-fuchsia-700 border-fuchsia-500/30 dark:text-fuchsia-300',
  'minimax-h3': 'bg-pink-500/10 text-pink-700 border-pink-500/30 dark:text-pink-300',
  'minimax-h3-compshare': 'bg-pink-500/10 text-pink-700 border-pink-500/30 dark:text-pink-300',
  '88api-video': 'bg-emerald-500/10 text-emerald-700 border-emerald-500/30 dark:text-emerald-300',
  'minimax-speech': 'bg-violet-500/10 text-violet-700 border-violet-500/30 dark:text-violet-300',
  'qwen-tts': 'bg-amber-500/10 text-amber-700 border-amber-500/30 dark:text-amber-300',
  'pixverse-v6': 'bg-lime-500/10 text-lime-700 border-lime-500/30 dark:text-lime-300',
  'grok-imagine-video': 'bg-cyan-500/10 text-cyan-700 border-cyan-500/30 dark:text-cyan-300',
  kuaishou: 'bg-sky-500/10 text-sky-700 border-sky-500/30 dark:text-sky-300',
}
const BADGE_DEFAULT = 'bg-slate-500/10 text-slate-600 border-slate-500/30 dark:text-slate-400'

// ── Light badge (softer bg, no border) ──────────────────────────────
const BADGE_LIGHT: Record<Platform, string> = {
  anthropic: 'bg-orange-500/10 text-orange-600 dark:bg-orange-500/10 dark:text-orange-300',
  openai: 'bg-green-500/10 text-green-600 dark:bg-green-500/10 dark:text-green-300',
  antigravity: 'bg-purple-500/10 text-purple-600 dark:bg-purple-500/10 dark:text-purple-300',
  gemini: 'bg-blue-500/10 text-blue-600 dark:bg-blue-500/10 dark:text-blue-300',
  grok: 'bg-zinc-800/10 text-zinc-800 dark:bg-zinc-500/10 dark:text-zinc-200',
  jimeng: 'bg-rose-500/10 text-rose-600 dark:bg-rose-500/10 dark:text-rose-300',
  doubao: 'bg-cyan-500/10 text-cyan-700 dark:bg-cyan-500/10 dark:text-cyan-300',
  qwen: 'bg-amber-500/10 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300',
  kimi: 'bg-indigo-500/10 text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-300',
  deepseek: 'bg-slate-700/10 text-slate-700 dark:bg-slate-500/10 dark:text-slate-200',
  midjourney: 'bg-pink-500/10 text-pink-700 dark:bg-pink-500/10 dark:text-pink-300',
  kling: 'bg-sky-500/10 text-sky-700 dark:bg-sky-500/10 dark:text-sky-300',
  happyhourse: 'bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300',
  seedance: 'bg-teal-500/10 text-teal-700 dark:bg-teal-500/10 dark:text-teal-300',
  'ctmo-seedance': 'bg-teal-500/10 text-teal-700 dark:bg-teal-500/10 dark:text-teal-300',
	'xingapi-seedance': 'bg-amber-500/10 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300',
	'jijiu-seedance': 'bg-orange-500/10 text-orange-700 dark:bg-orange-500/10 dark:text-orange-300',
	'miaowu-seedance': 'bg-lime-500/10 text-lime-700 dark:bg-lime-500/10 dark:text-lime-300',
	'paisio-seedance': 'bg-blue-500/10 text-blue-700 dark:bg-blue-500/10 dark:text-blue-300',
	'apophis-seedance': 'bg-yellow-500/10 text-yellow-700 dark:bg-yellow-500/10 dark:text-yellow-300',
	'xsn-seedance': 'bg-red-500/10 text-red-700 dark:bg-red-500/10 dark:text-red-300',
	'fengyue-seedance': 'bg-pink-500/10 text-pink-700 dark:bg-pink-500/10 dark:text-pink-300',
	'hongpei-seedance': 'bg-violet-500/10 text-violet-700 dark:bg-violet-500/10 dark:text-violet-300',
	'vshi-seedance': 'bg-teal-500/10 text-teal-700 dark:bg-teal-500/10 dark:text-teal-300',
	'huahuo-seedance': 'bg-fuchsia-500/10 text-fuchsia-700 dark:bg-fuchsia-500/10 dark:text-fuchsia-300',
	'zexi-seedance': 'bg-stone-500/10 text-stone-700 dark:bg-stone-500/10 dark:text-stone-300',
	'xm-seedance': 'bg-sky-500/10 text-sky-700 dark:bg-sky-500/10 dark:text-sky-300',
	'naiwa-seedance': 'bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300',
	'v6k-seedance': 'bg-indigo-500/10 text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-300',
	'kyy-wan': 'bg-indigo-500/10 text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-300',
  guangying: 'bg-rose-500/10 text-rose-700 dark:bg-rose-500/10 dark:text-rose-300',
  opone: 'bg-indigo-500/10 text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-300',
  'mai-token-seedance': 'bg-cyan-500/10 text-cyan-700 dark:bg-cyan-500/10 dark:text-cyan-300',
  'seedance-hub': 'bg-purple-500/10 text-purple-700 dark:bg-purple-500/10 dark:text-purple-300',
  bytedance: 'bg-violet-500/10 text-violet-700 dark:bg-violet-500/10 dark:text-violet-300',
  wan3: 'bg-fuchsia-500/10 text-fuchsia-700 dark:bg-fuchsia-500/10 dark:text-fuchsia-300',
  'minimax-h3': 'bg-pink-500/10 text-pink-700 dark:bg-pink-500/10 dark:text-pink-300',
  'minimax-h3-compshare': 'bg-pink-500/10 text-pink-700 dark:bg-pink-500/10 dark:text-pink-300',
  '88api-video': 'bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300',
  'minimax-speech': 'bg-violet-500/10 text-violet-700 dark:bg-violet-500/10 dark:text-violet-300',
  'qwen-tts': 'bg-amber-500/10 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300',
  'pixverse-v6': 'bg-lime-500/10 text-lime-700 dark:bg-lime-500/10 dark:text-lime-300',
  'grok-imagine-video': 'bg-cyan-500/10 text-cyan-700 dark:bg-cyan-500/10 dark:text-cyan-300',
  kuaishou: 'bg-sky-500/10 text-sky-700 dark:bg-sky-500/10 dark:text-sky-300',
}

// ── Border ──────────────────────────────────────────────────────────
const BORDER: Record<Platform, string> = {
  anthropic: 'border-orange-500/20 dark:border-orange-500/20',
  openai: 'border-green-500/20 dark:border-green-500/20',
  antigravity: 'border-purple-500/20 dark:border-purple-500/20',
  gemini: 'border-blue-500/20 dark:border-blue-500/20',
  grok: 'border-zinc-800/20 dark:border-zinc-500/20',
  jimeng: 'border-rose-500/20 dark:border-rose-500/20',
  doubao: 'border-cyan-500/20 dark:border-cyan-500/20',
  qwen: 'border-amber-500/20 dark:border-amber-500/20',
  kimi: 'border-indigo-500/20 dark:border-indigo-500/20',
  deepseek: 'border-slate-700/20 dark:border-slate-500/20',
  midjourney: 'border-pink-500/20 dark:border-pink-500/20',
  kling: 'border-sky-500/20 dark:border-sky-500/20',
  happyhourse: 'border-emerald-500/20 dark:border-emerald-500/20',
  seedance: 'border-teal-500/20 dark:border-teal-500/20',
  'ctmo-seedance': 'border-teal-500/20 dark:border-teal-500/20',
	'xingapi-seedance': 'border-amber-500/20 dark:border-amber-500/20',
	'jijiu-seedance': 'border-orange-500/20 dark:border-orange-500/20',
	'miaowu-seedance': 'border-lime-500/20 dark:border-lime-500/20',
	'paisio-seedance': 'border-blue-500/20 dark:border-blue-500/20',
	'apophis-seedance': 'border-yellow-500/20 dark:border-yellow-500/20',
	'xsn-seedance': 'border-red-500/20 dark:border-red-500/20',
	'fengyue-seedance': 'border-pink-500/20 dark:border-pink-500/20',
	'hongpei-seedance': 'border-violet-500/20 dark:border-violet-500/20',
	'vshi-seedance': 'border-teal-500/20 dark:border-teal-500/20',
	'huahuo-seedance': 'border-fuchsia-500/20 dark:border-fuchsia-500/20',
	'zexi-seedance': 'border-stone-500/20 dark:border-stone-500/20',
	'xm-seedance': 'border-sky-500/20 dark:border-sky-500/20',
	'naiwa-seedance': 'border-emerald-500/20 dark:border-emerald-500/20',
	'v6k-seedance': 'border-indigo-500/20 dark:border-indigo-500/20',
	'kyy-wan': 'border-indigo-500/20 dark:border-indigo-500/20',
  guangying: 'border-rose-500/20 dark:border-rose-500/20',
  opone: 'border-indigo-500/20 dark:border-indigo-500/20',
  'mai-token-seedance': 'border-cyan-500/20 dark:border-cyan-500/20',
  'seedance-hub': 'border-purple-500/20 dark:border-purple-500/20',
  bytedance: 'border-violet-500/20 dark:border-violet-500/20',
  wan3: 'border-fuchsia-500/20 dark:border-fuchsia-500/20',
  'minimax-h3': 'border-pink-500/20 dark:border-pink-500/20',
  'minimax-h3-compshare': 'border-pink-500/20 dark:border-pink-500/20',
  '88api-video': 'border-emerald-500/20 dark:border-emerald-500/20',
  'minimax-speech': 'border-violet-500/20 dark:border-violet-500/20',
  'qwen-tts': 'border-amber-500/20 dark:border-amber-500/20',
  'pixverse-v6': 'border-lime-500/20 dark:border-lime-500/20',
  'grok-imagine-video': 'border-cyan-500/20 dark:border-cyan-500/20',
  kuaishou: 'border-sky-500/20 dark:border-sky-500/20',
}
const BORDER_DEFAULT = 'border-gray-200 dark:border-dark-700'

// ── Accent bar (gradient) ───────────────────────────────────────────
const ACCENT_BAR: Record<Platform, string> = {
  anthropic: 'bg-gradient-to-r from-orange-400 to-orange-500',
  openai: 'bg-gradient-to-r from-emerald-400 to-emerald-500',
  antigravity: 'bg-gradient-to-r from-purple-400 to-purple-500',
  gemini: 'bg-gradient-to-r from-blue-400 to-blue-500',
  grok: 'bg-gradient-to-r from-zinc-700 to-zinc-900',
  jimeng: 'bg-gradient-to-r from-rose-400 to-pink-500',
  doubao: 'bg-gradient-to-r from-cyan-400 to-cyan-500',
  qwen: 'bg-gradient-to-r from-amber-400 to-orange-500',
  kimi: 'bg-gradient-to-r from-indigo-400 to-indigo-600',
  deepseek: 'bg-gradient-to-r from-slate-600 to-slate-800',
  midjourney: 'bg-gradient-to-r from-pink-400 to-fuchsia-600',
  kling: 'bg-gradient-to-r from-sky-400 to-sky-600',
  happyhourse: 'bg-gradient-to-r from-emerald-400 to-emerald-600',
  seedance: 'bg-gradient-to-r from-teal-400 to-teal-600',
  'ctmo-seedance': 'bg-gradient-to-r from-teal-400 to-teal-600',
	'xingapi-seedance': 'bg-gradient-to-r from-amber-400 to-amber-600',
	'jijiu-seedance': 'bg-gradient-to-r from-orange-400 to-orange-600',
	'miaowu-seedance': 'bg-gradient-to-r from-lime-400 to-lime-600',
	'paisio-seedance': 'bg-gradient-to-r from-blue-400 to-blue-600',
	'apophis-seedance': 'bg-gradient-to-r from-yellow-400 to-yellow-600',
	'xsn-seedance': 'bg-gradient-to-r from-red-400 to-red-600',
	'fengyue-seedance': 'bg-gradient-to-r from-pink-400 to-pink-600',
	'hongpei-seedance': 'bg-gradient-to-r from-violet-400 to-violet-600',
	'vshi-seedance': 'bg-gradient-to-r from-teal-400 to-teal-600',
	'huahuo-seedance': 'bg-gradient-to-r from-fuchsia-400 to-fuchsia-600',
	'zexi-seedance': 'bg-gradient-to-r from-stone-400 to-stone-600',
	'xm-seedance': 'bg-gradient-to-r from-sky-400 to-sky-600',
	'naiwa-seedance': 'bg-gradient-to-r from-emerald-400 to-emerald-600',
	'v6k-seedance': 'bg-gradient-to-r from-indigo-400 to-indigo-600',
	'kyy-wan': 'bg-gradient-to-r from-indigo-400 to-indigo-600',
  guangying: 'bg-gradient-to-r from-rose-400 to-rose-600',
  opone: 'bg-gradient-to-r from-indigo-400 to-indigo-600',
  'mai-token-seedance': 'bg-gradient-to-r from-cyan-400 to-cyan-500',
  'seedance-hub': 'bg-gradient-to-r from-purple-400 to-purple-500',
  bytedance: 'bg-gradient-to-r from-violet-400 to-violet-600',
  wan3: 'bg-gradient-to-r from-fuchsia-400 to-fuchsia-600',
  'minimax-h3': 'bg-gradient-to-r from-pink-400 to-pink-600',
  'minimax-h3-compshare': 'bg-gradient-to-r from-pink-400 to-pink-600',
  '88api-video': 'bg-gradient-to-r from-emerald-400 to-emerald-600',
  'minimax-speech': 'bg-gradient-to-r from-violet-400 to-violet-600',
  'qwen-tts': 'bg-gradient-to-r from-amber-400 to-orange-500',
  'pixverse-v6': 'bg-gradient-to-r from-lime-400 to-lime-600',
  'grok-imagine-video': 'bg-gradient-to-r from-cyan-400 to-cyan-600',
  kuaishou: 'bg-gradient-to-r from-sky-400 to-sky-600',
}
const ACCENT_BAR_DEFAULT = 'bg-gradient-to-r from-primary-400 to-primary-500'

// ── Text (price, icon) ─────────────────────────────────────────────
const TEXT: Record<Platform, string> = {
  anthropic: 'text-orange-600 dark:text-orange-400',
  openai: 'text-emerald-600 dark:text-emerald-400',
  antigravity: 'text-purple-600 dark:text-purple-400',
  gemini: 'text-blue-600 dark:text-blue-400',
  grok: 'text-zinc-800 dark:text-zinc-200',
  jimeng: 'text-rose-600 dark:text-rose-400',
  doubao: 'text-cyan-700 dark:text-cyan-300',
  qwen: 'text-amber-700 dark:text-amber-300',
  kimi: 'text-indigo-700 dark:text-indigo-300',
  deepseek: 'text-slate-700 dark:text-slate-200',
  midjourney: 'text-pink-700 dark:text-pink-300',
  kling: 'text-sky-700 dark:text-sky-300',
  happyhourse: 'text-emerald-700 dark:text-emerald-300',
  seedance: 'text-teal-700 dark:text-teal-300',
  'ctmo-seedance': 'text-teal-700 dark:text-teal-300',
	'xingapi-seedance': 'text-amber-700 dark:text-amber-300',
	'jijiu-seedance': 'text-orange-700 dark:text-orange-300',
	'miaowu-seedance': 'text-lime-700 dark:text-lime-300',
	'paisio-seedance': 'text-blue-700 dark:text-blue-300',
	'apophis-seedance': 'text-yellow-700 dark:text-yellow-300',
	'xsn-seedance': 'text-red-700 dark:text-red-300',
	'fengyue-seedance': 'text-pink-700 dark:text-pink-300',
	'hongpei-seedance': 'text-violet-700 dark:text-violet-300',
	'vshi-seedance': 'text-teal-700 dark:text-teal-300',
	'huahuo-seedance': 'text-fuchsia-700 dark:text-fuchsia-300',
	'zexi-seedance': 'text-stone-700 dark:text-stone-300',
	'xm-seedance': 'text-sky-700 dark:text-sky-300',
	'naiwa-seedance': 'text-emerald-700 dark:text-emerald-300',
	'v6k-seedance': 'text-indigo-700 dark:text-indigo-300',
	'kyy-wan': 'text-indigo-700 dark:text-indigo-300',
  guangying: 'text-rose-700 dark:text-rose-300',
  opone: 'text-indigo-700 dark:text-indigo-300',
  'mai-token-seedance': 'text-cyan-700 dark:text-cyan-300',
  'seedance-hub': 'text-purple-700 dark:text-purple-300',
  bytedance: 'text-violet-700 dark:text-violet-300',
  wan3: 'text-fuchsia-700 dark:text-fuchsia-300',
  'minimax-h3': 'text-pink-700 dark:text-pink-300',
  'minimax-h3-compshare': 'text-pink-700 dark:text-pink-300',
  '88api-video': 'text-emerald-700 dark:text-emerald-300',
  'minimax-speech': 'text-violet-700 dark:text-violet-300',
  'qwen-tts': 'text-amber-700 dark:text-amber-300',
  'pixverse-v6': 'text-lime-700 dark:text-lime-300',
  'grok-imagine-video': 'text-cyan-700 dark:text-cyan-300',
  kuaishou: 'text-sky-700 dark:text-sky-300',
}
const TEXT_DEFAULT = 'text-primary-600 dark:text-primary-400'

// ── Icon (check mark etc.) ──────────────────────────────────────────
const ICON: Record<Platform, string> = {
  anthropic: 'text-orange-500 dark:text-orange-400',
  openai: 'text-emerald-500 dark:text-emerald-400',
  antigravity: 'text-purple-500 dark:text-purple-400',
  gemini: 'text-blue-500 dark:text-blue-400',
  grok: 'text-zinc-800 dark:text-zinc-200',
  jimeng: 'text-rose-500 dark:text-rose-400',
  doubao: 'text-cyan-500 dark:text-cyan-300',
  qwen: 'text-amber-500 dark:text-amber-300',
  kimi: 'text-indigo-500 dark:text-indigo-300',
  deepseek: 'text-slate-600 dark:text-slate-200',
  midjourney: 'text-pink-500 dark:text-pink-300',
  kling: 'text-sky-500 dark:text-sky-300',
  happyhourse: 'text-emerald-500 dark:text-emerald-300',
  seedance: 'text-teal-500 dark:text-teal-300',
  'ctmo-seedance': 'text-teal-500 dark:text-teal-300',
	'xingapi-seedance': 'text-amber-500 dark:text-amber-300',
	'jijiu-seedance': 'text-orange-500 dark:text-orange-300',
	'miaowu-seedance': 'text-lime-500 dark:text-lime-300',
	'paisio-seedance': 'text-blue-500 dark:text-blue-300',
	'apophis-seedance': 'text-yellow-500 dark:text-yellow-300',
	'xsn-seedance': 'text-red-500 dark:text-red-300',
	'fengyue-seedance': 'text-pink-500 dark:text-pink-300',
	'hongpei-seedance': 'text-violet-500 dark:text-violet-300',
	'vshi-seedance': 'text-teal-500 dark:text-teal-300',
	'huahuo-seedance': 'text-fuchsia-500 dark:text-fuchsia-300',
	'zexi-seedance': 'text-stone-500 dark:text-stone-300',
	'xm-seedance': 'text-sky-500 dark:text-sky-300',
	'naiwa-seedance': 'text-emerald-500 dark:text-emerald-300',
	'v6k-seedance': 'text-indigo-500 dark:text-indigo-300',
	'kyy-wan': 'text-indigo-500 dark:text-indigo-300',
  guangying: 'text-rose-500 dark:text-rose-300',
  opone: 'text-indigo-500 dark:text-indigo-300',
  'mai-token-seedance': 'text-cyan-500 dark:text-cyan-300',
  'seedance-hub': 'text-purple-500 dark:text-purple-300',
  bytedance: 'text-violet-500 dark:text-violet-300',
  wan3: 'text-fuchsia-500 dark:text-fuchsia-300',
  'minimax-h3': 'text-pink-500 dark:text-pink-300',
  'minimax-h3-compshare': 'text-pink-500 dark:text-pink-300',
  '88api-video': 'text-emerald-500 dark:text-emerald-300',
  'minimax-speech': 'text-violet-500 dark:text-violet-300',
  'qwen-tts': 'text-amber-500 dark:text-amber-300',
  'pixverse-v6': 'text-lime-500 dark:text-lime-300',
  'grok-imagine-video': 'text-cyan-500 dark:text-cyan-300',
  kuaishou: 'text-sky-500 dark:text-sky-300',
}
const ICON_DEFAULT = 'text-primary-500 dark:text-primary-400'

// ── Button (solid bg) ───────────────────────────────────────────────
const BUTTON: Record<Platform, string> = {
  anthropic: 'bg-orange-500 text-white hover:bg-orange-600 active:bg-orange-700 dark:bg-orange-500/80 dark:hover:bg-orange-500',
  openai: 'bg-green-600 text-white hover:bg-green-700 active:bg-green-800 dark:bg-green-600/80 dark:hover:bg-green-600',
  antigravity: 'bg-purple-500 text-white hover:bg-purple-600 active:bg-purple-700 dark:bg-purple-500/80 dark:hover:bg-purple-500',
  gemini: 'bg-blue-500 text-white hover:bg-blue-600 active:bg-blue-700 dark:bg-blue-500/80 dark:hover:bg-blue-500',
  grok: 'bg-zinc-800 text-white hover:bg-zinc-900 active:bg-black dark:bg-zinc-700 dark:hover:bg-zinc-600',
  jimeng: 'bg-rose-500 text-white hover:bg-rose-600 active:bg-rose-700 dark:bg-rose-500/80 dark:hover:bg-rose-500',
  doubao: 'bg-cyan-600 text-white hover:bg-cyan-700 active:bg-cyan-800 dark:bg-cyan-600/80 dark:hover:bg-cyan-600',
  qwen: 'bg-amber-500 text-white hover:bg-amber-600 active:bg-amber-700 dark:bg-amber-500/80 dark:hover:bg-amber-500',
  kimi: 'bg-indigo-600 text-white hover:bg-indigo-700 active:bg-indigo-800 dark:bg-indigo-600/80 dark:hover:bg-indigo-600',
  deepseek: 'bg-slate-700 text-white hover:bg-slate-800 active:bg-slate-900 dark:bg-slate-600 dark:hover:bg-slate-500',
  midjourney: 'bg-pink-600 text-white hover:bg-pink-700 active:bg-pink-800 dark:bg-pink-600/80 dark:hover:bg-pink-600',
  kling: 'bg-sky-600 text-white hover:bg-sky-700 active:bg-sky-800 dark:bg-sky-600/80 dark:hover:bg-sky-600',
  happyhourse: 'bg-emerald-600 text-white hover:bg-emerald-700 active:bg-emerald-800 dark:bg-emerald-600/80 dark:hover:bg-emerald-600',
  seedance: 'bg-teal-600 text-white hover:bg-teal-700 active:bg-teal-800 dark:bg-teal-600/80 dark:hover:bg-teal-600',
  'ctmo-seedance': 'bg-teal-600 text-white hover:bg-teal-700 active:bg-teal-800 dark:bg-teal-600/80 dark:hover:bg-teal-600',
	'xingapi-seedance': 'bg-amber-600 text-white hover:bg-amber-700 active:bg-amber-800 dark:bg-amber-600/80 dark:hover:bg-amber-600',
	'jijiu-seedance': 'bg-orange-600 text-white hover:bg-orange-700 active:bg-orange-800 dark:bg-orange-600/80 dark:hover:bg-orange-600',
	'miaowu-seedance': 'bg-lime-600 text-white hover:bg-lime-700 active:bg-lime-800 dark:bg-lime-600/80 dark:hover:bg-lime-600',
	'paisio-seedance': 'bg-blue-600 text-white hover:bg-blue-700 active:bg-blue-800 dark:bg-blue-600/80 dark:hover:bg-blue-600',
	'apophis-seedance': 'bg-yellow-600 text-white hover:bg-yellow-700 active:bg-yellow-800 dark:bg-yellow-600/80 dark:hover:bg-yellow-600',
	'xsn-seedance': 'bg-red-600 text-white hover:bg-red-700 active:bg-red-800 dark:bg-red-600/80 dark:hover:bg-red-600',
	'fengyue-seedance': 'bg-pink-600 text-white hover:bg-pink-700 active:bg-pink-800 dark:bg-pink-600/80 dark:hover:bg-pink-600',
	'hongpei-seedance': 'bg-violet-600 text-white hover:bg-violet-700 active:bg-violet-800 dark:bg-violet-600/80 dark:hover:bg-violet-600',
	'vshi-seedance': 'bg-teal-600 text-white hover:bg-teal-700 active:bg-teal-800 dark:bg-teal-600/80 dark:hover:bg-teal-600',
	'huahuo-seedance': 'bg-fuchsia-600 text-white hover:bg-fuchsia-700 active:bg-fuchsia-800 dark:bg-fuchsia-600/80 dark:hover:bg-fuchsia-600',
	'zexi-seedance': 'bg-stone-600 text-white hover:bg-stone-700 active:bg-stone-800 dark:bg-stone-600/80 dark:hover:bg-stone-600',
	'xm-seedance': 'bg-sky-600 text-white hover:bg-sky-700 active:bg-sky-800 dark:bg-sky-600/80 dark:hover:bg-sky-600',
	'naiwa-seedance': 'bg-emerald-600 text-white hover:bg-emerald-700 active:bg-emerald-800 dark:bg-emerald-600/80 dark:hover:bg-emerald-600',
	'v6k-seedance': 'bg-indigo-600 text-white hover:bg-indigo-700 active:bg-indigo-800 dark:bg-indigo-600/80 dark:hover:bg-indigo-600',
	'kyy-wan': 'bg-indigo-600 text-white hover:bg-indigo-700 active:bg-indigo-800 dark:bg-indigo-600/80 dark:hover:bg-indigo-600',
  guangying: 'bg-rose-600 text-white hover:bg-rose-700 active:bg-rose-800 dark:bg-rose-600/80 dark:hover:bg-rose-600',
  opone: 'bg-indigo-600 text-white hover:bg-indigo-700 active:bg-indigo-800 dark:bg-indigo-600/80 dark:hover:bg-indigo-600',
  'mai-token-seedance': 'bg-cyan-600 text-white hover:bg-cyan-700 active:bg-cyan-800 dark:bg-cyan-600/80 dark:hover:bg-cyan-600',
  'seedance-hub': 'bg-purple-600 text-white hover:bg-purple-700 active:bg-purple-800 dark:bg-purple-600/80 dark:hover:bg-purple-600',
  bytedance: 'bg-violet-600 text-white hover:bg-violet-700 active:bg-violet-800 dark:bg-violet-600/80 dark:hover:bg-violet-600',
  wan3: 'bg-fuchsia-600 text-white hover:bg-fuchsia-700 active:bg-fuchsia-800 dark:bg-fuchsia-600/80 dark:hover:bg-fuchsia-600',
  'minimax-h3': 'bg-pink-600 text-white hover:bg-pink-700 active:bg-pink-800 dark:bg-pink-600/80 dark:hover:bg-pink-600',
  'minimax-h3-compshare': 'bg-pink-600 text-white hover:bg-pink-700 active:bg-pink-800 dark:bg-pink-600/80 dark:hover:bg-pink-600',
  '88api-video': 'bg-emerald-600 text-white hover:bg-emerald-700 active:bg-emerald-800 dark:bg-emerald-600/80 dark:hover:bg-emerald-600',
  'minimax-speech': 'bg-violet-600 text-white hover:bg-violet-700 active:bg-violet-800 dark:bg-violet-600/80 dark:hover:bg-violet-600',
  'qwen-tts': 'bg-amber-600 text-white hover:bg-amber-700 active:bg-amber-800 dark:bg-amber-600/80 dark:hover:bg-amber-600',
  'pixverse-v6': 'bg-lime-600 text-white hover:bg-lime-700 active:bg-lime-800 dark:bg-lime-600/80 dark:hover:bg-lime-600',
  'grok-imagine-video': 'bg-cyan-600 text-white hover:bg-cyan-700 active:bg-cyan-800 dark:bg-cyan-600/80 dark:hover:bg-cyan-600',
  kuaishou: 'bg-sky-600 text-white hover:bg-sky-700 active:bg-sky-800 dark:bg-sky-600/80 dark:hover:bg-sky-600',
}
const BUTTON_DEFAULT = 'bg-primary-500 text-white hover:bg-primary-600 dark:bg-primary-600 dark:hover:bg-primary-500'

// ── Discount badge ──────────────────────────────────────────────────
const DISCOUNT: Record<Platform, string> = {
  anthropic: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
  openai: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
  antigravity: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
  gemini: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
  grok: 'bg-zinc-100 text-zinc-800 dark:bg-zinc-800 dark:text-zinc-200',
  jimeng: 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300',
  doubao: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-300',
  qwen: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
  kimi: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300',
  deepseek: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200',
  midjourney: 'bg-pink-100 text-pink-700 dark:bg-pink-900/40 dark:text-pink-300',
  kling: 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300',
  happyhourse: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
  seedance: 'bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-300',
  'ctmo-seedance': 'bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-300',
	'xingapi-seedance': 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
	'jijiu-seedance': 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
	'miaowu-seedance': 'bg-lime-100 text-lime-700 dark:bg-lime-900/40 dark:text-lime-300',
	'paisio-seedance': 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
	'apophis-seedance': 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
	'xsn-seedance': 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
	'fengyue-seedance': 'bg-pink-100 text-pink-700 dark:bg-pink-900/40 dark:text-pink-300',
	'hongpei-seedance': 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300',
	'vshi-seedance': 'bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-300',
	'huahuo-seedance': 'bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900/40 dark:text-fuchsia-300',
	'zexi-seedance': 'bg-stone-100 text-stone-700 dark:bg-stone-900/40 dark:text-stone-300',
	'xm-seedance': 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300',
	'naiwa-seedance': 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
	'v6k-seedance': 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300',
	'kyy-wan': 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300',
  guangying: 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300',
  opone: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300',
  'mai-token-seedance': 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-300',
  'seedance-hub': 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
  bytedance: 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300',
  wan3: 'bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900/40 dark:text-fuchsia-300',
  'minimax-h3': 'bg-pink-100 text-pink-700 dark:bg-pink-900/40 dark:text-pink-300',
  'minimax-h3-compshare': 'bg-pink-100 text-pink-700 dark:bg-pink-900/40 dark:text-pink-300',
  '88api-video': 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
  'minimax-speech': 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300',
  'qwen-tts': 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
  'pixverse-v6': 'bg-lime-100 text-lime-700 dark:bg-lime-900/40 dark:text-lime-300',
  'grok-imagine-video': 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-300',
  kuaishou: 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300',
}
const DISCOUNT_DEFAULT = 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'

// ── Header gradient (subscription confirm) ─────────────────────────
const GRADIENT: Record<Platform, string> = {
  anthropic: 'from-orange-500 to-orange-600',
  openai: 'from-emerald-500 to-emerald-600',
  antigravity: 'from-purple-500 to-purple-600',
  gemini: 'from-blue-500 to-blue-600',
  grok: 'from-zinc-700 to-zinc-900',
  jimeng: 'from-rose-500 to-pink-600',
  doubao: 'from-cyan-500 to-cyan-600',
  qwen: 'from-amber-500 to-orange-600',
  kimi: 'from-indigo-500 to-indigo-600',
  deepseek: 'from-slate-700 to-slate-900',
  midjourney: 'from-pink-500 to-fuchsia-600',
  kling: 'from-sky-500 to-sky-600',
  happyhourse: 'from-emerald-500 to-emerald-600',
  seedance: 'from-teal-500 to-teal-600',
  'ctmo-seedance': 'from-teal-500 to-teal-600',
	'xingapi-seedance': 'from-amber-500 to-amber-600',
	'jijiu-seedance': 'from-orange-500 to-orange-600',
	'miaowu-seedance': 'from-lime-500 to-lime-600',
	'paisio-seedance': 'from-blue-500 to-blue-600',
	'apophis-seedance': 'from-yellow-500 to-yellow-600',
	'xsn-seedance': 'from-red-500 to-red-600',
	'fengyue-seedance': 'from-pink-500 to-pink-600',
	'hongpei-seedance': 'from-violet-500 to-violet-600',
	'vshi-seedance': 'from-teal-500 to-teal-600',
	'huahuo-seedance': 'from-fuchsia-500 to-fuchsia-600',
	'zexi-seedance': 'from-stone-500 to-stone-600',
	'xm-seedance': 'from-sky-500 to-sky-600',
	'naiwa-seedance': 'from-emerald-500 to-emerald-600',
	'v6k-seedance': 'from-indigo-500 to-indigo-600',
	'kyy-wan': 'from-indigo-500 to-indigo-600',
  guangying: 'from-rose-500 to-rose-600',
  opone: 'from-indigo-500 to-indigo-600',
  'mai-token-seedance': 'from-cyan-500 to-cyan-600',
  'seedance-hub': 'from-purple-500 to-purple-600',
  bytedance: 'from-violet-500 to-violet-600',
  wan3: 'from-fuchsia-500 to-fuchsia-600',
  'minimax-h3': 'from-pink-500 to-pink-600',
  'minimax-h3-compshare': 'from-pink-500 to-pink-600',
  '88api-video': 'from-emerald-500 to-emerald-600',
  'minimax-speech': 'from-violet-500 to-violet-600',
  'qwen-tts': 'from-amber-500 to-orange-600',
  'pixverse-v6': 'from-lime-500 to-lime-600',
  'grok-imagine-video': 'from-cyan-500 to-cyan-600',
  kuaishou: 'from-sky-500 to-sky-600',
}
const GRADIENT_DEFAULT = 'from-primary-500 to-primary-600'

// ── Header text (light text on gradient bg) ────────────────────────
const GRADIENT_TEXT: Record<Platform, string> = {
  anthropic: 'text-orange-100',
  openai: 'text-emerald-100',
  antigravity: 'text-purple-100',
  gemini: 'text-blue-100',
  grok: 'text-zinc-100',
  jimeng: 'text-rose-100',
  doubao: 'text-cyan-100',
  qwen: 'text-amber-100',
  kimi: 'text-indigo-100',
  deepseek: 'text-slate-100',
  midjourney: 'text-pink-100',
  kling: 'text-sky-100',
  happyhourse: 'text-emerald-100',
  seedance: 'text-teal-100',
  'ctmo-seedance': 'text-teal-100',
	'xingapi-seedance': 'text-amber-100',
	'jijiu-seedance': 'text-orange-100',
	'miaowu-seedance': 'text-lime-100',
	'paisio-seedance': 'text-blue-100',
	'apophis-seedance': 'text-yellow-100',
	'xsn-seedance': 'text-red-100',
	'fengyue-seedance': 'text-pink-100',
	'hongpei-seedance': 'text-violet-100',
	'vshi-seedance': 'text-teal-100',
	'huahuo-seedance': 'text-fuchsia-100',
	'zexi-seedance': 'text-stone-100',
	'xm-seedance': 'text-sky-100',
	'naiwa-seedance': 'text-emerald-100',
	'v6k-seedance': 'text-indigo-100',
	'kyy-wan': 'text-indigo-100',
  guangying: 'text-rose-100',
  opone: 'text-indigo-100',
  'mai-token-seedance': 'text-cyan-100',
  'seedance-hub': 'text-purple-100',
  bytedance: 'text-violet-100',
  wan3: 'text-fuchsia-100',
  'minimax-h3': 'text-pink-100',
  'minimax-h3-compshare': 'text-pink-100',
  '88api-video': 'text-emerald-100',
  'minimax-speech': 'text-violet-100',
  'qwen-tts': 'text-amber-100',
  'pixverse-v6': 'text-lime-100',
  'grok-imagine-video': 'text-cyan-100',
  kuaishou: 'text-sky-100',
}
const GRADIENT_TEXT_DEFAULT = 'text-primary-100'

const GRADIENT_SUBTEXT: Record<Platform, string> = {
  anthropic: 'text-orange-200',
  openai: 'text-emerald-200',
  antigravity: 'text-purple-200',
  gemini: 'text-blue-200',
  grok: 'text-zinc-300',
  jimeng: 'text-rose-200',
  doubao: 'text-cyan-200',
  qwen: 'text-amber-200',
  kimi: 'text-indigo-200',
  deepseek: 'text-slate-300',
  midjourney: 'text-pink-200',
  kling: 'text-sky-200',
  happyhourse: 'text-emerald-200',
  seedance: 'text-teal-200',
  'ctmo-seedance': 'text-teal-200',
	'xingapi-seedance': 'text-amber-200',
	'jijiu-seedance': 'text-orange-200',
	'miaowu-seedance': 'text-lime-200',
	'paisio-seedance': 'text-blue-200',
	'apophis-seedance': 'text-yellow-200',
	'xsn-seedance': 'text-red-200',
	'fengyue-seedance': 'text-pink-200',
	'hongpei-seedance': 'text-violet-200',
	'vshi-seedance': 'text-teal-200',
	'huahuo-seedance': 'text-fuchsia-200',
	'zexi-seedance': 'text-stone-200',
	'xm-seedance': 'text-sky-200',
	'naiwa-seedance': 'text-emerald-200',
	'v6k-seedance': 'text-indigo-200',
	'kyy-wan': 'text-indigo-200',
  guangying: 'text-rose-200',
  opone: 'text-indigo-200',
  'mai-token-seedance': 'text-cyan-200',
  'seedance-hub': 'text-purple-200',
  bytedance: 'text-violet-200',
  wan3: 'text-fuchsia-200',
  'minimax-h3': 'text-pink-200',
  'minimax-h3-compshare': 'text-pink-200',
  '88api-video': 'text-emerald-200',
  'minimax-speech': 'text-violet-200',
  'qwen-tts': 'text-amber-200',
  'pixverse-v6': 'text-lime-200',
  'grok-imagine-video': 'text-cyan-200',
  kuaishou: 'text-sky-200',
}
const GRADIENT_SUBTEXT_DEFAULT = 'text-primary-200'

// ── Public API ──────────────────────────────────────────────────────

function isPlatform(p: string): p is Platform {
	return p === 'anthropic' || p === 'openai' || p === 'antigravity' || p === 'gemini' || p === 'grok' || p === 'jimeng' || p === 'doubao' || p === 'qwen' || p === 'kimi' || p === 'deepseek' || p === 'midjourney' || p === 'kling' || p === 'happyhourse' || p === 'seedance' || p === 'ctmo-seedance' || p === 'xingapi-seedance' || p === 'jijiu-seedance' || p === 'miaowu-seedance' || p === 'paisio-seedance' || p === 'apophis-seedance' || p === 'xsn-seedance' || p === 'fengyue-seedance' || p === 'hongpei-seedance' || p === 'vshi-seedance' || p === 'huahuo-seedance' || p === 'zexi-seedance' || p === 'xm-seedance' || p === 'naiwa-seedance' || p === 'v6k-seedance' || p === 'guangying' || p === 'opone' || p === 'mai-token-seedance' || p === 'seedance-hub' || p === 'bytedance' || p === 'wan3' || p === 'kyy-wan' || p === 'minimax-h3' || p === 'minimax-h3-compshare' || p === '88api-video' || p === 'minimax-speech' || p === 'qwen-tts' || p === 'pixverse-v6' || p === 'grok-imagine-video' || p === 'kuaishou'
}

export function platformBadgeClass(p: string): string {
  return isPlatform(p) ? BADGE[p] : BADGE_DEFAULT
}

export function platformBadgeLightClass(p: string): string {
  return isPlatform(p) ? BADGE_LIGHT[p] : BADGE_DEFAULT
}

export function platformBorderClass(p: string): string {
  return isPlatform(p) ? BORDER[p] : BORDER_DEFAULT
}

export function platformAccentBarClass(p: string): string {
  return isPlatform(p) ? ACCENT_BAR[p] : ACCENT_BAR_DEFAULT
}

export function platformTextClass(p: string): string {
  return isPlatform(p) ? TEXT[p] : TEXT_DEFAULT
}

export function platformIconClass(p: string): string {
  return isPlatform(p) ? ICON[p] : ICON_DEFAULT
}

export function platformButtonClass(p: string): string {
  return isPlatform(p) ? BUTTON[p] : BUTTON_DEFAULT
}

export function platformDiscountClass(p: string): string {
  return isPlatform(p) ? DISCOUNT[p] : DISCOUNT_DEFAULT
}

export function platformGradientClass(p: string): string {
  return isPlatform(p) ? GRADIENT[p] : GRADIENT_DEFAULT
}

export function platformGradientTextClass(p: string): string {
  return isPlatform(p) ? GRADIENT_TEXT[p] : GRADIENT_TEXT_DEFAULT
}

export function platformGradientSubtextClass(p: string): string {
  return isPlatform(p) ? GRADIENT_SUBTEXT[p] : GRADIENT_SUBTEXT_DEFAULT
}

export function platformLabel(p: string): string {
  switch (p) {
    case 'anthropic': return 'Anthropic'
    case 'openai': return 'OpenAI'
    case 'antigravity': return 'Antigravity'
    case 'gemini': return 'Gemini'
    case 'grok': return 'Grok'
    case 'jimeng': return '即梦'
    case 'doubao': return '豆包'
    case 'qwen': return '千问'
    case 'kimi': return 'Kimi'
    case 'deepseek': return 'DeepSeek'
    case 'midjourney': return 'Midjourney'
    case 'kling': return 'K-Ling'
    case 'happyhourse': return 'Happy-Hourse'
    case 'seedance': return 'Seedance'
    case 'ctmo-seedance': return '浙创 OPC Seedance'
	case 'xingapi-seedance': return '星光 API Seedance'
	case 'jijiu-seedance': return '极九 API Seedance'
	case 'miaowu-seedance': return '喵呜 API Seedance'
	case 'paisio-seedance': return 'Paisio Seedance'
	case 'apophis-seedance': return 'Apophis Seedance'
	case 'xsn-seedance': return 'XSN Seedance'
	case 'fengyue-seedance': return '风月 Seedance'
	case 'hongpei-seedance': return '红佩 Seedance'
	case 'vshi-seedance': return 'VSHI Seedance'
	case 'huahuo-seedance': return 'HUAHUO Seedance'
	case 'zexi-seedance': return '泽西 Seedance'
	case 'xm-seedance': return 'XM Seedance'
	case 'naiwa-seedance': return '奶蛙 Seedance'
	case 'v6k-seedance': return 'V6K Seedance'
    case 'guangying': return '光影 AI Seedance'
    case 'opone': return 'OPONE Seedance'
    case 'mai-token-seedance': return 'Mai Token Seedance'
    case 'seedance-hub': return 'Seedance 综合'
    case 'bytedance': return 'ByteDance'
    case 'wan3': return 'Wan3.0'
    case 'kyy-wan': return '客易云 Wan3.0'
    case 'minimax-h3': return 'MiniMax-H3'
    case '88api-video': return '88API Video'
    case 'minimax-h3-compshare': return 'MiniMax-H3（优云智算）'
    case 'minimax-speech': return 'MiniMax-Speech'
    case 'qwen-tts': return 'Qwen TTS'
    case 'pixverse-v6': return 'Pixverse-V6'
    case 'grok-imagine-video': return 'Grok Imagine Video'
    case 'kuaishou': return 'Kuaishou'
    default: return p || 'API'
  }
}
