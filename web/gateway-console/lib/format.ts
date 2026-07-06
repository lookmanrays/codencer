export function formatDateTime(value: string) {
  return new Intl.DateTimeFormat("en", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "UTC",
  }).format(new Date(value));
}

export function countBy<T>(items: T[], predicate: (item: T) => boolean) {
  return items.filter(predicate).length;
}
