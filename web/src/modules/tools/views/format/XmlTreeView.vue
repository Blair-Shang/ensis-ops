<script setup lang="ts">
/**
 * XML / SOAP 元素树。标签可展开，属性和文本跟在标签后面。
 */
import { computed, ref } from 'vue'
import type { XmlTreeNode } from './xml-tree'

defineOptions({ name: 'XmlTreeView' })

const props = defineProps<{
  node: XmlTreeNode
}>()

const open = ref(true)
const branch = computed(() => props.node.children.length > 0)
const attrText = computed(() => props.node.attrs.map((attr) => `${attr.name}="${attr.value}"`).join(' '))
</script>

<template>
  <div class="nm-xml">
    <button v-if="branch" type="button" class="nm-xml__row" @click="open = !open">
      <span class="nm-xml__mark">{{ open ? '▾' : '▸' }}</span>
      <span class="nm-xml__tag">{{ node.name }}</span>
      <span v-if="attrText" class="nm-xml__attr">{{ attrText }}</span>
      <span v-if="node.text && !open" class="nm-xml__text">{{ node.text }}</span>
    </button>
    <div v-else class="nm-xml__row">
      <span class="nm-xml__mark" />
      <span class="nm-xml__tag">{{ node.name }}</span>
      <span v-if="attrText" class="nm-xml__attr">{{ attrText }}</span>
      <span v-if="node.text" class="nm-xml__text">{{ node.text }}</span>
    </div>
    <div v-if="open && branch" class="nm-xml__kids">
      <p v-if="node.text" class="nm-xml__text nm-xml__text--block">{{ node.text }}</p>
      <XmlTreeView v-for="(child, index) in node.children" :key="`${child.name}-${index}`" :node="child" />
    </div>
  </div>
</template>

<style scoped>
.nm-xml__row {
  display: flex;
  align-items: baseline;
  gap: 0.35rem;
  width: 100%;
  min-height: 1.35rem;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--rs-text);
  font: inherit;
  font-family: var(--rs-font-mono);
  font-size: var(--nm-font-caption);
  line-height: 1.45;
  text-align: left;
  cursor: pointer;
}

div.nm-xml__row {
  cursor: default;
}

.nm-xml__mark {
  flex: 0 0 0.8rem;
  color: var(--rs-muted);
}

.nm-xml__tag {
  color: var(--rs-primary);
}

.nm-xml__attr {
  color: var(--rs-muted);
}

.nm-xml__text {
  color: var(--rs-success, #1a7f37);
}

.nm-xml__text--block {
  margin: 0 0 0 1.15rem;
}

.nm-xml__kids {
  padding-left: 0.85rem;
}
</style>
