import type { Locale } from "@/lib/locales";

export type ModelCollectionCopy = {
  title: string;
  shortDescription: string;
  slogan: string;
  intro: string;
  seoTitle: string;
  seoDescription: string;
  empty: string;
};

export type CollectionCopyKey = "index" | "discounted-models" | "coding" | "roleplay-creative-writing" | "image-generation" | "video-generation" | "audio-generation-models" | "vision-models" | "text-to-speech-models";

// Card copy, visible page copy and search metadata have distinct roles.
// Keep all supported locales explicit: never substitute English marketing text.
export const MODEL_COLLECTION_COPY: Record<Locale, Record<CollectionCopyKey, ModelCollectionCopy>> = {
  "en": {
    "index": {
      "title": "AI Model Collections",
      "shortDescription": "Explore AI models by task and compare API prices.",
      "slogan": "Find the right AI models for what you’re building",
      "intro": "Browse AI model collections by task, explore discounted options, and compare capabilities and API prices. Find models for your next feature, from coding and creative writing to images, video, and speech.",
      "seoTitle": "AI Model Collections: Compare by Use Case | Flatkey",
      "seoDescription": "Explore AI model collections for coding, writing, image and video generation, vision, audio, and text to speech. Compare capabilities and API prices.",
      "empty": "No matching models are currently listed. Browse all models or check back for updates."
    },
    "discounted-models": {
      "title": "Discounted AI Models",
      "shortDescription": "Explore model discounts and compare API prices and applicable conditions.",
      "slogan": "Find AI models that fit your budget",
      "intro": "Explore discounted AI models and compare their API prices against the stated reference rates. Review input and output costs, model capabilities, and applicable conditions before choosing.",
      "seoTitle": "Discounted AI Models & API Pricing | Flatkey",
      "seoDescription": "Explore discounted AI models on Flatkey. Compare API prices, reference rates, and applicable conditions to find models that fit your budget.",
      "empty": "No matching models are currently listed. Browse all models or check back for updates."
    },
    "coding": {
      "title": "Coding Models",
      "shortDescription": "Compare LLMs for code generation, debugging, and code review.",
      "slogan": "Find the right LLM for your coding workflow",
      "intro": "Explore AI models for writing, explaining, debugging, and reviewing code. Compare capabilities and API costs to find the best fit for your development tasks.",
      "seoTitle": "LLMs for Coding: Compare Models & API Prices | Flatkey",
      "seoDescription": "Compare LLMs for coding, debugging, and code review. Explore model capabilities and API prices to choose a model for your development workflow.",
      "empty": "No matching models are currently listed. Browse all models or check back for updates."
    },
    "roleplay-creative-writing": {
      "title": "Roleplay & Creative Writing",
      "shortDescription": "Explore models for character dialogue, storytelling, and creative writing.",
      "slogan": "Build characters. Write their stories.",
      "intro": "Explore LLMs for roleplay, character conversations, and creative writing. Compare model options and API costs to choose a fit for your storytelling or dialogue application.",
      "seoTitle": "LLMs for Roleplay & Creative Writing | Flatkey",
      "seoDescription": "Explore LLMs for roleplay, character dialogue, and creative writing. Compare model options and API prices for storytelling and conversational applications.",
      "empty": "No matching models are currently listed. Browse all models or check back for updates."
    },
    "image-generation": {
      "title": "Image Generation Models",
      "shortDescription": "Compare image generation models, supported inputs, and API prices.",
      "slogan": "Find the model for your next image",
      "intro": "Explore image generation models for your application or creative workflow. Compare supported inputs, output options, and API prices to choose a model for the images you want to create.",
      "seoTitle": "AI Image Generation Models & APIs | Flatkey",
      "seoDescription": "Compare AI image generation models and API prices. Explore supported inputs and output options to choose a model for your image generation workflow.",
      "empty": "No matching models are currently listed. Browse all models or check back for updates."
    },
    "video-generation": {
      "title": "Video Generation Models",
      "shortDescription": "Compare video models by input type, duration, resolution, and price.",
      "slogan": "Find the model for your next video",
      "intro": "Explore video generation models for text and image inputs. Compare each model’s supported duration, resolution, audio options, and pricing to find a fit for your production workflow.",
      "seoTitle": "AI Video Generation Models & APIs | Flatkey",
      "seoDescription": "Explore AI video generation models for text and image inputs. Compare supported durations, resolutions, and API pricing for your video workflow.",
      "empty": "No matching models are currently listed. Browse all models or check back for updates."
    },
    "audio-generation-models": {
      "title": "Audio Generation Models",
      "shortDescription": "Explore models for text-to-speech and music generation for video.",
      "slogan": "Find the right model for speech and soundtracks",
      "intro": "Browse audio generation models for turning text into speech or creating music for video. Choose your task to explore available models, input requirements, and API prices.",
      "seoTitle": "Audio Generation Models for Speech & Video Music | Flatkey",
      "seoDescription": "Explore audio generation models for speech synthesis and video soundtracks. Compare available models, input requirements, and API prices on Flatkey.",
      "empty": "No matching models are currently listed. Browse all models or check back for updates."
    },
    "vision-models": {
      "title": "Vision & Multimodal Models",
      "shortDescription": "Find models for image understanding and visual questions.",
      "slogan": "Turn visual inputs into useful answers",
      "intro": "Explore vision and multimodal models that interpret images and answer questions about visual content. Compare supported inputs, capabilities, and API prices for your application.",
      "seoTitle": "Vision Models & Multimodal AI APIs | Flatkey",
      "seoDescription": "Compare vision and multimodal models for image understanding. Explore supported inputs, model capabilities, and API prices for visual applications.",
      "empty": "No matching models are currently listed. Browse all models or check back for updates."
    },
    "text-to-speech-models": {
      "title": "Text-to-Speech Models",
      "shortDescription": "Compare TTS models, available voice options, and API prices.",
      "slogan": "Give your words a voice",
      "intro": "Explore text-to-speech models for narration and spoken responses. Compare available voices, languages, and API prices to find a fit for your TTS application.",
      "seoTitle": "Text-to-Speech Models & TTS APIs | Flatkey",
      "seoDescription": "Compare text-to-speech models and TTS API prices. Explore available voice and language options to choose a model for narration and spoken responses.",
      "empty": "No matching models are currently listed. Browse all models or check back for updates."
    }
  },
  "zh": {
    "index": {
      "title": "AI 模型集合",
      "shortDescription": "按任务浏览 AI 模型，比较能力与 API 价格。",
      "slogan": "为你的应用，找到合适的 AI 模型",
      "intro": "按任务浏览 AI 模型，比较能力与 API 价格，查看可用优惠。无论是编程、创意写作，还是图像、视频与语音生成，都能从对应集合开始选型。",
      "seoTitle": "AI 模型集合 | Flatkey",
      "seoDescription": "按任务浏览 AI 模型，比较能力与 API 价格，查看可用优惠。无论是编程、创意写作，还是图像、视频与语音生成，都能从对应集合开始选型。",
      "empty": "当前暂无符合条件的模型，可浏览全部模型或稍后查看。"
    },
    "discounted-models": {
      "title": "折扣模型",
      "shortDescription": "查看模型优惠，比较 API 价格与适用条件。",
      "slogan": "为你的预算，找到合适的 AI 模型",
      "intro": "查看相对标明参考价格有优惠的模型，比较输入与输出费用、适用条件和模型能力，为你的应用选择合适的方案。",
      "seoTitle": "折扣模型 | Flatkey",
      "seoDescription": "查看相对标明参考价格有优惠的模型，比较输入与输出费用、适用条件和模型能力，为你的应用选择合适的方案。",
      "empty": "当前暂无符合条件的模型，可浏览全部模型或稍后查看。"
    },
    "coding": {
      "title": "编程模型",
      "shortDescription": "比较适合代码生成、调试与代码审查的 AI 模型。",
      "slogan": "为你的开发任务，选择合适的编程模型",
      "intro": "浏览支持代码编写、解释、调试与审查的 AI 模型，结合任务需求、模型能力和 API 成本，选择适合开发流程的模型。",
      "seoTitle": "编程模型 | Flatkey",
      "seoDescription": "浏览支持代码编写、解释、调试与审查的 AI 模型，结合任务需求、模型能力和 API 成本，选择适合开发流程的模型。",
      "empty": "当前暂无符合条件的模型，可浏览全部模型或稍后查看。"
    },
    "roleplay-creative-writing": {
      "title": "角色扮演及创意写作",
      "shortDescription": "探索用于角色对话、故事创作与创意写作的 AI 模型。",
      "slogan": "塑造角色，写出他们的故事",
      "intro": "探索用于角色对话、情节发展和创意写作的模型，结合对话与叙事需求比较不同选项，并查看对应的 API 价格。",
      "seoTitle": "角色扮演及创意写作 | Flatkey",
      "seoDescription": "探索用于角色对话、情节发展和创意写作的模型，结合对话与叙事需求比较不同选项，并查看对应的 API 价格。",
      "empty": "当前暂无符合条件的模型，可浏览全部模型或稍后查看。"
    },
    "image-generation": {
      "title": "图像生成模型",
      "shortDescription": "比较图像生成模型、支持的输入方式与 API 价格。",
      "slogan": "为下一张图片，找到合适的模型",
      "intro": "浏览适合应用开发和创作流程的图像生成模型，比较输入方式、输出选项与 API 价格，按你的图片需求选择模型。",
      "seoTitle": "图像生成模型 | Flatkey",
      "seoDescription": "浏览适合应用开发和创作流程的图像生成模型，比较输入方式、输出选项与 API 价格，按你的图片需求选择模型。",
      "empty": "当前暂无符合条件的模型，可浏览全部模型或稍后查看。"
    },
    "video-generation": {
      "title": "视频生成模型",
      "shortDescription": "按输入方式、生成时长、分辨率与价格比较视频模型。",
      "slogan": "为下一段视频，找到合适的模型",
      "intro": "按输入方式、生成时长、分辨率与价格比较视频模型，为你的素材、应用或制作流程选择合适的生成方案。",
      "seoTitle": "视频生成模型 | Flatkey",
      "seoDescription": "按输入方式、生成时长、分辨率与价格比较视频模型，为你的素材、应用或制作流程选择合适的生成方案。",
      "empty": "当前暂无符合条件的模型，可浏览全部模型或稍后查看。"
    },
    "audio-generation-models": {
      "title": "音频生成模型",
      "shortDescription": "探索用于文字转语音与视频配乐的音频生成模型。",
      "slogan": "从语音到配乐，找到合适的声音",
      "intro": "浏览将文字生成语音、为视频生成配乐的模型。从具体任务出发，了解可用型号、输入要求与 API 价格。",
      "seoTitle": "音频生成模型 | Flatkey",
      "seoDescription": "浏览将文字生成语音、为视频生成配乐的模型。从具体任务出发，了解可用型号、输入要求与 API 价格。",
      "empty": "当前暂无符合条件的模型，可浏览全部模型或稍后查看。"
    },
    "vision-models": {
      "title": "视觉理解与多模态",
      "shortDescription": "查找能够理解图片、回答视觉问题的多模态模型。",
      "slogan": "让图片成为可理解的信息",
      "intro": "探索能够理解图片、回答视觉问题的多模态模型，比较支持的输入类型与 API 价格，为你的视觉应用选择合适的模型。",
      "seoTitle": "视觉理解与多模态 | Flatkey",
      "seoDescription": "探索能够理解图片、回答视觉问题的多模态模型，比较支持的输入类型与 API 价格，为你的视觉应用选择合适的模型。",
      "empty": "当前暂无符合条件的模型，可浏览全部模型或稍后查看。"
    },
    "text-to-speech-models": {
      "title": "文本转语音模型",
      "shortDescription": "比较 TTS 模型、可用声音选项与 API 价格。",
      "slogan": "让文字，拥有声音",
      "intro": "浏览用于内容旁白和语音回复的 TTS 模型，比较可用声音、语言选项与 API 价格，为你的应用选择合适的语音生成模型。",
      "seoTitle": "文本转语音模型 | Flatkey",
      "seoDescription": "浏览用于内容旁白和语音回复的 TTS 模型，比较可用声音、语言选项与 API 价格，为你的应用选择合适的语音生成模型。",
      "empty": "当前暂无符合条件的模型，可浏览全部模型或稍后查看。"
    }
  },
  "es": {
    "index": {
      "title": "Colecciones de modelos de IA",
      "shortDescription": "Explora modelos por tarea y compara precios de API.",
      "slogan": "Encuentra los modelos de IA adecuados para tu proyecto",
      "intro": "Explora modelos por tarea, consulta descuentos y compara capacidades y precios de API para programación, escritura, imágenes, vídeo y voz.",
      "seoTitle": "Colecciones de modelos de IA | Flatkey",
      "seoDescription": "Explora modelos por tarea, consulta descuentos y compara capacidades y precios de API para programación, escritura, imágenes, vídeo y voz.",
      "empty": "Actualmente no hay modelos que coincidan. Consulta todos los modelos o vuelve más tarde."
    },
    "discounted-models": {
      "title": "Modelos de IA con descuento",
      "shortDescription": "Consulta descuentos, precios de API y condiciones.",
      "slogan": "Encuentra modelos de IA que se ajusten a tu presupuesto",
      "intro": "Compara los precios de API con las tarifas de referencia indicadas. Revisa los costes de entrada y salida, las capacidades y las condiciones antes de elegir.",
      "seoTitle": "Modelos de IA con descuento | Flatkey",
      "seoDescription": "Compara los precios de API con las tarifas de referencia indicadas. Revisa los costes de entrada y salida, las capacidades y las condiciones antes de elegir.",
      "empty": "Actualmente no hay modelos que coincidan. Consulta todos los modelos o vuelve más tarde."
    },
    "coding": {
      "title": "Modelos para programación",
      "shortDescription": "Compara LLM para generar, depurar y revisar código.",
      "slogan": "Encuentra el LLM adecuado para tu trabajo de programación",
      "intro": "Explora modelos para escribir, explicar, depurar y revisar código. Compara capacidades y costes de API según tus tareas de desarrollo.",
      "seoTitle": "Modelos para programación | Flatkey",
      "seoDescription": "Explora modelos para escribir, explicar, depurar y revisar código. Compara capacidades y costes de API según tus tareas de desarrollo.",
      "empty": "Actualmente no hay modelos que coincidan. Consulta todos los modelos o vuelve más tarde."
    },
    "roleplay-creative-writing": {
      "title": "Roleplay y escritura creativa",
      "shortDescription": "Explora modelos para diálogos, historias y escritura creativa.",
      "slogan": "Crea personajes. Escribe sus historias.",
      "intro": "Explora LLM para interpretar personajes, desarrollar diálogos y escribir historias. Compara modelos y costes de API para tu aplicación narrativa.",
      "seoTitle": "Roleplay y escritura creativa | Flatkey",
      "seoDescription": "Explora LLM para interpretar personajes, desarrollar diálogos y escribir historias. Compara modelos y costes de API para tu aplicación narrativa.",
      "empty": "Actualmente no hay modelos que coincidan. Consulta todos los modelos o vuelve más tarde."
    },
    "image-generation": {
      "title": "Modelos de generación de imágenes",
      "shortDescription": "Compara modelos de imágenes, entradas y precios de API.",
      "slogan": "Encuentra el modelo para tu próxima imagen",
      "intro": "Explora modelos de generación de imágenes para tu aplicación o proceso creativo. Compara entradas admitidas, opciones de salida y precios de API.",
      "seoTitle": "Modelos de generación de imágenes | Flatkey",
      "seoDescription": "Explora modelos de generación de imágenes para tu aplicación o proceso creativo. Compara entradas admitidas, opciones de salida y precios de API.",
      "empty": "Actualmente no hay modelos que coincidan. Consulta todos los modelos o vuelve más tarde."
    },
    "video-generation": {
      "title": "Modelos de generación de vídeo",
      "shortDescription": "Compara entradas, duración, resolución y precios de vídeo.",
      "slogan": "Encuentra el modelo para tu próximo vídeo",
      "intro": "Explora modelos de vídeo a partir de texto e imágenes. Compara duración, resolución, opciones de audio y precios según las capacidades de cada modelo.",
      "seoTitle": "Modelos de generación de vídeo | Flatkey",
      "seoDescription": "Explora modelos de vídeo a partir de texto e imágenes. Compara duración, resolución, opciones de audio y precios según las capacidades de cada modelo.",
      "empty": "Actualmente no hay modelos que coincidan. Consulta todos los modelos o vuelve más tarde."
    },
    "audio-generation-models": {
      "title": "Modelos de generación de audio",
      "shortDescription": "Explora modelos de texto a voz y música para vídeo.",
      "slogan": "Encuentra el modelo para voces y bandas sonoras",
      "intro": "Explora modelos para convertir texto en voz o crear música para vídeos. Elige una tarea y consulta modelos, requisitos de entrada y precios de API.",
      "seoTitle": "Modelos de generación de audio | Flatkey",
      "seoDescription": "Explora modelos para convertir texto en voz o crear música para vídeos. Elige una tarea y consulta modelos, requisitos de entrada y precios de API.",
      "empty": "Actualmente no hay modelos que coincidan. Consulta todos los modelos o vuelve más tarde."
    },
    "vision-models": {
      "title": "Modelos de visión y multimodales",
      "shortDescription": "Encuentra modelos para comprender imágenes y responder preguntas visuales.",
      "slogan": "Convierte imágenes en respuestas útiles",
      "intro": "Explora modelos que interpretan imágenes y responden preguntas sobre contenido visual. Compara entradas admitidas, capacidades y precios de API.",
      "seoTitle": "Modelos de visión y multimodales | Flatkey",
      "seoDescription": "Explora modelos que interpretan imágenes y responden preguntas sobre contenido visual. Compara entradas admitidas, capacidades y precios de API.",
      "empty": "Actualmente no hay modelos que coincidan. Consulta todos los modelos o vuelve más tarde."
    },
    "text-to-speech-models": {
      "title": "Modelos de texto a voz",
      "shortDescription": "Compara modelos TTS, voces disponibles y precios de API.",
      "slogan": "Dale voz a tus palabras",
      "intro": "Explora modelos de texto a voz para narraciones y respuestas habladas. Compara voces, idiomas y precios de API para tu aplicación.",
      "seoTitle": "Modelos de texto a voz | Flatkey",
      "seoDescription": "Explora modelos de texto a voz para narraciones y respuestas habladas. Compara voces, idiomas y precios de API para tu aplicación.",
      "empty": "Actualmente no hay modelos que coincidan. Consulta todos los modelos o vuelve más tarde."
    }
  },
  "fr": {
    "index": {
      "title": "Collections de modèles IA",
      "shortDescription": "Explorez les modèles par tâche et comparez les tarifs API.",
      "slogan": "Trouvez les modèles IA adaptés à votre projet",
      "intro": "Parcourez les modèles par tâche, découvrez les remises et comparez les capacités et tarifs API pour le code, l’écriture, l’image, la vidéo et la voix.",
      "seoTitle": "Collections de modèles IA | Flatkey",
      "seoDescription": "Parcourez les modèles par tâche, découvrez les remises et comparez les capacités et tarifs API pour le code, l’écriture, l’image, la vidéo et la voix.",
      "empty": "Aucun modèle correspondant n’est actuellement proposé. Consultez tous les modèles ou revenez plus tard."
    },
    "discounted-models": {
      "title": "Modèles IA à prix réduit",
      "shortDescription": "Comparez les remises, les tarifs API et leurs conditions.",
      "slogan": "Trouvez des modèles IA adaptés à votre budget",
      "intro": "Comparez les tarifs API aux prix de référence indiqués. Vérifiez les coûts d’entrée et de sortie, les capacités et les conditions avant de choisir.",
      "seoTitle": "Modèles IA à prix réduit | Flatkey",
      "seoDescription": "Comparez les tarifs API aux prix de référence indiqués. Vérifiez les coûts d’entrée et de sortie, les capacités et les conditions avant de choisir.",
      "empty": "Aucun modèle correspondant n’est actuellement proposé. Consultez tous les modèles ou revenez plus tard."
    },
    "coding": {
      "title": "Modèles pour le code",
      "shortDescription": "Comparez les LLM pour générer, déboguer et relire du code.",
      "slogan": "Trouvez le LLM adapté à votre développement",
      "intro": "Explorez les modèles pour écrire, expliquer, déboguer et relire du code. Comparez leurs capacités et coûts API selon vos tâches.",
      "seoTitle": "Modèles pour le code | Flatkey",
      "seoDescription": "Explorez les modèles pour écrire, expliquer, déboguer et relire du code. Comparez leurs capacités et coûts API selon vos tâches.",
      "empty": "Aucun modèle correspondant n’est actuellement proposé. Consultez tous les modèles ou revenez plus tard."
    },
    "roleplay-creative-writing": {
      "title": "Jeu de rôle et écriture créative",
      "shortDescription": "Découvrez des modèles pour les dialogues, les récits et la création.",
      "slogan": "Créez des personnages. Racontez leur histoire.",
      "intro": "Explorez les LLM pour le jeu de rôle, les dialogues et l’écriture créative. Comparez les modèles et coûts API pour votre application narrative.",
      "seoTitle": "Jeu de rôle et écriture créative | Flatkey",
      "seoDescription": "Explorez les LLM pour le jeu de rôle, les dialogues et l’écriture créative. Comparez les modèles et coûts API pour votre application narrative.",
      "empty": "Aucun modèle correspondant n’est actuellement proposé. Consultez tous les modèles ou revenez plus tard."
    },
    "image-generation": {
      "title": "Modèles de génération d’images",
      "shortDescription": "Comparez les modèles d’images, les entrées et les tarifs API.",
      "slogan": "Trouvez le modèle pour votre prochaine image",
      "intro": "Explorez les modèles de génération d’images pour votre application ou travail créatif. Comparez les entrées, les options de sortie et les tarifs API.",
      "seoTitle": "Modèles de génération d’images | Flatkey",
      "seoDescription": "Explorez les modèles de génération d’images pour votre application ou travail créatif. Comparez les entrées, les options de sortie et les tarifs API.",
      "empty": "Aucun modèle correspondant n’est actuellement proposé. Consultez tous les modèles ou revenez plus tard."
    },
    "video-generation": {
      "title": "Modèles de génération vidéo",
      "shortDescription": "Comparez les entrées, la durée, la résolution et les prix.",
      "slogan": "Trouvez le modèle pour votre prochaine vidéo",
      "intro": "Explorez les modèles vidéo à partir de texte et d’images. Comparez les durées, résolutions, options audio et tarifs pris en charge par chaque modèle.",
      "seoTitle": "Modèles de génération vidéo | Flatkey",
      "seoDescription": "Explorez les modèles vidéo à partir de texte et d’images. Comparez les durées, résolutions, options audio et tarifs pris en charge par chaque modèle.",
      "empty": "Aucun modèle correspondant n’est actuellement proposé. Consultez tous les modèles ou revenez plus tard."
    },
    "audio-generation-models": {
      "title": "Modèles de génération audio",
      "shortDescription": "Découvrez la synthèse vocale et la musique pour vidéo.",
      "slogan": "Trouvez le modèle pour vos voix et bandes-son",
      "intro": "Parcourez les modèles qui transforment le texte en parole ou créent de la musique pour une vidéo. Comparez les entrées requises et les tarifs API.",
      "seoTitle": "Modèles de génération audio | Flatkey",
      "seoDescription": "Parcourez les modèles qui transforment le texte en parole ou créent de la musique pour une vidéo. Comparez les entrées requises et les tarifs API.",
      "empty": "Aucun modèle correspondant n’est actuellement proposé. Consultez tous les modèles ou revenez plus tard."
    },
    "vision-models": {
      "title": "Modèles de vision et multimodaux",
      "shortDescription": "Trouvez des modèles de compréhension d’images et de questions visuelles.",
      "slogan": "Transformez les images en réponses utiles",
      "intro": "Explorez les modèles qui interprètent des images et répondent à des questions visuelles. Comparez les entrées, les capacités et les tarifs API.",
      "seoTitle": "Modèles de vision et multimodaux | Flatkey",
      "seoDescription": "Explorez les modèles qui interprètent des images et répondent à des questions visuelles. Comparez les entrées, les capacités et les tarifs API.",
      "empty": "Aucun modèle correspondant n’est actuellement proposé. Consultez tous les modèles ou revenez plus tard."
    },
    "text-to-speech-models": {
      "title": "Modèles de synthèse vocale",
      "shortDescription": "Comparez les modèles TTS, les voix et les tarifs API.",
      "slogan": "Donnez une voix à vos mots",
      "intro": "Explorez les modèles de synthèse vocale pour la narration et les réponses parlées. Comparez les voix, les langues et les tarifs API.",
      "seoTitle": "Modèles de synthèse vocale | Flatkey",
      "seoDescription": "Explorez les modèles de synthèse vocale pour la narration et les réponses parlées. Comparez les voix, les langues et les tarifs API.",
      "empty": "Aucun modèle correspondant n’est actuellement proposé. Consultez tous les modèles ou revenez plus tard."
    }
  },
  "pt": {
    "index": {
      "title": "Coleções de modelos de IA",
      "shortDescription": "Explore modelos por tarefa e compare preços de API.",
      "slogan": "Encontre os modelos de IA certos para seu projeto",
      "intro": "Explore modelos por tarefa, confira descontos e compare recursos e preços de API para programação, escrita, imagens, vídeos e voz.",
      "seoTitle": "Coleções de modelos de IA | Flatkey",
      "seoDescription": "Explore modelos por tarefa, confira descontos e compare recursos e preços de API para programação, escrita, imagens, vídeos e voz.",
      "empty": "Nenhum modelo correspondente está listado no momento. Veja todos os modelos ou volte mais tarde."
    },
    "discounted-models": {
      "title": "Modelos de IA com desconto",
      "shortDescription": "Confira descontos, preços de API e condições aplicáveis.",
      "slogan": "Encontre modelos de IA que cabem no seu orçamento",
      "intro": "Compare preços de API com as tarifas de referência indicadas. Confira custos de entrada e saída, recursos e condições antes de escolher.",
      "seoTitle": "Modelos de IA com desconto | Flatkey",
      "seoDescription": "Compare preços de API com as tarifas de referência indicadas. Confira custos de entrada e saída, recursos e condições antes de escolher.",
      "empty": "Nenhum modelo correspondente está listado no momento. Veja todos os modelos ou volte mais tarde."
    },
    "coding": {
      "title": "Modelos para programação",
      "shortDescription": "Compare LLMs para gerar, depurar e revisar código.",
      "slogan": "Encontre o LLM certo para seu desenvolvimento",
      "intro": "Explore modelos para escrever, explicar, depurar e revisar código. Compare recursos e custos de API conforme suas tarefas de desenvolvimento.",
      "seoTitle": "Modelos para programação | Flatkey",
      "seoDescription": "Explore modelos para escrever, explicar, depurar e revisar código. Compare recursos e custos de API conforme suas tarefas de desenvolvimento.",
      "empty": "Nenhum modelo correspondente está listado no momento. Veja todos os modelos ou volte mais tarde."
    },
    "roleplay-creative-writing": {
      "title": "Interpretação de personagens e escrita criativa",
      "shortDescription": "Explore modelos para diálogos, histórias e escrita criativa.",
      "slogan": "Crie personagens. Escreva suas histórias.",
      "intro": "Explore LLMs para interpretar personagens, criar diálogos e desenvolver histórias. Compare modelos e custos de API para sua aplicação narrativa.",
      "seoTitle": "Interpretação de personagens e escrita criativa | Flatkey",
      "seoDescription": "Explore LLMs para interpretar personagens, criar diálogos e desenvolver histórias. Compare modelos e custos de API para sua aplicação narrativa.",
      "empty": "Nenhum modelo correspondente está listado no momento. Veja todos os modelos ou volte mais tarde."
    },
    "image-generation": {
      "title": "Modelos de geração de imagens",
      "shortDescription": "Compare modelos de imagens, entradas e preços de API.",
      "slogan": "Encontre o modelo para sua próxima imagem",
      "intro": "Explore modelos de geração de imagens para sua aplicação ou trabalho criativo. Compare entradas aceitas, opções de saída e preços de API.",
      "seoTitle": "Modelos de geração de imagens | Flatkey",
      "seoDescription": "Explore modelos de geração de imagens para sua aplicação ou trabalho criativo. Compare entradas aceitas, opções de saída e preços de API.",
      "empty": "Nenhum modelo correspondente está listado no momento. Veja todos os modelos ou volte mais tarde."
    },
    "video-generation": {
      "title": "Modelos de geração de vídeo",
      "shortDescription": "Compare entradas, duração, resolução e preços de vídeo.",
      "slogan": "Encontre o modelo para seu próximo vídeo",
      "intro": "Explore modelos de vídeo a partir de texto e imagens. Compare duração, resolução, opções de áudio e preços disponíveis em cada modelo.",
      "seoTitle": "Modelos de geração de vídeo | Flatkey",
      "seoDescription": "Explore modelos de vídeo a partir de texto e imagens. Compare duração, resolução, opções de áudio e preços disponíveis em cada modelo.",
      "empty": "Nenhum modelo correspondente está listado no momento. Veja todos os modelos ou volte mais tarde."
    },
    "audio-generation-models": {
      "title": "Modelos de geração de áudio",
      "shortDescription": "Explore modelos de texto para fala e música para vídeo.",
      "slogan": "Encontre o modelo para vozes e trilhas sonoras",
      "intro": "Explore modelos para transformar texto em fala ou criar música para vídeos. Escolha uma tarefa e compare modelos, entradas necessárias e preços de API.",
      "seoTitle": "Modelos de geração de áudio | Flatkey",
      "seoDescription": "Explore modelos para transformar texto em fala ou criar música para vídeos. Escolha uma tarefa e compare modelos, entradas necessárias e preços de API.",
      "empty": "Nenhum modelo correspondente está listado no momento. Veja todos os modelos ou volte mais tarde."
    },
    "vision-models": {
      "title": "Modelos de visão e multimodais",
      "shortDescription": "Encontre modelos para compreender imagens e responder a perguntas visuais.",
      "slogan": "Transforme imagens em respostas úteis",
      "intro": "Explore modelos que interpretam imagens e respondem a perguntas sobre conteúdo visual. Compare entradas aceitas, recursos e preços de API.",
      "seoTitle": "Modelos de visão e multimodais | Flatkey",
      "seoDescription": "Explore modelos que interpretam imagens e respondem a perguntas sobre conteúdo visual. Compare entradas aceitas, recursos e preços de API.",
      "empty": "Nenhum modelo correspondente está listado no momento. Veja todos os modelos ou volte mais tarde."
    },
    "text-to-speech-models": {
      "title": "Modelos de texto para fala",
      "shortDescription": "Compare modelos TTS, vozes disponíveis e preços de API.",
      "slogan": "Dê voz às suas palavras",
      "intro": "Explore modelos de texto para fala para narração e respostas faladas. Compare vozes, idiomas e preços de API para sua aplicação.",
      "seoTitle": "Modelos de texto para fala | Flatkey",
      "seoDescription": "Explore modelos de texto para fala para narração e respostas faladas. Compare vozes, idiomas e preços de API para sua aplicação.",
      "empty": "Nenhum modelo correspondente está listado no momento. Veja todos os modelos ou volte mais tarde."
    }
  },
  "ru": {
    "index": {
      "title": "Подборки ИИ-моделей",
      "shortDescription": "Выбирайте модели по задачам и сравнивайте цены API.",
      "slogan": "Найдите ИИ-модели для своего проекта",
      "intro": "Изучайте модели по задачам, предложения со скидками, возможности и цены API для кода, творчества, изображений, видео и речи.",
      "seoTitle": "Подборки ИИ-моделей | Flatkey",
      "seoDescription": "Изучайте модели по задачам, предложения со скидками, возможности и цены API для кода, творчества, изображений, видео и речи.",
      "empty": "Подходящих моделей пока нет. Посмотрите все модели или проверьте позже."
    },
    "discounted-models": {
      "title": "ИИ-модели со скидкой",
      "shortDescription": "Сравнивайте скидки, цены API и условия доступа.",
      "slogan": "Найдите ИИ-модели под свой бюджет",
      "intro": "Сравните цены API с указанными базовыми тарифами. Проверьте стоимость ввода и вывода, возможности моделей и условия скидки.",
      "seoTitle": "ИИ-модели со скидкой | Flatkey",
      "seoDescription": "Сравните цены API с указанными базовыми тарифами. Проверьте стоимость ввода и вывода, возможности моделей и условия скидки.",
      "empty": "Подходящих моделей пока нет. Посмотрите все модели или проверьте позже."
    },
    "coding": {
      "title": "Модели для программирования",
      "shortDescription": "Сравнивайте LLM для генерации, отладки и проверки кода.",
      "slogan": "Найдите LLM для своих задач разработки",
      "intro": "Изучайте модели для написания, объяснения, отладки и проверки кода. Сравнивайте возможности и стоимость API с учётом своих задач.",
      "seoTitle": "Модели для программирования | Flatkey",
      "seoDescription": "Изучайте модели для написания, объяснения, отладки и проверки кода. Сравнивайте возможности и стоимость API с учётом своих задач.",
      "empty": "Подходящих моделей пока нет. Посмотрите все модели или проверьте позже."
    },
    "roleplay-creative-writing": {
      "title": "Ролевые диалоги и творчество",
      "shortDescription": "Изучайте модели для диалогов персонажей, историй и творческих текстов.",
      "slogan": "Создавайте персонажей. Рассказывайте их истории.",
      "intro": "Изучайте LLM для ролевых диалогов и творческих текстов. Сравнивайте модели и стоимость API для повествовательных и диалоговых приложений.",
      "seoTitle": "Ролевые диалоги и творчество | Flatkey",
      "seoDescription": "Изучайте LLM для ролевых диалогов и творческих текстов. Сравнивайте модели и стоимость API для повествовательных и диалоговых приложений.",
      "empty": "Подходящих моделей пока нет. Посмотрите все модели или проверьте позже."
    },
    "image-generation": {
      "title": "Генерация изображений",
      "shortDescription": "Сравнивайте модели изображений, входные данные и цены API.",
      "slogan": "Найдите модель для следующего изображения",
      "intro": "Изучайте модели генерации изображений для приложений и творчества. Сравнивайте поддерживаемые входные данные, параметры вывода и цены API.",
      "seoTitle": "Генерация изображений | Flatkey",
      "seoDescription": "Изучайте модели генерации изображений для приложений и творчества. Сравнивайте поддерживаемые входные данные, параметры вывода и цены API.",
      "empty": "Подходящих моделей пока нет. Посмотрите все модели или проверьте позже."
    },
    "video-generation": {
      "title": "Генерация видео",
      "shortDescription": "Сравнивайте входные данные, длительность, разрешение и цены.",
      "slogan": "Найдите модель для следующего видео",
      "intro": "Изучайте генерацию видео по тексту и изображениям. Сравнивайте длительность, разрешение, параметры звука и цены каждой модели.",
      "seoTitle": "Генерация видео | Flatkey",
      "seoDescription": "Изучайте генерацию видео по тексту и изображениям. Сравнивайте длительность, разрешение, параметры звука и цены каждой модели.",
      "empty": "Подходящих моделей пока нет. Посмотрите все модели или проверьте позже."
    },
    "audio-generation-models": {
      "title": "Генерация аудио",
      "shortDescription": "Изучайте синтез речи и создание музыки для видео.",
      "slogan": "Найдите модель для речи и музыкального сопровождения",
      "intro": "Выбирайте модели для озвучивания текста или создания музыки к видео. Сравнивайте доступные модели, требования к вводу и цены API.",
      "seoTitle": "Генерация аудио | Flatkey",
      "seoDescription": "Выбирайте модели для озвучивания текста или создания музыки к видео. Сравнивайте доступные модели, требования к вводу и цены API.",
      "empty": "Подходящих моделей пока нет. Посмотрите все модели или проверьте позже."
    },
    "vision-models": {
      "title": "Зрение и мультимодальные модели",
      "shortDescription": "Находите модели для понимания изображений и визуальных вопросов.",
      "slogan": "Превращайте изображения в полезные ответы",
      "intro": "Изучайте модели, которые интерпретируют изображения и отвечают на вопросы о них. Сравнивайте входные данные, возможности и цены API.",
      "seoTitle": "Зрение и мультимодальные модели | Flatkey",
      "seoDescription": "Изучайте модели, которые интерпретируют изображения и отвечают на вопросы о них. Сравнивайте входные данные, возможности и цены API.",
      "empty": "Подходящих моделей пока нет. Посмотрите все модели или проверьте позже."
    },
    "text-to-speech-models": {
      "title": "Синтез речи",
      "shortDescription": "Сравнивайте TTS-модели, доступные голоса и цены API.",
      "slogan": "Дайте словам голос",
      "intro": "Изучайте модели синтеза речи для озвучивания и голосовых ответов. Сравнивайте голоса, языки и цены API для своего приложения.",
      "seoTitle": "Синтез речи | Flatkey",
      "seoDescription": "Изучайте модели синтеза речи для озвучивания и голосовых ответов. Сравнивайте голоса, языки и цены API для своего приложения.",
      "empty": "Подходящих моделей пока нет. Посмотрите все модели или проверьте позже."
    }
  },
  "ja": {
    "index": {
      "title": "AI モデルコレクション",
      "shortDescription": "用途別にモデルを探し、API 料金を比較できます。",
      "slogan": "開発したいものに合う AI モデルを見つけよう",
      "intro": "用途別にモデルを探し、割引、機能、API 料金を比較できます。コードや創作から画像、動画、音声まで、次の機能に合うモデルを選びましょう。",
      "seoTitle": "AI モデルコレクション | Flatkey",
      "seoDescription": "用途別にモデルを探し、割引、機能、API 料金を比較できます。コードや創作から画像、動画、音声まで、次の機能に合うモデルを選びましょう。",
      "empty": "現在、条件に合うモデルはありません。すべてのモデルを見るか、後ほどご確認ください。"
    },
    "discounted-models": {
      "title": "割引 AI モデル",
      "shortDescription": "モデルの割引、API 料金、適用条件を確認できます。",
      "slogan": "予算に合う AI モデルを見つけよう",
      "intro": "表示された基準料金と API 料金を比較できます。入力・出力の費用、モデルの機能、適用条件を確認して選びましょう。",
      "seoTitle": "割引 AI モデル | Flatkey",
      "seoDescription": "表示された基準料金と API 料金を比較できます。入力・出力の費用、モデルの機能、適用条件を確認して選びましょう。",
      "empty": "現在、条件に合うモデルはありません。すべてのモデルを見るか、後ほどご確認ください。"
    },
    "coding": {
      "title": "コーディングモデル",
      "shortDescription": "コード生成、デバッグ、レビュー向け LLM を比較できます。",
      "slogan": "開発作業に合う LLM を見つけよう",
      "intro": "コードの作成、説明、デバッグ、レビューに使えるモデルを探せます。機能と API コストを比較し、開発作業に合うモデルを選びましょう。",
      "seoTitle": "コーディングモデル | Flatkey",
      "seoDescription": "コードの作成、説明、デバッグ、レビューに使えるモデルを探せます。機能と API コストを比較し、開発作業に合うモデルを選びましょう。",
      "empty": "現在、条件に合うモデルはありません。すべてのモデルを見るか、後ほどご確認ください。"
    },
    "roleplay-creative-writing": {
      "title": "ロールプレイと創作",
      "shortDescription": "キャラクターの会話、物語、創作向けモデルを探せます。",
      "slogan": "キャラクターを生み出し、物語を書こう。",
      "intro": "ロールプレイ、キャラクターの会話、創作向け LLM を探せます。モデルと API コストを比較し、物語や対話のアプリに合うものを選びましょう。",
      "seoTitle": "ロールプレイと創作 | Flatkey",
      "seoDescription": "ロールプレイ、キャラクターの会話、創作向け LLM を探せます。モデルと API コストを比較し、物語や対話のアプリに合うものを選びましょう。",
      "empty": "現在、条件に合うモデルはありません。すべてのモデルを見るか、後ほどご確認ください。"
    },
    "image-generation": {
      "title": "画像生成モデル",
      "shortDescription": "画像生成モデルの入力形式と API 料金を比較できます。",
      "slogan": "次の画像に合うモデルを見つけよう",
      "intro": "アプリや創作に使える画像生成モデルを探せます。対応する入力、出力の選択肢、API 料金を比較して選びましょう。",
      "seoTitle": "画像生成モデル | Flatkey",
      "seoDescription": "アプリや創作に使える画像生成モデルを探せます。対応する入力、出力の選択肢、API 料金を比較して選びましょう。",
      "empty": "現在、条件に合うモデルはありません。すべてのモデルを見るか、後ほどご確認ください。"
    },
    "video-generation": {
      "title": "動画生成モデル",
      "shortDescription": "入力形式、長さ、解像度、料金で動画モデルを比較できます。",
      "slogan": "次の動画に合うモデルを見つけよう",
      "intro": "テキストや画像から動画を生成するモデルを探せます。モデルごとの長さ、解像度、音声の選択肢、料金を比較できます。",
      "seoTitle": "動画生成モデル | Flatkey",
      "seoDescription": "テキストや画像から動画を生成するモデルを探せます。モデルごとの長さ、解像度、音声の選択肢、料金を比較できます。",
      "empty": "現在、条件に合うモデルはありません。すべてのモデルを見るか、後ほどご確認ください。"
    },
    "audio-generation-models": {
      "title": "音声・音楽生成モデル",
      "shortDescription": "音声合成や動画向け音楽生成のモデルを探せます。",
      "slogan": "音声と動画の音楽に合うモデルを見つけよう",
      "intro": "テキストを音声に変換するモデルや動画の音楽を生成するモデルを探せます。用途を選び、入力要件と API 料金を確認できます。",
      "seoTitle": "音声・音楽生成モデル | Flatkey",
      "seoDescription": "テキストを音声に変換するモデルや動画の音楽を生成するモデルを探せます。用途を選び、入力要件と API 料金を確認できます。",
      "empty": "現在、条件に合うモデルはありません。すべてのモデルを見るか、後ほどご確認ください。"
    },
    "vision-models": {
      "title": "画像理解・マルチモーダルモデル",
      "shortDescription": "画像の理解や視覚的な質問に対応するモデルを探せます。",
      "slogan": "画像から役立つ答えを得よう",
      "intro": "画像を解釈し、視覚的な質問に答えるモデルを探せます。入力形式、機能、API 料金を比較し、アプリに合うモデルを選びましょう。",
      "seoTitle": "画像理解・マルチモーダルモデル | Flatkey",
      "seoDescription": "画像を解釈し、視覚的な質問に答えるモデルを探せます。入力形式、機能、API 料金を比較し、アプリに合うモデルを選びましょう。",
      "empty": "現在、条件に合うモデルはありません。すべてのモデルを見るか、後ほどご確認ください。"
    },
    "text-to-speech-models": {
      "title": "テキスト読み上げモデル",
      "shortDescription": "TTS モデル、利用できる声、API 料金を比較できます。",
      "slogan": "言葉に声を与えよう",
      "intro": "ナレーションや音声応答向けの TTS モデルを探せます。声、言語、API 料金を比較し、アプリに合うモデルを選びましょう。",
      "seoTitle": "テキスト読み上げモデル | Flatkey",
      "seoDescription": "ナレーションや音声応答向けの TTS モデルを探せます。声、言語、API 料金を比較し、アプリに合うモデルを選びましょう。",
      "empty": "現在、条件に合うモデルはありません。すべてのモデルを見るか、後ほどご確認ください。"
    }
  },
  "vi": {
    "index": {
      "title": "Bộ sưu tập mô hình AI",
      "shortDescription": "Khám phá mô hình theo tác vụ và so sánh giá API.",
      "slogan": "Tìm mô hình AI phù hợp cho ứng dụng của bạn",
      "intro": "Duyệt mô hình theo tác vụ, xem ưu đãi và so sánh khả năng cùng giá API cho lập trình, sáng tác, hình ảnh, video và giọng nói.",
      "seoTitle": "Bộ sưu tập mô hình AI | Flatkey",
      "seoDescription": "Duyệt mô hình theo tác vụ, xem ưu đãi và so sánh khả năng cùng giá API cho lập trình, sáng tác, hình ảnh, video và giọng nói.",
      "empty": "Hiện chưa có mô hình phù hợp. Xem tất cả mô hình hoặc quay lại sau."
    },
    "discounted-models": {
      "title": "Mô hình AI giảm giá",
      "shortDescription": "Xem ưu đãi, giá API và điều kiện áp dụng.",
      "slogan": "Tìm mô hình AI phù hợp với ngân sách",
      "intro": "So sánh giá API với mức giá tham chiếu được ghi rõ. Xem chi phí đầu vào, đầu ra, khả năng mô hình và điều kiện trước khi chọn.",
      "seoTitle": "Mô hình AI giảm giá | Flatkey",
      "seoDescription": "So sánh giá API với mức giá tham chiếu được ghi rõ. Xem chi phí đầu vào, đầu ra, khả năng mô hình và điều kiện trước khi chọn.",
      "empty": "Hiện chưa có mô hình phù hợp. Xem tất cả mô hình hoặc quay lại sau."
    },
    "coding": {
      "title": "Mô hình lập trình",
      "shortDescription": "So sánh LLM để tạo, gỡ lỗi và đánh giá mã.",
      "slogan": "Tìm LLM phù hợp cho công việc lập trình",
      "intro": "Khám phá mô hình để viết, giải thích, gỡ lỗi và đánh giá mã. So sánh khả năng và chi phí API theo nhu cầu phát triển của bạn.",
      "seoTitle": "Mô hình lập trình | Flatkey",
      "seoDescription": "Khám phá mô hình để viết, giải thích, gỡ lỗi và đánh giá mã. So sánh khả năng và chi phí API theo nhu cầu phát triển của bạn.",
      "empty": "Hiện chưa có mô hình phù hợp. Xem tất cả mô hình hoặc quay lại sau."
    },
    "roleplay-creative-writing": {
      "title": "Nhập vai và sáng tác",
      "shortDescription": "Khám phá mô hình cho hội thoại nhân vật, kể chuyện và sáng tác.",
      "slogan": "Tạo nhân vật. Viết câu chuyện của họ.",
      "intro": "Khám phá LLM cho nhập vai, hội thoại nhân vật và sáng tác. So sánh mô hình và chi phí API cho ứng dụng kể chuyện hoặc hội thoại.",
      "seoTitle": "Nhập vai và sáng tác | Flatkey",
      "seoDescription": "Khám phá LLM cho nhập vai, hội thoại nhân vật và sáng tác. So sánh mô hình và chi phí API cho ứng dụng kể chuyện hoặc hội thoại.",
      "empty": "Hiện chưa có mô hình phù hợp. Xem tất cả mô hình hoặc quay lại sau."
    },
    "image-generation": {
      "title": "Mô hình tạo ảnh",
      "shortDescription": "So sánh mô hình tạo ảnh, đầu vào và giá API.",
      "slogan": "Tìm mô hình cho bức ảnh tiếp theo",
      "intro": "Khám phá mô hình tạo ảnh cho ứng dụng hoặc quy trình sáng tạo. So sánh đầu vào được hỗ trợ, tùy chọn đầu ra và giá API.",
      "seoTitle": "Mô hình tạo ảnh | Flatkey",
      "seoDescription": "Khám phá mô hình tạo ảnh cho ứng dụng hoặc quy trình sáng tạo. So sánh đầu vào được hỗ trợ, tùy chọn đầu ra và giá API.",
      "empty": "Hiện chưa có mô hình phù hợp. Xem tất cả mô hình hoặc quay lại sau."
    },
    "video-generation": {
      "title": "Mô hình tạo video",
      "shortDescription": "So sánh đầu vào, thời lượng, độ phân giải và giá video.",
      "slogan": "Tìm mô hình cho video tiếp theo",
      "intro": "Khám phá mô hình tạo video từ văn bản và hình ảnh. So sánh thời lượng, độ phân giải, tùy chọn âm thanh và giá của từng mô hình.",
      "seoTitle": "Mô hình tạo video | Flatkey",
      "seoDescription": "Khám phá mô hình tạo video từ văn bản và hình ảnh. So sánh thời lượng, độ phân giải, tùy chọn âm thanh và giá của từng mô hình.",
      "empty": "Hiện chưa có mô hình phù hợp. Xem tất cả mô hình hoặc quay lại sau."
    },
    "audio-generation-models": {
      "title": "Mô hình tạo âm thanh",
      "shortDescription": "Khám phá mô hình chuyển văn bản thành giọng nói và tạo nhạc cho video.",
      "slogan": "Tìm mô hình cho giọng nói và nhạc nền",
      "intro": "Duyệt mô hình tạo giọng nói từ văn bản hoặc tạo nhạc cho video. Chọn tác vụ để xem mô hình, yêu cầu đầu vào và giá API.",
      "seoTitle": "Mô hình tạo âm thanh | Flatkey",
      "seoDescription": "Duyệt mô hình tạo giọng nói từ văn bản hoặc tạo nhạc cho video. Chọn tác vụ để xem mô hình, yêu cầu đầu vào và giá API.",
      "empty": "Hiện chưa có mô hình phù hợp. Xem tất cả mô hình hoặc quay lại sau."
    },
    "vision-models": {
      "title": "Mô hình thị giác và đa phương thức",
      "shortDescription": "Tìm mô hình hiểu hình ảnh và trả lời câu hỏi thị giác.",
      "slogan": "Biến hình ảnh thành câu trả lời hữu ích",
      "intro": "Khám phá mô hình diễn giải hình ảnh và trả lời câu hỏi về nội dung thị giác. So sánh đầu vào, khả năng và giá API.",
      "seoTitle": "Mô hình thị giác và đa phương thức | Flatkey",
      "seoDescription": "Khám phá mô hình diễn giải hình ảnh và trả lời câu hỏi về nội dung thị giác. So sánh đầu vào, khả năng và giá API.",
      "empty": "Hiện chưa có mô hình phù hợp. Xem tất cả mô hình hoặc quay lại sau."
    },
    "text-to-speech-models": {
      "title": "Mô hình chuyển văn bản thành giọng nói",
      "shortDescription": "So sánh mô hình TTS, giọng đọc và giá API.",
      "slogan": "Mang giọng nói đến cho câu chữ",
      "intro": "Khám phá mô hình TTS cho lời dẫn và phản hồi bằng giọng nói. So sánh giọng đọc, ngôn ngữ và giá API cho ứng dụng của bạn.",
      "seoTitle": "Mô hình chuyển văn bản thành giọng nói | Flatkey",
      "seoDescription": "Khám phá mô hình TTS cho lời dẫn và phản hồi bằng giọng nói. So sánh giọng đọc, ngôn ngữ và giá API cho ứng dụng của bạn.",
      "empty": "Hiện chưa có mô hình phù hợp. Xem tất cả mô hình hoặc quay lại sau."
    }
  },
  "de": {
    "index": {
      "title": "KI-Modellsammlungen",
      "shortDescription": "Entdecken Sie Modelle nach Aufgabe und vergleichen Sie API-Preise.",
      "slogan": "Finden Sie die passenden KI-Modelle für Ihr Projekt",
      "intro": "Entdecken Sie Modelle nach Aufgabe, prüfen Sie Rabatte und vergleichen Sie Funktionen und API-Preise für Code, kreatives Schreiben, Bilder, Video und Sprache.",
      "seoTitle": "KI-Modellsammlungen | Flatkey",
      "seoDescription": "Entdecken Sie Modelle nach Aufgabe, prüfen Sie Rabatte und vergleichen Sie Funktionen und API-Preise für Code, kreatives Schreiben, Bilder, Video und Sprache.",
      "empty": "Derzeit sind keine passenden Modelle gelistet. Sehen Sie sich alle Modelle an oder schauen Sie später wieder vorbei."
    },
    "discounted-models": {
      "title": "Vergünstigte KI-Modelle",
      "shortDescription": "Vergleichen Sie Rabatte, API-Preise und Bedingungen.",
      "slogan": "Finden Sie KI-Modelle für Ihr Budget",
      "intro": "Vergleichen Sie API-Preise mit den angegebenen Referenzpreisen. Prüfen Sie Ein- und Ausgabekosten, Funktionen und Bedingungen vor Ihrer Auswahl.",
      "seoTitle": "Vergünstigte KI-Modelle | Flatkey",
      "seoDescription": "Vergleichen Sie API-Preise mit den angegebenen Referenzpreisen. Prüfen Sie Ein- und Ausgabekosten, Funktionen und Bedingungen vor Ihrer Auswahl.",
      "empty": "Derzeit sind keine passenden Modelle gelistet. Sehen Sie sich alle Modelle an oder schauen Sie später wieder vorbei."
    },
    "coding": {
      "title": "Modelle für Programmierung",
      "shortDescription": "Vergleichen Sie LLMs für Codegenerierung, Debugging und Code-Reviews.",
      "slogan": "Finden Sie das passende LLM für Ihre Entwicklung",
      "intro": "Entdecken Sie Modelle zum Schreiben, Erklären, Debuggen und Prüfen von Code. Vergleichen Sie Funktionen und API-Kosten für Ihre Aufgaben.",
      "seoTitle": "Modelle für Programmierung | Flatkey",
      "seoDescription": "Entdecken Sie Modelle zum Schreiben, Erklären, Debuggen und Prüfen von Code. Vergleichen Sie Funktionen und API-Kosten für Ihre Aufgaben.",
      "empty": "Derzeit sind keine passenden Modelle gelistet. Sehen Sie sich alle Modelle an oder schauen Sie später wieder vorbei."
    },
    "roleplay-creative-writing": {
      "title": "Rollenspiel und kreatives Schreiben",
      "shortDescription": "Entdecken Sie Modelle für Figurendialoge, Geschichten und kreative Texte.",
      "slogan": "Erschaffen Sie Figuren. Schreiben Sie ihre Geschichten.",
      "intro": "Entdecken Sie LLMs für Rollenspiel, Figurendialoge und kreatives Schreiben. Vergleichen Sie Modelle und API-Kosten für Ihre Erzähl- oder Dialoganwendung.",
      "seoTitle": "Rollenspiel und kreatives Schreiben | Flatkey",
      "seoDescription": "Entdecken Sie LLMs für Rollenspiel, Figurendialoge und kreatives Schreiben. Vergleichen Sie Modelle und API-Kosten für Ihre Erzähl- oder Dialoganwendung.",
      "empty": "Derzeit sind keine passenden Modelle gelistet. Sehen Sie sich alle Modelle an oder schauen Sie später wieder vorbei."
    },
    "image-generation": {
      "title": "Bildgenerierungsmodelle",
      "shortDescription": "Vergleichen Sie Bildmodelle, Eingabeformate und API-Preise.",
      "slogan": "Finden Sie das Modell für Ihr nächstes Bild",
      "intro": "Entdecken Sie Bildgenerierungsmodelle für Anwendungen und kreative Arbeit. Vergleichen Sie unterstützte Eingaben, Ausgabeoptionen und API-Preise.",
      "seoTitle": "Bildgenerierungsmodelle | Flatkey",
      "seoDescription": "Entdecken Sie Bildgenerierungsmodelle für Anwendungen und kreative Arbeit. Vergleichen Sie unterstützte Eingaben, Ausgabeoptionen und API-Preise.",
      "empty": "Derzeit sind keine passenden Modelle gelistet. Sehen Sie sich alle Modelle an oder schauen Sie später wieder vorbei."
    },
    "video-generation": {
      "title": "Videogenerierungsmodelle",
      "shortDescription": "Vergleichen Sie Eingaben, Dauer, Auflösung und Videopreise.",
      "slogan": "Finden Sie das Modell für Ihr nächstes Video",
      "intro": "Entdecken Sie Videomodelle für Text- und Bildeingaben. Vergleichen Sie unterstützte Dauer, Auflösung, Audiooptionen und Preise je Modell.",
      "seoTitle": "Videogenerierungsmodelle | Flatkey",
      "seoDescription": "Entdecken Sie Videomodelle für Text- und Bildeingaben. Vergleichen Sie unterstützte Dauer, Auflösung, Audiooptionen und Preise je Modell.",
      "empty": "Derzeit sind keine passenden Modelle gelistet. Sehen Sie sich alle Modelle an oder schauen Sie später wieder vorbei."
    },
    "audio-generation-models": {
      "title": "Audiogenerierungsmodelle",
      "shortDescription": "Entdecken Sie Sprachsynthese und Musikgenerierung für Videos.",
      "slogan": "Finden Sie das Modell für Sprache und Soundtracks",
      "intro": "Entdecken Sie Modelle für Text-to-Speech oder Musik zu Videos. Wählen Sie eine Aufgabe und vergleichen Sie Modelle, Eingabeanforderungen und API-Preise.",
      "seoTitle": "Audiogenerierungsmodelle | Flatkey",
      "seoDescription": "Entdecken Sie Modelle für Text-to-Speech oder Musik zu Videos. Wählen Sie eine Aufgabe und vergleichen Sie Modelle, Eingabeanforderungen und API-Preise.",
      "empty": "Derzeit sind keine passenden Modelle gelistet. Sehen Sie sich alle Modelle an oder schauen Sie später wieder vorbei."
    },
    "vision-models": {
      "title": "Vision- und multimodale Modelle",
      "shortDescription": "Finden Sie Modelle für Bildverständnis und visuelle Fragen.",
      "slogan": "Gewinnen Sie hilfreiche Antworten aus Bildern",
      "intro": "Entdecken Sie Modelle, die Bilder interpretieren und visuelle Fragen beantworten. Vergleichen Sie unterstützte Eingaben, Funktionen und API-Preise.",
      "seoTitle": "Vision- und multimodale Modelle | Flatkey",
      "seoDescription": "Entdecken Sie Modelle, die Bilder interpretieren und visuelle Fragen beantworten. Vergleichen Sie unterstützte Eingaben, Funktionen und API-Preise.",
      "empty": "Derzeit sind keine passenden Modelle gelistet. Sehen Sie sich alle Modelle an oder schauen Sie später wieder vorbei."
    },
    "text-to-speech-models": {
      "title": "Text-to-Speech-Modelle",
      "shortDescription": "Vergleichen Sie TTS-Modelle, verfügbare Stimmen und API-Preise.",
      "slogan": "Geben Sie Ihren Worten eine Stimme",
      "intro": "Entdecken Sie TTS-Modelle für Erzählungen und gesprochene Antworten. Vergleichen Sie Stimmen, Sprachen und API-Preise für Ihre Anwendung.",
      "seoTitle": "Text-to-Speech-Modelle | Flatkey",
      "seoDescription": "Entdecken Sie TTS-Modelle für Erzählungen und gesprochene Antworten. Vergleichen Sie Stimmen, Sprachen und API-Preise für Ihre Anwendung.",
      "empty": "Derzeit sind keine passenden Modelle gelistet. Sehen Sie sich alle Modelle an oder schauen Sie später wieder vorbei."
    }
  },
  "id": {
    "index": {
      "title": "Koleksi Model AI",
      "shortDescription": "Jelajahi model berdasarkan tugas dan bandingkan harga API.",
      "slogan": "Temukan model AI yang tepat untuk aplikasi Anda",
      "intro": "Jelajahi model berdasarkan tugas, lihat diskon, dan bandingkan kemampuan serta harga API untuk coding, penulisan kreatif, gambar, video, dan suara.",
      "seoTitle": "Koleksi Model AI | Flatkey",
      "seoDescription": "Jelajahi model berdasarkan tugas, lihat diskon, dan bandingkan kemampuan serta harga API untuk coding, penulisan kreatif, gambar, video, dan suara.",
      "empty": "Belum ada model yang sesuai. Lihat semua model atau periksa kembali nanti."
    },
    "discounted-models": {
      "title": "Model AI Diskon",
      "shortDescription": "Lihat diskon model, harga API, dan ketentuan yang berlaku.",
      "slogan": "Temukan model AI sesuai anggaran Anda",
      "intro": "Bandingkan harga API dengan tarif referensi yang tercantum. Tinjau biaya input dan output, kemampuan model, serta ketentuan sebelum memilih.",
      "seoTitle": "Model AI Diskon | Flatkey",
      "seoDescription": "Bandingkan harga API dengan tarif referensi yang tercantum. Tinjau biaya input dan output, kemampuan model, serta ketentuan sebelum memilih.",
      "empty": "Belum ada model yang sesuai. Lihat semua model atau periksa kembali nanti."
    },
    "coding": {
      "title": "Model Coding",
      "shortDescription": "Bandingkan LLM untuk membuat, men-debug, dan meninjau kode.",
      "slogan": "Temukan LLM yang tepat untuk alur kerja coding Anda",
      "intro": "Jelajahi model untuk menulis, menjelaskan, men-debug, dan meninjau kode. Bandingkan kemampuan serta biaya API sesuai tugas pengembangan Anda.",
      "seoTitle": "Model Coding | Flatkey",
      "seoDescription": "Jelajahi model untuk menulis, menjelaskan, men-debug, dan meninjau kode. Bandingkan kemampuan serta biaya API sesuai tugas pengembangan Anda.",
      "empty": "Belum ada model yang sesuai. Lihat semua model atau periksa kembali nanti."
    },
    "roleplay-creative-writing": {
      "title": "Roleplay dan Penulisan Kreatif",
      "shortDescription": "Jelajahi model untuk dialog karakter, cerita, dan penulisan kreatif.",
      "slogan": "Ciptakan karakter. Tulis kisah mereka.",
      "intro": "Jelajahi LLM untuk roleplay, percakapan karakter, dan penulisan kreatif. Bandingkan model serta biaya API untuk aplikasi cerita atau dialog Anda.",
      "seoTitle": "Roleplay dan Penulisan Kreatif | Flatkey",
      "seoDescription": "Jelajahi LLM untuk roleplay, percakapan karakter, dan penulisan kreatif. Bandingkan model serta biaya API untuk aplikasi cerita atau dialog Anda.",
      "empty": "Belum ada model yang sesuai. Lihat semua model atau periksa kembali nanti."
    },
    "image-generation": {
      "title": "Model Pembuatan Gambar",
      "shortDescription": "Bandingkan model gambar, input yang didukung, dan harga API.",
      "slogan": "Temukan model untuk gambar Anda berikutnya",
      "intro": "Jelajahi model pembuatan gambar untuk aplikasi atau alur kerja kreatif. Bandingkan input yang didukung, opsi output, dan harga API.",
      "seoTitle": "Model Pembuatan Gambar | Flatkey",
      "seoDescription": "Jelajahi model pembuatan gambar untuk aplikasi atau alur kerja kreatif. Bandingkan input yang didukung, opsi output, dan harga API.",
      "empty": "Belum ada model yang sesuai. Lihat semua model atau periksa kembali nanti."
    },
    "video-generation": {
      "title": "Model Pembuatan Video",
      "shortDescription": "Bandingkan input, durasi, resolusi, dan harga video.",
      "slogan": "Temukan model untuk video Anda berikutnya",
      "intro": "Jelajahi model video dengan input teks dan gambar. Bandingkan durasi, resolusi, opsi audio, dan harga yang didukung setiap model.",
      "seoTitle": "Model Pembuatan Video | Flatkey",
      "seoDescription": "Jelajahi model video dengan input teks dan gambar. Bandingkan durasi, resolusi, opsi audio, dan harga yang didukung setiap model.",
      "empty": "Belum ada model yang sesuai. Lihat semua model atau periksa kembali nanti."
    },
    "audio-generation-models": {
      "title": "Model Pembuatan Audio",
      "shortDescription": "Jelajahi model text-to-speech dan pembuatan musik untuk video.",
      "slogan": "Temukan model untuk suara dan musik latar",
      "intro": "Jelajahi model untuk mengubah teks menjadi suara atau membuat musik untuk video. Pilih tugas untuk melihat model, kebutuhan input, dan harga API.",
      "seoTitle": "Model Pembuatan Audio | Flatkey",
      "seoDescription": "Jelajahi model untuk mengubah teks menjadi suara atau membuat musik untuk video. Pilih tugas untuk melihat model, kebutuhan input, dan harga API.",
      "empty": "Belum ada model yang sesuai. Lihat semua model atau periksa kembali nanti."
    },
    "vision-models": {
      "title": "Model Vision dan Multimodal",
      "shortDescription": "Temukan model untuk memahami gambar dan menjawab pertanyaan visual.",
      "slogan": "Ubah gambar menjadi jawaban yang berguna",
      "intro": "Jelajahi model yang menafsirkan gambar dan menjawab pertanyaan tentang konten visual. Bandingkan input, kemampuan, dan harga API.",
      "seoTitle": "Model Vision dan Multimodal | Flatkey",
      "seoDescription": "Jelajahi model yang menafsirkan gambar dan menjawab pertanyaan tentang konten visual. Bandingkan input, kemampuan, dan harga API.",
      "empty": "Belum ada model yang sesuai. Lihat semua model atau periksa kembali nanti."
    },
    "text-to-speech-models": {
      "title": "Model Text-to-Speech",
      "shortDescription": "Bandingkan model TTS, pilihan suara, dan harga API.",
      "slogan": "Beri suara pada kata-kata Anda",
      "intro": "Jelajahi model text-to-speech untuk narasi dan respons lisan. Bandingkan suara, bahasa, dan harga API untuk aplikasi TTS Anda.",
      "seoTitle": "Model Text-to-Speech | Flatkey",
      "seoDescription": "Jelajahi model text-to-speech untuk narasi dan respons lisan. Bandingkan suara, bahasa, dan harga API untuk aplikasi TTS Anda.",
      "empty": "Belum ada model yang sesuai. Lihat semua model atau periksa kembali nanti."
    }
  }
};
