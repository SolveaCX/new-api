import { LOCALES, type Locale } from "./locales";
import type { PricingModel } from "./pricing";

// The bundled entries keep the existing artwork and localized launch copy for
// models that have it. Which models appear is controlled by the public pricing
// payload (`featured_order`), which is managed from the console. New models can
// therefore be promoted without editing this file; their tags and description
// come directly from model metadata configured in the model list.

export type FeaturedSlide = {
  modelName: string;
  displayName: string;
  vendor: string;
  /** Poster image; also the still shown before a video decodes. */
  image: string;
  /** Bundled fallback used when the CDN poster is unavailable. */
  fallbackImage?: string;
  /** Optional looping clip. Autoplays muted — browsers block audio autoplay. */
  video?: string;
  tags: Record<Locale, string[]>;
  blurb: Record<Locale, string>;
};

export const FEATURED_SLIDES: FeaturedSlide[] = [
  {
    modelName: "claude-fable-5.1",
    displayName: "Claude Fable 5.1",
    vendor: "Anthropic",
    image: "/assets/models-featured/claude-fable-5.1.png",
    tags: {
      en: ["Coding", "Agents", "Computer Use"],
      zh: ["编程", "智能体", "计算机操作"],
      es: ["Programación", "Agentes", "Uso del ordenador"],
      fr: ["Code", "Agents", "Utilisation de l’ordinateur"],
      pt: ["Programação", "Agentes", "Uso do computador"],
      ru: ["Программирование", "Агенты", "Управление компьютером"],
      ja: ["コーディング", "エージェント", "コンピューター操作"],
      vi: ["Lập trình", "Tác tử", "Điều khiển máy tính"],
      de: ["Coding", "Agenten", "Computersteuerung"],
      id: ["Coding", "Agen", "Penggunaan komputer"],
    },
    blurb: {
      en: "Anthropic's strongest coding model yet: 55.8% on Terminal-Bench 4.0, scientific research more than doubled, and up to 45% cheaper on long agentic runs.",
      zh: "Anthropic 迄今最强的编程模型：Terminal-Bench 4.0 得分 55.8%，科学研究能力提升一倍以上，长时间智能体运行成本最高可降低 45%。",
      es: "El modelo de programación más potente de Anthropic hasta la fecha: 55,8 % en Terminal-Bench 4.0, la investigación científica se duplicó con creces y hasta un 45 % menos de coste en ejecuciones largas de agentes.",
      fr: "Le modèle de programmation le plus puissant d’Anthropic à ce jour : 55,8 % sur Terminal-Bench 4.0, une capacité de recherche scientifique plus que doublée et jusqu’à 45 % de coût en moins pour les longues exécutions d’agents.",
      pt: "O modelo de programação mais forte da Anthropic até agora: 55,8% no Terminal-Bench 4.0, pesquisa científica mais que dobrou e até 45% menos custo em execuções longas de agentes.",
      ru: "Самая сильная модель Anthropic для программирования: 55,8% в Terminal-Bench 4.0, результат в научных исследованиях вырос более чем вдвое, а длительные агентные запуски стоят до 45% дешевле.",
      ja: "Anthropic史上最強のコーディングモデル。Terminal-Bench 4.0で55.8%、科学研究の性能は2倍以上に向上し、長時間のエージェント実行では最大45%低コストです。",
      vi: "Mô hình lập trình mạnh nhất từ trước đến nay của Anthropic: đạt 55,8% trên Terminal-Bench 4.0, năng lực nghiên cứu khoa học tăng hơn gấp đôi và chi phí chạy tác tử dài giảm tới 45%.",
      de: "Anthropics bisher stärkstes Coding-Modell: 55,8 % im Terminal-Bench 4.0, mehr als doppelte Leistung bei wissenschaftlicher Forschung und bis zu 45 % geringere Kosten bei langen Agentenläufen.",
      id: "Model coding terkuat Anthropic sejauh ini: 55,8% di Terminal-Bench 4.0, kemampuan riset ilmiah meningkat lebih dari dua kali lipat, dan biaya proses agen panjang hingga 45% lebih murah.",
    },
  },
  {
    modelName: "glm-5.3-flash",
    displayName: "GLM-5.3 Flash",
    vendor: "Zhipu AI",
    image: "/assets/models-featured/glm-5.3-flash.png",
    tags: {
      en: ["Coding", "Multimodal", "Long Context"],
      zh: ["编程", "多模态", "长上下文"],
      es: ["Programación", "Multimodal", "Contexto largo"],
      fr: ["Code", "Multimodal", "Contexte long"],
      pt: ["Programação", "Multimodal", "Contexto longo"],
      ru: ["Программирование", "Мультимодальность", "Длинный контекст"],
      ja: ["コーディング", "マルチモーダル", "長文コンテキスト"],
      vi: ["Lập trình", "Đa phương thức", "Ngữ cảnh dài"],
      de: ["Coding", "Multimodal", "Langer Kontext"],
      id: ["Coding", "Multimodal", "Konteks panjang"],
    },
    blurb: {
      en: "Ox Alpha, unmasked. GLM-5.3 Flash ran 62 trillion tokens anonymously before Zhipu claimed it - 1M-token multimodal context, open MIT weights, Chinese silicon.",
      zh: "Ox Alpha，未加掩饰。GLM-5.3 Flash 在智谱认领它之前，曾以匿名状态运行了 62 万亿个 token——100 万 token 多模态上下文、开放 MIT 许可证权重，以及来自中国的硅基实力。",
      es: "Ox Alpha, sin máscara. GLM-5.3 Flash procesó 62 billones de tokens de forma anónima antes de que Zhipu lo reclamara: contexto multimodal de 1M de tokens, pesos abiertos bajo MIT y silicio chino.",
      fr: "Ox Alpha, sans masque. GLM-5.3 Flash a exécuté 62 000 milliards de tokens anonymement avant que Zhipu ne le revendique : contexte multimodal d'un million de tokens, poids ouverts sous MIT et silicium chinois.",
      pt: "Ox Alpha, sem máscara. O GLM-5.3 Flash processou 62 trilhões de tokens anonimamente antes de a Zhipu reivindicá-lo: contexto multimodal de 1M de tokens, pesos abertos sob MIT e silício chinês.",
      ru: "Ox Alpha без маски. GLM-5.3 Flash анонимно обработал 62 трлн токенов, прежде чем Zhipu заявил права на него: мультимодальный контекст на 1 млн токенов, открытые веса под MIT и китайский кремний.",
      ja: "Ox Alpha、覆面を脱いだ姿。GLM-5.3 Flash は Zhipu が名乗り出るまで匿名で 62 兆トークンを処理していた——100 万トークンのマルチモーダルコンテキスト、MIT のオープンウェイト、中国発のシリコン。",
      vi: "Ox Alpha, không che giấu. GLM-5.3 Flash đã xử lý 62 nghìn tỷ token trong ẩn danh trước khi Zhipu nhận mình là chủ nhân — ngữ cảnh đa phương thức 1 triệu token, trọng số mở theo MIT và silicon Trung Quốc.",
      de: "Ox Alpha, unverhüllt. GLM-5.3 Flash verarbeitete anonym 62 Billionen Tokens, bevor Zhipu es für sich beanspruchte: multimodaler Kontext mit 1 Mio. Tokens, offene MIT-Gewichte und chinesisches Silizium.",
      id: "Ox Alpha, tanpa penyamaran. GLM-5.3 Flash menjalankan 62 triliun token secara anonim sebelum Zhipu mengakuinya — konteks multimodal 1 juta token, bobot terbuka berlisensi MIT, dan silikon Tiongkok.",
    },
  },
  {
    modelName: "seedance-2.5",
    displayName: "Seedance 2.5",
    vendor: "ByteDance",
    image: "/assets/models-featured/bytedance.jpg",
    video: "/assets/models-featured/bytedance.mp4",
    tags: {
      en: ["Image to Video", "Text to Video", "Video to Video"],
      zh: ["图生视频", "文生视频", "视频转视频"],
      es: ["Imagen a vídeo", "Texto a vídeo", "Vídeo a vídeo"],
      fr: ["Image vers vidéo", "Texte vers vidéo", "Vidéo vers vidéo"],
      pt: ["Imagem para vídeo", "Texto para vídeo", "Vídeo para vídeo"],
      ru: ["Изображение в видео", "Текст в видео", "Видео в видео"],
      ja: ["画像から動画", "テキストから動画", "動画から動画"],
      vi: ["Ảnh thành video", "Văn bản thành video", "Video thành video"],
      de: ["Bild zu Video", "Text zu Video", "Video zu Video"],
      id: ["Gambar ke video", "Teks ke video", "Video ke video"],
    },
    blurb: {
      en: "Creates cinematic, high-motion videos from text or images, with strong prompt adherence, smooth camera movement, and polished visual consistency.",
      zh: "把可控视频生成推上新台阶：更长镜头、多模态参考、精确的分镜编辑，以及跨剪辑更强的一致性，可直接用于成片级创作。",
      es: "Eleva el listón del vídeo controlable: tomas más largas, referencias multimodales, edición precisa de planos y una consistencia mucho mayor entre cortes para trabajo creativo listo para producción.",
      fr: "Rehausse le niveau de la vidéo contrôlable : prises plus longues, références multimodales, montage précis des plans et bien plus de cohérence entre les coupes, pour un travail créatif prêt pour la production.",
      pt: "Eleva o nível do vídeo controlável: tomadas mais longas, referências multimodais, edição precisa de planos e consistência muito maior entre cortes para trabalho criativo pronto para produção.",
      ru: "Поднимает планку управляемого видео: более длинные дубли, мультимодальные референсы, точный монтаж кадров и заметно большая согласованность между склейками — для продакшн-готовых работ.",
      ja: "制御可能な動画生成の水準を引き上げます。より長いカット、マルチモーダルな参照、精密なショット編集、そしてカット間の一貫性が大幅に向上し、そのまま制作に使える品質です。",
      vi: "Nâng chuẩn cho video có thể kiểm soát: cảnh quay dài hơn, tham chiếu đa phương thức, chỉnh sửa khung hình chính xác và độ nhất quán cao hơn nhiều giữa các cảnh cắt, sẵn sàng cho sản xuất.",
      de: "Setzt neue Maßstäbe für steuerbares Video: längere Takes, multimodale Referenzen, präzise Shot-Bearbeitung und deutlich mehr Konsistenz über Schnitte hinweg — produktionsreif.",
      id: "Menaikkan standar video terkendali: pengambilan lebih panjang, referensi multimodal, penyuntingan shot yang presisi, dan konsistensi antar potongan yang jauh lebih baik untuk karya siap produksi.",
    },
  },
  {
    modelName: "MiniMax-H3",
    displayName: "MiniMax H3",
    vendor: "MiniMax",
    image: "/assets/models-featured/minimax.jpg",
    video: "/assets/models-featured/minimax.mp4",
    tags: {
      en: ["Text to Video", "Image to Video", "Native Audio"],
      zh: ["文生视频", "图生视频", "原生音频"],
      es: ["Texto a vídeo", "Imagen a vídeo", "Audio nativo"],
      fr: ["Texte vers vidéo", "Image vers vidéo", "Audio natif"],
      pt: ["Texto para vídeo", "Imagem para vídeo", "Áudio nativo"],
      ru: ["Текст в видео", "Изображение в видео", "Родное аудио"],
      ja: ["テキストから動画", "画像から動画", "ネイティブ音声"],
      vi: ["Văn bản thành video", "Ảnh thành video", "Âm thanh gốc"],
      de: ["Text zu Video", "Bild zu Video", "Natives Audio"],
      id: ["Teks ke video", "Gambar ke video", "Audio native"],
    },
    blurb: {
      en: "An omni-modal video model that understands text, images, video, and audio in one context, generating up to 15 seconds of 2K video with native stereo sound.",
      zh: "全模态生成模型，在同一上下文中理解文本、图像、视频与音频，可生成最长 15 秒的 2K 视频，并同步生成原生立体声。",
      es: "Un modelo de generación omnimodal que entiende texto, imágenes, vídeo y audio en un solo contexto, y produce hasta 15 segundos de vídeo 2K con sonido estéreo nativo generado junto con la imagen.",
      fr: "Un modèle de génération omnimodal qui comprend texte, images, vidéo et audio dans un même contexte, et produit jusqu'à 15 secondes de vidéo 2K avec un son stéréo natif généré en même temps que l'image.",
      pt: "Um modelo de geração omnimodal que entende texto, imagens, vídeo e áudio em um único contexto, produzindo até 15 segundos de vídeo 2K com som estéreo nativo gerado junto com a imagem.",
      ru: "Омнимодальная генеративная модель: понимает текст, изображения, видео и аудио в одном контексте и создаёт до 15 секунд видео 2K с родным стереозвуком, генерируемым вместе с картинкой.",
      ja: "テキスト・画像・動画・音声を一つのコンテキストで理解するオムニモーダル生成モデル。最長 15 秒の 2K 動画を、映像と同時に生成されるネイティブステレオ音声付きで出力します。",
      vi: "Mô hình sinh đa phương thức hiểu văn bản, hình ảnh, video và âm thanh trong cùng một ngữ cảnh, tạo ra video 2K dài tới 15 giây kèm âm thanh stereo gốc được sinh cùng hình ảnh.",
      de: "Ein omnimodales Generierungsmodell, das Text, Bild, Video und Audio in einem Kontext versteht und bis zu 15 Sekunden 2K-Video mit nativem Stereoton erzeugt, der zusammen mit dem Bild entsteht.",
      id: "Model generasi omnimodal yang memahami teks, gambar, video, dan audio dalam satu konteks, menghasilkan video 2K hingga 15 detik dengan suara stereo native yang dibuat bersamaan dengan gambarnya.",
    },
  },
  {
    modelName: "deepseek-v4-pro",
    displayName: "DeepSeek V4 Pro",
    vendor: "DeepSeek",
    image: "/assets/models-featured/deepseek.jpg",
    tags: {
      en: ["Chat", "Reasoning", "Long Context"],
      zh: ["对话", "推理", "长上下文"],
      es: ["Chat", "Razonamiento", "Contexto largo"],
      fr: ["Chat", "Raisonnement", "Contexte long"],
      pt: ["Chat", "Raciocínio", "Contexto longo"],
      ru: ["Чат", "Рассуждение", "Длинный контекст"],
      ja: ["チャット", "推論", "長文コンテキスト"],
      vi: ["Trò chuyện", "Suy luận", "Ngữ cảnh dài"],
      de: ["Chat", "Reasoning", "Langer Kontext"],
      id: ["Chat", "Penalaran", "Konteks panjang"],
    },
    blurb: {
      en: "A frontier reasoning model with a 1M-token window, built for multi-step analysis, repository-scale code review and research workflows — at a fraction of the cost of comparable frontier models.",
      zh: "具备 100 万 token 上下文的前沿推理模型，面向多步分析、仓库级代码审查与研究型工作流，成本仅为同级前沿模型的一小部分。",
      es: "Un modelo de razonamiento de frontera con ventana de 1M de tokens, pensado para análisis en varios pasos, revisión de código a escala de repositorio y flujos de investigación, a una fracción del coste de modelos comparables.",
      fr: "Un modèle de raisonnement de pointe doté d'une fenêtre d'un million de tokens, conçu pour l'analyse en plusieurs étapes, la revue de code à l'échelle d'un dépôt et les flux de recherche, pour une fraction du coût des modèles équivalents.",
      pt: "Um modelo de raciocínio de fronteira com janela de 1M de tokens, feito para análise em várias etapas, revisão de código em escala de repositório e fluxos de pesquisa, por uma fração do custo de modelos comparáveis.",
      ru: "Передовая модель рассуждения с окном в 1 млн токенов — для многошагового анализа, ревью кода масштаба репозитория и исследовательских задач, за долю стоимости сопоставимых моделей.",
      ja: "100 万トークンのコンテキストを持つフロンティア推論モデル。多段階の分析、リポジトリ規模のコードレビュー、研究ワークフロー向けで、同等モデルの何分の一かのコストで利用できます。",
      vi: "Mô hình suy luận tiên tiến với cửa sổ 1 triệu token, dành cho phân tích nhiều bước, rà soát mã ở quy mô kho lưu trữ và quy trình nghiên cứu, với chi phí chỉ bằng một phần nhỏ so với các mô hình tương đương.",
      de: "Ein Spitzenmodell fürs Reasoning mit 1-Mio.-Token-Fenster — für mehrstufige Analysen, Code-Reviews in Repository-Größe und Forschungs-Workflows, zu einem Bruchteil der Kosten vergleichbarer Modelle.",
      id: "Model penalaran terdepan dengan jendela 1 juta token, dibuat untuk analisis bertahap, tinjauan kode skala repositori, dan alur kerja riset — dengan biaya jauh lebih rendah dari model setara.",
    },
  },
  {
    modelName: "kimi-k3",
    displayName: "Kimi K3",
    vendor: "Moonshot AI",
    image: "/assets/models-featured/moonshot.jpg",
    tags: {
      en: ["Chat", "Agents", "Long Context"],
      zh: ["对话", "智能体", "长上下文"],
      es: ["Chat", "Agentes", "Contexto largo"],
      fr: ["Chat", "Agents", "Contexte long"],
      pt: ["Chat", "Agentes", "Contexto longo"],
      ru: ["Чат", "Агенты", "Длинный контекст"],
      ja: ["チャット", "エージェント", "長文コンテキスト"],
      vi: ["Trò chuyện", "Tác tử", "Ngữ cảnh dài"],
      de: ["Chat", "Agenten", "Langer Kontext"],
      id: ["Chat", "Agen", "Konteks panjang"],
    },
    blurb: {
      en: "Tuned for long-horizon agent work. It holds a full 1M-token context, chains tool calls without losing the thread, and keeps an entire codebase in view across a session.",
      zh: "为长周期智能体任务调优：完整保留 100 万 token 上下文，连续工具调用不丢失线索，整个代码库在一次会话中始终可见。",
      es: "Ajustado para trabajo de agentes de largo alcance. Mantiene un contexto completo de 1M de tokens, encadena llamadas a herramientas sin perder el hilo y conserva toda una base de código a la vista durante la sesión.",
      fr: "Optimisé pour le travail d'agents sur la durée. Il conserve un contexte complet d'un million de tokens, enchaîne les appels d'outils sans perdre le fil et garde toute une base de code en vue pendant la session.",
      pt: "Ajustado para trabalho de agentes de longo prazo. Mantém contexto completo de 1M de tokens, encadeia chamadas de ferramentas sem perder o fio e mantém uma base de código inteira à vista durante a sessão.",
      ru: "Настроен на длительную работу агентов: удерживает полный контекст в 1 млн токенов, выстраивает цепочки вызовов инструментов, не теряя нить, и держит всю кодовую базу в поле зрения на протяжении сессии.",
      ja: "長期にわたるエージェント作業向けにチューニング。100 万トークンのコンテキストを保持し、ツール呼び出しを連鎖させても文脈を見失わず、セッション全体でコードベース全体を見渡せます。",
      vi: "Được tinh chỉnh cho công việc tác tử dài hạn. Giữ trọn ngữ cảnh 1 triệu token, nối chuỗi lệnh gọi công cụ mà không mất mạch, và bao quát toàn bộ mã nguồn trong suốt phiên làm việc.",
      de: "Für langlaufende Agenten-Arbeit optimiert: hält den vollen 1-Mio.-Token-Kontext, verkettet Tool-Aufrufe ohne den Faden zu verlieren und behält eine ganze Codebasis über die Sitzung im Blick.",
      id: "Disetel untuk kerja agen jangka panjang. Menyimpan konteks penuh 1 juta token, merangkai pemanggilan alat tanpa kehilangan alur, dan menjaga seluruh basis kode tetap terlihat sepanjang sesi.",
    },
  },
  {
    modelName: "gpt-5.6-sol",
    displayName: "GPT-5.6 Sol",
    vendor: "OpenAI",
    // Dedicated artwork supplied for the GPT-5.6 Sol banner. Keep this slide
    // image-only so the artwork remains the stable visual identity instead of
    // switching to the generic OpenAI background video.
    image: "/assets/models-featured/gpt-5.6-sol.png",
    tags: {
      en: ["Chat", "Reasoning", "Tool Use"],
      zh: ["对话", "推理", "工具调用"],
      es: ["Chat", "Razonamiento", "Uso de herramientas"],
      fr: ["Chat", "Raisonnement", "Appel d'outils"],
      pt: ["Chat", "Raciocínio", "Uso de ferramentas"],
      ru: ["Чат", "Рассуждение", "Вызов инструментов"],
      ja: ["チャット", "推論", "ツール利用"],
      vi: ["Trò chuyện", "Suy luận", "Dùng công cụ"],
      de: ["Chat", "Reasoning", "Tool-Nutzung"],
      id: ["Chat", "Penalaran", "Penggunaan alat"],
    },
    blurb: {
      en: "Advanced reasoning model for coding, analysis, and complex problem solving, delivering strong instruction following, reliable tool use, and efficient long-context performance.",
      zh: "GPT 系列里最稳的主力：通用推理扎实、结构化输出可靠、工具调用一流，价格足以让它直接跑在生产应用的核心链路上。",
      es: "El caballo de batalla fiable de la línea GPT: razonamiento general sólido, salida estructurada confiable y llamadas a herramientas de primer nivel, a un precio que permite ponerlo en la ruta crítica de una app en producción.",
      fr: "La valeur sûre de la gamme GPT : raisonnement général solide, sortie structurée fiable et appels d'outils de premier ordre, à un prix qui permet de le placer sur le chemin critique d'une application en production.",
      pt: "O cavalo de batalha confiável da linha GPT: raciocínio geral sólido, saída estruturada confiável e chamadas de ferramentas de primeira, com preço que permite colocá-lo no caminho crítico de um app em produção.",
      ru: "Надёжная рабочая лошадка линейки GPT: уверенное общее рассуждение, стабильный структурированный вывод и первоклассный вызов инструментов — по цене, позволяющей ставить его на горячий путь продакшн-приложения.",
      ja: "GPT シリーズの頼れる主力。一般的な推論に強く、構造化出力は安定、ツール呼び出しも一級です。本番アプリのホットパスに載せられる価格帯です。",
      vi: "Chủ lực đáng tin cậy của dòng GPT — suy luận tổng quát tốt, đầu ra có cấu trúc ổn định và gọi công cụ hàng đầu, với mức giá đủ để đặt vào đường dẫn nóng của ứng dụng production.",
      de: "Das verlässliche Arbeitspferd der GPT-Reihe: starkes allgemeines Reasoning, zuverlässige strukturierte Ausgaben und erstklassiges Tool-Calling — zu einem Preis, der den Einsatz im Hot Path einer Produktions-App erlaubt.",
      id: "Andalan tepercaya di lini GPT — penalaran umum yang kuat, keluaran terstruktur yang andal, dan pemanggilan alat kelas satu, dengan harga yang memungkinkan dipakai di jalur kritis aplikasi produksi.",
    },
  },
  {
    modelName: "glm-5.3",
    displayName: "GLM-5.3",
    vendor: "Zhipu AI",
    image: "/assets/models-featured/zhipu.jpg",
    tags: {
      en: ["Chat", "Coding", "Agents"],
      zh: ["对话", "编程", "智能体"],
      es: ["Chat", "Programación", "Agentes"],
      fr: ["Chat", "Code", "Agents"],
      pt: ["Chat", "Programação", "Agentes"],
      ru: ["Чат", "Программирование", "Агенты"],
      ja: ["チャット", "コーディング", "エージェント"],
      vi: ["Trò chuyện", "Lập trình", "Tác tử"],
      de: ["Chat", "Coding", "Agenten"],
      id: ["Chat", "Coding", "Agen"],
    },
    blurb: {
      en: "Fast, capable general-purpose model for coding, reasoning, and multilingual tasks, with strong instruction following, efficient tool use, and reliable everyday performance.",
      zh: "越来越多团队转向它做智能体编程：指令遵循精准、函数调用可靠、权重开放，价格让长时间自主运行真正划算。",
      es: "El modelo al que se están pasando los equipos para programación con agentes: seguimiento preciso de instrucciones, llamadas a funciones fiables y pesos abiertos, a un precio que hace viables las ejecuciones autónomas largas.",
      fr: "Le modèle vers lequel les équipes migrent pour le codage agentique : suivi précis des instructions, appels de fonctions fiables et poids ouverts, à un prix qui rend les longues exécutions autonomes réellement abordables.",
      pt: "O modelo para o qual as equipes estão migrando na programação com agentes: obediência precisa a instruções, chamadas de função confiáveis e pesos abertos, a um preço que torna execuções autônomas longas viáveis.",
      ru: "Модель, на которую команды переходят для агентного программирования: точное следование инструкциям, надёжные вызовы функций и открытые веса — по цене, при которой длительные автономные прогоны действительно окупаются.",
      ja: "エージェント型コーディングで乗り換えが進んでいるモデル。指示追従が正確で関数呼び出しも安定、オープンウェイトかつ、長時間の自律実行が現実的なコストに収まります。",
      vi: "Mô hình mà các đội đang chuyển sang cho lập trình bằng tác tử: tuân thủ chỉ dẫn chính xác, gọi hàm đáng tin cậy và trọng số mở, với mức giá khiến các lượt chạy tự động dài trở nên khả thi.",
      de: "Das Modell, zu dem Teams für agentisches Coding wechseln: präzises Befolgen von Anweisungen, verlässliches Function Calling und offene Gewichte — zu einem Preis, der lange autonome Läufe wirklich bezahlbar macht.",
      id: "Model yang mulai dipilih tim untuk coding berbasis agen: mengikuti instruksi dengan tepat, pemanggilan fungsi andal, dan bobot terbuka, dengan harga yang membuat proses otonom panjang benar-benar terjangkau.",
    },
  },
];

/**
 * Build the visible slides from the live catalogue. A retired model drops out
 * of the carousel rather than linking to a page that no longer exists.
 */
function localizedRecord<T>(value: T, fallback?: Record<Locale, T>): Record<Locale, T> {
  return Object.fromEntries(
    LOCALES.map((locale) => [locale, fallback?.[locale] ?? value])
  ) as Record<Locale, T>;
}

function safeBannerAsset(value?: string): string | undefined {
  const trimmed = value?.trim();
  if (!trimmed) return undefined;
  if (/^data:image\/(png|jpeg|webp|gif);base64,/i.test(trimmed)) return trimmed;
  if (trimmed.startsWith("/")) return trimmed;
  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === "http:" || parsed.protocol === "https:"
      ? parsed.toString()
      : undefined;
  } catch {
    return undefined;
  }
}

function buildSlideFromModel(model: PricingModel): FeaturedSlide {
  const curated = FEATURED_SLIDES.find((slide) => slide.modelName === model.model_name);
  const config = model.featured_config;
  const modelTags = (config?.tags ?? model.tags ?? "")
    .split(",")
    .map((tag) => tag.trim())
    .filter(Boolean)
    .slice(0, 4);
  const tags = modelTags.length > 0
    ? localizedRecord(modelTags)
    : curated?.tags ?? localizedRecord([]);
  const description = (config?.description ?? model.description)?.trim();
  const uploadedImage = safeBannerAsset(config?.background_image);
  const configuredImage = safeBannerAsset(config?.background_image_url);
  const configuredFallback = safeBannerAsset(config?.fallback_background_image);

  return {
    modelName: model.model_name,
    displayName: config?.display_name?.trim() || curated?.displayName || model.model_name,
    vendor: model.vendor_name ?? curated?.vendor ?? "AI model",
    image: uploadedImage ?? configuredImage ?? curated?.image ?? model.icon ?? "/assets/models-featured/flatkey-model-cover-clean.png",
    fallbackImage: configuredFallback ?? curated?.fallbackImage,
    video: safeBannerAsset(config?.video) ?? curated?.video,
    tags,
    blurb: description ? localizedRecord(description) : curated?.blurb ?? localizedRecord(""),
  };
}

/**
 * Builds the banner from the models returned by the public pricing API.
 *
 * When at least one model has a configured `featured_order`, only those models
 * are shown in that order. Before the first console configuration, retain the
 * launch slate from FEATURED_SLIDES so an empty database does not remove the
 * banner altogether.
 */
export function buildFeaturedSlides(models: PricingModel[]): FeaturedSlide[] {
  const configured = models
    .filter((model) => model.featured_order != null)
    .sort((left, right) => (left.featured_order ?? 0) - (right.featured_order ?? 0));

  if (configured.length > 0) return configured.map(buildSlideFromModel);

  const live = new Map(models.map((model) => [model.model_name, model]));
  return FEATURED_SLIDES.flatMap((slide) => {
    const model = live.get(slide.modelName);
    return model ? [buildSlideFromModel(model)] : [];
  });
}
