export const HISTORY_PAGE_SIZE = 200
export const HISTORY_MAX_URL_DISPLAY_LENGTH = 80
export const HISTORY_GROUP_BUCKETS = ['Today', 'Yesterday', 'This week', 'Earlier'] as const
export type HistoryGroupBucket = typeof HISTORY_GROUP_BUCKETS[number]
