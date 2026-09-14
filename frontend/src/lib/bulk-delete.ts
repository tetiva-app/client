// Every id is attempted even after one is refused; the refused ones come back to the caller.
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
