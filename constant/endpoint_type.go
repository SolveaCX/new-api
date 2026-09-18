package constant

type EndpointType string

const (
	EndpointTypeOpenAI                EndpointType = "openai"
	EndpointTypeOpenAIResponse        EndpointType = "openai-response"
	EndpointTypeOpenAIResponseCompact EndpointType = "openai-response-compact"
	// EndpointTypeOpenRouterDecisions is OpenRouter's native structured
	// decision endpoint. It is intentionally distinct from chat completions so
	// channel selection cannot route the request to a non-OpenRouter adapter.
	EndpointTypeOpenRouterDecisions EndpointType = "openrouter-decisions"
	EndpointTypeAnthropic           EndpointType = "anthropic"
	EndpointTypeGemini              EndpointType = "gemini"
	EndpointTypeJinaRerank          EndpointType = "jina-rerank"
	EndpointTypeImageGeneration     EndpointType = "image-generation"
	EndpointTypeEmbeddings          EndpointType = "embeddings"
	EndpointTypeOpenAIVideo         EndpointType = "openai-video"
	EndpointTypeVideo               EndpointType = "video"
	EndpointTypeVideoToMusic        EndpointType = "video-to-music"
	//EndpointTypeMidjourney     EndpointType = "midjourney-proxy"
	//EndpointTypeSuno           EndpointType = "suno-proxy"
	//EndpointTypeKling          EndpointType = "kling"
	//EndpointTypeJimeng         EndpointType = "jimeng"
)
