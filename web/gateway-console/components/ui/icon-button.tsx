import type { LucideIcon } from "lucide-react";
import { Button, type ButtonProps } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

type IconButtonProps = Omit<ButtonProps, "children" | "size"> & {
  icon: LucideIcon;
  label: string;
};

export function IconButton({ icon: Icon, label, ...props }: IconButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button aria-label={label} size="icon" variant="quiet" {...props}>
          <Icon aria-hidden="true" className="h-4 w-4" />
        </Button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}
