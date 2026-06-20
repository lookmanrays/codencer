"use client";

import * as AccordionPrimitive from "@radix-ui/react-accordion";
import { ChevronDown } from "lucide-react";
import * as React from "react";
import { cn } from "@/lib/cn";

export const Accordion = AccordionPrimitive.Root;
export const AccordionItem = React.forwardRef<
  React.ElementRef<typeof AccordionPrimitive.Item>,
  React.ComponentPropsWithoutRef<typeof AccordionPrimitive.Item>
>(({ className, ...props }, ref) => (
  <AccordionPrimitive.Item className={cn("border-b border-border", className)} ref={ref} {...props} />
));
AccordionItem.displayName = AccordionPrimitive.Item.displayName;

export const AccordionTrigger = React.forwardRef<
  React.ElementRef<typeof AccordionPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof AccordionPrimitive.Trigger>
>(({ children, className, ...props }, ref) => (
  <AccordionPrimitive.Header>
    <AccordionPrimitive.Trigger
      className={cn("flex w-full items-center justify-between py-md text-left font-semibold", className)}
      ref={ref}
      {...props}
    >
      {children}
      <ChevronDown aria-hidden="true" className="h-4 w-4" />
    </AccordionPrimitive.Trigger>
  </AccordionPrimitive.Header>
));
AccordionTrigger.displayName = AccordionPrimitive.Trigger.displayName;

export const AccordionContent = React.forwardRef<
  React.ElementRef<typeof AccordionPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof AccordionPrimitive.Content>
>(({ className, ...props }, ref) => (
  <AccordionPrimitive.Content className={cn("pb-md text-body-sm text-ink-secondary", className)} ref={ref} {...props} />
));
AccordionContent.displayName = AccordionPrimitive.Content.displayName;
