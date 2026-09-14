"use client";

import { useState } from "react";
import { Dialog } from "@base-ui/react/dialog";
import { Smartphone, X } from "lucide-react";
import type { ConsoleCurrentUserPayload } from "@/lib/console-session-hint";
import { shouldShowPhoneBindingPrompt } from "@/lib/console-session-hint";
import type { Locale } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";
import { phoneBindingCopy } from "@/lib/phone-binding-copy";

export function PhoneBindingPrompt(props: {
  locale: Locale;
  user: ConsoleCurrentUserPayload | null;
}) {
  const [dismissedUserId, setDismissedUserId] = useState<number | null>(null);
  const userId =
    typeof props.user?.data?.id === "number" ? props.user.data.id : null;
  const open =
    shouldShowPhoneBindingPrompt(props.user) && dismissedUserId !== userId;
  const copy = phoneBindingCopy[props.locale];

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) setDismissedUserId(userId);
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-[100] bg-[#18111f]/40 backdrop-blur-[3px]" />
        <Dialog.Popup className="fixed top-1/2 left-1/2 z-[101] flex max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] max-w-[460px] -translate-x-1/2 -translate-y-1/2 flex-col gap-7 overflow-hidden rounded-[28px] border border-black/5 bg-white px-6 pt-7 pb-6 text-[#18181b] shadow-[0_32px_90px_-24px_rgba(37,18,56,0.42)] outline-none sm:px-8 sm:pt-9 sm:pb-8">
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-[#8b5cf6] via-[#a855f7] to-[#ec4899]"
          />
          <Dialog.Close
            aria-label={copy.close}
            className="absolute top-4 right-4 inline-flex size-9 items-center justify-center rounded-full bg-[#f7f5f9] text-[#6f6875] transition hover:bg-[#ede8f1] hover:text-[#18181b] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#7c3aed]"
          >
            <X className="size-[18px]" aria-hidden="true" />
          </Dialog.Close>
          <div className="flex flex-col items-start gap-3.5 pr-8">
            <span className="flex size-14 items-center justify-center rounded-2xl bg-[linear-gradient(145deg,#f1eaff,#fcecf7)] text-[#7c3aed] ring-1 ring-[#7c3aed]/10">
              <Smartphone className="size-7" strokeWidth={1.9} aria-hidden="true" />
            </span>
            <Dialog.Title className="text-[26px] leading-8 font-semibold tracking-[-0.025em]">
              {copy.title}
            </Dialog.Title>
            <Dialog.Description className="max-w-[380px] text-[15px] leading-6 text-[#6f6875]">
              {copy.description}
            </Dialog.Description>
          </div>
          <div className="flex flex-col gap-2.5">
            <a
              href={consoleUrl("/profile")}
              className="flex min-h-12 items-center justify-center rounded-xl bg-[#18181b] px-5 py-2.5 text-sm font-semibold !text-white shadow-sm transition hover:bg-[#302b34] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#7c3aed]"
            >
              {copy.action}
            </a>
            <Dialog.Close className="min-h-11 rounded-xl px-4 text-sm font-medium text-[#6f6875] transition hover:bg-[#f7f5f9] hover:text-[#18181b] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#7c3aed]">
              {copy.later}
            </Dialog.Close>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
