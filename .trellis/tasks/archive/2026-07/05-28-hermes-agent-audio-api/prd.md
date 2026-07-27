# brainstorm: 外部音频消息播放接口

## Goal

为外部调用方提供一个稳定的入站接口，让 agent、脚本或其他服务可以向 taskdaemon 发起音频播放请求。taskdaemon 接收后根据服务端配置决定是否直接播放音频，并兼容直接上传音频文件与提交外部文件链接两种来源。播放目标按配置建模，MVP 先实现 Go 后端播放。

## What I already know

* 用户希望解决 hermes agent 发送音频的问题，但接口不应和 hermes 强绑定。
* 初步计划是由本服务开放通用接口给外部调用方调用。
* 需要支持“发音频”和“发文件链接”两种形态。
* 音频最终语义是“直接播放”，不是作为任务输入进入调度/执行流程。
* 页面需要增加服务端配置开关，控制收到外部音频消息时是否自动播放。
* 播放位置应建模为可配置项，可区分前端播放和后端播放。
* 当前版本先做后端播放，也就是由 Go 侧负责播放音频。
* 用户希望引入 Go 音频库直接播放，避免系统播放器命令在跨平台场景里变成多套系统适配。
* 当前项目 HTTP API 边界集中在 `services/taskdaemon-go/internal/httpapi`。
* 当前 router 已有 `/api/health`、`/api/auth`、`/api/tasks`、`/api/config`，未看到现成的 audio/media/webhook 模块。
* 项目规范要求 HTTP handler 负责请求绑定、认证校验、DTO 转换、稳定错误码和统一 envelope，业务判断下沉到 service 层。

## Assumptions (temporary)

* hermes agent 只是第一类调用方，不应出现在通用 API 路径、DTO 或核心 service 命名里。
* 外部调用方不应复用浏览器管理员 Cookie 作为主要认证方式。
* 音频消息后续可能不仅直接播放，还可能触发桌面通知、转写、任务关联或运行日志记录。
* 播放目标后续可扩展到前端播放，但 MVP 不实现前端播放器链路。
* MVP 不需要先实现完整对象存储，只需要定义稳定入站契约和最小处理能力。

## Delivery Plan

* 阶段 1：后端配置模型、数据库记录、文件归档、Token 哈希与验证。验收重点是可安全接收入站请求并保存本地副本与历史记录。
* 阶段 2：入站播放请求 API、历史查询 API、手动重放 API。验收重点是 JSON URL 与 multipart upload 两个入口、管理员历史查询、统一 envelope 与错误码。
* 阶段 3：FFmpeg/ffprobe 转码诊断、Oto 后端播放、播放队列。验收重点是 backend 播放、多格式输入、队列顺序播放、超限状态。
* 阶段 4：音频播放配置面板与最近记录页面。验收重点是常用配置热生效、Token 生成一次性显示、诊断状态和手动重放。
* 阶段 5：正式发行的 FFmpeg 内置与跨平台打包完善。验收重点是普通用户无需手动安装 FFmpeg，Windows/macOS/Linux 诊断路径清晰。

## Requirements (evolving)

* 提供通用外部音频播放请求 API，不和管理员管理 API 混用，不在核心契约里绑定 hermes。
* 支持二选一输入：直接上传音频文件，或提交远程文件 URL。
* URL 提交和文件上传使用两个清晰 endpoint，不混用 JSON 与 multipart。
* 页面提供服务端自动播放开关；关闭时仍可记录/展示最近收到的音频，但不自动播放。
* 服务端配置包含播放目标字段，候选值至少包括 `backend` 和 `frontend`；MVP 仅支持 `backend`，`frontend` 作为未来枚举值保留。
* 当播放目标为 `backend` 时，由 Go 后端负责下载/读取音频并调用系统播放能力。
* 自动播放开启且目标为 `backend` 时，连续收到多条音频播放请求应进入后端播放队列并顺序播放。
* 自动播放关闭时，仅保存历史记录，不进入播放队列。
* 播放队列长度可配置，默认 20；配置为 0 时表示不设上限。
* 超出播放队列上限时，仍保存历史记录，但状态标记为 `queued_skipped`。
* 页面最近记录提供手动重放操作。
* 手动重放进入同一后端播放队列，不打断当前播放，不混音播放。
* 手动重放受播放队列上限限制；队列已满时返回稳定错误，不改变原历史记录。
* 后端播放优先采用 Go 音频库直接播放，不走系统播放器命令。
* 后端播放库应满足 Windows/macOS/Linux 跨平台，尽量避免 cgo，方便桌面应用构建和发布。
* 支持格式需要按方案明确。用户希望支持更多格式，不希望 MVP 被限制在 MP3。
* FFmpeg 不要求普通用户手动安装；正式桌面发行应内置平台匹配的 FFmpeg/ffprobe 二进制，同时允许高级用户配置外部路径。
* 多格式方案已确认采用 FFmpeg/ffprobe 解码或转码，再由 Oto 负责 Go 后端播放。
* 持久化最近入站音频记录，供页面展示、排障和手动重放。
* 最近记录需要同时保存音频文件副本；上传文件和远程 URL 下载结果都归档到本地受控目录。
* 第一版本地音频副本目录固定在 taskdaemon 用户数据目录下，不提供用户自定义路径。
* 最近记录条数可配置，默认 50 条；配置为 0 时表示不设上限。
* 对请求来源做机器身份认证，避免公开接口被任意调用。
* MVP 机器认证采用服务端配置的静态 Bearer Token。
* 对音频大小、MIME 类型、URL 协议、下载超时和重定向次数设置限制。
* 音频文件大小上限可配置，默认 200MB。
* URL 下载安全策略可配置；内网地址可通过配置显式允许。
* URL 下载协议白名单可配置，默认仅允许 `https`；需要内网 `http` 时显式配置。
* 默认策略应偏保守，避免无意间把本机服务或内网服务暴露给外部 Bearer Token 调用方。
* 设置页面需要开放一部分音频配置，但不是所有配置都适合放页面编辑。
* 常用运行时行为配置可以进入设置页面；安全敏感、路径、发布部署类配置优先留在配置文件。
* 设置页面建议提供“音频播放”配置面板，而不是通用 YAML 编辑器。
* 音频播放配置面板提供入站 Bearer Token 管理：显示是否已配置、生成新 Token、生成后一次性显示、复制调用示例，不明文回显已保存 Token。
* 入站 Bearer Token 只保存哈希，不保存明文；验证时对请求 Token 做哈希比对。
* 配置面板保存常用音频配置后应运行时热生效；FFmpeg 路径、内置二进制定位等部署类配置需要重启。
* 返回统一 API envelope、traceId 和稳定错误码。
* 不在错误响应、日志或测试快照中暴露 token、签名或敏感 URL 参数。

## Acceptance Criteria (evolving)

* [x] 外部调用方可以通过接口提交音频文件并发起播放请求。
* [x] 外部调用方可以通过接口提交文件 URL 并发起播放请求。
* [x] 服务端配置中自动播放开关打开、播放目标为 `backend` 时，收到音频后由 Go 后端播放。
* [x] 多条音频连续到达时，后端按接收顺序排队播放，不打断当前播放。
* [x] 播放队列默认最多等待 20 条。
* [x] 播放队列上限可配置为 0，表示无上限。
* [x] 超出播放队列上限时，记录仍持久化，状态为 `queued_skipped`。
* [x] 服务端配置中自动播放开关关闭时，收到音频后不自动播放。
* [x] 当前版本不要求前端播放，但配置模型不阻塞后续加入 `frontend`。
* [ ] 后端播放方案在 Windows/macOS/Linux 目标上有清晰的构建与运行边界。
* [ ] 普通用户不需要手动安装 FFmpeg；缺失或不可执行时页面能展示明确诊断。
* [x] 页面可以查看最近入站音频记录。
* [x] 最近入站音频记录包含可重放的本地音频副本。
* [x] 页面可以对最近音频记录发起手动重放。
* [x] 手动重放进入后端播放队列，并遵循队列上限。
* [x] 最近记录默认最多保留 50 条。
* [x] 最近记录保留上限可配置为 0，表示无上限。
* [x] 非法认证、非法 MIME、超限文件、不可下载 URL 都返回稳定错误码。
* [x] 超过 200MB 默认大小上限的上传或 URL 下载会被拒绝。
* [x] URL 下载策略可以配置是否允许内网地址。
* [x] URL 下载协议白名单默认仅允许 `https`，并可配置允许 `http`。
* [ ] 设置页面可以管理常用音频播放配置。
* [x] 高级安全/路径/二进制配置保留在配置文件中，并在页面展示当前状态或提示配置文件位置。
* [ ] 音频配置面板提供连接/能力检测，例如 FFmpeg 可用性、播放目标、最近一次播放错误。
* [ ] 音频配置面板可以生成入站 Bearer Token，生成后只显示一次。
* [ ] 音频配置面板不明文展示已保存 Bearer Token。
* [x] 已保存 Token 不以明文形式落配置、数据库、日志或 API 响应。
* [x] 常用音频配置保存后运行时生效，并在 reload/apply 结果中展示。
* [ ] 需要重启的音频配置会明确提示。
* [x] 后端 route 测试覆盖成功路径和主要失败路径。

## Definition of Done

* Tests added/updated (unit/integration where appropriate)
* Lint / typecheck / CI green
* Docs/notes updated if behavior changes
* Rollout/rollback considered if risky

## Out of Scope

* 暂不实现复杂媒体库或长期对象存储管理。
* 暂不把所有外部 agent 能力一次性平台化。
* 暂不支持任意文件类型，MVP 先聚焦音频。
* 暂不实现前端播放链路。

## Technical Notes

* `services/taskdaemon-go/internal/httpapi/router.go` 是 route 装配入口。
* `services/taskdaemon-go/internal/httpapi/*_routes.go` 和 `*_dto.go` 是新增接口的自然落点。
* `.trellis/spec/backend/api-contracts.md` 要求新增 API 同步检查 DTO、前端 API client、CLI daemon client 和测试，避免契约漂移。
* 这类外部入站接口建议新增独立 service 边界，例如 `internal/inbound`、`internal/media` 或 `internal/audio`，避免 handler 直接处理下载、校验、持久化或通知派发。
* API 命名决策：使用 `/api/inbound/audio-play-requests` 表达“外部调用方请求 taskdaemon 播放音频”，避免 `audio-messages` 只表达消息投递而缺少播放语义。
* 入站接口形态：`POST /api/inbound/audio-play-requests` 使用 JSON 提交 URL；`POST /api/inbound/audio-play-requests/upload` 使用 `multipart/form-data` 上传文件。
* 配置命名候选：`audio.autoplay.enabled`、`audio.autoplay.target`，其中 target MVP 使用 `backend`。
* 播放队列配置候选：`audio.playback.queueLimit`，默认值 20，值为 0 表示无上限。
* 入站认证配置候选：`audio.inbound.token`。请求使用 `Authorization: Bearer <token>`。
* 最近记录配置候选：`audio.history.limit`，默认值 50，值为 0 表示无上限。超过上限时按创建时间清理最旧记录。
* URL 下载配置候选：`audio.inbound.url.allowPrivateNetworks`，默认 false；开启后允许内网、回环、本机链路地址等私有网络目标。
* URL 下载可进一步支持域名/IP allowlist，例如 `audio.inbound.url.allowedHosts`，用于比“允许全部内网”更细的控制。
* URL 协议配置候选：`audio.inbound.url.allowedSchemes`，默认 `["https"]`；内网 HTTP 场景可配置为 `["https", "http"]`。
* 音频大小配置候选：`audio.inbound.maxBytes`，默认 `209715200`，即 200MB。
* 下载与转码超时配置候选：`audio.inbound.url.downloadTimeoutSeconds` 默认 60，`audio.inbound.url.maxRedirects` 默认 3，`audio.ffmpeg.transcodeTimeoutSeconds` 默认 120。
* 设置页面开放配置建议：
  * 开放：`audio.autoplay.enabled`、`audio.autoplay.target`、`audio.playback.queueLimit`、`audio.history.limit`、`audio.inbound.maxBytes`。
  * 谨慎开放或仅显示：`audio.inbound.url.allowedSchemes`、`audio.inbound.url.allowPrivateNetworks`、`audio.inbound.url.allowedHosts`。
  * 配置文件优先：`audio.inbound.token`、`audio.ffmpeg.path`、`audio.ffmpeg.probePath`、`audio.ffmpeg.transcodeTimeoutSeconds`、下载超时和重定向上限。
* 音频配置面板建议分组：
  * 基础：自动播放开关、播放目标、队列上限、历史保留条数。
  * 入站接口：Bearer Token 状态、复制调用示例、上传/URL endpoint 展示。
  * URL 安全：允许协议、是否允许内网、Host 白名单。
  * 诊断：FFmpeg/ffprobe 状态、Oto 播放状态、最近错误、测试播放按钮。
  * 高级：显示配置文件路径，并提供“打开配置文件/重新加载配置”入口。
* 现有配置规范显示运行时热重载目前只支持 `logging.http` 与 `observability.traceId`；音频配置若要页面保存后立即生效，需要新增线程安全 runtime holder 和 reload/apply 测试。若第一版不做页面写配置，则页面可以先展示配置状态并提示编辑配置文件后重载。
* 音频配置热生效范围：`audio.autoplay.enabled`、`audio.autoplay.target`、`audio.playback.queueLimit`、`audio.history.limit`、`audio.inbound.maxBytes`、`audio.inbound.url.allowedSchemes`、`audio.inbound.url.allowPrivateNetworks`、`audio.inbound.url.allowedHosts`、`audio.inbound.tokenHash`。
* 音频配置重启生效范围：`audio.ffmpeg.path`、`audio.ffmpeg.probePath`、内置 FFmpeg 二进制发现策略、可能影响 app 装配的底层播放驱动初始化参数。
* 音频副本存放到 taskdaemon 用户数据目录下的受控子目录，例如 `audio/inbound/`；数据库只保存相对路径、原始文件名、来源、MIME、大小、校验值、创建时间、播放状态和错误摘要。
* 播放状态建议包含：`received`、`queued`、`playing`、`played`、`failed`、`skipped`、`queued_skipped`。
* 手动重放 API：`POST /api/audio/history/{id}/replay`，受管理员 session 保护，不使用外部 Bearer Token。
* Go 音频库候选：`github.com/ebitengine/oto/v3`。Context7 文档显示 Oto v3 是跨平台 PCM 播放库，支持 Windows、macOS、Linux 等平台，桌面平台不需要 cgo；同一进程只能创建一个 audio context，适合在应用启动时或首次播放时初始化复用。
* Oto 播放 PCM，需要配套解码器。若使用纯 Go 解码器组合，可逐个接入 MP3/WAV/OGG/FLAC 等解码器，但格式越多，维护矩阵越大。
* Go 播放框架候选：`github.com/gopxl/beep`。Context7 文档显示 beep 提供播放与音频处理抽象，并有 MP3 等 decoder 包；它更高层，但仍需确认输出驱动、依赖和目标平台构建表现。
* FFmpeg 方案：Context7 文档显示 FFmpeg 是跨平台音视频处理工具，支持大量音频编解码与格式转换。可由 Go 调用受控的 ffmpeg/ffprobe，把输入统一转为 PCM/WAV，再交给 Oto 播放；优点是格式覆盖最强，缺点是需要管理跨平台 FFmpeg 二进制、体积、许可证与安全边界。
* FFmpeg 获取策略：解析顺序建议为 `audio.ffmpeg.path` / `audio.ffmpeg.probePath` 配置覆盖 -> 应用内置二进制 -> `PATH`。开发期可以依赖本机 PATH 或显式配置；正式包不依赖用户自行安装。
* 安全边界：远程 URL 由 taskdaemon 自己下载到受控临时文件，再交给 FFmpeg 处理；不要让 FFmpeg 直接拉远程 URL，避免协议面和 SSRF 风险扩大。
* 决策：采用“内置 FFmpeg 优先，配置路径覆盖，PATH 兜底”。普通用户不需要手动安装 FFmpeg。
* 决策：外部音频入站 API 的 MVP 认证采用静态 Bearer Token；HMAC 签名和 integration key 留给后续扩展。
* 决策：持久化最近入站音频记录，条数默认 50，可配置为 0 表示无上限。
* 决策：最近记录保存音频文件副本，避免远程 URL 过期或源文件删除后无法重放。
* 决策：第一版本地音频副本目录固定在 taskdaemon 用户数据目录下，不提供路径配置。
* 决策：入站音频 API 拆成两个 endpoint，JSON URL 与 multipart upload 分开处理，并统一命名为 audio play request。
* 决策：连续收到多条音频播放请求时，采用后端队列顺序播放；不打断当前播放，也不混音播放。
* 决策：播放队列上限可配置，默认 20，配置为 0 表示无上限；超限请求保存记录但不进入队列。
* 决策：第一版页面最近记录提供手动重放。
* 决策：手动重放进入同一后端播放队列，不打断当前播放；队列已满时返回错误，不改变原历史记录。
* 决策：URL 下载允许内网场景，但必须由服务端配置显式放开；默认不允许私有网络目标。
* 决策：URL 下载协议白名单可配置，默认仅允许 `https`。
* 决策：音频文件大小上限可配置，默认 200MB。
* 决策：下载超时默认 60 秒，重定向上限默认 3，FFmpeg 转码超时默认 120 秒。
* 决策：设置页面开放常用音频配置，安全敏感、路径和二进制定位类配置优先保留在配置文件。
* 决策：新增专用“音频播放”配置面板，采用基础配置、入站接口、URL 安全、诊断和高级配置文件入口的分组。
* 决策：音频播放配置面板支持生成入站 Bearer Token、一次性显示和复制调用示例；不回显已保存 Token 明文。
* 决策：入站 Bearer Token 只保存哈希，不保存明文；Token 丢失时通过面板重新生成。
* 决策：常用音频配置支持运行时热生效；FFmpeg 路径和二进制定位类配置提示需要重启。
