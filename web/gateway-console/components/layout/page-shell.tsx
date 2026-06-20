import { PageHeader } from "@/components/layout/page-header";

export function PageShell({
  actions,
  breadcrumbs,
  children,
  description,
  kicker,
  title,
}: {
  actions?: React.ReactNode;
  breadcrumbs?: { label: string; href?: string }[];
  children: React.ReactNode;
  description: string;
  kicker: string;
  title: string;
}) {
  return (
    <main
      className="mx-auto w-full max-w-[var(--container-max)] px-[var(--container-pad)] py-lg"
      id="main-content"
    >
      <PageHeader
        actions={actions}
        breadcrumbs={breadcrumbs}
        description={description}
        kicker={kicker}
        title={title}
      />
      {children}
    </main>
  );
}
