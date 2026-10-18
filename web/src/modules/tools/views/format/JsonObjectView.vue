<script setup lang="ts">
/**
 * JSON 对象树。键和值分行展开，数组用序号。
 */
import { computed, ref } from 'vue'

defineOptions({ name: 'JsonObjectView' })

const props = defineProps<{
  data: unknown
  label?: string
}>()

const open = ref(true)
const kind = computed(() => {
  if (props.data === null) return 'null'
  if (Array.isArray(props.data)) return 'array'
  return typeof props.data
})
const entries = computed((): { key: string; value: unknown }[] => {
  if (Array.isArray(props.data)) {
    return props.data.map((value, index) => ({ key: String(index), value }))
  }
  if (kind.value === 'object' && props.data) {
    return Object.keys(props.data as Record<string, unknown>).map((key) => ({
      key,
      value: (props.data as Record<string, unknown>)[key],
    }))
  }
  return []
})
const summary = computed(() => {
  if (kind.value === 'array') return `[${(props.data as unknown[]).length}]`
  if (kind.value === 'object') return `{${entries.value.length}}`
  return ''
})
const text = computed(() => {
  if (typeof props.data === 'string') return JSON.stringify(props.data)
  if (props.data === undefined) return ''
  return String(props.data)
})
</script>

<template>
  <div class="nm-json">
    <button v-if="kind === 'object' || kind === 'array'" type="button" class="nm-json__row" @click="open = !open">
      <span class="nm-json__mark">{{ open ? '▾' : '▸' }}</span>
      <span v-if="label !== undefined" class="nm-json__key">{{ label }}</span>
      <span class="nm-json__meta">{{ summary }}</span>
    </button>
    <div v-else class="nm-json__row">
      <span class="nm-json__mark" />
      <span v-if="label !== undefined" class="nm-json__key">{{ label }}</span>
      <span class="nm-json__value" :class="`nm-json__value--${kind}`">{{ text }}</span>
    </div>
    <div v-if="open && (kind === 'object' || kind === 'array')" class="nm-json__kids">
      <JsonObjectView v-for="item in entries" :key="item.key" :label="item.key" :data="item.value" />
    </div>
  </div>
</template>

<style scoped>
.nm-json__row {
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

div.nm-json__row {
  cursor: default;
}

.nm-json__mark {
  flex: 0 0 0.8rem;
  color: var(--rs-muted);
}

.nm-json__key {
  color: var(--rs-primary);
}

.nm-json__key::after {
  content: ':';
  color: var(--rs-muted);
}

.nm-json__meta {
  color: var(--rs-muted);
}

.nm-json__value--string {
  color: var(--rs-success, #1a7f37);
}

.nm-json__value--number {
  color: var(--rs-danger);
}

.nm-json__value--boolean,
.nm-json__value--null {
  color: var(--rs-warning, #9a6700);
}

.nm-json__kids {
  padding-left: 0.85rem;
}
</style>
