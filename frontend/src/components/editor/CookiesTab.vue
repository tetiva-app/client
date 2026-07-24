<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import { ChevronDown, ChevronRight } from 'lucide-vue-next'
import { getCookieService, type CookieDTO } from '@/services'
import { useCookieModalUi } from '@/stores/cookieModalUi'

const cookieModalUi = useCookieModalUi()

const props = defineProps<{
  workspaceId: string
  url: string
  setCookieHeaders: string[]   // value of response.Headers["Set-Cookie"], may be empty
}>()

const sentCookies = ref<CookieDTO[]>([])
const sentLoaded = ref(false)
const sentExpanded = ref(true)
const receivedExpanded = ref(true)

watchEffect(async () => {
  if (!props.url) {
    sentCookies.value = []
    sentLoaded.value = true
    return
  }
  try {
    const svc = await getCookieService()
    const res = await svc.getForURL(props.workspaceId, props.url)
    sentCookies.value = res.error ? [] : res.data
  } catch {
    sentCookies.value = []
  } finally {
    sentLoaded.value = true
  }
})

interface ParsedSetCookie {
  name: string
  value: string
  domain: string
  path: string
  expires: string
  httpOnly: boolean
  secure: boolean
  sameSite: string
}

// Lightweight Set-Cookie parser. Spec is messy; we cover the common attributes.
function parseSetCookie(raw: string): ParsedSetCookie {
  const parts = raw.split(';').map((s) => s.trim())
  const [first, ...attrs] = parts
  const eq = first.indexOf('=')
  const name = eq >= 0 ? first.slice(0, eq) : first
  const value = eq >= 0 ? first.slice(eq + 1) : ''

  const out: ParsedSetCookie = {
    name, value, domain: '', path: '', expires: '',
    httpOnly: false, secure: false, sameSite: '',
  }
  for (const a of attrs) {
    const lower = a.toLowerCase()
    if (lower === 'httponly') out.httpOnly = true
    else if (lower === 'secure') out.secure = true
    else if (lower.startsWith('domain=')) out.domain = a.slice(7)
    else if (lower.startsWith('path=')) out.path = a.slice(5)
    else if (lower.startsWith('expires=')) out.expires = a.slice(8)
    else if (lower.startsWith('max-age=') && !out.expires) {
      const sec = parseInt(a.slice(8), 10)
      if (!Number.isNaN(sec)) {
        out.expires = new Date(Date.now() + sec * 1000).toUTCString()
      }
    } else if (lower.startsWith('samesite=')) out.sameSite = a.slice(9)
  }
  return out
}

const receivedCookies = computed<ParsedSetCookie[]>(() =>
  props.setCookieHeaders.map(parseSetCookie),
)

const totalCount = computed(() => sentCookies.value.length + receivedCookies.value.length)

function truncate(s: string, n = 40): string {
  return s.length > n ? s.slice(0, n) + '…' : s
}
</script>

<template>
  <div class="p-3 text-[13px]">
    <div v-if="totalCount === 0 && sentLoaded" class="text-muted-foreground py-6 text-center">
      No cookies sent or received with this request.
    </div>

    <div v-if="sentCookies.length > 0" class="mb-4">
      <button
        class="flex items-center gap-1 mb-2 font-medium text-foreground"
        @click="sentExpanded = !sentExpanded"
      >
        <ChevronDown v-if="sentExpanded" class="size-3.5" />
        <ChevronRight v-else class="size-3.5" />
        Sent ({{ sentCookies.length }})
      </button>
      <table v-if="sentExpanded" class="w-full text-[13px]">
        <thead>
          <tr class="text-left text-muted-foreground border-b border-border">
            <th class="py-1.5 pr-3 font-normal">Name</th>
            <th class="py-1.5 pr-3 font-normal">Value</th>
            <th class="py-1.5 pr-3 font-normal">Domain</th>
            <th class="py-1.5 pr-3 font-normal">Path</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in sentCookies" :key="c.domain + '|' + c.path + '|' + c.name" class="border-b border-border/50">
            <td class="py-1.5 pr-3 font-mono">{{ c.name }}</td>
            <td class="py-1.5 pr-3 font-mono" :title="c.value">{{ truncate(c.value) }}</td>
            <td class="py-1.5 pr-3">{{ c.domain || '—' }}</td>
            <td class="py-1.5 pr-3">{{ c.path || '/' }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="receivedCookies.length > 0">
      <button
        class="flex items-center gap-1 mb-2 font-medium text-foreground"
        @click="receivedExpanded = !receivedExpanded"
      >
        <ChevronDown v-if="receivedExpanded" class="size-3.5" />
        <ChevronRight v-else class="size-3.5" />
        Received ({{ receivedCookies.length }})
      </button>
      <table v-if="receivedExpanded" class="w-full text-[13px]">
        <thead>
          <tr class="text-left text-muted-foreground border-b border-border">
            <th class="py-1.5 pr-3 font-normal">Name</th>
            <th class="py-1.5 pr-3 font-normal">Value</th>
            <th class="py-1.5 pr-3 font-normal">Domain</th>
            <th class="py-1.5 pr-3 font-normal">Path</th>
            <th class="py-1.5 pr-3 font-normal">Expires</th>
            <th class="py-1.5 pr-3 font-normal">Flags</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(c, i) in receivedCookies" :key="i" class="border-b border-border/50">
            <td class="py-1.5 pr-3 font-mono">{{ c.name }}</td>
            <td class="py-1.5 pr-3 font-mono" :title="c.value">{{ truncate(c.value) }}</td>
            <td class="py-1.5 pr-3">{{ c.domain || '—' }}</td>
            <td class="py-1.5 pr-3">{{ c.path || '/' }}</td>
            <td class="py-1.5 pr-3">{{ c.expires || 'session' }}</td>
            <td class="py-1.5 pr-3 text-muted-foreground">
              <span v-if="c.httpOnly" title="HttpOnly">H</span>
              <span v-if="c.secure" title="Secure">S</span>
              <span v-if="c.sameSite" :title="'SameSite=' + c.sameSite">{{ c.sameSite[0] }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="mt-4 pt-3 border-t border-border">
      <button
        class="text-xs text-primary hover:underline cursor-pointer"
        @click="cookieModalUi.show()"
      >
        Manage cookies →
      </button>
    </div>
  </div>
</template>
