<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { EditorView, keymap, placeholder as cmPlaceholder, drawSelection } from '@codemirror/view'
import { EditorState, Compartment, Prec } from '@codemirror/state'
import { history, historyKeymap } from '@codemirror/commands'
import { editorKeymap } from '@/lib/cm-keymap'
import { autocompletion, completionKeymap } from '@codemirror/autocomplete'
import {
  variableHighlightPlugin,
  variableHighlightTheme,
  variableHoverTooltip,
  variableCompletionSource,
  varsChangedEffect,
} from '@/lib/codemirror-variables'
import { BRAND_ACCENT_SELECTION } from '@/constants/defaults'
import { isCurlCommand } from '@/lib/curl-paste'
import { externalReplace, isExternal } from '@/lib/cm-external'
import { singleLine } from '@/lib/cm-single-line'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  multiline?: boolean
  disabled?: boolean
  readonly?: boolean
  masked?: boolean
  variant?: 'default' | 'borderless'
  contentClass?: string
  // Assistive-technology state of the editable element; the wrapper div is not
  // the input, so a label or a message must point at .cm-content itself.
  invalid?: boolean
  describedBy?: string
  resolvedVariables?: Record<string, string>
  secretKeys?: Set<string>
  availableVariables?: Array<{ key: string; value: string; isSecret: boolean }>
  detectCurl?: boolean
}>(), {
  placeholder: '',
  multiline: false,
  disabled: false,
  readonly: false,
  masked: false,
  variant: 'default',
  contentClass: '',
  invalid: false,
  describedBy: '',
  detectCurl: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'submit'): void
  (e: 'focus'): void
  (e: 'blur'): void
  (e: 'paste-curl', text: string): void
}>()

const editorRef = ref<HTMLDivElement>()
let view: EditorView | null = null

const getVars = () => props.resolvedVariables ?? {}
const getSecrets = () => props.secretKeys ?? new Set<string>()
const getAvailableVars = () => props.availableVariables ?? []

const maskedCompartment = new Compartment()
const editableCompartment = new Compartment()
const contentAttrsCompartment = new Compartment()

function getContentAttrsExtension() {
  const attrs: Record<string, string> = {}
  if (props.contentClass) attrs.class = props.contentClass
  if (props.invalid) attrs['aria-invalid'] = 'true'
  if (props.describedBy) attrs['aria-describedby'] = props.describedBy

  return Object.keys(attrs).length > 0 ? EditorView.contentAttributes.of(attrs) : []
}

function getMaskedExtension() {
  return props.masked
    ? EditorView.contentAttributes.of({ style: '-webkit-text-security: disc' })
    : []
}

function getEditableExtension() {
  if (props.disabled) {
    return [EditorView.editable.of(false), EditorState.readOnly.of(true)]
  } else if (props.readonly) {
    return EditorState.readOnly.of(true)
  }
  return []
}

function createExtensions() {
  const extensions = [
    drawSelection(),
    history(),
    keymap.of([...completionKeymap, ...editorKeymap, ...historyKeymap]),
    variableHighlightPlugin(getVars),
    variableHoverTooltip(getVars, getSecrets),
    autocompletion({
      override: [variableCompletionSource(getVars, getSecrets, props.availableVariables ? getAvailableVars : undefined)],
    }),
    variableHighlightTheme,
    cmPlaceholder(props.placeholder),
    EditorView.updateListener.of((update) => {
      if (!update.docChanged) return
      const value = update.state.doc.toString()
      if (update.transactions.some(isExternal) && value === props.modelValue) return
      emit('update:modelValue', value)
    }),
    EditorView.domEventHandlers({
      focus: () => { emit('focus'); return false },
      blur: () => { emit('blur'); return false },
      // Read the clipboard before CodeMirror does: the single-line
      // transactionFilter below flattens newlines, and cURL needs them intact.
      paste: (event) => {
        if (!props.detectCurl) return false
        const text = event.clipboardData?.getData('text') ?? ''
        if (!isCurlCommand(text)) return false
        event.preventDefault()
        emit('paste-curl', text)
        return true
      },
    }),
    EditorView.theme({
      '&': {
        fontSize: '14px',
        fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
        backgroundColor: 'transparent',
      },
      '&.cm-focused': { outline: 'none' },
      '.cm-content': { cursor: 'text' },
      // The placeholder is a span inside .cm-content and would inherit the
      // masking: an empty secret field would show dots and read as filled.
      '.cm-placeholder': { WebkitTextSecurity: 'none' },
      '.cm-scroller': { cursor: 'text' },
      '.cm-cursor, .cm-dropCursor': {
        borderLeftColor: 'var(--foreground, #e4e4e7)',
      },
      '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
        backgroundColor: BRAND_ACCENT_SELECTION + ' !important',
      },
      '.cm-content ::selection': {
        backgroundColor: BRAND_ACCENT_SELECTION,
      },
    }),
    maskedCompartment.of(getMaskedExtension()),
    editableCompartment.of(getEditableExtension()),
    contentAttrsCompartment.of(getContentAttrsExtension()),
  ]

  if (!props.multiline) {
    extensions.push(singleLine)
    // Above defaultKeymap's insertNewline; an open completion list still takes Enter first.
    extensions.push(
      Prec.high(keymap.of([{
        key: 'Enter',
        run: () => { emit('submit'); return true },
      }])),
    )
    extensions.push(
      EditorView.theme({
        '.cm-scroller': {
          overflow: 'hidden',
          alignItems: 'center',
        },
        '.cm-content': {
          padding: '0 12px',
          whiteSpace: 'nowrap',
        },
        '.cm-line': {
          lineHeight: '2rem',
        },
      }),
    )
  } else {
    extensions.push(
      EditorView.lineWrapping,
      EditorView.theme({
        '.cm-content': { padding: '8px 12px' },
      }),
    )
  }

  return extensions
}

onMounted(() => {
  if (!editorRef.value) return

  view = new EditorView({
    state: EditorState.create({
      doc: props.modelValue,
      extensions: createExtensions(),
    }),
    parent: editorRef.value,
  })
})

onBeforeUnmount(() => {
  view?.destroy()
  view = null
})

watch(() => props.modelValue, (newVal) => {
  if (!view) return
  const spec = externalReplace(view.state, newVal)
  if (spec) view.dispatch(spec)
})

watch(() => props.resolvedVariables, () => {
  if (!view) return
  view.dispatch({ effects: varsChangedEffect.of(undefined) })
}, { deep: true })

watch(() => props.masked, () => {
  if (!view) return
  view.dispatch({ effects: maskedCompartment.reconfigure(getMaskedExtension()) })
})

watch([() => props.contentClass, () => props.invalid, () => props.describedBy], () => {
  if (!view) return
  view.dispatch({ effects: contentAttrsCompartment.reconfigure(getContentAttrsExtension()) })
})

watch([() => props.disabled, () => props.readonly], () => {
  if (!view) return
  view.dispatch({ effects: editableCompartment.reconfigure(getEditableExtension()) })
})

defineExpose({
  focus() {
    view?.focus()
  },
})
</script>

<template>
  <div
    ref="editorRef"
    class="variable-input"
    :class="[
      variant === 'default'
        ? 'rounded-md border border-border focus-within:ring-1 focus-within:ring-primary bg-background'
        : 'h-full',
      multiline ? 'min-h-[80px]' : 'h-8',
      disabled ? 'opacity-50 pointer-events-none' : '',
    ]"
  />
</template>
