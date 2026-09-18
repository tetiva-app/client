export async function removeEach(
  ids: string[],
  remove: (id: string) => Promise<boolean>,
): Promise<string[]> {
  const failed: string[] = []
  for (const id of ids) {
    if (!(await remove(id))) failed.push(id)
  }
  return failed
}
