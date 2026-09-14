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
        <Dialog.Backdrop className="fixed inset-0 z-[100] bg-black/30 backdrop-blur-sm" />
        <Dialog.Popup className="fixed left-1/2 top-1/2 z-[101] flex max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] max-w-[440px] -translate-x-1/2 -translate-y-1/2 flex-col gap-6 overflow-y-auto rounded-2xl bg-white p-6 text-[#18181b] shadow-2xl outline-none sm:p-8">
          <Dialog.Close
            aria-label={copy.close}
            className="absolute right-4 top-4 rounded-lg p-2 text-[#71717a] hover:bg-[#f4f4f5] focus-visible:outline-2"
          >
            <X className="size-4" aria-hidden="true" />
          </Dialog.Close>
          <div className="flex flex-col gap-3">
            <span className="mb-1 flex size-12 items-center justify-center rounded-2xl bg-[#f3edff] text-[#7c3aed]">
              <Smartphone className="size-6" aria-hidden="true" />
            </span>
            <Dialog.Title className="text-2xl font-semibold tracking-tight">
              {copy.title}
            </Dialog.Title>
            <Dialog.Description className="text-sm leading-6 text-[#71717a]">
              {copy.description}
            </Dialog.Description>
          </div>
          <div className="flex flex-col gap-2">
            <a
              href={consoleUrl("/profile")}
              className="flex min-h-11 items-center justify-center rounded-lg bg-[#18181b] px-4 py-2 text-sm font-semibold text-white hover:bg-[#27272a] focus-visible:outline-2 focus-visible:outline-offset-2"
            >
              {copy.action}
            </a>
            <Dialog.Close className="min-h-10 rounded-lg px-4 text-sm text-[#71717a] hover:bg-[#f4f4f5] focus-visible:outline-2">
              {copy.later}
            </Dialog.Close>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
