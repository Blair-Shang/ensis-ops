export interface XmlTreeNode {
  name: string
  attrs: { name: string; value: string }[]
  text: string
  children: XmlTreeNode[]
}

/** 把一段 XML 收成元素树。解析失败时返回 null。 */
export function parseXmlTree(source: string): XmlTreeNode | null {
  const doc = new DOMParser().parseFromString(source, 'application/xml')
  const root = doc.documentElement
  if (!root || root.nodeName === 'parsererror') return null
  return walkElement(root)
}

function walkElement(el: Element): XmlTreeNode {
  const attrs = [...el.attributes].map((attr) => ({ name: attr.name, value: attr.value }))
  const children: XmlTreeNode[] = []
  const texts: string[] = []
  for (const node of el.childNodes) {
    if (node.nodeType === Node.ELEMENT_NODE) {
      children.push(walkElement(node as Element))
      continue
    }
    if (node.nodeType !== Node.TEXT_NODE && node.nodeType !== Node.CDATA_SECTION_NODE) continue
    const text = node.textContent?.trim() ?? ''
    if (text) texts.push(text)
  }
  return { name: el.tagName, attrs, text: texts.join(' '), children }
}
