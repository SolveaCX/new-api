export const STATUS_REVALIDATE_SECONDS = 60;

export type StatusValue = "operational" | "degraded" | "outage" | "unknown" | "maintenance";
export type StatusOverallValue = StatusValue | "monitoring_incomplete";
export type StatusHistoryRange = "24h" | "7d" | "30d" | "90d";

export interface StatusApiEnvelope<T> {
  success: boolean;
  message?: string;
  data: T;
}

export interface StatusMetadata {
  generated_at: number;
  last_trustworthy_update_at: number;
  coverage: number;
}

export interface StatusComponent {
  id: number;
  slug: string;
  kind: string;
  display_name: string;
  capability?: string;
  lifecycle: string;
  status: StatusValue;
  last_trustworthy_update_at: number;
  coverage: number;
}

export interface StatusSummary extends StatusMetadata {
  status: StatusOverallValue;
  message?: string;
  components: StatusComponent[];
}

export interface StatusComponentsData extends StatusMetadata {
  components: StatusComponent[];
}

export interface StatusComponentData extends StatusMetadata {
  component: StatusComponent;
}

export interface StatusAvailability {
  availability_micros: number;
  coverage_micros: number;
  known_bucket_count: number;
  unknown_bucket_count: number;
  maintenance_bucket_count: number;
}

export interface StatusPeriod {
  period_start: number;
  availability: number;
  coverage: number;
  status: StatusValue;
  maintenance_count?: number;
}

export interface StatusComponentHistoryData extends StatusMetadata {
  component: StatusComponent;
  range: StatusHistoryRange;
  availability: StatusAvailability;
  periods: StatusPeriod[];
}

export interface StatusIncidentUpdate {
  state: string;
  body: string;
  published_at: number;
}

export interface StatusIncident {
  id: string;
  kind: "incident" | "maintenance";
  title: string;
  impact: string;
  status: string;
  scheduled_start_at?: number;
  scheduled_end_at?: number;
  started_at?: number;
  resolved_at?: number;
  updated_at: number;
  component_ids: number[];
  updates: StatusIncidentUpdate[];
}

export interface StatusIncidentsData extends StatusMetadata {
  incidents: StatusIncident[];
}

export interface StatusIncidentData extends StatusMetadata {
  incident: StatusIncident;
}

export interface StatusMaintenanceData extends StatusMetadata {
  maintenance: StatusIncident[];
}

export interface StatusSubscriptionInput {
  email: string;
  component_ids: number[];
}

export interface StatusUnsubscribeInput {
  token: string;
}

export interface StatusSubscriptionResponse {
  message: string;
}

export interface StatusUnsubscribePreview extends StatusSubscriptionResponse {
  can_unsubscribe: boolean;
}

export interface StatusComponentQuery {
  kind?: "router" | "model";
  query?: string;
  capability?: string;
  status?: StatusValue;
}

export type StatusApiResult<T> =
  | { state: "fresh" | "stale"; data: T }
  | { state: "monitoring-unavailable"; data: null };

type NextRevalidateRequestInit = RequestInit & { next: { revalidate: number } };

export function fetchStatusSummary(): Promise<StatusApiResult<StatusSummary>> {
  return fetchStatusData("/api/status/summary");
}

export function fetchStatusComponents(query: StatusComponentQuery = {}): Promise<StatusApiResult<StatusComponentsData>> {
  const search = new URLSearchParams();
  if (query.kind) search.set("kind", query.kind);
  if (query.query) search.set("query", query.query);
  if (query.capability) search.set("capability", query.capability);
  if (query.status) search.set("status", query.status);
  return fetchStatusData(withSearch("/api/status/components", search));
}

export function fetchStatusComponent(slug: string): Promise<StatusApiResult<StatusComponentData>> {
  return fetchStatusData(`/api/status/components/${encodeURIComponent(slug)}`);
}

export function fetchStatusComponentHistory(
  slug: string,
  range: StatusHistoryRange = "24h"
): Promise<StatusApiResult<StatusComponentHistoryData>> {
  const search = new URLSearchParams({ range });
  return fetchStatusData(withSearch(`/api/status/components/${encodeURIComponent(slug)}/history`, search));
}

export function fetchStatusIncidents(): Promise<StatusApiResult<StatusIncidentsData>> {
  return fetchStatusData("/api/status/incidents");
}

export function fetchStatusIncident(id: string): Promise<StatusApiResult<StatusIncidentData>> {
  return fetchStatusData(`/api/status/incidents/${encodeURIComponent(id)}`);
}

export function fetchStatusMaintenance(): Promise<StatusApiResult<StatusMaintenanceData>> {
  return fetchStatusData("/api/status/maintenance");
}

export function subscribeToStatus(input: StatusSubscriptionInput): Promise<StatusApiResult<StatusSubscriptionResponse>> {
  return mutateStatus("/api/status/subscriptions", input);
}

export function verifyStatusSubscription(token: string): Promise<StatusApiResult<StatusSubscriptionResponse>> {
  return fetchStatusData(withSearch("/api/status/subscriptions/verify", new URLSearchParams({ token })));
}

export function previewStatusUnsubscribe(token: string): Promise<StatusApiResult<StatusUnsubscribePreview>> {
  return fetchStatusData(withSearch("/api/status/subscriptions/unsubscribe", new URLSearchParams({ token })));
}

export function unsubscribeFromStatus(input: StatusUnsubscribeInput): Promise<StatusApiResult<StatusSubscriptionResponse>> {
  return mutateStatus("/api/status/subscriptions/unsubscribe", input);
}

async function fetchStatusData<T>(path: string): Promise<StatusApiResult<T>> {
  const init: NextRevalidateRequestInit = {
    headers: { accept: "application/json" },
    next: { revalidate: STATUS_REVALIDATE_SECONDS },
  };
  return requestStatus(path, init);
}

async function mutateStatus<T>(path: string, input: object): Promise<StatusApiResult<T>> {
  return requestStatus(path, {
    method: "POST",
    headers: { accept: "application/json", "content-type": "application/json" },
    body: JSON.stringify(input),
    cache: "no-store",
  });
}

async function requestStatus<T>(path: string, init: RequestInit): Promise<StatusApiResult<T>> {
  try {
    const response = await fetch(path, init);
    if (!response.ok) return unavailable();
    const envelope = (await response.json()) as Partial<StatusApiEnvelope<T>>;
    if (envelope.success !== true || envelope.data == null) return unavailable();
    return {
      state: isStale(envelope.data) ? "stale" : "fresh",
      data: envelope.data,
    };
  } catch {
    return unavailable();
  }
}

function isStale(data: unknown): boolean {
  if (!data || typeof data !== "object" || !("generated_at" in data)) return false;
  const generatedAt = data.generated_at;
  return typeof generatedAt === "number" && Math.floor(Date.now() / 1000) - generatedAt > STATUS_REVALIDATE_SECONDS;
}

function unavailable<T>(): StatusApiResult<T> {
  return { state: "monitoring-unavailable", data: null };
}

function withSearch(path: string, search: URLSearchParams): string {
  const query = search.toString();
  return query ? `${path}?${query}` : path;
}
