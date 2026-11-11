import { describe, expect, it } from 'vitest'
import { renderApiDoc, type RenderApiDocOptions } from '../apiDocsRender'
import { findApiDocPage } from '../apiDocsCatalog'

const options: RenderApiDocOptions = {
  currentPath: 'PP_VIDEO_GENERATION_API.md',
  resolveSlug: (_current, href) => (href.startsWith('video-api/wan.md') ? 'wan' : null),
  routeFor: (slug) => `/api-docs/${slug}`,
  copyLabel: '复制',
}

function parse(html: string) {
  const root = document.createElement('div')
  root.innerHTML = html
  return root
}

describe('renderApiDoc', () => {
	 it('renders the complete Seedance2.5-480全参 public workflow', () => {
	   const page = findApiDocPage('seedance-series')
	   expect(page).toBeDefined()
	   const { html } = renderApiDoc(page!.markdown, { ...options, currentPath: page!.path })
	   const text = parse(html).textContent ?? ''

	   expect(text).toContain('Seedance2.5-480全参')
	   expect(text).toContain('4-30')
	   expect(text).toContain('固定 480P')
	   expect(text).toContain('JSON')
	   expect(text).toContain('video-v1')
	   expect(text).toContain('multipart/form-data')
	   expect(text).toContain('最多 30 张')
	   expect(text).toContain('billing_pending')
	   expect(text).toContain('GET /v1/videos/{task_id}')
	   expect(text).toContain('原 API Key')
	 })

  it('gives h2/h3 headings stable ids and returns them for the table of contents', () => {
    const { html, headings } = renderApiDoc('# Title\n\n## One\n\ntext\n\n### Two\n\n## Three', options)

    expect(headings).toEqual([
      { id: 'api-doc-h1', text: 'One', level: 2 },
      { id: 'api-doc-h2', text: 'Two', level: 3 },
      { id: 'api-doc-h3', text: 'Three', level: 2 },
    ])
    const root = parse(html)
    expect(root.querySelector('h1')?.getAttribute('id')).toBeNull()
    expect(root.querySelector('#api-doc-h2')?.textContent).toBe('Two')
  })

  it('turns links to other docs into in-site routes and drops dead ones', () => {
    const { html } = renderApiDoc('[wan](video-api/wan.md) [gone](video-api/gone.md#x) [plain](https://example.com/a)', options)
    const anchors = [...parse(html).querySelectorAll('a')]

    expect(anchors[0].getAttribute('href')).toBe('/api-docs/wan')
    expect(anchors[0].getAttribute('data-api-doc-slug')).toBe('wan')
    expect(anchors[1].hasAttribute('href')).toBe(false)
    expect(anchors[2].getAttribute('target')).toBe('_blank')
    expect(anchors[2].getAttribute('rel')).toBe('noopener noreferrer')
  })

  it('wraps code blocks with a copy button and tables with a scroll container', () => {
    const { html } = renderApiDoc('```bash\ncurl https://x\n```\n\n| a | b |\n| --- | --- |\n| 1 | 2 |', options)
    const root = parse(html)

    const button = root.querySelector('.api-doc-code > button[data-api-doc-copy]')
    expect(button?.textContent).toBe('复制')
    expect(root.querySelector('.api-doc-code > pre')?.textContent).toContain('curl https://x')
    expect(root.querySelector('.api-doc-table > table')).not.toBeNull()
  })

  it('keeps single newlines inside a paragraph as plain text, not <br>', () => {
    const { html } = renderApiDoc('first line\nsecond line', options)

    expect(html).not.toContain('<br')
  })

  it('neutralises scripts, event handlers and javascript: links', () => {
    const { html } = renderApiDoc(
      '<script>alert(1)</script>\n\n<img src=x onerror="alert(1)">\n\n[x](javascript:alert(1))',
      options,
    )

    expect(html).not.toMatch(/<script|onerror|javascript:/i)
  })
})
