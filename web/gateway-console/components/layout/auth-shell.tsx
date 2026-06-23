import { PageHeader } from "@/components/layout/page-header";

export function AuthShell({
  children,
  description,
  kicker,
  title,
}: {
  children: React.ReactNode;
  description: string;
  kicker: string;
  title: string;
}) {
  return (
    <main
      className="min-h-dvh bg-paper px-[var(--container-pad)] py-xl"
      id="main-content"
    >
      <div className="mx-auto grid w-full max-w-[760px] gap-lg">
        <div className="border-b border-border pb-md">
          <span className="block text-h3 font-bold leading-none">CODENCER</span>
          <span className="mt-xs block font-mono text-mono tracking-[0.04em] text-ink-muted">
            Gateway authorization
          </span>
        </div>
        <PageHeader description={description} kicker={kicker} title={title} />
        {children}
      </div>
    </main>
  );
}
