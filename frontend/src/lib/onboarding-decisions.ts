// Main-window, first-launch only: child windows (detached request, schema viewer) skip it.
export function shouldShowOnboarding(completedAt: string | null, windowMode: string | null): boolean {
  return completedAt === null && windowMode === null
}

// Installs upgraded from a build without the flag would otherwise get the welcome
// on top of an app they already use; a stored flag means the reset was deliberate.
export function shouldBackfillOnboarding(
  completedAt: string | null,
  lastSeenWhatsNewVersion: string | null,
  flagStored: boolean,
): boolean {
  return completedAt === null && lastSeenWhatsNewVersion !== null && !flagStored
}
