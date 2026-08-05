# StockGod.xyz - 部署指南

**最后更新**: 2026年7月3日  
**目标**: 将 StockGod.xyz 部署到生产环境

---

## 📋 目录

1. [环境要求](#环境要求)
2. [本地构建](#本地构建)
3. [Docker 部署](#docker-部署)
4. [云服务部署](#云服务部署)
5. [域名和 SSL](#域名和-ssl)
6. [监控和日志](#监控和日志)
7. [性能优化](#性能优化)
8. [安全建议](#安全建议)

---

## 🔧 环境要求

### 生产环境
- **操作系统**: Linux (Ubuntu 20.04+ 推荐)
- **Go**: 1.21+
- **Node.js**: 20+
- **数据库**: SQLite3 或 PostgreSQL
- **反向代理**: Nginx 或 Caddy
- **进程管理**: systemd 或 PM2

### 最低配置
- **CPU**: 2 核
- **内存**: 2GB
- **磁盘**: 20GB
- **带宽**: 10Mbps

### 推荐配置
- **CPU**: 4 核
- **内存**: 4GB
- **磁盘**: 50GB SSD
- **带宽**: 100Mbps

---

## 🏗️ 本地构建

### 1. 构建后端

```bash
cd app/backend

# 编译 Go 二进制文件
go build -o trading-agents main.go

# 或者编译为特定平台
# Linux
GOOS=linux GOARCH=amd64 go build -o trading-agents-linux main.go

# macOS (ARM)
GOOS=darwin GOARCH=arm64 go build -o trading-agents-mac main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o trading-agents.exe main.go
```

### 2. 构建前端

```bash
cd app/frontend

# 安装依赖
npm install

# 构建生产版本
npm run build

# 输出目录: dist/
# 包含: index.html, assets/
```

### 3. 测试构建

```bash
# 测试后端
cd app/backend
./trading-agents

# 测试前端（本地预览）
cd app/frontend
npm run preview
```

---

## 🐳 Docker 部署

### 1. 创建 Dockerfile（后端）

`app/backend/Dockerfile`:
```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=1 GOOS=linux go build -o trading-agents main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/trading-agents .

# Create data directory
RUN mkdir -p /app/data

# Expose port
EXPOSE 8765

# Run
CMD ["./trading-agents"]
```

### 2. 创建 Dockerfile（前端）

`app/frontend/Dockerfile`:
```dockerfile
# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./
RUN npm ci

# Copy source code
COPY . .

# Build
RUN npm run build

# Runtime stage with Nginx
FROM nginx:alpine

# Copy built files
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

### 3. Nginx 配置

`app/frontend/nginx.conf`:
```nginx
server {
    listen 80;
    server_name _;
    
    root /usr/share/nginx/html;
    index index.html;
    
    # API 代理
    location /api/ {
        proxy_pass http://backend:8765;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
    
    # WebSocket 代理
    location /ws {
        proxy_pass http://backend:8765;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_set_header Host $host;
    }
    
    # SPA 路由
    location / {
        try_files $uri $uri/ /index.html;
    }
    
    # 缓存静态资源
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

### 4. Docker Compose

`docker-compose.yml`:
```yaml
version: '3.8'

services:
  backend:
    build:
      context: ./app/backend
      dockerfile: Dockerfile
    container_name: stockgod-backend
    ports:
      - "8765:8765"
    environment:
      - PORT=8765
      - DATABASE_PATH=/app/data/trades.db
    volumes:
      - backend-data:/app/data
    restart: unless-stopped
    networks:
      - stockgod-network

  frontend:
    build:
      context: ./app/frontend
      dockerfile: Dockerfile
    container_name: stockgod-frontend
    ports:
      - "80:80"
    depends_on:
      - backend
    restart: unless-stopped
    networks:
      - stockgod-network

volumes:
  backend-data:

networks:
  stockgod-network:
    driver: bridge
```

### 5. 启动 Docker 容器

```bash
# 构建并启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止
docker-compose down

# 重启
docker-compose restart
```

---

## ☁️ 云服务部署

### AWS EC2

#### 1. 启动 EC2 实例
```bash
# 选择 Ubuntu 20.04 LTS
# 实例类型: t3.medium (2 vCPU, 4GB RAM)
# 安全组: 开放 22 (SSH), 80 (HTTP), 443 (HTTPS)
```

#### 2. 连接到实例
```bash
ssh -i your-key.pem ubuntu@your-ec2-ip
```

#### 3. 安装依赖
```bash
# 更新系统
sudo apt update && sudo apt upgrade -y

# 安装 Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# 安装 Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
```

#### 4. 部署应用
```bash
# 克隆代码
git clone <your-repo> /home/ubuntu/stockgod
cd /home/ubuntu/stockgod

# 启动服务
docker-compose up -d
```

### Google Cloud Platform (GCP)

#### 使用 Cloud Run
```bash
# 构建镜像
gcloud builds submit --tag gcr.io/your-project/stockgod-backend
gcloud builds submit --tag gcr.io/your-project/stockgod-frontend

# 部署到 Cloud Run
gcloud run deploy stockgod-backend \
  --image gcr.io/your-project/stockgod-backend \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated

gcloud run deploy stockgod-frontend \
  --image gcr.io/your-project/stockgod-frontend \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated
```

### Vercel (前端)

#### 1. 安装 Vercel CLI
```bash
npm install -g vercel
```

#### 2. 部署前端
```bash
cd app/frontend

# 登录
vercel login

# 部署
vercel --prod
```

#### 3. 配置环境变量
```bash
# 在 Vercel Dashboard 设置
VITE_API_BASE_URL=https://your-backend-api.com
```

### Railway (全栈)

#### 1. 安装 Railway CLI
```bash
npm install -g @railway/cli
```

#### 2. 部署
```bash
# 登录
railway login

# 初始化项目
railway init

# 部署后端
cd app/backend
railway up

# 部署前端
cd app/frontend
railway up
```

---

## 🔐 域名和 SSL

### 使用 Caddy (自动 HTTPS)

#### 1. 安装 Caddy
```bash
# Ubuntu/Debian
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy
```

#### 2. 配置 Caddyfile
`/etc/caddy/Caddyfile`:
```caddyfile
stockgod.xyz {
    # 前端
    reverse_proxy / localhost:80
    
    # 后端 API
    reverse_proxy /api/* localhost:8765
    
    # WebSocket
    reverse_proxy /ws localhost:8765 {
        header_up Upgrade websocket
        header_up Connection Upgrade
    }
    
    # 日志
    log {
        output file /var/log/caddy/stockgod.log
    }
}
```

#### 3. 启动 Caddy
```bash
sudo systemctl enable caddy
sudo systemctl start caddy
```

### 使用 Nginx + Let's Encrypt

#### 1. 安装 Certbot
```bash
sudo apt install certbot python3-certbot-nginx
```

#### 2. 获取 SSL 证书
```bash
sudo certbot --nginx -d stockgod.xyz -d www.stockgod.xyz
```

#### 3. Nginx 配置
`/etc/nginx/sites-available/stockgod`:
```nginx
server {
    listen 443 ssl http2;
    server_name stockgod.xyz www.stockgod.xyz;
    
    ssl_certificate /etc/letsencrypt/live/stockgod.xyz/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/stockgod.xyz/privkey.pem;
    
    # 前端
    location / {
        proxy_pass http://localhost:80;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
    
    # 后端 API
    location /api/ {
        proxy_pass http://localhost:8765;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
    
    # WebSocket
    location /ws {
        proxy_pass http://localhost:8765;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_set_header Host $host;
    }
}

# HTTP to HTTPS redirect
server {
    listen 80;
    server_name stockgod.xyz www.stockgod.xyz;
    return 301 https://$server_name$request_uri;
}
```

---

## 📊 监控和日志

### 使用 systemd（后端）

#### 1. 创建 systemd 服务
`/etc/systemd/system/stockgod-backend.service`:
```ini
[Unit]
Description=StockGod Backend Service
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/stockgod/app/backend
ExecStart=/home/ubuntu/stockgod/app/backend/trading-agents
Restart=always
RestartSec=10

# 日志
StandardOutput=append:/var/log/stockgod/backend.log
StandardError=append:/var/log/stockgod/backend-error.log

[Install]
WantedBy=multi-user.target
```

#### 2. 启动服务
```bash
# 创建日志目录
sudo mkdir -p /var/log/stockgod
sudo chown ubuntu:ubuntu /var/log/stockgod

# 启用服务
sudo systemctl enable stockgod-backend
sudo systemctl start stockgod-backend

# 查看状态
sudo systemctl status stockgod-backend

# 查看日志
sudo journalctl -u stockgod-backend -f
```

### 使用 PM2（前端开发服务器）

```bash
# 安装 PM2
npm install -g pm2

# 启动应用
cd app/frontend
pm2 start npm --name "stockgod-frontend" -- run preview

# 保存配置
pm2 save

# 开机自启
pm2 startup
```

---

## ⚡ 性能优化

### 前端优化

#### 1. 启用 Gzip 压缩
```nginx
# Nginx
gzip on;
gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;
gzip_min_length 1000;
```

#### 2. CDN 加速
```typescript
// vite.config.ts
export default defineConfig({
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'react-vendor': ['react', 'react-dom', 'react-router-dom'],
          'ui-vendor': ['zustand', 'react-markdown'],
        }
      }
    }
  }
});
```

#### 3. 图片优化
```bash
# 使用 WebP 格式
# 使用 CDN 托管静态资源
```

### 后端优化

#### 1. 数据库连接池
```go
// Go backend
db.DB().SetMaxOpenConns(100)
db.DB().SetMaxIdleConns(10)
db.DB().SetConnMaxLifetime(time.Hour)
```

#### 2. 缓存层（Redis）
```bash
# 安装 Redis
sudo apt install redis-server

# 启动 Redis
sudo systemctl enable redis-server
sudo systemctl start redis-server
```

---

## 🔒 安全建议

### 1. 环境变量保护
```bash
# 使用 .env 文件（不要提交到 Git）
echo ".env" >> .gitignore

# 使用环境变量管理工具
# - AWS Systems Manager Parameter Store
# - Google Cloud Secret Manager
# - HashiCorp Vault
```

### 2. API 限流
```go
// Go backend - 使用 gin-limiter
import "github.com/ulule/limiter/v3"

rate := limiter.Rate{
    Period: 1 * time.Minute,
    Limit:  100,
}
```

### 3. CORS 配置
```go
// 只允许特定域名
router.Use(cors.New(cors.Config{
    AllowOrigins: []string{"https://stockgod.xyz"},
    AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
}))
```

### 4. 防火墙规则
```bash
# UFW (Ubuntu)
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

### 5. 定期更新
```bash
# 自动更新安全补丁
sudo apt install unattended-upgrades
sudo dpkg-reconfigure -plow unattended-upgrades
```

---

## 🚀 部署检查清单

### 部署前
- [ ] 代码已推送到 Git
- [ ] 环境变量已配置
- [ ] 数据库已备份
- [ ] SSL 证书已申请
- [ ] 域名 DNS 已配置

### 部署中
- [ ] 后端服务已启动
- [ ] 前端服务已启动
- [ ] 反向代理已配置
- [ ] SSL 证书已安装
- [ ] 日志系统已配置

### 部署后
- [ ] 健康检查通过
- [ ] API 端点测试通过
- [ ] 前端页面正常访问
- [ ] WebSocket 连接正常
- [ ] 监控系统已配置
- [ ] 备份计划已建立

---

## 📞 问题排查

### 后端无法启动
```bash
# 检查端口占用
sudo netstat -tulpn | grep 8765

# 检查日志
sudo journalctl -u stockgod-backend -n 100

# 检查权限
ls -la /home/ubuntu/stockgod/app/backend/trading-agents
```

### 前端无法访问
```bash
# 检查 Nginx 状态
sudo systemctl status nginx

# 检查 Nginx 配置
sudo nginx -t

# 重启 Nginx
sudo systemctl restart nginx
```

### 数据库连接失败
```bash
# 检查 SQLite 文件权限
ls -la /home/ubuntu/stockgod/app/backend/trades.db

# 检查磁盘空间
df -h
```

---

**文档版本**: v1.0  
**最后更新**: 2026年7月3日  
**维护者**: AI Assistant
