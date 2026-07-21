import type { Locale } from "./locales";
import type { StatusValue } from "./status";

export type StatusFreshness = "fresh" | "stale" | "monitoring-unavailable";

export interface StatusCopy {
  title: string;
  description: string;
  overallLabel: string;
  coverageLabel: string;
  lastTrustworthyUpdateLabel: string;
  componentsTitle: string;
  filters: {
    nameLabel: string;
    namePlaceholder: string;
    capabilityLabel: string;
    allCapabilities: string;
    statusLabel: string;
    allStatuses: string;
    applyLabel: string;
  };
  states: Record<StatusValue, string>;
  freshness: {
    fresh: string;
    stale: string;
    unavailable: string;
  };
  lifecycle: {
    retired: string;
  };
  history: {
    title: string;
    availabilityLabel: string;
    coverageLabel: string;
    incidentCountLabel: string;
    noEvidence: string;
    ranges: Record<"24h" | "7d" | "30d" | "90d", string>;
  };
  incidents: {
    title: string;
    empty: string;
  };
  maintenance: {
    title: string;
    empty: string;
  };
  subscribe: {
    title: string;
    description: string;
    emailLabel: string;
    emailPlaceholder: string;
    componentLegend: string;
    submitLabel: string;
    submittingLabel: string;
    successLabel: string;
    errorLabel: string;
  };
  model: {
    currentStatusLabel: string;
    evidenceLabel: string;
    backLabel: string;
  };
}

export interface StatusPresentationInput {
  locale: Locale;
  status: StatusValue;
  freshness: StatusFreshness;
  lifecycle: string;
}

export interface StatusPresentation {
  status: StatusValue;
  text: string;
  icon: "check" | "warning" | "outage" | "unknown" | "maintenance" | "retired";
  colorClass: string;
  barClass: string;
}

const sharedRanges = { "24h": "24h", "7d": "7d", "30d": "30d", "90d": "90d" } as const;

const statusCopy: Record<Exclude<Locale, "id">, StatusCopy> = {
  en: {
    title: "Flatkey status",
    description: "Current service health and trustworthy monitoring history for the Router and every public model.",
    overallLabel: "Overall status",
    coverageLabel: "Monitoring coverage",
    lastTrustworthyUpdateLabel: "Last trustworthy update",
    componentsTitle: "Services and models",
    filters: { nameLabel: "Name", namePlaceholder: "Search models", capabilityLabel: "Capability", allCapabilities: "All capabilities", statusLabel: "Status", allStatuses: "All statuses", applyLabel: "Apply filters" },
    states: { operational: "Operational", degraded: "Degraded performance", outage: "Outage", unknown: "Unknown", maintenance: "Maintenance" },
    freshness: { fresh: "Monitoring is current", stale: "Stale monitoring data", unavailable: "Monitoring unavailable" },
    lifecycle: { retired: "Retired" },
    history: { title: "Status history", availabilityLabel: "Availability", coverageLabel: "Monitoring coverage", incidentCountLabel: "Incident count", noEvidence: "No trustworthy monitoring data", ranges: sharedRanges },
    incidents: { title: "Recent incidents", empty: "No recent incidents" },
    maintenance: { title: "Scheduled maintenance", empty: "No scheduled maintenance" },
    subscribe: { title: "Status notifications", description: "Receive email updates for the services you select.", emailLabel: "Email address", emailPlaceholder: "you@example.com", componentLegend: "Services to follow", submitLabel: "Subscribe", submittingLabel: "Subscribing…", successLabel: "Check your email to confirm the subscription.", errorLabel: "Subscription could not be created. Try again." },
    model: { currentStatusLabel: "Current status", evidenceLabel: "Trustworthy evidence", backLabel: "Back to status overview" },
  },
  zh: {
    title: "Flatkey 服务状态",
    description: "查看 Router 与所有公开模型的当前健康状态和可信监控历史。",
    overallLabel: "总体状态",
    coverageLabel: "监控覆盖率",
    lastTrustworthyUpdateLabel: "最近可信更新时间",
    componentsTitle: "服务与模型",
    filters: { nameLabel: "名称", namePlaceholder: "搜索模型", capabilityLabel: "能力", allCapabilities: "全部能力", statusLabel: "状态", allStatuses: "全部状态", applyLabel: "应用筛选" },
    states: { operational: "运行正常", degraded: "性能下降", outage: "服务中断", unknown: "状态未知", maintenance: "维护中" },
    freshness: { fresh: "监控数据为最新", stale: "监控数据已过期", unavailable: "监控暂不可用" },
    lifecycle: { retired: "已退役" },
    history: { title: "状态历史", availabilityLabel: "可用率", coverageLabel: "监控覆盖率", incidentCountLabel: "事件数量", noEvidence: "暂无可信监控数据", ranges: sharedRanges },
    incidents: { title: "近期事件", empty: "近期没有事件" },
    maintenance: { title: "计划维护", empty: "暂无计划维护" },
    subscribe: { title: "状态通知", description: "通过电子邮件接收所选服务的更新。", emailLabel: "电子邮箱", emailPlaceholder: "you@example.com", componentLegend: "关注的服务", submitLabel: "订阅", submittingLabel: "正在订阅…", successLabel: "请查收邮件并确认订阅。", errorLabel: "无法创建订阅，请重试。" },
    model: { currentStatusLabel: "当前状态", evidenceLabel: "可信依据", backLabel: "返回状态总览" },
  },
  es: {
    title: "Estado de Flatkey",
    description: "Salud actual e historial de monitorización fiable del Router y de todos los modelos públicos.",
    overallLabel: "Estado general",
    coverageLabel: "Cobertura de monitorización",
    lastTrustworthyUpdateLabel: "Última actualización fiable",
    componentsTitle: "Servicios y modelos",
    filters: { nameLabel: "Nombre", namePlaceholder: "Buscar modelos", capabilityLabel: "Capacidad", allCapabilities: "Todas las capacidades", statusLabel: "Estado", allStatuses: "Todos los estados", applyLabel: "Aplicar filtros" },
    states: { operational: "Operativo", degraded: "Rendimiento degradado", outage: "Interrupción", unknown: "Desconocido", maintenance: "Mantenimiento" },
    freshness: { fresh: "La monitorización está actualizada", stale: "Datos de monitorización obsoletos", unavailable: "Monitorización no disponible" },
    lifecycle: { retired: "Retirado" },
    history: { title: "Historial de estado", availabilityLabel: "Disponibilidad", coverageLabel: "Cobertura de monitorización", incidentCountLabel: "Número de incidentes", noEvidence: "No hay datos fiables de monitorización", ranges: sharedRanges },
    incidents: { title: "Incidentes recientes", empty: "No hay incidentes recientes" },
    maintenance: { title: "Mantenimiento programado", empty: "No hay mantenimiento programado" },
    subscribe: { title: "Avisos de estado", description: "Recibe por correo las novedades de los servicios elegidos.", emailLabel: "Correo electrónico", emailPlaceholder: "tu@ejemplo.com", componentLegend: "Servicios que deseas seguir", submitLabel: "Suscribirse", submittingLabel: "Suscribiendo…", successLabel: "Revisa tu correo para confirmar la suscripción.", errorLabel: "No se pudo crear la suscripción. Inténtalo de nuevo." },
    model: { currentStatusLabel: "Estado actual", evidenceLabel: "Evidencia fiable", backLabel: "Volver al resumen de estado" },
  },
  fr: {
    title: "État de Flatkey",
    description: "Santé actuelle et historique de surveillance fiable du Router et de tous les modèles publics.",
    overallLabel: "État général",
    coverageLabel: "Couverture de surveillance",
    lastTrustworthyUpdateLabel: "Dernière mise à jour fiable",
    componentsTitle: "Services et modèles",
    filters: { nameLabel: "Nom", namePlaceholder: "Rechercher des modèles", capabilityLabel: "Capacité", allCapabilities: "Toutes les capacités", statusLabel: "État", allStatuses: "Tous les états", applyLabel: "Appliquer les filtres" },
    states: { operational: "Opérationnel", degraded: "Performances dégradées", outage: "Panne", unknown: "Inconnu", maintenance: "Maintenance" },
    freshness: { fresh: "La surveillance est à jour", stale: "Données de surveillance périmées", unavailable: "Surveillance indisponible" },
    lifecycle: { retired: "Retiré" },
    history: { title: "Historique d’état", availabilityLabel: "Disponibilité", coverageLabel: "Couverture de surveillance", incidentCountLabel: "Nombre d’incidents", noEvidence: "Aucune donnée de surveillance fiable", ranges: sharedRanges },
    incidents: { title: "Incidents récents", empty: "Aucun incident récent" },
    maintenance: { title: "Maintenance planifiée", empty: "Aucune maintenance planifiée" },
    subscribe: { title: "Alertes d’état", description: "Recevez par e-mail les mises à jour des services choisis.", emailLabel: "Adresse e-mail", emailPlaceholder: "vous@exemple.com", componentLegend: "Services à suivre", submitLabel: "S’abonner", submittingLabel: "Abonnement…", successLabel: "Consultez votre e-mail pour confirmer l’abonnement.", errorLabel: "Impossible de créer l’abonnement. Réessayez." },
    model: { currentStatusLabel: "État actuel", evidenceLabel: "Données fiables", backLabel: "Retour à la vue d’ensemble" },
  },
  pt: {
    title: "Status da Flatkey",
    description: "Saúde atual e histórico de monitoramento confiável do Router e de todos os modelos públicos.",
    overallLabel: "Status geral",
    coverageLabel: "Cobertura de monitoramento",
    lastTrustworthyUpdateLabel: "Última atualização confiável",
    componentsTitle: "Serviços e modelos",
    filters: { nameLabel: "Nome", namePlaceholder: "Buscar modelos", capabilityLabel: "Capacidade", allCapabilities: "Todas as capacidades", statusLabel: "Status", allStatuses: "Todos os status", applyLabel: "Aplicar filtros" },
    states: { operational: "Operacional", degraded: "Desempenho reduzido", outage: "Indisponível", unknown: "Desconhecido", maintenance: "Manutenção" },
    freshness: { fresh: "O monitoramento está atualizado", stale: "Dados de monitoramento desatualizados", unavailable: "Monitoramento indisponível" },
    lifecycle: { retired: "Desativado" },
    history: { title: "Histórico de status", availabilityLabel: "Disponibilidade", coverageLabel: "Cobertura de monitoramento", incidentCountLabel: "Número de incidentes", noEvidence: "Sem dados de monitoramento confiáveis", ranges: sharedRanges },
    incidents: { title: "Incidentes recentes", empty: "Nenhum incidente recente" },
    maintenance: { title: "Manutenção programada", empty: "Nenhuma manutenção programada" },
    subscribe: { title: "Notificações de status", description: "Receba por e-mail atualizações dos serviços selecionados.", emailLabel: "Endereço de e-mail", emailPlaceholder: "voce@exemplo.com", componentLegend: "Serviços para acompanhar", submitLabel: "Assinar", submittingLabel: "Assinando…", successLabel: "Confira seu e-mail para confirmar a assinatura.", errorLabel: "Não foi possível criar a assinatura. Tente novamente." },
    model: { currentStatusLabel: "Status atual", evidenceLabel: "Evidência confiável", backLabel: "Voltar ao resumo de status" },
  },
  ru: {
    title: "Статус Flatkey",
    description: "Текущее состояние и достоверная история мониторинга Router и всех публичных моделей.",
    overallLabel: "Общий статус",
    coverageLabel: "Покрытие мониторинга",
    lastTrustworthyUpdateLabel: "Последнее достоверное обновление",
    componentsTitle: "Сервисы и модели",
    filters: { nameLabel: "Название", namePlaceholder: "Поиск моделей", capabilityLabel: "Возможность", allCapabilities: "Все возможности", statusLabel: "Статус", allStatuses: "Все статусы", applyLabel: "Применить фильтры" },
    states: { operational: "Работает", degraded: "Сниженная производительность", outage: "Сбой", unknown: "Неизвестно", maintenance: "Техобслуживание" },
    freshness: { fresh: "Данные мониторинга актуальны", stale: "Данные мониторинга устарели", unavailable: "Мониторинг недоступен" },
    lifecycle: { retired: "Выведено из эксплуатации" },
    history: { title: "История статуса", availabilityLabel: "Доступность", coverageLabel: "Покрытие мониторинга", incidentCountLabel: "Количество инцидентов", noEvidence: "Нет достоверных данных мониторинга", ranges: sharedRanges },
    incidents: { title: "Недавние инциденты", empty: "Недавних инцидентов нет" },
    maintenance: { title: "Плановое обслуживание", empty: "Плановых работ нет" },
    subscribe: { title: "Уведомления о статусе", description: "Получайте по почте обновления выбранных сервисов.", emailLabel: "Электронная почта", emailPlaceholder: "you@example.com", componentLegend: "Отслеживаемые сервисы", submitLabel: "Подписаться", submittingLabel: "Подписка…", successLabel: "Подтвердите подписку по ссылке в письме.", errorLabel: "Не удалось оформить подписку. Повторите попытку." },
    model: { currentStatusLabel: "Текущий статус", evidenceLabel: "Достоверные данные", backLabel: "Вернуться к обзору статуса" },
  },
  ja: {
    title: "Flatkey 稼働状況",
    description: "Router とすべての公開モデルの現在の状態と信頼できる監視履歴です。",
    overallLabel: "全体の状態",
    coverageLabel: "監視カバレッジ",
    lastTrustworthyUpdateLabel: "最終信頼更新",
    componentsTitle: "サービスとモデル",
    filters: { nameLabel: "名前", namePlaceholder: "モデルを検索", capabilityLabel: "機能", allCapabilities: "すべての機能", statusLabel: "状態", allStatuses: "すべての状態", applyLabel: "絞り込む" },
    states: { operational: "正常稼働", degraded: "性能低下", outage: "障害", unknown: "不明", maintenance: "メンテナンス" },
    freshness: { fresh: "監視情報は最新です", stale: "監視情報が古くなっています", unavailable: "監視を利用できません" },
    lifecycle: { retired: "提供終了" },
    history: { title: "稼働履歴", availabilityLabel: "可用性", coverageLabel: "監視カバレッジ", incidentCountLabel: "インシデント数", noEvidence: "信頼できる監視データがありません", ranges: sharedRanges },
    incidents: { title: "最近のインシデント", empty: "最近のインシデントはありません" },
    maintenance: { title: "予定メンテナンス", empty: "予定メンテナンスはありません" },
    subscribe: { title: "稼働状況の通知", description: "選択したサービスの更新をメールで受け取ります。", emailLabel: "メールアドレス", emailPlaceholder: "you@example.com", componentLegend: "通知するサービス", submitLabel: "購読する", submittingLabel: "登録中…", successLabel: "確認メールをご確認ください。", errorLabel: "購読を登録できませんでした。もう一度お試しください。" },
    model: { currentStatusLabel: "現在の状態", evidenceLabel: "信頼できる情報", backLabel: "稼働状況一覧に戻る" },
  },
  vi: {
    title: "Trạng thái Flatkey",
    description: "Tình trạng hiện tại và lịch sử giám sát đáng tin cậy của Router cùng mọi mô hình công khai.",
    overallLabel: "Trạng thái tổng thể",
    coverageLabel: "Mức độ giám sát",
    lastTrustworthyUpdateLabel: "Cập nhật đáng tin cậy gần nhất",
    componentsTitle: "Dịch vụ và mô hình",
    filters: { nameLabel: "Tên", namePlaceholder: "Tìm mô hình", capabilityLabel: "Khả năng", allCapabilities: "Mọi khả năng", statusLabel: "Trạng thái", allStatuses: "Mọi trạng thái", applyLabel: "Áp dụng bộ lọc" },
    states: { operational: "Hoạt động bình thường", degraded: "Hiệu năng suy giảm", outage: "Gián đoạn", unknown: "Chưa xác định", maintenance: "Bảo trì" },
    freshness: { fresh: "Dữ liệu giám sát đang cập nhật", stale: "Dữ liệu giám sát đã cũ", unavailable: "Không thể giám sát" },
    lifecycle: { retired: "Đã ngừng cung cấp" },
    history: { title: "Lịch sử trạng thái", availabilityLabel: "Độ sẵn sàng", coverageLabel: "Mức độ giám sát", incidentCountLabel: "Số sự cố", noEvidence: "Không có dữ liệu giám sát đáng tin cậy", ranges: sharedRanges },
    incidents: { title: "Sự cố gần đây", empty: "Không có sự cố gần đây" },
    maintenance: { title: "Bảo trì theo lịch", empty: "Không có lịch bảo trì" },
    subscribe: { title: "Thông báo trạng thái", description: "Nhận cập nhật qua email cho các dịch vụ đã chọn.", emailLabel: "Địa chỉ email", emailPlaceholder: "ban@example.com", componentLegend: "Dịch vụ cần theo dõi", submitLabel: "Đăng ký", submittingLabel: "Đang đăng ký…", successLabel: "Hãy kiểm tra email để xác nhận đăng ký.", errorLabel: "Không thể tạo đăng ký. Vui lòng thử lại." },
    model: { currentStatusLabel: "Trạng thái hiện tại", evidenceLabel: "Bằng chứng đáng tin cậy", backLabel: "Quay lại tổng quan trạng thái" },
  },
  de: {
    title: "Flatkey-Status",
    description: "Aktueller Zustand und verlässlicher Überwachungsverlauf des Routers und aller öffentlichen Modelle.",
    overallLabel: "Gesamtstatus",
    coverageLabel: "Überwachungsabdeckung",
    lastTrustworthyUpdateLabel: "Letzte verlässliche Aktualisierung",
    componentsTitle: "Dienste und Modelle",
    filters: { nameLabel: "Name", namePlaceholder: "Modelle suchen", capabilityLabel: "Fähigkeit", allCapabilities: "Alle Fähigkeiten", statusLabel: "Status", allStatuses: "Alle Status", applyLabel: "Filter anwenden" },
    states: { operational: "Betriebsbereit", degraded: "Beeinträchtigte Leistung", outage: "Ausfall", unknown: "Unbekannt", maintenance: "Wartung" },
    freshness: { fresh: "Überwachung ist aktuell", stale: "Veraltete Überwachungsdaten", unavailable: "Überwachung nicht verfügbar" },
    lifecycle: { retired: "Eingestellt" },
    history: { title: "Statusverlauf", availabilityLabel: "Verfügbarkeit", coverageLabel: "Überwachungsabdeckung", incidentCountLabel: "Anzahl der Vorfälle", noEvidence: "Keine verlässlichen Überwachungsdaten", ranges: sharedRanges },
    incidents: { title: "Aktuelle Vorfälle", empty: "Keine aktuellen Vorfälle" },
    maintenance: { title: "Geplante Wartung", empty: "Keine geplante Wartung" },
    subscribe: { title: "Statusmeldungen", description: "Erhalte E-Mail-Updates für die ausgewählten Dienste.", emailLabel: "E-Mail-Adresse", emailPlaceholder: "du@beispiel.de", componentLegend: "Zu beobachtende Dienste", submitLabel: "Abonnieren", submittingLabel: "Wird abonniert…", successLabel: "Bestätige das Abonnement über die E-Mail.", errorLabel: "Das Abonnement konnte nicht erstellt werden. Versuche es erneut." },
    model: { currentStatusLabel: "Aktueller Status", evidenceLabel: "Verlässliche Daten", backLabel: "Zurück zur Statusübersicht" },
  },
};

export function getStatusCopy(locale: Locale): StatusCopy {
  return locale === "id" ? statusCopy.en : statusCopy[locale];
}

export function getStatusPresentation(input: StatusPresentationInput): StatusPresentation {
  const copy = getStatusCopy(input.locale);
  if (input.lifecycle === "retired") {
    return presentation("unknown", copy.lifecycle.retired, "retired", "slate");
  }
  if (input.freshness !== "fresh") {
    const text = input.freshness === "stale" ? copy.freshness.stale : copy.freshness.unavailable;
    return presentation("unknown", text, "unknown", "slate");
  }

  switch (input.status) {
    case "operational":
      return presentation("operational", copy.states.operational, "check", "emerald");
    case "degraded":
      return presentation("degraded", copy.states.degraded, "warning", "amber");
    case "outage":
      return presentation("outage", copy.states.outage, "outage", "red");
    case "maintenance":
      return presentation("maintenance", copy.states.maintenance, "maintenance", "blue");
    default:
      return presentation("unknown", copy.states.unknown, "unknown", "slate");
  }
}

function presentation(
  status: StatusValue,
  text: string,
  icon: StatusPresentation["icon"],
  tone: "emerald" | "amber" | "red" | "blue" | "slate"
): StatusPresentation {
  const colors = {
    emerald: ["text-emerald-800 bg-emerald-50 border-emerald-200", "bg-emerald-500"],
    amber: ["text-amber-900 bg-amber-50 border-amber-200", "bg-amber-500"],
    red: ["text-red-800 bg-red-50 border-red-200", "bg-red-500"],
    blue: ["text-blue-800 bg-blue-50 border-blue-200", "bg-blue-500"],
    slate: ["text-slate-700 bg-slate-100 border-slate-300", "bg-slate-400"],
  } as const;
  return { status, text, icon, colorClass: colors[tone][0], barClass: colors[tone][1] };
}
