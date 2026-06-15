export type WebsiteSessionState = {
  authenticated: boolean;
};

type SelfResponse = {
  success?: boolean;
  data?: unknown;
};

export async function getWebsiteSessionState(
  appConsoleOrigin: string,
  fetcher: typeof fetch = fetch
): Promise<WebsiteSessionState> {
  try {
    const response = await fetcher(`${appConsoleOrigin}/api/user/self`, {
      credentials: "include",
      cache: "no-store",
    });

    if (!response.ok) {
      return { authenticated: false };
    }

    const payload = (await response.json()) as SelfResponse;
    return { authenticated: payload.success === true && Boolean(payload.data) };
  } catch {
    return { authenticated: false };
  }
}
