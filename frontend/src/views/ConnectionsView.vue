<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useHugsStore, type ConnectionItem } from '@/stores/hugs'
import { profileLink } from '@/lib/profileLink'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'

const hugsStore = useHugsStore()
const connections = ref<ConnectionItem[]>([])
const loading = ref(true)

async function load() {
  loading.value = true
  try {
    connections.value = await hugsStore.getConnections(50, 0)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-2xl space-y-4 sm:space-y-6">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight">Связи</h1>
      <p class="text-muted-foreground">Ваши ближайшие связи по близости</p>
    </div>

    <div v-if="loading" class="space-y-3">
      <Skeleton v-for="i in 6" :key="i" class="h-16 w-full rounded-lg" />
    </div>

    <div
      v-else-if="connections.length === 0"
      class="py-12 text-center text-muted-foreground sm:py-16"
    >
      <p class="text-base font-medium sm:text-lg">Пока нет связей</p>
      <p class="mt-1 text-sm">Обнимайтесь, чтобы укрепить близость</p>
    </div>

    <div v-else class="divide-y rounded-md border">
      <RouterLink
        v-for="conn in connections"
        :key="conn.user_id"
        :to="profileLink(conn.username, conn.user_id)"
        class="flex items-center gap-3 px-3 py-3 transition-colors hover:bg-muted/50 sm:px-4"
      >
        <Avatar class="size-10 shrink-0">
          <AvatarFallback class="text-xs">
            {{ (conn.display_name || conn.username).slice(0, 2).toUpperCase() }}
          </AvatarFallback>
        </Avatar>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium">
            {{ conn.display_name || conn.username }}
          </p>
          <p v-if="conn.display_name" class="truncate text-xs text-muted-foreground">
            @{{ conn.username }}
          </p>
        </div>
        <div class="flex shrink-0 flex-col items-end gap-1">
          <Badge
            variant="secondary"
            class="text-[10px] bg-prod-yellow/15 text-prod-yellow border-prod-yellow/20"
          >
            {{ conn.intimacy.tier_name }}
          </Badge>
          <span class="text-xs tabular-nums text-muted-foreground">
            {{ conn.intimacy.raw_score }} очков
          </span>
        </div>
        <!-- Progress to next tier -->
        <div v-if="conn.intimacy.next_tier_at != null" class="hidden w-16 sm:block">
          <div class="h-1.5 w-full rounded-full bg-muted overflow-hidden">
            <div
              class="h-full rounded-full bg-prod-yellow transition-all"
              :style="{
                width:
                  Math.min(
                    (conn.intimacy.raw_score / conn.intimacy.next_tier_at!) * 100,
                    100,
                  ) + '%',
              }"
            />
          </div>
        </div>
      </RouterLink>
    </div>
  </div>
</template>
