# AI 原生最佳实践 & 省 Token 配置

面向本仓库（TradingAgents / 「我不是神」本地产品）。配置已落在 `AGENTS.md`、`docs/`、`.cursor/rules/`、`.cursorignore`。

---

## 一、最佳实践（怎么用 AI）

### 1. 分层上下文，不要「整仓塞进对话」

| 层级 | 放什么 | 何时读 |
|---|---|---|
| **Always（极短）** | `.cursor/rules/project.mdc` | 每轮自动 |
| **入口** | `AGENTS.md` | 新会话 / 大任务开头 |
| **按需** | `docs/ARCHITECTURE|TESTING|DATA.md` | 对应任务才 `@` |
| **近况** | `HANDOFF.md` | 接锅、查近期坑 |
| **技能** | `.cursor/skills/*` | 特定流程（如验报价） |

原则：**稳定约定进 rules/docs；临时状态进 HANDOFF；大 JSON 永不贴全文。**

### 2. 任务写法（省来回）

一次说清四件事：

1. **目标**（改哪个 API / 页面）  
2. **非目标**（不要开 LIVE_MIRROR、不要动驾驶舱…）  
3. **路径边界**（`internal/market` 等）  
4. **验收命令**（`go test` / `./scripts/verify-local.sh`）

示例：

> 读 AGENTS.md。只修 `stocks_handlers` 的 market=cn；补单测；跑 `go test ./internal/api/`；后端起来后 `./scripts/verify-local.sh`。中文回复。

### 3. 关门标准

- 行情/列表：`go test` + `verify-local.sh`  
- UI：`npm run build` + TESTING 页面清单  
- 文档：新坑写进 DATA/HANDOFF，不靠聊天记忆  

### 4. 文档养护

- `AGENTS.md`：保持一屏能读完（入口 + 命令 + 硬原则）  
- `docs/*`：稳定知识  
- `HANDOFF.md`：可勾销的近况  
- 规则：`alwaysApply` 只留总则；细节用 `globs`  

---

## 二、省 Token 配置（本仓库已做 / 建议）

### 1. `.cursorignore`（已加）

忽略：`node_modules`、`dist`、巨量 `us-stocks.json` / panel / reports、`recovered_source/`、`*.db`、二进制、缓存。

效果：索引更小，Agent 更少误读大文件。需要验数时用脚本或：

```bash
python3 -c "import json;d=json.load(open('app/frontend/public/data/us-stocks.json'));print(d['generated_at'], [s for s in d['stocks'] if s['sym']=='RKLB'][0])"
```

### 2. Rules：短 + 分文件

| 文件 | alwaysApply | 说明 |
|---|---|---|
| `project.mdc` | ✅ | ≤15 行硬原则 |
| `backend-go.mdc` | ❌ + `app/backend/**/*.go` | 改 Go 才加载 |
| `frontend-react.mdc` | ❌ + `app/frontend/src/**` | 改前端才加载 |

**禁止**：把整份架构、长测试报告写进 `alwaysApply`。

### 3. Skills：按需触发

验报价用 `.cursor/skills/verify-local-quotes`，不要每次会话粘贴 HANDOFF 全文。

### 4. Cursor / 对话侧习惯

| 做法 | 省 token |
|---|---|
| 新开 Chat 做无关任务 | 避免超长历史 |
| `@` 精确文件，不 `@文件夹` 含 data | 少扫大树 |
| 让 AI `rg`/`jq` 抽样，不 `Read` 整份 JSON | 巨大 |
| 回复要求「直接改、少复述」 | 少输出 |
| 截图只贴关键区域 | 少视觉 token |
| Max Mode / 多 Agent 仅复杂任务开 | 贵 |

### 5. 模型与模式（产品侧）

- 日常改 bug / 小文件：普通 Agent 即可  
- 大范围探索：先 Plan 定边界，再 Agent 实施（少无效读写）  
- 子 Agent：只给「路径 + 验收」，不传整仓历史  

### 6. 仓库卫生

```bash
# 勿提交进 git / 勿让 AI 通读
trades.db, trading-agents*, recovered_source 探针,
llm-panel-cache, notes-cache, .env
```

---

## 三、推荐会话模板

**省 token 修 bug**

```
约束：读 AGENTS.md，勿读 us-stocks 全文，勿开 LIVE_MIRROR。
任务：…（单点）
验收：go test ./internal/market/ && ./scripts/verify-local.sh
输出：根因 3 行 + 改动文件列表 + 验收结果
```

**架构咨询（只读）**

```
只根据 docs/ARCHITECTURE.md 与 AGENTS.md 回答，不要扫 recovered_source。
```

---

## 四、自检清单

- [ ] `alwaysApply` 规则是否仍很短？  
- [ ] 大 JSON 是否在 `.cursorignore`？  
- [ ] 提示词是否带「非目标 + 验收」？  
- [ ] 是否用脚本验数而不是粘贴 6000 行股票？  
- [ ] HANDOFF 是否定期删掉已修好的条目？  

---

## 五、相关路径

- 入口：`AGENTS.md` · `LOCAL.md`  
- 规则：`.cursor/rules/`  
- 忽略：`.cursorignore`  
- 验收：`scripts/verify-local.sh` · `docs/TESTING.md`
