"use client";

import { useEffect, useState } from "react";
import { ArrowRight, CheckCircle2, X } from "lucide-react";
import { FlatkeyTallyEmbed } from "@/components/flatkey-tally-embed";
import type { Locale } from "@/lib/locales";
import { signUpUrlForLocale } from "@/lib/pricing-links";
import type { PricingPlan } from "@/components/pricing-page";

type PricingPlansGridContactCopy = {
  closeLabel: string;
  eyebrow: string;
  title: string;
  description: string;
};

type PricingPlansGridProps = {
  plans: PricingPlan[];
  locale: Locale;
  contactCopy: PricingPlansGridContactCopy;
};

export function PricingPlansGrid(props: PricingPlansGridProps) {
  const [contactOpen, setContactOpen] = useState(false);

  useEffect(() => {
    if (!contactOpen) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setContactOpen(false);
    };
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [contactOpen]);

  return (
    <>
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
              <button
                type="button"
                onClick={() => setContactOpen(true)}
                className="flatkey-cta-secondary mt-6 inline-flex h-11 items-center justify-center px-4 text-sm"
              >
                {plan.cta}
                <ArrowRight className="ml-2 size-4" />
              </button>
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

      <div
        className={[
          "fixed inset-0 z-50 flex items-center justify-center bg-[#101014]/76 p-4 backdrop-blur-sm transition-opacity duration-200",
          contactOpen ? "visible opacity-100" : "pointer-events-none invisible opacity-0",
        ].join(" ")}
        role="dialog"
        aria-hidden={!contactOpen}
        aria-modal="true"
        aria-labelledby="enterprise-contact-title"
        onMouseDown={(event) => {
          if (event.target === event.currentTarget) setContactOpen(false);
        }}
      >
        <div className="relative max-h-[94dvh] w-full max-w-3xl overflow-y-auto rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6] p-4 shadow-[6px_6px_0_#101014] dark:border-white/24 dark:bg-[#111116] dark:shadow-[6px_6px_0_rgba(255,255,255,0.16)] sm:p-5 lg:max-w-4xl">
          <button
            type="button"
            onClick={() => setContactOpen(false)}
            className="absolute top-4 right-4 inline-flex size-9 items-center justify-center rounded-full border-2 border-[#101014] bg-white text-[#101014] shadow-[3px_3px_0_#101014] transition-colors hover:bg-[#F9F871] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)] dark:hover:bg-white dark:hover:text-[#101014]"
            aria-label={props.contactCopy.closeLabel}
            tabIndex={contactOpen ? 0 : -1}
          >
            <X className="size-4" />
          </button>
          <div className="pr-10">
            <p className="font-mono text-xs font-black tracking-normal text-[#7C3AED] uppercase dark:text-[#C8A8FF]">{props.contactCopy.eyebrow}</p>
            <h2 id="enterprise-contact-title" className="mt-2 text-2xl font-black text-[#101014] dark:text-white">
              {props.contactCopy.title}
            </h2>
            <p className="mt-2 text-sm leading-6 text-[#5C5861] dark:text-white/62">
              {props.contactCopy.description}
            </p>
          </div>
          {contactOpen ? (
            <FlatkeyTallyEmbed
              locale={props.locale}
              loading="lazy"
              iframeClassName="block h-[72dvh] min-h-[620px] w-full border-0 bg-transparent"
              className="mt-4 rounded-[1rem] border-2 border-[#101014]/14 bg-white/62 p-2 dark:border-white/14 dark:bg-white/[0.06]"
            />
          ) : null}
        </div>
      </div>
    </>
  );
}
