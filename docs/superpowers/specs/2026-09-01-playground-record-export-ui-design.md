# Playground 记录批量导出入口设计

状态：已完成交互设计确认，等待实现计划。

## 背景与目标

管理员已经可以通过 `GET /api/playground/records/export` 下载服务端保存的
Playground 记录，默认返回 Excel 工作簿。当前入口需要手工调用接口，管理员在
Playground 页面无法发现或触发导出。

本次目标是在截图标注的新版 Playground 会话侧栏中增加一个管理员专用的
“批量导出”按钮，位置紧挨现有“新建”按钮的上方。首版只支持一次导出全部
记录；筛选、预览和可视化留给后续版本。

## 范围与非目标

### 范围

- 在新版 `web/default` Playground 会话侧栏增加导出入口。
- 仅 `role >= 10` 的管理员和 root 用户渲染入口。
- 点击后调用现有管理员导出 API，下载 `.xlsx` 文件。
- 导出期间提供忙碌状态、重复点击保护和可访问性标记。
- 处理成功、权限失败、网络失败和无记录等结果，并保持侧栏状态稳定。
- 为下载 helper 和按钮行为补充针对性测试。

### 非目标

- 不修改导出 API、数据库模型、Excel 列或权限中间件。
- 不在首版增加用户 ID、日期、模型或状态筛选。
- 不在首版增加导出预览表、图表、任务队列或后台导出历史。
- 不在经典主题 `web/classic` 复制新版组件；经典主题的对齐另立任务。
- 不读取或上传浏览器 localStorage/IndexedDB 中未落库的内容。

## 基础版本与代码落点

截图对应 `web/default` 最新主分支中的会话管理组件
`src/features/playground/components/playground-conversation-list.tsx`。
当前 Excel 导出开发分支早于该组件的若干主分支提交，因此实现分支应以
`origin/main` 为基线，确认既有 Playground 会话列表和导出后端均在树中；不要
通过手工恢复旧分支的整套会话 API 来解决基线差异。

计划涉及的文件边界：

- `web/default/src/features/playground/api.ts`：增加浏览器下载用的导出 helper。
- `web/default/src/features/playground/components/playground-conversation-list.tsx`：
  在侧栏动作区插入管理员按钮并绑定状态。
- `web/default/src/features/playground/components/playground-conversation-list.test.tsx`
  （若主分支没有该测试文件则新建）：覆盖权限、点击和状态行为。
- `web/default/src/features/playground/api.test.ts`：覆盖请求参数和二进制响应。
- `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi,es,pt}.json` 及静态 key：
  增加按钮、状态和错误文案。

不改 `controller/`、`model/` 或 `router/`；后端接口的 AdminAuth 保护继续由已合并
的服务端实现负责。

## 方案选择

### 方案 A：侧栏直接按钮（采用）

在现有会话动作导航中按如下顺序渲染：

```text
批量导出
新建
批量操作
```

按钮与现有 `Button`、侧栏颜色和间距复用，视觉上与截图红框一致。导出逻辑
封装为独立 API helper，组件只负责权限、忙碌状态和 Toast。入口显眼、改动
范围窄，也不会把后续筛选设计锁死。

### 方案 B：批量操作模式内入口

只有点击“批量操作”后才显示导出。它可以复用选择状态，但“全部导出”并不需要
先选会话，且管理员容易忽略入口，因此不采用。

### 方案 C：独立管理员导出页

适合未来复杂筛选和异步任务，但会新增路由、导航和页面状态，偏离本次指定的
红框位置，首版不采用。

## 组件与职责

### `downloadPlaygroundRecords`

`api.ts` 暴露一个无参数函数，使用项目统一 `api` 实例：

```text
GET /api/playground/records/export
  params: { format: "xlsx" }
  responseType: "blob"
  disableDuplicate: true
  skipErrorHandler: true
```

返回一个包含 `blob` 和可选文件名的结果，或抛出带用户可读消息的 Error。
helper 负责：

1. 校验响应类型为 Blob；
2. 从 `Content-Disposition` 提取安全文件名，解析失败时使用
   `playground-records-YYYYMMDD-HHmmss.xlsx`；
3. 对 Blob 错误响应尽量读取统一错误 JSON，不把响应正文写入日志；
4. 不在 helper 内操作 DOM，便于单测。

### `PlaygroundConversationListContent`

- 通过现有 `useIsAdmin()` 读取当前用户权限，不复制 role 常量。
- 维护局部 `isExporting` 状态；导出期间禁用导出、新建、批量操作和会话切换，
  避免下载过程中改变侧栏上下文。
- 调用 helper 后使用一个小型下载函数创建临时 `<a>`，触发浏览器下载并移除节点。
- 在点击后的短时间内不撤销 Blob URL；使用定时延迟释放（默认 60 秒），并在
  页面卸载时清理仍存活的 URL，避免 Chrome 在文件尚未读取时报告资源不足。
- 成功显示“导出已开始”；失败显示服务端消息或通用重试提示；无记录仍视为
  成功，因为后端会返回带说明页和表头的有效工作簿。

### 可访问性与响应式

- 按钮使用 `type="button"`、明确的可见文案、`aria-busy` 和加载中的禁用状态。
- 侧栏折叠时不新增浮动按钮；移动端沿用现有展开/收起机制。
- 不用颜色单独表达成功或失败，Toast 文案与图标同时提供信息。

## 数据流

```text
管理员点击
    ↓
侧栏将 isExporting 设为 true
    ↓
api.get(..., responseType="blob")
    ↓
解析 Content-Disposition + 创建 Blob URL
    ↓
临时链接下载 playground-records-*.xlsx
    ↓
显示成功 Toast，延迟释放 URL，恢复按钮
```

前端不传 `user_id`，因此始终使用后端定义的全量导出语义。即使前端角色信息
过期或被篡改，服务端 `AdminAuth` 仍会返回 401/403，数据不会泄露。

## 错误与边界处理

| 场景 | UI 行为 | 数据行为 |
| --- | --- | --- |
| 管理员且请求成功 | 下载文件，显示成功 Toast | 不改变记录 |
| 无记录 | 下载有效空工作簿，显示成功 Toast | 不改变记录 |
| 401/403 | 显示无权限/登录过期提示，按钮恢复 | 不创建本地文件 |
| 网络超时或 5xx | 显示“导出失败，请重试”，按钮恢复 | 不重试、不重复写入 |
| 重复点击 | 第二次点击被禁用 | 只保留一个请求 |
| 浏览器不支持 Blob 下载 | 显示失败 Toast，释放已创建资源 | 不改变记录 |
| 文件名头缺失或非法 | 使用固定安全回退文件名 | 下载内容不变 |

不在前端展示记录数量或正文，避免为“全部导出”额外发起查询，也避免大数据量
时因渲染列表造成内存压力。

## 国际化

新增 key（英文作为基准，其他语言提供对应翻译）：

- `Batch export`
- `Exporting Playground records...`
- `Playground records export started`
- `Failed to export Playground records`
- `You do not have permission to export Playground records`

按钮和 Toast 均通过 `useTranslation().t()` 渲染；静态 key 按 `web/default` 现有
提取规则登记，避免只在中文环境可见。

## 测试策略

按改动范围执行轻量、针对性验证，不运行无关的大规模构建：

### API helper

- 请求路径为 `/api/playground/records/export`，参数明确为 `format=xlsx`。
- 请求使用 `responseType: 'blob'`、跳过去重和统一错误拦截。
- 能从合法和缺失的 `Content-Disposition` 得到安全文件名。
- Blob 错误响应能提取服务端消息，解析失败使用通用消息。

### 侧栏组件

- admin/root 渲染“批量导出”，普通用户不渲染。
- 点击只触发一次 helper；pending 时按钮和会话动作被禁用并带 `aria-busy`。
- 成功调用下载触发器并显示成功 Toast；失败显示错误 Toast 且恢复可点击。
- 侧栏折叠、空会话列表和移动端默认状态不改变现有行为。

### 验证命令

- `bun test src/features/playground/api.test.ts src/features/playground/components/playground-conversation-list.test.tsx`
- `bun run typecheck`（`web/default` 目录）
- 对改动文件运行 ESLint/格式检查；不执行完整生产构建，除非类型或依赖验证要求。

## 发布与回滚

- 前端发布前确认目标环境后端已包含 Excel 导出接口；否则按钮在管理员侧会出现
  失败提示，但不会泄露数据。
- 这是纯前端可回滚改动：回滚前端提交即可恢复手工 API 调用方式，不影响已生成
  的 Excel 文件或数据库记录。
- 后续筛选版本应沿用同一 helper 的参数接口，新增显式 filter 类型，不在本版
  通过自由拼接查询字符串扩展权限范围。

## 验收标准

1. 管理员在截图红框对应侧栏看到“批量导出”，且位于“新建”正上方。
2. 普通用户看不到该按钮，直接调用接口仍被后端拒绝。
3. 点击一次能下载有效 `.xlsx`；文件名安全且可追溯时间。
4. 下载期间不会产生重复请求；成功或失败后按钮可再次使用。
5. Chrome 不再因立即释放 Blob URL 导致 `ERR_INSUFFICIENT_RESOURCES`。
6. 现有新建、批量操作、会话切换和折叠行为保持不变。
7. 相关单测、类型检查和改动文件 lint 通过。

