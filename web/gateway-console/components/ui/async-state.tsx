import { Alert } from "@/components/ui/alert";
import { EmptyState } from "@/components/ui/empty-state";
import { LoadingPanel } from "@/components/ui/skeleton";

type AsyncStateProps<T> = {
  data: T | undefined;
  isLoading: boolean;
  error: Error | null;
  isEmpty: (data: T) => boolean;
  emptyTitle: string;
  emptyDescription: string;
  children: (data: T) => React.ReactNode;
};

export function AsyncState<T>({
  children,
  data,
  emptyDescription,
  emptyTitle,
  error,
  isEmpty,
  isLoading,
}: AsyncStateProps<T>) {
  if (isLoading) return <LoadingPanel />;
  if (error) {
    return (
      <Alert title="Error" tone="danger">
        {error.message}
      </Alert>
    );
  }
  if (!data || isEmpty(data)) {
    return <EmptyState description={emptyDescription} title={emptyTitle} />;
  }
  return children(data);
}
