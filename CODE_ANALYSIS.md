# new-api 代码结构分析

## 1. 项目概述

**new-api** 是一个基于 Go + React 的 AI API 网关/聚合平台，支持将 OpenAI、Claude、Gemini、Azure、AWS Bedrock 等 40+ 上游 AI 提供者的接口统一封装为标准 API，并提供用户管理、计费、限流、渠道调度、管理后台等功能。

- **后端**：Go 1.22+，Gin Web 框架，GORM v2 ORM
- **前端**：双主题架构
  - Default 主题：React 19 + TypeScript + Rsbuild + Base UI + Tailwind CSS
  - Classic 主题：React 18 + Vite + Semi Design
- **数据库**：SQLite / MySQL >= 5.7.8 / PostgreSQL >= 9.6（同时支持）
- **缓存**：Redis + 内存缓存
- **部署**：Docker / docker-compose，支持 Electron 桌面端

---

## 2. 后端架构

后端采用清晰的分层架构：

```
Router -> Middleware -> Controller -> Service -> Model
```

### 2.1 目录职责

| 目录 | 职责 |
|------|------|
| `router/` | HTTP 路由注册（API、relay、dashboard、web 静态资源） |
| `controller/` | HTTP 请求处理器，负责参数解析和响应组装 |
| `service/` | 业务逻辑层，处理渠道选择、计费、转换等核心逻辑 |
| `model/` | 数据模型与数据库访问（GORM） |
| `relay/` | AI API 转发/代理，含各提供者的 channel adapter |
| `middleware/` | 认证、限流、CORS、日志、分发、安全校验等 |
| `setting/` | 配置管理，按功能模块拆分 |
| `dto/` | 请求/响应数据结构 |
| `constant/` | 常量定义（API 类型、渠道类型、上下文键等） |
| `common/` | 公共工具（JSON、加解密、Redis、数据库、限流等） |
| `types/` | 类型定义（relay 格式、错误类型等） |
| `oauth/` | OAuth 提供者实现（GitHub、Discord、OIDC 等） |
| `pkg/` | 内部包（billingexpr、cachex、ionet 等） |
| `i18n/` | 后端国际化（go-i18n，en/zh） |

### 2.2 核心入口

- **主入口**：`main.go`
- **路由总控**：`router/main.go`
- **API 路由**：`router/api-router.go`
- **Relay 路由**：`router/relay-router.go`
- **Web 路由**：`router/web-router.go`

### 2.3 配置管理

配置采用模块化注册机制，统一由 `setting/config/config.go` 管理：

```
setting/
├── operation_setting/      # 通用运营配置（文档链接、额度显示等）
├── system_setting/         # 系统级配置（OAuth、Passkey、主题等）
├── model_setting/          # 模型相关配置（Claude、Gemini、Grok 等）
├── ratio_setting/          # 计费比例配置
├── performance_setting/    # 性能配置
├── console_setting/        # 控制台配置
└── config/                 # 配置注册与读取核心
```

配置项通过 `config.GlobalConfig.Register("name", &struct)` 注册，运行时可通过管理后台修改。

例如 `general_setting.go` 中定义了 `DocsLink` 默认值 `https://docs.newapi.pro`，该字段会作为 `status.docs_link` 返回给前端菜单栏使用。

### 2.4 数据模型

`model/` 目录使用 GORM 定义所有数据表，主要包括：

- `channel.go`：渠道模型
- `user.go`：用户模型
- `token.go`：API Key / Token 模型
- `log.go`：请求日志
- `pricing.go`：模型计费
- `redemption.go`：充值码
- `task.go`：异步任务
- `option.go`：键值对配置

数据库兼容性要求同时支持 SQLite、MySQL、PostgreSQL，因此在 `model/main.go` 中定义了跨数据库兼容的变量（如 `commonGroupCol`、`commonTrueVal` 等）。

### 2.5 Relay 转发架构

`relay/` 是项目核心，负责将统一格式的请求转换为各上游提供者的专有格式。

```
relay/
├── channel/           # 各提供者适配器
│   ├── openai/        # OpenAI 兼容适配器（也是基础适配器）
│   ├── claude/
│   ├── gemini/
│   ├── aws/
│   ├── azure/
│   ├── deepseek/
│   └── ...
├── common/            # 通用 relay 工具
├── common_handler/    # 通用处理器
├── helper/            # 计费、模型映射、流处理等
└── *_handler.go       # 按能力分类的处理器（audio、image、embedding 等）
```

每个 channel 通常包含：
- `adaptor.go`：适配器接口实现
- `dto.go`：请求/响应 DTO
- `constants.go`：渠道常量
- `relay-*.go`：具体协议转换逻辑

### 2.6 请求处理流程

一个典型的 AI 聊天请求流转：

1. `router/relay-router.go` 接收请求
2. `middleware/auth.go` 校验 Token
3. `middleware/distributor.go` 进行请求分发/限流
4. `controller/relay.go` 处理 relay 请求
5. `service/channel_select.go` 选择合适的渠道
6. `relay/channel/{provider}/adaptor.go` 转换请求格式
7. 发送给上游提供者
8. 上游响应经适配器转回统一格式
9. `service/billing.go` / `service/tiered_settle.go` 进行计费结算
10. 返回给客户端并记录日志

### 2.7 计费系统

计费支持固定比例和动态表达式两种模式：

- **固定比例**：`ratio_setting/` 中配置模型倍率
- **动态表达式**：`pkg/billingexpr/` 实现基于 token 数、输入输出等的表达式计费

关键文件：
- `pkg/billingexpr/expr.md`：表达式语言设计文档
- `service/pre_consume_quota.go`：预扣费
- `service/tiered_settle.go`：结算
- `service/billing.go`：计费核心

---

## 3. 前端架构

前端包含两个独立的主题，构建后由 Go 后端统一托管。

### 3.1 Default 主题

```
web/default/
├── src/
│   ├── components/          # 通用 UI 组件（基于 shadcn/ui）
│   ├── features/            # 按功能域组织的业务模块
│   │   ├── home/            # 首页
│   │   ├── auth/            # 认证
│   │   ├── pricing/         # 模型广场/定价
│   │   ├── profile/         # 个人设置
│   │   ├── system-settings/ # 系统管理后台
│   │   └── ...
│   ├── hooks/               # 自定义 Hooks
│   ├── i18n/                # 国际化（6 种语言）
│   ├── lib/                 # 工具函数
│   ├── routes/              # TanStack Router 路由
│   ├── stores/              # Zustand 状态管理
│   └── styles/              # 全局样式与主题
├── public/                  # 静态资源
├── rsbuild.config.ts        # 构建配置
└── package.json
```

**首页结构**：
- `features/home/components/sections/hero.tsx`：首屏 Hero
- `features/home/components/sections/features.tsx`：核心功能
- `features/home/components/sections/how-it-works.tsx`：使用流程
- `features/home/components/sections/stats.tsx`：数据统计
- `features/home/components/sections/cta.tsx`：底部 CTA

**顶部导航**：`hooks/use-top-nav-links.ts`，根据后端 `status.HeaderNavModules` 和 `status.docs_link` 动态生成。

**国际化**：`i18n/locales/{lang}.json`，通过 `react-i18next` 的 `useTranslation()` 使用。

### 3.2 Classic 主题

```
web/classic/
├── src/
│   ├── components/     # 通用组件
│   ├── pages/          # 页面组件
│   ├── helpers/        # 工具函数与状态缓存
│   ├── hooks/          # 自定义 Hooks
│   ├── i18n/           # 国际化
│   └── context/        # React Context
├── public/
├── vite.config.js
└── package.json
```

Classic 主题采用更传统的按页面组织方式，使用 Semi Design 组件库。

### 3.3 双主题部署

`Dockerfile` 中分别构建两个主题：

```dockerfile
# Default 主题构建
FROM oven/bun:1 ... AS builder
WORKDIR /build
COPY web/default/package.json web/default/bun.lock ./
RUN bun install
COPY ./web/default .
RUN DISABLE_ESLINT_PLUGIN='true' ... bun run build

# Classic 主题构建
FROM oven/bun:1 ... AS builder-classic
...

# 最终镜像
FROM golang:1.22 ...
COPY --from=builder /build/dist ./web/default/dist
COPY --from=builder-classic /build/dist ./web/classic/dist
RUN go build -o new-api
```

后端通过 `embed-file-system.go` 将构建后的静态资源嵌入二进制或作为文件系统服务。

---

## 4. 部署与构建

### 4.1 Docker 构建

- `Dockerfile`：生产构建，多阶段构建前后端
- `Dockerfile.dev`：开发构建
- `docker-compose.yml`：生产部署
- `docker-compose.dev.yml`：开发部署

**注意**：
- 镜像 tag 为 `new-api:local`
- 构建时使用 `--no-cache` 可避免 Docker 缓存导致的前端旧代码问题
- 前端静态资源构建在容器内进行，本地 `bun run build` 不会直接进入镜像

### 4.2 开发模式

```bash
cd web/default
bun install
bun run dev          # 默认端口 3000，若被占用会提示 3001
```

后端开发：
```bash
go run main.go
```

---

## 5. 关键约定与规范

### 5.1 JSON 处理

所有 JSON marshal/unmarshal 必须使用 `common/json.go` 中的包装函数，禁止直接调用 `encoding/json`。

### 5.2 数据库兼容性

- 优先使用 GORM 方法
- 必须使用跨数据库兼容的列引用方式
- 布尔值使用 `commonTrueVal` / `commonFalseVal`
- 禁止使用 MySQL/PostgreSQL 专属函数而无 fallback

### 5.3 前端代码规范

- Default 主题优先使用 Bun
- 新增 UI 文本必须加入 i18n 翻译文件
- 使用 Tailwind CSS 进行样式开发
- 主题变量通过 `theme.css` / `index.css` 管理

### 5.4 Relay DTO 规范

对于从客户端 JSON 解析并重新 marshal 给上游的请求结构：
- 可选标量字段使用指针类型 + `omitempty`
- 确保显式零值（`0`、`false`）能正确传递给上游

---

## 6. 扩展点

### 6.1 新增上游渠道

1. 在 `relay/channel/` 下新建提供者目录
2. 实现 `adaptor.go` 中的适配器接口
3. 定义 `constants.go` 中的渠道类型常量
4. 在 `constant/channel.go` 中注册渠道类型
5. 在 `dto/` 中定义请求/响应 DTO
6. 如支持 StreamOptions，需加入 `streamSupportedChannels`

### 6.2 新增前端页面

**Default 主题**：
1. 在 `routes/` 下创建路由文件
2. 在 `features/` 下创建对应业务模块
3. 如需加入导航，修改 `hooks/use-top-nav-links.ts` 或后端 `HeaderNavModules`

**Classic 主题**：
1. 在 `pages/` 下创建页面
2. 在 `App.jsx` 或对应路由配置中注册

### 6.3 新增系统配置

1. 在 `setting/` 下创建或修改对应模块的配置结构体
2. 通过 `config.GlobalConfig.Register()` 注册
3. 在前端 system-settings 中添加对应设置界面
4. 通过 `status` 接口暴露给前端时，确保字段返回

---

## 7. 总结

new-api 的整体设计清晰：
- **后端**通过 Router-Controller-Service-Model 分层，配合 Relay 适配器模式实现了多提供者统一接入。
- **配置**采用模块化注册，便于后台动态管理。
- **前端**双主题独立演进，Default 主题面向现代化营销/管理后台，Classic 主题保留传统后台风格。
- **部署**通过 Docker 多阶段构建统一前后端，适合单机或容器化部署。

理解 `relay/channel/` 的适配器模式、`service/` 的渠道选择与计费逻辑、`setting/` 的配置注册机制，是掌握该项目代码的关键。
