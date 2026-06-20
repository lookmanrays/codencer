import { Kicker } from "@/components/layout/section-kicker";
import { Breadcrumbs } from "@/components/layout/breadcrumbs";

type PageHeaderProps = {
  kicker: string;
  title: string;
  description: string;
  actions?: React.ReactNode;
  breadcrumbs?: { label: string; href?: string }[];
};

export function PageHeader({
  actions,
  breadcrumbs,
  description,
  kicker,
  title,
}: PageHeaderProps) {
  return (
    <header className="mb-lg">
      {breadcrumbs ? <Breadcrumbs items={breadcrumbs} /> : null}
      <div className="mt-md flex flex-wrap items-end justify-between gap-md">
        <div className="max-w-[780px]">
          <Kicker label={kicker} />
          <h1 className="m-0 mt-md text-h1 font-bold leading-[1.1] tracking-[-0.01em]">
            {title}
          </h1>
          <p className="mb-0 mt-md text-body-lg leading-[1.6] text-ink-secondary">
            {description}
          </p>
        </div>
        {actions ? (
          <div className="flex flex-wrap gap-sm">{actions}</div>
        ) : null}
      </div>
    </header>
  );
}
