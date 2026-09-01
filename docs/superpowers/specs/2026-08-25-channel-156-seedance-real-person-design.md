# Channel 156 Seedance Gateway 真人认证接入设计

## 目标

让平台现有 `/v1/real-persons` 真人认证接口能够在显式配置 `provider=seedance_proxy` 的渠道（包括渠道 156）上工作。平台负责用户隔离、幂等、敏感字段加密、状态机和后台轮询；provider 只负责调用 Seedance Gateway 的真人认证与真人素材接口。

## 上游映射

- 创建认证：`POST {gateway}/api/seedance/face-verifications`，请求体只传可选 `return_url`；平台不向上游传递客户回调、项目名或 BytePlus AK/SK。
- 查询认证：`GET {gateway}/api/seedance/face-verifications/{verification_id}`。平台把 `verification_id` 加密保存到现有验证 token 字段，认证完成时将 `group_id` 写入现有真人档案。
- 创建素材：继续使用 `POST {gateway}/api/seedance/proxy/assets`，但 `GroupId` 使用认证返回的人像组，而不是渠道普通素材配置组。
- 查询素材：继续使用 `GET {gateway}/api/seedance/proxy/assets/{asset_id}`；列表和删除复用现有 seedance 素材协议（真人接口当前核心状态机只依赖创建、状态、删除，列表沿用 provider 现有能力）。

## 状态与错误

Gateway `waiting_user`、`callback_received`、`resolving` 映射为可重试的 pending；`verified` 返回 `group_id`；`failed`、`expired` 映射为确定性上游错误并终止本地会话。网络、超时和 5xx 保持可重试，不泄漏 API key、认证 ID、H5 签名或素材 URL。

## 凭据与路由

provider 从渠道启用 key 中选择唯一 key；多 key 渠道若启用 key 不唯一则拒绝真人认证，避免后续轮询切换到不同上游账号。`seedance_proxy` 只通过显式渠道配置进入真人 provider，不进入无指定渠道的自动候选。Gateway 地址使用现有渠道 `gateway_base_url`，必须是安全 HTTPS 地址；普通素材的配置组仍不参与真人认证组选择。

## 测试与边界

先增加 provider 选择、HTTP 请求、状态映射和 156 指定渠道回归测试，再实现代码。保留原生 BytePlus 与 TokenSpace 行为不变；不新增对外接口、不做生产发布、不需要数据库迁移。
