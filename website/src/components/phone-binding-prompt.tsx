"use client";

import { useSyncExternalStore, useState } from "react";
import { Dialog } from "@base-ui/react/dialog";
import { Smartphone, X } from "lucide-react";
import type { Locale } from "@/lib/locales";
import { phoneBindingCopy } from "@/lib/phone-binding-copy";
import { needsPhoneBinding, type PhoneBindingUser } from "@/lib/phone-binding";

export function PhoneBindingPrompt(props: {
  locale: Locale;
  user: PhoneBindingUser | null;
}) {
  const preview = useSyncExternalStore(
    () => () => {},
    () =>
      process.env.NODE_ENV === "development" &&
      new URLSearchParams(window.location.search).get("phoneBindingPreview") ===
        "1",
    () => false,
  );
  const [dismissedUser, setDismissedUser] = useState<unknown>(null);
  const identity = props.user?.id ?? "preview";
  const open =
    (preview || needsPhoneBinding(props.user)) && dismissedUser !== identity;
  const copy = phoneBindingCopy[props.locale];
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) setDismissedUser(identity);
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-[100] bg-black/30 backdrop-blur-sm" />
        <Dialog.Popup className="fixed left-1/2 top-1/2 z-[101] flex max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] max-w-[440px] -translate-x-1/2 -translate-y-1/2 flex-col gap-6 overflow-y-auto rounded-2xl bg-white p-6 text-[#18181b] shadow-2xl outline-none sm:p-8">
          <Dialog.Close
            aria-label={copy.close}
            className="absolute right-4 top-4 rounded-lg p-2 text-[#71717a] hover:bg-[#f4f4f5] focus-visible:outline-2"
          >
            <X className="size-4" />
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
          <form
            className="flex flex-col gap-5"
            onSubmit={(event) => event.preventDefault()}
          >
            <div className="flex flex-col gap-2">
              <label htmlFor="binding-phone" className="text-sm font-medium">
                {copy.phone}
              </label>
              <div className="flex gap-2">
                <input
                  aria-label={copy.callingCode}
                  type="tel"
                  autoComplete="tel-country-code"
                  defaultValue={props.locale === "zh" ? "+86" : "+1"}
                  maxLength={4}
                  className="h-11 w-20 shrink-0 rounded-lg border border-[#e4e4e7] px-3 text-sm outline-none focus:border-[#8b5cf6] focus:ring-2 focus:ring-[#ede9fe]"
                />
                <input
                  id="binding-phone"
                  type="tel"
                  autoComplete="tel-national"
                  placeholder={copy.phonePlaceholder}
                  className="h-11 min-w-0 flex-1 rounded-lg border border-[#e4e4e7] px-3 text-sm outline-none focus:border-[#8b5cf6] focus:ring-2 focus:ring-[#ede9fe]"
                />
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <label htmlFor="binding-code" className="text-sm font-medium">
                {copy.code}
              </label>
              <div className="flex gap-2">
                <input
                  id="binding-code"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  placeholder={copy.codePlaceholder}
                  maxLength={6}
                  className="h-11 min-w-0 flex-1 rounded-lg border border-[#e4e4e7] px-3 text-sm outline-none focus:border-[#8b5cf6] focus:ring-2 focus:ring-[#ede9fe]"
                />
                <button
                  type="button"
                  disabled
                  aria-describedby="binding-unavailable"
                  className="min-h-11 max-w-[45%] shrink-0 rounded-lg border border-[#e4e4e7] px-3 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {copy.send}
                </button>
              </div>
            </div>
            <p
              id="binding-unavailable"
              className="text-xs leading-5 text-[#71717a]"
            >
              {copy.unavailable}
            </p>
            <div className="flex flex-col gap-2">
              <button
                type="submit"
                disabled
                aria-describedby="binding-unavailable"
                className="min-h-11 w-full rounded-lg bg-[#18181b] px-4 py-2 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
              >
                {copy.submit}
              </button>
              <Dialog.Close className="min-h-10 rounded-lg px-4 text-sm text-[#71717a] hover:bg-[#f4f4f5] focus-visible:outline-2">
                {copy.later}
              </Dialog.Close>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
