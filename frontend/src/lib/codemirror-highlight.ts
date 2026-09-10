import { HighlightStyle } from '@codemirror/language'
import { tags } from '@lezer/highlight'

// One Dark Pro inspired — for the dark theme.
export const darkHighlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: '#c678dd' },
  { tag: tags.operator, color: '#56b6c2' },
  { tag: tags.string, color: '#98c379' },
  { tag: tags.number, color: '#d19a66' },
  { tag: tags.bool, color: '#d19a66' },
  { tag: tags.null, color: '#d19a66' },
  { tag: tags.propertyName, color: '#e06c75' },
  { tag: tags.punctuation, color: '#abb2bf' },
  { tag: tags.bracket, color: '#abb2bf' },
  { tag: tags.typeName, color: '#e5c07b' },
  { tag: tags.className, color: '#e5c07b' },
  { tag: tags.definition(tags.variableName), color: '#61afef' },
  { tag: tags.tagName, color: '#e06c75' },
  { tag: tags.attributeName, color: '#d19a66' },
  { tag: tags.attributeValue, color: '#98c379' },
  { tag: tags.comment, color: '#5c6370', fontStyle: 'italic' },
  { tag: tags.content, color: '#abb2bf' },
])

// GitHub light syntax palette — high contrast on the white light-theme background.
export const lightHighlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: '#CF222E' },
  { tag: tags.operator, color: '#0550AE' },
  { tag: tags.string, color: '#0A3069' },
  { tag: tags.number, color: '#0550AE' },
  { tag: tags.bool, color: '#0550AE' },
  { tag: tags.null, color: '#0550AE' },
  { tag: tags.propertyName, color: '#953800' },
  { tag: tags.punctuation, color: '#1F2328' },
  { tag: tags.bracket, color: '#1F2328' },
  { tag: tags.typeName, color: '#6F42C1' },
  { tag: tags.className, color: '#6F42C1' },
  { tag: tags.definition(tags.variableName), color: '#0550AE' },
  { tag: tags.tagName, color: '#116329' },
  { tag: tags.attributeName, color: '#953800' },
  { tag: tags.attributeValue, color: '#0A3069' },
  { tag: tags.comment, color: '#6E7781', fontStyle: 'italic' },
  { tag: tags.content, color: '#1F2328' },
])

// Markdown source styling for the description editor. Markers stay dim so the
// prose reads first; heading sizes give the raw text some of the shape it will
// have in the preview.
const markdownStructure = [
  { tag: tags.heading1, fontSize: '1.35em', fontWeight: 'bold' },
  { tag: tags.heading2, fontSize: '1.2em', fontWeight: 'bold' },
  { tag: tags.heading3, fontSize: '1.1em', fontWeight: 'bold' },
  { tag: [tags.heading4, tags.heading5, tags.heading6, tags.heading], fontWeight: 'bold' },
  { tag: tags.strong, fontWeight: 'bold' },
  { tag: tags.emphasis, fontStyle: 'italic' },
  { tag: tags.strikethrough, textDecoration: 'line-through' },
]

export const markdownDarkHighlightStyle = HighlightStyle.define([
  ...markdownStructure,
  { tag: [tags.heading1, tags.heading2, tags.heading3, tags.heading4, tags.heading5, tags.heading6, tags.heading], color: '#61afef' },
  { tag: [tags.link, tags.url], color: '#56b6c2' },
  { tag: tags.monospace, color: '#98c379', backgroundColor: 'color-mix(in srgb, var(--muted-foreground) 14%, transparent)', borderRadius: '3px' },
  { tag: tags.quote, color: '#7f848e', fontStyle: 'italic' },
  { tag: tags.list, color: '#c678dd' },
  { tag: tags.labelName, color: '#e5c07b' },
  // Table pipes and the delimiter row carry the shape of a table, so they stay above AA.
  { tag: [tags.processingInstruction, tags.contentSeparator], color: '#7D8590' },
])

export const markdownLightHighlightStyle = HighlightStyle.define([
  ...markdownStructure,
  { tag: [tags.heading1, tags.heading2, tags.heading3, tags.heading4, tags.heading5, tags.heading6, tags.heading], color: '#0550AE' },
  { tag: [tags.link, tags.url], color: '#0969DA' },
  { tag: tags.monospace, color: '#0A3069', backgroundColor: 'color-mix(in srgb, var(--muted-foreground) 14%, transparent)', borderRadius: '3px' },
  { tag: tags.quote, color: '#6E7781', fontStyle: 'italic' },
  { tag: tags.list, color: '#6F42C1' },
  { tag: tags.labelName, color: '#953800' },
  { tag: [tags.processingInstruction, tags.contentSeparator], color: '#6A737D' },
])
