# StockGod.xyz - 快速启动指南

## 📦 环境要求

### 后端
- Go 1.21+ 
- SQLite3

### 前端
- Node.js 20+ (推荐使用 Volta 管理)
- npm 或 yarn

---

## 🚀 快速启动

### 1. 克隆项目

```bash
git clone <repository-url>
cd TradingAgents
```

### 2. 启动后端服务器

```bash
cd app/backend

# 安装依赖
go mod download

# 编译并运行
go build -o trading-agents main.go
./trading-agents
```

**后端运行在**: http://localhost:8765

#### 验证后端是否启动成功

```bash
curl http://localhost:8765/api/health
```

应该返回: `{"status":"ok"}`

### 3. 启动前端开发服务器

打开新的终端窗口:

```bash
cd app/frontend

# 安装依赖
npm install

# 启动开发服务器
npm run dev
```

**前端运行在**: http://localhost:5173

### 4. 访问应用

在浏览器中打开: http://localhost:5173/

---

## 🗺️ 页面导航

### ✅ 已实现页面

| 页面 | 路径 | 功能描述 |
|------|------|----------|
| **Portfolio** | `/portfolio` | 观察列表管理、持仓查看 |
| **ETF** | `/etf` | ETF 板块浏览、筛选、搜索 |
| **Reports** | `/reports` | 盘报查看、市场日历 |
| **Scan** | `/scan` | 股票列表、五方评分、排序筛选 |
| **Arena** | `/arena` | 股票对决、五方对比 |

### ⏸️ 占位符页面

| 页面 | 路径 | 状态 |
|------|------|------|
| **Home** | `/` | 热力图（待实现） |

---

## 🎨 功能特性

### Portfolio 页面
- 观察列表管理（LocalStorage 持久化）
- 双标签页（观察列表 / 持仓）
- 移除功能
- 空状态提示

### ETF 页面
- 8个类别筛选（宽基、行业、主题等）
- 4种排序方式（规模、近1年、近5年、抗跌）
- 实时搜索
- 47个板块展示

### Reports 页面
- 双列布局（市场日历 + 盘报列表）
- 盘前看点 & 收盘复盘
- Markdown 内容渲染
- 展开/折叠功能

### Scan 页面
- 11列数据表格
- 五方评分雷达图（五边形 SVG）
- 列头排序
- 搜索防抖
- 分页导航
- 收藏功能

### Arena 页面
- 双股票选择器（自动补全）
- VS 对决布局
- 五方评分对比（双雷达图）
- 基本指标对比
- 赢家高亮

---

## 🔧 开发命令

### 后端

```bash
cd app/backend

# 运行开发服务器
go run main.go

# 编译
go build -o trading-agents main.go

# 运行编译后的二进制
./trading-agents

# 运行测试
go test ./...

# 代码格式化
go fmt ./...
```

### 前端

```bash
cd app/frontend

# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 构建生产版本
npm run build

# 预览生产版本
npm run preview

# 代码检查
npm run lint

# TypeScript 类型检查
npm run type-check
```

---

## 🔌 API 端点

### 股票相关
- `GET /api/stocks` - 获取股票列表
  - Query: `page`, `limit`, `sort`, `order`, `search`
- `GET /api/stocks/search` - 搜索股票
  - Query: `q` (搜索关键词)

### ETF 相关
- `GET /api/etf/sectors` - 获取 ETF 板块列表
  - Query: `category`, `sort`, `page`, `limit`
- `GET /api/etf/sectors/:id` - 获取板块详情
- `GET /api/etf/search` - 搜索 ETF
  - Query: `q` (搜索关键词)

### 盘报相关
- `GET /api/reports` - 获取盘报列表
  - Query: `type` (premarket/postmarket), `page`, `limit`
- `GET /api/reports/:id` - 获取盘报详情
- `GET /api/market/calendar` - 获取市场日历

### 系统相关
- `GET /api/health` - 健康检查

---

## 🎨 设计系统

### 颜色变量

```css
/* 背景 */
--bg-background: #08090b;
--bg-surface: #111317;

/* 文字 */
--text-ink: #ecedf0;      /* 主要文字 */
--text-muted: #8e919b;    /* 次要文字 */
--text-faint: #585b66;    /* 弱化文字 */
--text-accent: #d98a6a;   /* 强调色（橙色） */

/* 涨跌 */
--text-up: #10b981;       /* 上涨（绿色） */
--text-down: #ef4444;     /* 下跌（红色） */

/* 边框 */
--border-line: rgba(235,238,245,0.07);
```

### 评分颜色编码
- **0-49**: 蓝色 (#3b82f6)
- **50-69**: 橙色 (#f97316)
- **70+**: 绿色 (#22c55e)

### 字体
- **标题**: Outfit (sans-serif)
- **代码/数字**: Fira Code (monospace)

---

## 📁 项目结构

```
TradingAgents/
├── app/
│   ├── backend/              # Go 后端
│   │   ├── cmd/              # 命令行工具
│   │   ├── internal/         # 内部包
│   │   │   ├── agents/       # AI 代理
│   │   │   ├── api/          # API 路由和处理器
│   │   │   ├── config/       # 配置
│   │   │   ├── database/     # 数据库
│   │   │   ├── dataflows/    # 数据流
│   │   │   ├── llm/          # LLM 客户端
│   │   │   ├── models/       # 数据模型
│   │   │   └── orchestrator/ # 编排器
│   │   ├── main.go           # 入口文件
│   │   └── trades.db         # SQLite 数据库
│   │
│   ├── frontend/             # React 前端
│   │   ├── src/
│   │   │   ├── components/   # 组件
│   │   │   │   ├── common/   # 通用组件
│   │   │   │   ├── layout/   # 布局组件
│   │   │   │   ├── arena/    # Arena 页面组件
│   │   │   │   ├── etf/      # ETF 页面组件
│   │   │   │   ├── portfolio/# Portfolio 页面组件
│   │   │   │   ├── reports/  # Reports 页面组件
│   │   │   │   └── scan/     # Scan 页面组件
│   │   │   ├── hooks/        # 自定义 Hooks
│   │   │   ├── pages/        # 页面组件
│   │   │   ├── stores/       # Zustand 状态管理
│   │   │   ├── types/        # TypeScript 类型
│   │   │   ├── utils/        # 工具函数
│   │   │   ├── App.tsx       # 根组件
│   │   │   ├── AppRoutes.tsx # 路由配置
│   │   │   ├── main.tsx      # 入口文件
│   │   │   └── index.css     # 全局样式
│   │   ├── .env              # 环境变量
│   │   └── package.json      # 依赖配置
│   │
│   └── backend/cmd/desktop/  # Wails 桌面应用
│       ├── main.go           # Wails 入口
│       ├── app.go            # 内嵌 Go 后端启动
│       └── wails.json        # 桌面构建配置
│
├── AI_README.md              # AI 开发者指南
├── STOCKGOD_IMPLEMENTATION.md # 实现总结
├── FEATURES_COMPLETED.md     # 功能完成清单
└── QUICKSTART.md            # 快速启动指南（本文件）
```

---

## 🐛 常见问题

### 1. 后端无法启动

**问题**: `./trading-agents: command not found`

**解决**:
```bash
cd app/backend
go build -o trading-agents main.go
./trading-agents
```

### 2. 前端无法连接后端

**问题**: `Failed to fetch` 或 `Network Error`

**解决**:
1. 确保后端服务器正在运行: `curl http://localhost:8765/api/health`
2. 检查 `app/frontend/.env` 中的 `VITE_API_BASE_URL` 是否正确:
   ```
   VITE_API_BASE_URL=http://localhost:8765
   ```
3. 重启前端开发服务器

### 3. 端口冲突

**问题**: `Error: listen EADDRINUSE: address already in use :::8765`

**解决**:
```bash
# 查找占用端口的进程
lsof -i :8765

# 杀死进程
kill -9 <PID>
```

### 4. CORS 错误

**问题**: `Access to fetch at 'http://localhost:8765/api/stocks' from origin 'http://localhost:5173' has been blocked by CORS policy`

**解决**: 后端已配置 CORS，确保后端服务器正在运行即可。

---

## 📊 数据说明

### 当前数据状态
- ⚠️ **所有 API 返回的都是 Mock 数据**
- ⚠️ **数据仅用于演示，不是真实市场数据**

### Mock 数据示例

#### 股票数据
- 50+ 个股票（Apple, Microsoft, Tesla, 茅台, 宁德时代等）
- 包含五方评分（巴菲特、段永平、宁静致远、Druckenmiller、市场情绪）
- 价格、涨跌、市值、成交量等基本指标

#### ETF 数据
- 47 个板块（科技、消费、医药、能源等）
- 8 个类别（宽基、行业、主题、策略等）
- 管理规模、头部产品、最大回撤等信息

#### 盘报数据
- 盘前看点 & 收盘复盘
- Markdown 格式内容
- 市场日历事件

---

## 🚀 生产部署

### 方式一: Wails 桌面应用（推荐）

#### 运行生产模式

```bash
# 首次安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 构建桌面静态资源 + 打包应用
make build-desktop
```

**注意事项**:
1. Wails 入口位于 `app/backend/cmd/desktop`，合法复用后端 `internal/` 包。
2. 桌面模式会在同一进程内启动 Go 后端，监听 `127.0.0.1:8765`。
3. 构建桌面资源时会把前端 API 基座固定为 `http://127.0.0.1:8765`，WebSocket 也走同一内嵌后端。

#### 打包为独立应用

```bash
make build-desktop
```

### 方式二: Web 应用

#### 后端编译

```bash
cd app/backend

# Linux
GOOS=linux GOARCH=amd64 go build -o trading-agents-linux main.go

# macOS
GOOS=darwin GOARCH=arm64 go build -o trading-agents-mac main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o trading-agents.exe main.go
```

#### 前端构建

```bash
cd app/frontend

# 构建生产版本
npm run build

# 输出目录: dist/
```

#### 部署

1. 部署后端二进制到服务器
2. 使用 Nginx/Caddy 托管前端静态文件
3. 配置反向代理 `/api` → 后端服务器

---

## 📚 更多文档

- [STOCKGOD_IMPLEMENTATION.md](./STOCKGOD_IMPLEMENTATION.md) - 完整实现总结
- [FEATURES_COMPLETED.md](./FEATURES_COMPLETED.md) - 功能完成清单
- [AI_README.md](./AI_README.md) - 项目架构和规范
- [README.md](./README.md) - 项目介绍

---

## 🤝 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启 Pull Request

---

## 📝 License

MIT

---

**最后更新**: 2026年7月3日  
**维护者**: AI Assistant
