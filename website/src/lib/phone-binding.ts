/** Provisional UI contract. Missing status is unknown, never unbound. */
export type PhoneBindingUser = {
  id?: unknown;
  phone_bound?: unknown;
  role?: unknown;
};

export function needsPhoneBinding(
  user: PhoneBindingUser | null | undefined,
): boolean {
  return (
    typeof user?.id === "number" &&
    user.id > 0 &&
    Number.isInteger(user.id) &&
    (typeof user.role !== "number" || user.role < 10) &&
    user.phone_bound === false
  );
}
