import { SITE_ORIGIN } from "./origins";

export const STATUS_REVALIDATE_SECONDS = 60;
export const STATUS_OVERALL_VALUES = [
  "major_outage",
  "some_systems_affected",
  "degraded_performance",
  "monitoring_incomplete",
  "maintenance",
  "all_systems_operational",
] as const;

export type StatusValue = "operational" | "degraded" | "outage" | "unknown" | "maintenance";
export type StatusOverallValue = (typeof STATUS_OVERALL_VALUES)[number];
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

export type StatusSummaryResult = {
  state: "fresh" | "stale" | "monitoring-unavailable";
  data: StatusSummary;
};

type NextRevalidateRequestInit = RequestInit & { next: { revalidate: number } };
type StatusDataGuard<T> = (value: unknown) => value is T;

const STATUS_VALUES = ["operational", "degraded", "outage", "unknown", "maintenance"] as const;
const STATUS_HISTORY_RANGES = ["24h", "7d", "30d", "90d"] as const;
const MAX_FUTURE_SKEW_SECONDS = 60;
const MAX_MICROS = 1_000_000;

export async function fetchStatusSummary(): Promise<StatusSummaryResult> {
  const result = await fetchStatusData("/api/status/summary", isStatusSummary);
  if (result.state === "monitoring-unavailable") {
    return { state: "monitoring-unavailable", data: unavailableStatusSummary() };
  }
  if (result.state === "stale") {
    return { state: "stale", data: { ...result.data, status: "monitoring_incomplete" } };
  }
  return result;
}

export function fetchStatusComponents(query: StatusComponentQuery = {}): Promise<StatusApiResult<StatusComponentsData>> {
  const search = new URLSearchParams();
  if (query.kind) search.set("kind", query.kind);
  if (query.query) search.set("query", query.query);
  if (query.capability) search.set("capability", query.capability);
  if (query.status) search.set("status", query.status);
  return fetchStatusData(withSearch("/api/status/components", search), isStatusComponentsData);
}

export function fetchStatusComponent(slug: string): Promise<StatusApiResult<StatusComponentData>> {
  return fetchStatusData(`/api/status/components/${encodeURIComponent(slug)}`, isStatusComponentData);
}

export function fetchStatusComponentHistory(
  slug: string,
  range: StatusHistoryRange = "24h"
): Promise<StatusApiResult<StatusComponentHistoryData>> {
  const search = new URLSearchParams({ range });
  return fetchStatusData(withSearch(`/api/status/components/${encodeURIComponent(slug)}/history`, search), isStatusComponentHistoryData);
}

export function fetchStatusIncidents(): Promise<StatusApiResult<StatusIncidentsData>> {
  return fetchStatusData("/api/status/incidents", isStatusIncidentsData);
}

export function fetchStatusIncident(id: string): Promise<StatusApiResult<StatusIncidentData>> {
  return fetchStatusData(`/api/status/incidents/${encodeURIComponent(id)}`, isStatusIncidentData);
}

export function fetchStatusMaintenance(): Promise<StatusApiResult<StatusMaintenanceData>> {
  return fetchStatusData("/api/status/maintenance", isStatusMaintenanceData);
}

export function subscribeToStatus(input: StatusSubscriptionInput): Promise<StatusApiResult<StatusSubscriptionResponse>> {
  return mutateStatus("/api/status/subscriptions", input, isStatusSubscriptionResponse);
}

export function verifyStatusSubscription(token: string): Promise<StatusApiResult<StatusSubscriptionResponse>> {
  return fetchStatusData(withSearch("/api/status/subscriptions/verify", new URLSearchParams({ token })), isStatusSubscriptionResponse);
}

export function previewStatusUnsubscribe(token: string): Promise<StatusApiResult<StatusUnsubscribePreview>> {
  return fetchStatusData(withSearch("/api/status/subscriptions/unsubscribe", new URLSearchParams({ token })), isStatusUnsubscribePreview);
}

export function unsubscribeFromStatus(input: StatusUnsubscribeInput): Promise<StatusApiResult<StatusSubscriptionResponse>> {
  return mutateStatus("/api/status/subscriptions/unsubscribe", input, isStatusSubscriptionResponse);
}

async function fetchStatusData<T>(path: string, guard: StatusDataGuard<T>): Promise<StatusApiResult<T>> {
  const init: NextRevalidateRequestInit = {
    headers: { accept: "application/json" },
    next: { revalidate: STATUS_REVALIDATE_SECONDS },
  };
  return requestStatus(path, init, guard);
}

async function mutateStatus<T>(path: string, input: object, guard: StatusDataGuard<T>): Promise<StatusApiResult<T>> {
  return requestStatus(path, {
    method: "POST",
    headers: { accept: "application/json", "content-type": "application/json" },
    body: JSON.stringify(input),
    cache: "no-store",
  }, guard);
}

async function requestStatus<T>(path: string, init: RequestInit, guard: StatusDataGuard<T>): Promise<StatusApiResult<T>> {
  try {
    const response = await fetch(statusRequestUrl(path), init);
    if (!response.ok) return unavailable();
    const envelope: unknown = await response.json();
    if (!isSuccessfulEnvelope(envelope, guard)) return unavailable();
    return {
      state: isStale(envelope.data) ? "stale" : "fresh",
      data: envelope.data,
    };
  } catch {
    return unavailable();
  }
}

function statusRequestUrl(path: string): string {
  return typeof window === "undefined" ? new URL(path, SITE_ORIGIN).toString() : path;
}

function isStale(data: unknown): boolean {
  if (!data || typeof data !== "object" || !("generated_at" in data)) return false;
  const generatedAt = data.generated_at;
  return typeof generatedAt === "number" && Math.floor(Date.now() / 1000) - generatedAt > STATUS_REVALIDATE_SECONDS;
}

function isStatusOverallValue(value: unknown): value is StatusOverallValue {
  return typeof value === "string" && STATUS_OVERALL_VALUES.includes(value as StatusOverallValue);
}

function isSuccessfulEnvelope<T>(value: unknown, guard: StatusDataGuard<T>): value is StatusApiEnvelope<T> {
  return isRecord(value) && value.success === true && isOptionalString(value.message) && guard(value.data);
}

function isStatusSummary(value: unknown): value is StatusSummary {
  return hasStatusMetadata(value) && isStatusOverallValue(value.status) && isOptionalString(value.message) &&
    isArrayOf(value.components, isStatusComponent);
}

function isStatusComponentsData(value: unknown): value is StatusComponentsData {
  return hasStatusMetadata(value) && isArrayOf(value.components, isStatusComponent);
}

function isStatusComponentData(value: unknown): value is StatusComponentData {
  return hasStatusMetadata(value) && isStatusComponent(value.component);
}

function isStatusComponentHistoryData(value: unknown): value is StatusComponentHistoryData {
  return hasStatusMetadata(value) && isStatusComponent(value.component) && isStatusHistoryRange(value.range) &&
    isStatusAvailability(value.availability) && isArrayOf(value.periods, isStatusPeriod);
}

function isStatusIncidentsData(value: unknown): value is StatusIncidentsData {
  return hasStatusMetadata(value) && isArrayOf(value.incidents, (incident) => isStatusIncident(incident, "incident"));
}

function isStatusIncidentData(value: unknown): value is StatusIncidentData {
  return hasStatusMetadata(value) && isStatusIncident(value.incident, "incident");
}

function isStatusMaintenanceData(value: unknown): value is StatusMaintenanceData {
  return hasStatusMetadata(value) && isArrayOf(value.maintenance, (incident) => isStatusIncident(incident, "maintenance"));
}

function isStatusSubscriptionResponse(value: unknown): value is StatusSubscriptionResponse {
  return isRecord(value) && typeof value.message === "string";
}

function isStatusUnsubscribePreview(value: unknown): value is StatusUnsubscribePreview {
  return isRecord(value) && typeof value.can_unsubscribe === "boolean" && isStatusSubscriptionResponse(value);
}

function hasStatusMetadata(value: unknown): value is Record<string, unknown> & StatusMetadata {
  return isRecord(value) && isBoundedEvidenceTimestamp(value.generated_at) &&
    isBoundedEvidenceTimestamp(value.last_trustworthy_update_at) && isMicros(value.coverage);
}

function isStatusComponent(value: unknown): value is StatusComponent {
  return isRecord(value) && isPositiveInteger(value.id) && isNonEmptyString(value.slug) &&
    (value.kind === "router" || value.kind === "model") && isNonEmptyString(value.display_name) &&
    isOptionalString(value.capability) && (value.lifecycle === "active" || value.lifecycle === "retired") &&
    isStatusValue(value.status) && isBoundedEvidenceTimestamp(value.last_trustworthy_update_at) &&
    isMicros(value.coverage);
}

function isStatusAvailability(value: unknown): value is StatusAvailability {
  return isRecord(value) && isMicros(value.availability_micros) && isMicros(value.coverage_micros) &&
    isNonNegativeInteger(value.known_bucket_count) && isNonNegativeInteger(value.unknown_bucket_count) &&
    isNonNegativeInteger(value.maintenance_bucket_count);
}

function isStatusPeriod(value: unknown): value is StatusPeriod {
  return isRecord(value) && isTimestamp(value.period_start) && isMicros(value.availability) &&
    isMicros(value.coverage) && isStatusValue(value.status) &&
    (value.maintenance_count === undefined || isNonNegativeInteger(value.maintenance_count));
}

function isStatusIncident(value: unknown, expectedKind: StatusIncident["kind"]): value is StatusIncident {
  return isRecord(value) && isNonEmptyString(value.id) && value.kind === expectedKind &&
    typeof value.title === "string" && typeof value.impact === "string" && typeof value.status === "string" &&
    isOptionalTimestamp(value.scheduled_start_at) && isOptionalTimestamp(value.scheduled_end_at) &&
    isOptionalTimestamp(value.started_at) && isOptionalTimestamp(value.resolved_at) && isTimestamp(value.updated_at) &&
    isArrayOf(value.component_ids, isPositiveInteger) && isArrayOf(value.updates, isStatusIncidentUpdate);
}

function isStatusIncidentUpdate(value: unknown): value is StatusIncidentUpdate {
  return isRecord(value) && typeof value.state === "string" && typeof value.body === "string" &&
    isTimestamp(value.published_at);
}

function isStatusValue(value: unknown): value is StatusValue {
  return typeof value === "string" && STATUS_VALUES.includes(value as StatusValue);
}

function isStatusHistoryRange(value: unknown): value is StatusHistoryRange {
  return typeof value === "string" && STATUS_HISTORY_RANGES.includes(value as StatusHistoryRange);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isArrayOf<T>(value: unknown, guard: StatusDataGuard<T>): value is T[] {
  return Array.isArray(value) && value.every(guard);
}

function isOptionalString(value: unknown): value is string | undefined {
  return value === undefined || typeof value === "string";
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}

function isNonNegativeInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isSafeInteger(value) && value >= 0;
}

function isPositiveInteger(value: unknown): value is number {
  return isNonNegativeInteger(value) && value > 0;
}

function isTimestamp(value: unknown): value is number {
  return isNonNegativeInteger(value);
}

function isOptionalTimestamp(value: unknown): value is number | undefined {
  return value === undefined || isTimestamp(value);
}

function isBoundedEvidenceTimestamp(value: unknown): value is number {
  return isTimestamp(value) && value <= Math.floor(Date.now() / 1000) + MAX_FUTURE_SKEW_SECONDS;
}

function isMicros(value: unknown): value is number {
  return isNonNegativeInteger(value) && value <= MAX_MICROS;
}

function unavailableStatusSummary(): StatusSummary {
  return {
    generated_at: 0,
    last_trustworthy_update_at: 0,
    coverage: 0,
    status: "monitoring_incomplete",
    components: [],
  };
}

function unavailable<T>(): StatusApiResult<T> {
  return { state: "monitoring-unavailable", data: null };
}

function withSearch(path: string, search: URLSearchParams): string {
  const query = search.toString();
  return query ? `${path}?${query}` : path;
}
