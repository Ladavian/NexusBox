<template>
  <div ref="editorRef" class="cm-editor-wrapper"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { EditorView, keymap, lineNumbers, placeholder as cmPlaceholder, highlightActiveLine } from '@codemirror/view'
import { EditorState, type Extension } from '@codemirror/state'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { yaml } from '@codemirror/lang-yaml'
import { oneDark } from '@codemirror/theme-one-dark'
import { syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  readonly?: boolean
}>(), {
  placeholder: '',
  readonly: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const editorRef = ref<HTMLElement | null>(null)
let view: EditorView | null = null

onMounted(() => {
  if (!editorRef.value) return

  const extensions: Extension[] = [
    lineNumbers(),
    highlightActiveLine(),
    yaml(),
    oneDark,
    syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
    keymap.of([...defaultKeymap, ...historyKeymap]),
    history(),
    EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        const value = update.state.doc.toString()
        emit('update:modelValue', value)
      }
    }),
    EditorView.theme({
      '&': {
        fontSize: '13px',
        height: '100%',
      },
      '.cm-scroller': {
        fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'SF Mono', 'Menlo', monospace",
        lineHeight: '1.625',
      },
      '.cm-gutters': {
        borderRight: '1px solid #2d2d2d',
        backgroundColor: '#1e1e1e',
        color: '#858585',
      },
      '.cm-activeLineGutter': {
        backgroundColor: '#2a2d2e',
      },
      '.cm-activeLine': {
        backgroundColor: '#2a2d2e44',
      },
      '.cm-cursor': {
        borderLeftColor: '#aeafad',
      },
      '.cm-selectionBackground, ::selection': {
        backgroundColor: '#264f78 !important',
      },
    }),
    EditorState.tabSize.of(2),
  ]

  if (props.placeholder) {
    extensions.push(cmPlaceholder(props.placeholder))
  }

  if (props.readonly) {
    extensions.push(EditorState.readOnly.of(true))
    extensions.push(EditorView.editable.of(false))
  }

  const state = EditorState.create({
    doc: props.modelValue,
    extensions,
  })

  view = new EditorView({
    state,
    parent: editorRef.value,
  })
})

watch(() => props.modelValue, (newVal) => {
  if (view && newVal !== view.state.doc.toString()) {
    view.dispatch({
      changes: {
        from: 0,
        to: view.state.doc.length,
        insert: newVal,
      },
    })
  }
})

onUnmounted(() => {
  view?.destroy()
  view = null
})
</script>

<style scoped>
.cm-editor-wrapper {
  height: 100%;
  overflow: auto;
}
.cm-editor-wrapper :deep(.cm-editor) {
  height: 100%;
}
.cm-editor-wrapper :deep(.cm-scroller) {
  overflow: auto;
}
</style>
