import { z } from "zod";

export function collectionField<T extends z.ZodTypeAny>(item: T) {
  return z.preprocess((value) => value ?? [], z.array(item));
}
