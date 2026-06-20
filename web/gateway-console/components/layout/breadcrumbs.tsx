import Link from "next/link";

export function Breadcrumbs({ items }: { items: { label: string; href?: string }[] }) {
  return (
    <nav aria-label="Breadcrumb" className="font-mono text-mono tracking-[0.04em] text-ink-muted">
      <ol className="m-0 flex list-none flex-wrap items-center gap-xs p-0">
        {items.map((item, index) => (
          <li className="flex items-center gap-xs" key={`${item.label}-${index}`}>
            {index > 0 ? <span aria-hidden="true">/</span> : null}
            {item.href ? (
              <Link className="text-ink-secondary no-underline hover:text-accent" href={item.href}>
                {item.label}
              </Link>
            ) : (
              <span>{item.label}</span>
            )}
          </li>
        ))}
      </ol>
    </nav>
  );
}
