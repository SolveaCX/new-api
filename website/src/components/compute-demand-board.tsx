"use client";

import { useEffect, useMemo, useState } from "react";
import {
  type DemandRow,
  annualValue,
  createSeededRandom,
  formatRemaining,
  formatUsdCompact,
  generateDemandRow,
  summarizeDemand,
  totalGpus,
} from "@/lib/compute-demand";

export type DemandBoardCopy = {
  matching: string;
  bidsSuffix: string;
  ending: string;
  matched: string;
  delivered: string;
  lowest: string;
  awaitingBid: string;
  months: string;
  annual: string;
  anonTitle: string;
  anonBody: string;
  anonCta: string;
  bid: string;
  minutesAgo: string;
  hoursAgo: string;
  colRequest: string;
  colSpec: string;
  colTerm: string;
  colPrice: string;
  colStatus: string;
  colBuyer: string;
  colValue: string;
  perHour: string;
  liveNote: string;
  allLabel: string;
};

type Props = {
  rows: DemandRow[];
  variant: "ticker" | "table";
  copy: DemandBoardCopy;
  bidHref: string;
  joinHref: string;
  /** Add a synthetic new request every N ms after mount (0 = off). */
  liveIntervalMs?: number;
  seed?: number;
  /** Rows shown in the table variant; the rest live in the console marketplace. */
  maxRows?: number;
  allHref?: string;
  allLabel?: string;
};

const LIVE_SEED_OFFSET = 9000;

function StatusPill({ row, copy }: { row: DemandRow; copy: DemandBoardCopy }) {
  if (row.status === "matching") {
    const ending = row.remainingMinutes < 60;
    return (
      <span className={`cm-pill${ending ? " cm-pill-hot" : ""}`}>
        <i />
        {ending ? `${copy.ending} ${formatRemaining(row.remainingMinutes)}` : copy.matching} · {row.bids} {copy.bidsSuffix}
      </span>
    );
  }
  if (row.status === "matched") {
    return (
      <span className="cm-pill cm-pill-matched">
        <i />
        {copy.matched} ${row.lowest.toFixed(2)}
      </span>
    );
  }
  return (
    <span className="cm-pill cm-pill-done">
      <i />
      {copy.delivered}
    </span>
  );
}

function Nickname({ row, copy, joinHref }: { row: DemandRow; copy: DemandBoardCopy; joinHref: string }) {
  return (
    <span className="cm-nick" tabIndex={0}>
      <span className="cm-av" aria-hidden>
        {row.nickname[0]}
      </span>
      {row.nickname}
      <small> · {row.kind}</small>
      <span className="cm-tip" role="tooltip">
        <b>{copy.anonTitle}</b>
        {copy.anonBody}
        <a href={joinHref}>{copy.anonCta} →</a>
      </span>
    </span>
  );
}

export function ComputeDemandBoard(props: Props) {
  const { copy } = props;
  const [rows, setRows] = useState<DemandRow[]>(props.rows);
  const [freshId, setFreshId] = useState<number | null>(null);
  const [paused, setPaused] = useState(false);

  useEffect(() => {
    if (!props.liveIntervalMs) return;
    const random = createSeededRandom((props.seed ?? 7) + LIVE_SEED_OFFSET + Date.now() % 1000);
    let next = LIVE_SEED_OFFSET;
    const timer = window.setInterval(() => {
      if (paused) return;
      const fresh = generateDemandRow(next++, random);
      fresh.status = "matching";
      fresh.bids = 0;
      fresh.lowest = 0;
      fresh.ageMinutes = 0;
      fresh.remainingMinutes = 24 * 60;
      setRows((current) => [fresh, ...current.map((r) => ({ ...r, ageMinutes: r.ageMinutes + 1 }))].slice(0, 48));
      setFreshId(fresh.id);
    }, props.liveIntervalMs);
    return () => window.clearInterval(timer);
  }, [props.liveIntervalMs, props.seed, paused]);

  const stats = useMemo(() => summarizeDemand(rows), [rows]);
  const age = (m: number) => (m < 60 ? `${m} ${copy.minutesAgo}` : `${Math.floor(m / 60)} ${copy.hoursAgo}`);

  if (props.variant === "ticker") {
    const chips = rows.slice(0, 24);
    const lane = (list: DemandRow[], reverse: boolean) => (
      <div className={`cm-lane${reverse ? " cm-lane-rev" : ""}`} aria-hidden={reverse}>
        <div className="cm-track">
          {[...list, ...list].map((row, i) => (
            <span className="cm-chip" key={`${row.id}-${i}`}>
              <b>
                {row.gpu} × {row.nodes}
              </b>
              <small>
                {row.region} · {row.termMonths} {copy.months}
              </small>
              <em>≤ ${row.ceiling.toFixed(2)}{copy.perHour}</em>
              <StatusPill row={row} copy={copy} />
              <small>{row.nickname}</small>
            </span>
          ))}
        </div>
      </div>
    );
    return (
      <div className="cm-ticker" onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)}>
        {lane(chips.slice(0, 12), false)}
        {lane(chips.slice(12), true)}
      </div>
    );
  }

  return (
    <div className="cm-board" onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)}>
      <div className="cm-board-head">
        <div>
          <span className="cm-kick">LIVE · {stats.matching} {copy.matching.toLowerCase()}</span>
          <strong>
            {formatUsdCompact(stats.openValue)} {copy.annual}
          </strong>
        </div>
        <span className="cm-note">{copy.liveNote}</span>
      </div>
      <div className="cm-table-wrap">
        <table className="cm-table">
          <thead>
            <tr>
              <th>{copy.colRequest}</th>
              <th>{copy.colSpec}</th>
              <th>{copy.colTerm}</th>
              <th>{copy.colPrice}</th>
              <th>{copy.colStatus}</th>
              <th>{copy.colBuyer}</th>
              <th>{copy.colValue}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {rows.slice(0, props.maxRows ?? rows.length).map((row) => (
              <tr key={row.id} className={row.id === freshId ? "cm-fresh" : undefined}>
                <td>
                  <b>RFQ-{row.id}</b>
                  <small>{age(row.ageMinutes)}</small>
                </td>
                <td>
                  <b>
                    {row.gpu} × {row.nodes}
                  </b>{" "}
                  <small>= {totalGpus(row)} GPU</small>
                  <small>{row.delivery}</small>
                </td>
                <td>
                  {row.termMonths} {copy.months}
                  <small>{row.region}</small>
                </td>
                <td>
                  <em>≤ ${row.ceiling.toFixed(2)}</em>
                  <small>{row.lowest ? `${copy.lowest} $${row.lowest.toFixed(2)}` : copy.awaitingBid}</small>
                </td>
                <td>
                  <StatusPill row={row} copy={copy} />
                </td>
                <td>
                  <Nickname row={row} copy={copy} joinHref={props.joinHref} />
                </td>
                <td>
                  <b>{formatUsdCompact(annualValue(row))}</b>
                </td>
                <td>
                  <a className="cm-bid" href={props.bidHref}>
                    {copy.bid}
                  </a>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {props.allHref && props.allLabel && (
        <div className="cm-board-foot">
          <span>{rows.length}+</span>
          <a href={props.allHref}>{props.allLabel}</a>
        </div>
      )}
    </div>
  );
}
