<script setup lang="ts">
import { toast } from 'vue-sonner'
import { Copy, ExternalLink, Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const open = defineModel<boolean>('open', { required: true })

defineProps<{
  botUserId: string
  botUrl: string
  command: string
  mode?: 'login' | 'register'
  error?: string | null
}>()

const emit = defineEmits<{
  cancel: []
}>()

async function copyCommand(command: string) {
  try {
    await navigator.clipboard.writeText(command)
    toast.success('Команда скопирована')
  } catch {
    toast.error('Не удалось скопировать')
  }
}

function handleClose(newOpen: boolean) {
  if (!newOpen) emit('cancel')
  open.value = newOpen
}
</script>

<template>
  <Dialog :open="open" @update:open="handleClose">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{
          mode === 'login' ? 'Вход через Matrix' : 'Регистрация через Matrix'
        }}</DialogTitle>
        <DialogDescription>
          Открой бота в своём Matrix клиенте и отправь ему команду ниже.
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4">
        <!-- Step 1: open bot -->
        <div class="space-y-2">
          <p class="text-xs text-muted-foreground">
            Шаг 1. Открой чат с ботом
            <span class="font-mono text-foreground">{{ botUserId }}</span>
          </p>
          <Button
            variant="outline"
            size="sm"
            class="w-full gap-2 rounded-[21px]"
            as-child
          >
            <a :href="botUrl" target="_blank" rel="noopener noreferrer">
              <ExternalLink class="size-4" />
              Открыть бота
            </a>
          </Button>
        </div>

        <!-- Step 2: copy command -->
        <div class="space-y-2">
          <p class="text-xs text-muted-foreground">
            Шаг 2. Отправь эту команду боту
          </p>
          <div class="flex items-center gap-2">
            <code
              class="flex-1 overflow-x-auto rounded-md border bg-muted px-3 py-2 font-mono text-xs"
            >
              {{ command }}
            </code>
            <Button
              variant="outline"
              size="sm"
              class="shrink-0 gap-1.5 rounded-[21px]"
              @click="copyCommand(command)"
            >
              <Copy class="size-3.5" />
              Копировать
            </Button>
          </div>
        </div>

        <!-- Waiting state -->
        <div class="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 class="size-3.5 animate-spin" />
          Ожидание ответа от бота...
        </div>

        <p v-if="error" class="text-sm text-destructive">
          {{ error }}
        </p>
      </div>
    </DialogContent>
  </Dialog>
</template>
