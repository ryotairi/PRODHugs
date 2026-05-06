<script setup lang="ts">
import { useAnnouncementStore } from '@/stores/announcement'
import { X, Megaphone } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const store = useAnnouncementStore()

function dismiss() {
  if (store.active) {
    store.dismiss(store.active.id)
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition-all duration-300 ease-out"
      enter-from-class="opacity-0 translate-y-4 scale-95"
      enter-to-class="opacity-100 translate-y-0 scale-100"
      leave-active-class="transition-all duration-200 ease-in"
      leave-from-class="opacity-100 translate-y-0 scale-100"
      leave-to-class="opacity-0 translate-y-4 scale-95"
    >
      <div
        v-if="store.active"
        class="fixed inset-x-0 bottom-20 z-50 mx-auto w-[calc(100%-2rem)] max-w-sm md:bottom-6"
      >
        <div
          class="relative rounded-xl border border-prod-yellow/30 bg-popover p-4 shadow-lg ring-1 ring-foreground/5"
        >
          <Button
            variant="ghost"
            size="icon-sm"
            class="absolute right-2 top-2"
            @click="dismiss"
          >
            <X class="size-3.5" />
          </Button>

          <div class="flex items-start gap-3 pr-6">
            <div
              class="flex size-8 shrink-0 items-center justify-center rounded-full bg-prod-yellow/15"
            >
              <Megaphone class="size-4 text-prod-yellow" />
            </div>
            <div class="min-w-0 space-y-1">
              <p class="text-xs font-medium text-prod-yellow">Объявление</p>
              <p class="text-sm text-foreground">{{ store.active.message }}</p>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
