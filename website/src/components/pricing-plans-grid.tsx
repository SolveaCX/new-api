"use client";

import Link from "next/link";
import { ArrowRight, CheckCircle2 } from "lucide-react";
import { localizePath, type Locale } from "@/lib/locales";
import { signUpUrlForLocale } from "@/lib/pricing-links";
import type { PricingPlan } from "@/components/pricing-page";

type PricingPlansGridProps = {
  plans: PricingPlan[];
  locale: Locale;
};

export function PricingPlansGrid(props: PricingPlansGridProps) {
  return (
    <section className="mt-10 grid gap-4 lg:grid-cols-4">
      {props.plans.map((plan) => (
        <article
          key={plan.name}
          className={[
            "relative flex min-h-[440px] flex-col rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 p-6 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]",
            plan.featured ? "bg-[#EEE4FF]/95 ring-2 ring-[#101014]/10 dark:bg-white/12 dark:ring-white/12" : "",
          ].join(" ")}
        >
          {plan.badge ? (
            <span className="absolute top-0 left-1/2 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-[#101014] bg-[#F9F871] px-3.5 py-1 text-xs font-black whitespace-nowrap text-[#101014] shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-[#C8A8FF] dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">{plan.badge}</span>
          ) : null}
          <h2 className="text-xl font-black text-[#101014] dark:text-white">{plan.name}</h2>
          <p className="mt-2 h-[4.5rem] text-sm leading-6 text-[#5C5861] dark:text-white/62">{plan.description}</p>
          <div className="mt-6 flex flex-wrap items-center gap-x-3 gap-y-2">
            <span className={plan.action === "contact" ? "text-4xl font-black tracking-normal text-[#101014] dark:text-white" : "text-5xl font-black tracking-normal text-[#101014] dark:text-white"}>{plan.price}</span>
            {plan.discount ? (
              <span className="rounded-full border border-[#101014]/14 bg-white/72 px-2.5 py-1 text-xs font-black whitespace-nowrap text-[#2F7D32] dark:border-white/14 dark:bg-white/8 dark:text-emerald-200">{plan.discount}</span>
            ) : null}
          </div>
          <p className={[
            "mt-3 min-h-10 text-sm leading-5 font-semibold",
            plan.action === "contact" ? "text-[#5C5861] dark:text-white/58" : "text-[#2F7D32] dark:text-emerald-200",
          ].join(" ")}>{plan.caption}</p>
          {plan.action === "contact" ? (
            <Link
              href={localizePath("/contact", props.locale)}
              className="flatkey-cta-secondary mt-6 inline-flex h-11 items-center justify-center px-4 text-sm"
            >
              {plan.cta}
              <ArrowRight className="ml-2 size-4" />
            </Link>
          ) : (
            <a
              href={plan.checkoutUrl ?? signUpUrlForLocale(props.locale)}
              className={[
                "mt-6 inline-flex h-11 items-center justify-center px-4 text-sm",
                plan.featured ? "flatkey-primary-cta" : "flatkey-cta-secondary",
              ].join(" ")}
            >
              {plan.cta}
              <ArrowRight className="ml-2 size-4" />
            </a>
          )}
          <div className="mt-6 space-y-3">
            {plan.features.map((feature) => (
              <p key={feature} className="flex gap-2 text-sm leading-6 text-[#3F3F48] dark:text-white/72">
                <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-[#7C3AED] dark:text-[#C8A8FF]" />
                <span>{feature}</span>
              </p>
            ))}
          </div>
        </article>
      ))}
    </section>
  );
}
