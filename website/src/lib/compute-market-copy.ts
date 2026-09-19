import type { Locale } from "@/lib/locales";
import type { DemandBoardCopy } from "@/components/compute-demand-board";
import type { QuickRequestCopy } from "@/components/compute-quick-request";
import type { PoolStripCopy } from "@/components/compute-pool-strip";

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
  quick: QuickRequestCopy;
  pools: PoolStripCopy;
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
    ctaQuick: string;
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
    featured: "Featured",
  },
  quick: {
    title: "Post your GPU need in 3 seconds",
    hint: "No account. Three fields. Your request joins the pool for that GPU and flatkey negotiates one large order for everyone.",
    pasteLabel: "Or paste a sentence and we fill it in",
    pastePlaceholder: "e.g. need 16 H200 for 6 months, US West, start October",
    gpuLabel: "GPU",
    gpusLabel: "GPUs",
    termLabel: "Term",
    termUnit: "mo",
    contactLabel: "Where should we reach you?",
    contactPlaceholder: "email, phone or WeChat",
    contactNote: "Seen only by flatkey. Never shown to suppliers.",
    submit: "Join the pool →",
    submitting: "Submitting…",
    refBanner: "Invited by #{code} · you both get flatkey credits when the pool closes",
    doneTitle: "You're in the pool.",
    doneBody: "We'll reach out within one business day with the pooled price. Bring peers in and the whole pool moves down a tier.",
    poolLine: "{gpu} pool: {gpus} GPUs from {n} requests",
    poolNext: "{need} more GPUs unlock the next tier (~{pct}% under on-demand)",
    poolTop: "Top tier reached (~{pct}% under on-demand)",
    shareTitle: "Grow your pool",
    shareBody: "Everyone who joins through your link adds to your pool. Each referral earns you both $50 in flatkey credits once the pool closes.",
    shareCopy: "Copy link",
    shareCopied: "Copied ✓",
    shareX: "Share on X",
    shareText: "Pooling {gpu} demand on flatkey ({gpus} GPUs so far). Add yours and we all get a lower price: {url}",
    referrals: "{n} joined through your link",
    another: "Post another request",
    error: "Could not submit right now. Please try again or email support@flatkey.ai.",
  },
  pools: {
    title: "Demand pools right now",
    body: "Small orders pooled per GPU. Every tier the pool crosses lowers the price for everyone in it.",
    pooled: "GPUs pooled",
    requests: "{n} requests",
    next: "{need} to next tier",
    top: "top tier",
    tierLabel: "~{pct}% off",
    pooling: "pooling",
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
    ctaQuick: "Post a need in 3 seconds",
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
    featured: "精选",
  },
  quick: {
    title: "3 秒发布你的 GPU 需求",
    hint: "不用注册，三个字段。你的需求会进入该型号的拼单池，flatkey 把小单凑成大单去谈价。",
    pasteLabel: "或者粘贴一句话，我们自动填",
    pastePlaceholder: "例如：要 16 张 H200，租 6 个月，美西，10 月开始",
    gpuLabel: "GPU 型号",
    gpusLabel: "卡数",
    termLabel: "租期",
    termUnit: "个月",
    contactLabel: "怎么联系你？",
    contactPlaceholder: "邮箱 / 手机 / 微信",
    contactNote: "只有 flatkey 能看到，绝不展示给供给方。",
    submit: "加入拼单 →",
    submitting: "提交中…",
    refBanner: "由 #{code} 邀请 · 拼单成交后你们两人都获得 flatkey 额度",
    doneTitle: "已进入拼单池。",
    doneBody: "一个工作日内我们会带着拼单价联系你。拉同行进来，整个池子一起降一档。",
    poolLine: "{gpu} 拼单池：{n} 个需求，共 {gpus} 张",
    poolNext: "再凑 {need} 张进入下一档（约低于按需价 {pct}%）",
    poolTop: "已到最高档（约低于按需价 {pct}%）",
    shareTitle: "把池子做大",
    shareBody: "通过你的链接加入的需求都算进你的池子。每成功邀请一位，拼单成交后你们双方各得 $50 flatkey 额度。",
    shareCopy: "复制链接",
    shareCopied: "已复制 ✓",
    shareX: "分享到 X",
    shareText: "我在 flatkey 拼 {gpu} 算力（已凑 {gpus} 张），加进来一起拿更低的价：{url}",
    referrals: "已有 {n} 人通过你的链接加入",
    another: "再发一条需求",
    error: "暂时无法提交，请重试或发邮件到 support@flatkey.ai。",
  },
  pools: {
    title: "正在拼的算力池",
    body: "按 GPU 型号把小单凑成大单。池子每跨过一档，池内所有人的价格一起降。",
    pooled: "张已凑",
    requests: "{n} 个需求",
    next: "再凑 {need} 张升档",
    top: "最高档",
    tierLabel: "约省 {pct}%",
    pooling: "拼单中",
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
    ctaQuick: "3 秒发需求",
    ctaSupply: "我有 GPU，想接单",
    note: "昵称均为匿名，加入平台后可见正式信息",
  },
};

export function getComputeMarketCopy(locale: Locale): ComputeMarketCopy {
  return locale === "zh" ? zh : en;
}
