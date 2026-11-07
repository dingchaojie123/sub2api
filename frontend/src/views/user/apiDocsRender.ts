import { marked } from 'marked'
import DOMPurify from 'dompurify'

export interface ApiDocHeading {
  id: string
  text: string
  level: 2 | 3
}

export interface RenderedApiDoc {
  html: string
  headings: ApiDocHeading[]
}

export interface RenderApiDocOptions {
  /** 当前文档相对 docs/ 的路径，用来解析相对链接 */
  currentPath: string
  /** 相对 .md 链接 -> 页面 slug，解析不到返回 null */
  resolveSlug: (currentPath: string, href: string) => string | null
  /** 页面 slug -> 站内路由 */
  routeFor: (slug: string) => string
  /** 代码块复制按钮上的文字 */
  copyLabel: string
}

const EXTERNAL_LINK = /^(https?:)?\/\//i

/**
 * 把 Markdown 渲染成可直接 v-html 的安全 HTML：
 *  1. marked 解析 -> DOMPurify 清洗（先清洗，再做下面的 DOM 加工，加工用的属性不会被清洗掉）；
 *  2. 二、三级标题加稳定 id，供「本页目录」跳转；
 *  3. 指向其他文档的相对 .md 链接改成站内路由；解析不到的去掉 href，不留死链；外链新窗口打开；
 *  4. 每个代码块加「复制」按钮，每张表格包一层以便小屏横向滚动。
 */
export function renderApiDoc(markdown: string, options: RenderApiDocOptions): RenderedApiDoc {
  const parsed = marked.parse(markdown, { gfm: true, breaks: false, async: false }) as string
  const clean = DOMPurify.sanitize(parsed)

  const template = document.createElement('template')
  template.innerHTML = clean
  const root = template.content

  const headings: ApiDocHeading[] = []
  root.querySelectorAll('h2, h3').forEach((el, index) => {
    const id = `api-doc-h${index + 1}`
    el.setAttribute('id', id)
    headings.push({ id, text: (el.textContent ?? '').trim(), level: el.tagName === 'H2' ? 2 : 3 })
  })

  root.querySelectorAll('a[href]').forEach((anchor) => {
    const href = anchor.getAttribute('href') ?? ''
    if (EXTERNAL_LINK.test(href)) {
      anchor.setAttribute('target', '_blank')
      anchor.setAttribute('rel', 'noopener noreferrer')
      return
    }
    if (/\.md(#.*)?$/i.test(href)) {
      const slug = options.resolveSlug(options.currentPath, href)
      if (slug) {
        anchor.setAttribute('href', options.routeFor(slug))
        anchor.setAttribute('data-api-doc-slug', slug)
      } else {
        anchor.removeAttribute('href')
      }
    }
  })

  root.querySelectorAll('pre').forEach((pre) => {
    const wrapper = document.createElement('div')
    wrapper.className = 'api-doc-code'
    const button = document.createElement('button')
    button.type = 'button'
    button.className = 'api-doc-copy'
    button.setAttribute('data-api-doc-copy', '')
    button.textContent = options.copyLabel
    pre.parentNode?.insertBefore(wrapper, pre)
    wrapper.append(button, pre)
  })

  root.querySelectorAll('table').forEach((table) => {
    const wrapper = document.createElement('div')
    wrapper.className = 'api-doc-table'
    table.parentNode?.insertBefore(wrapper, table)
    wrapper.append(table)
  })

  return { html: template.innerHTML, headings }
}
