// Keep model URL encoding independent from pricing and UI modules so the
// request proxy can use the same canonical slug without importing them.
export function modelPublicSlug(modelName: string): string {
  if (modelName.toLowerCase() === "minimax-h3") return "minimax-h3";
  // Next.js redirects a %2F inside a dynamic segment to itself. Escape
  // literal tildes first to keep this slash marker unambiguous.
  return encodeURIComponent(modelName).replaceAll("~", "~7E").replaceAll("%2F", "~2F");
}

export function modelPublicPath(modelName: string): string {
  return `/models/${modelPublicSlug(modelName)}`;
}
