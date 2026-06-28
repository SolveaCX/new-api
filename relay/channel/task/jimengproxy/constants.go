package jimengproxy

// ChannelName 即梦反代(iptag/jimeng-api)异步视频渠道名。
const ChannelName = "jimengproxy"

// ModelList 是即梦反代支持的【视频】模型(异步任务,走 submit/query 轮询)。
// 图像模型 jimeng 是同步 OpenAI 图片协议,由 ChannelTypeJimengProxy -> APITypeOpenAI
// 处理,不在此列表内。
var ModelList = []string{
	"jimeng-video-3.0",
	"jimeng-video-3.0-pro",
	"jimeng-video-2.0",
	"jimeng-video-2.0-pro",
}
