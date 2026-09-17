import type { Locale } from "@/lib/locales";
import type { DemandBoardCopy } from "@/components/compute-demand-board";

export type ComputeMarketCopy = {
  metaTitle: string;
  metaDescription: string;
  kicker: string;
  title: string;
  subtitle: string;
  ctaPost: string;
  ctaSupply: string;
  indexNote: string;
  boardTitle: string;
  supplyTitle: string;
  supplyBody: string;
  supplyCta: string;
  demandTitle: string;
  demandBody: string;
  demandCta: string;
  whyTitle: string;
  why: string[];
  footNote: string;
  board: DemandBoardCopy;
  home: {
    kicker: string;
    title: string;
    body: string;
    statOpen: string;
    statMatching: string;
    statBids: string;
    statMatched: string;
    ctaAll: string;
    ctaSupply: string;
    note: string;
  };
};

const en: ComputeMarketCopy = {
  metaTitle: "Flatkey Compute Market - Post GPU requests, suppliers bid, flatkey escrows",
  metaDescription:
    "The demand side of AI infra: post an anonymous GPU request, verified suppliers bid publicly for 24 hours, pick from the lowest three, flatkey holds the contract and escrow.",
  kicker: "FLATKEY COMPUTE · THE DEMAND SIDE OF AI INFRA",
  title: "Where AI infra demand meets supply.",
  subtitle:
    "The largest place to post and take on GPU capacity in the AI infra era. Buyers post anonymously, verified suppliers bid publicly for 24 hours, buyers pick from the lowest three, and flatkey signs one contract and holds the escrow.",
  ctaPost: "Post a compute request",
  ctaSupply: "I have GPUs · take orders",
  indexNote: "index updated daily",
  boardTitle: "Requests looking for capacity right now",
  supplyTitle: "I have GPUs and want orders",
  supplyBody: "Free to register. Verify one node and bid on any request; after a match flatkey pays out monthly through escrow.",
  supplyCta: "Become a supplier →",
  demandTitle: "I need capacity",
  demandBody: "Paste a paragraph and AI fills the form. Verified suppliers bid within 24 hours. You stay anonymous.",
  demandCta: "Post a request →",
  whyTitle: "Why trade here",
  why: [
    "Both sides stay anonymous; contact details never cross the market",
    "Bids are public and only go down; last-15-minute bids extend the window",
    "One contract with flatkey; first month + deposit held in escrow",
    "Released only after acceptance tests; SLA shortfalls are deducted",
  ],
  footNote: "Nicknames are anonymous. Company and contact details are visible only after you join the platform and pass supplier verification.",
  board: {
    matching: "Matching",
    bidsSuffix: "bids",
    ending: "Ends in",
    matched: "Matched",
    delivered: "Delivered",
    lowest: "lowest",
    awaitingBid: "awaiting first bid",
    months: "mo",
    annual: "open demand / yr",
    anonTitle: "Anonymous buyer",
    anonBody: "Buyers appear as nicknames. Company, contact and target price are visible only to verified suppliers on flatkey.",
    anonCta: "Join flatkey Compute",
    bid: "Bid",
    minutesAgo: "min ago",
    hoursAgo: "h ago",
    colRequest: "Request",
    colSpec: "Spec",
    colTerm: "Term · region",
    colPrice: "Ceiling / lowest",
    colStatus: "Status",
    colBuyer: "Buyer",
    colValue: "Per year",
    perHour: "/h",
    liveNote: "New requests appear at the top · anonymous nicknames · hover for details",
    allLabel: "Open the full marketplace in the console →",
  },
  home: {
    kicker: "COMPUTE · LIVE DEMAND BOARD",
    title: "Models, tools — and the GPUs behind them.",
    body: "flatkey does not stop at routing models and tools. AI companies post GPU requests here, verified suppliers bid, and flatkey holds the escrow. These requests are matching right now.",
    statOpen: "open demand · per year",
    statMatching: "requests matching",
    statBids: "supplier bids · 24h",
    statMatched: "matched this week",
    ctaAll: "See all requests →",
    ctaSupply: "I have GPUs, take orders",
    note: "Nicknames are anonymous · full details after you join",
  },
};

const zh: ComputeMarketCopy = {
  metaTitle: "Flatkey 算力市场 - 发布 GPU 需求，供给方竞价，flatkey 担保",
  metaDescription: "AI Infra 时代的算力需求侧：匿名发布 GPU 需求，认证供给方 24 小时公开竞价，从最低 3 家里选，flatkey 一份合同、托管放款。",
  kicker: "FLATKEY COMPUTE · AI INFRA 的需求侧",
  title: "AI 需要的算力，在这里发布，在这里承接。",
  subtitle: "AI Infra 时代最大的算力发需求 / 承接平台。需求方匿名发单，认证供给方 24 小时公开竞价，需求方从最低 3 家里选，flatkey 一份合同、托管放款。",
  ctaPost: "发布算力需求",
  ctaSupply: "我有 GPU · 接单",
  indexNote: "指数每日更新",
  boardTitle: "此刻正在寻找算力的需求",
  supplyTitle: "我有 GPU，想接单",
  supplyBody: "注册免费。验证一台节点后即可对任意需求单出价，撮合后 flatkey 按月托管放款。",
  supplyCta: "成为供给方 →",
  demandTitle: "我要租算力",
  demandBody: "粘贴一段话，AI 自动填表；24 小时内拿到认证供给方报价，全程匿名。",
  demandCta: "发布需求单 →",
  whyTitle: "为什么在这里交易",
  why: ["双方匿名，联系方式不经过市场", "报价公开只降不升，最后 15 分钟自动延长", "一份合同对 flatkey，首月 + 押金托管", "验收通过才放款，SLA 不达标扣减"],
  footNote: "昵称均为匿名。公司名与联系人只有加入平台并通过供给方认证后才能看到。",
  board: {
    matching: "匹配中",
    bidsSuffix: "家出价",
    ending: "最后",
    matched: "已撮合",
    delivered: "已交付",
    lowest: "最低",
    awaitingBid: "等待首个报价",
    months: "个月",
    annual: "开放需求 · 年",
    anonTitle: "匿名需求方",
    anonBody: "需求方以匿名昵称展示。公司名、联系人和心理价只有加入平台并通过认证的供给方才能看到。",
    anonCta: "加入 flatkey Compute",
    bid: "出价",
    minutesAgo: "分钟前",
    hoursAgo: "小时前",
    colRequest: "需求单",
    colSpec: "规格",
    colTerm: "期限 · 地区",
    colPrice: "上限 / 最低价",
    colStatus: "状态",
    colBuyer: "需求方",
    colValue: "年合同额",
    perHour: "/h",
    liveNote: "新需求自动置顶 · 匿名昵称 · 悬停查看说明",
    allLabel: "在控制台查看全部需求 →",
  },
  home: {
    kicker: "COMPUTE · 实时需求板",
    title: "模型、工具，以及它们背后的 GPU。",
    body: "flatkey 不只路由模型和工具。AI 公司在这里发布算力需求，认证的算力方竞价承接，flatkey 担保交易。下面是此刻正在匹配的需求。",
    statOpen: "开放需求 · 年合同额",
    statMatching: "正在匹配的需求单",
    statBids: "供给方报价 · 24h",
    statMatched: "本周已撮合",
    ctaAll: "查看全部需求 →",
    ctaSupply: "我有 GPU，想接单",
    note: "昵称均为匿名，加入平台后可见正式信息",
  },
};

export function getComputeMarketCopy(locale: Locale): ComputeMarketCopy {
  return locale === "zh" ? zh : en;
}
