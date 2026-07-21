"use client";

import { useState, type FormEvent } from "react";
import type { Locale } from "@/lib/locales";
import { getStatusCopy } from "@/lib/status-copy";
import { subscribeToStatus, type StatusComponent } from "@/lib/status";

interface StatusSubscribeProps {
  locale: Locale;
  components: StatusComponent[];
}

const MAX_STATUS_SUBSCRIPTION_COMPONENTS = 100;

export function initialStatusSubscriptionComponentIds(components: StatusComponent[]): number[] {
  return components.slice(0, MAX_STATUS_SUBSCRIPTION_COMPONENTS).map((component) => component.id);
}

export function StatusSubscribe({ locale, components }: StatusSubscribeProps) {
  const copy = getStatusCopy(locale).subscribe;
  const [selectedIds, setSelectedIds] = useState(() => initialStatusSubscriptionComponentIds(components));
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const form = new FormData(event.currentTarget);
    const email = String(form.get("email") ?? "").trim();
    if (!email || email.length > 254 || selectedIds.length === 0 || selectedIds.length > MAX_STATUS_SUBSCRIPTION_COMPONENTS) {
      setMessage(copy.errorLabel);
      return;
    }

    setPending(true);
    setMessage("");
    const result = await subscribeToStatus({ email, component_ids: selectedIds });
    setMessage(result.state === "fresh" ? copy.successLabel : copy.errorLabel);
    setPending(false);
  }

  function toggle(componentId: number) {
    setSelectedIds((current) => current.includes(componentId)
      ? current.filter((id) => id !== componentId)
      : [...current, componentId].slice(0, MAX_STATUS_SUBSCRIPTION_COMPONENTS));
  }

  return (
    <section aria-labelledby="status-subscription-title" className="rounded-2xl border border-slate-200 bg-slate-50 p-5 dark:border-slate-800 dark:bg-slate-900 sm:p-6">
      <h2 id="status-subscription-title" className="text-xl font-bold text-slate-950 dark:text-white">{copy.title}</h2>
      <p id="status-subscription-description" className="mt-2 text-sm text-slate-600 dark:text-slate-300">{copy.description}</p>
      <form className="mt-5 space-y-5" onSubmit={submit} aria-describedby="status-subscription-description">
        <div>
          <label htmlFor="status-subscription-email" className="block text-sm font-semibold text-slate-800 dark:text-slate-100">{copy.emailLabel}</label>
          <input
            id="status-subscription-email"
            name="email"
            type="email"
            autoComplete="email"
            maxLength={254}
            required
            placeholder={copy.emailPlaceholder}
            className="mt-2 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-slate-950 outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:border-slate-700 dark:bg-slate-950 dark:text-white"
          />
        </div>
        <fieldset>
          <legend className="text-sm font-semibold text-slate-800 dark:text-slate-100">{copy.componentLegend}</legend>
          <div className="mt-2 grid gap-2 sm:grid-cols-2">
            {components.slice(0, MAX_STATUS_SUBSCRIPTION_COMPONENTS).map((component) => (
              <label key={component.id} className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200">
                <input
                  type="checkbox"
                  name="component_ids"
                  value={component.id}
                  checked={selectedIds.includes(component.id)}
                  onChange={() => toggle(component.id)}
                  className="size-4 rounded border-slate-300 focus-visible:ring-2 focus-visible:ring-blue-500"
                />
                <span>{component.display_name}</span>
              </label>
            ))}
          </div>
        </fieldset>
        <button type="submit" disabled={pending} className="rounded-lg bg-blue-600 px-4 py-2 font-semibold text-white outline-none hover:bg-blue-700 focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 disabled:opacity-60">
          {pending ? copy.submittingLabel : copy.submitLabel}
        </button>
        <p role="status" aria-live="polite" className="text-sm text-slate-700 dark:text-slate-200">{message}</p>
      </form>
    </section>
  );
}
