# OSWBBGraph - OSWatcher 数据分析和图表展示系统

基于 Go 语言开发的 OSWatcher (OSWbb) 数据分析工具，提供 Web 界面进行交互式图表展示。

## 功能特性

- 📊 **多种数据类型支持**：VMstat、IOstat、MemInfo、Top、Ifconfig、MPstat
- 📈 **交互式图表**：基于 ECharts 的可缩放、可筛选时序图表
- ⏱️ **时间范围筛选**：支持自定义时间范围查看数据
- 🗂️ **多格式归档支持**：支持 `.tar`、`.tar.gz`、`.tgz`、`.zip` 格式
- 📦 **压缩文件支持**：自动处理 `.dat`、`.dat.gz`、`.zip` 内嵌压缩
- 🔄 **自动数据清理**：支持按天数自动清理旧数据
- 🖥️ **离线部署**：所有前端资源本地化，无需外网访问
- 🚀 **轻量级**：单文件部署，内嵌 SQLite 数据库

## 快速开始

### 下载编译

```bash
git clone https://github.com/majunyang/go-oswbb.git
cd go-oswbb
#编译windows环境二进制文件 
$env:CGO_ENABLED = "0";$env:GOOS = "windows";$env:GOARCH = "amd64"
go build -o bin/go-oswbb.exe .

#编译linux环境二进制文件 
$env:CGO_ENABLED = "0";$env:GOOS = "linux";$env:GOARCH = "amd64"
go build -o bin/go-oswbb .
```

### 启动方式

#### 方式一：上传归档文件

```bash
go-oswbb.exe
# 访问 http://localhost:3001 上传 OSWatcher 归档文件
```

#### 方式二：指定归档目录

```bash
go-oswbb.exe -archive /path/to/oswbb/archive
# 直接访问 http://localhost:3001 查看图表
```

## 命令行参数

| 参数 | 环境变量 | 默认值 | 说明 |
|------|---------|--------|------|
| `-port` | `OSWBB_PORT` | `3001` | 服务器端口 |
| `-data-dir` | `OSWBB_DATA_DIR` | `./data` | 数据目录 |
| `-upload-dir` | `OSWBB_UPLOAD_DIR` | `./uploads` | 上传目录 |
| `-db-path` | - | `./data/oswbb.db` | SQLite 数据库路径 |
| `-archive` | `OSWBB_ARCHIVE` | - | 预解压的归档目录路径 |
| `-retention-days` | `OSWBB_RETENTION_DAYS` | `3` | 数据保留天数（0=不清理） |

### 使用示例

```bash
# 基本启动
go-oswbb.exe

# 指定端口和归档目录
go-oswbb.exe -port 8080 -archive /home/oracle/oswbb/archive

# 使用环境变量
set OSWBB_PORT=8080
set OSWBB_ARCHIVE=/path/to/archive
set OSWBB_RETENTION_DAYS=7
go-oswbb.exe

# 不清理旧数据
go-oswbb.exe -archive /path/to/archive -retention-days 0
```

## 支持的数据类型

### VMstat (虚拟内存)

| 图表 | 说明 |
|------|------|
| CPU 使用率 | 用户态、系统态、空闲、IO等待 |
| 进程队列 | 运行队列(r)、阻塞队列(b) |
| 内存 | 空闲内存、缓冲区、缓存 |
| 交换空间 | 已用交换、换入/换出 |
| I/O | 块读写 |
| 系统 | 中断、上下文切换 |

### IOstat (磁盘I/O)

| 图表 | 说明 |
|------|------|
| IOPS | 读写请求数/秒 |
| 吞吐量 | 读写KB/秒 |
| 响应时间 | 读写等待时间(ms) |
| 利用率 | 设备使用百分比 |
| 队列深度 | 平均队列长度 |
| 请求大小 | 平均请求大小(KB) |

### MemInfo (内存详情)

| 图表 | 说明 |
|------|------|
| 内存概览 | 总内存、空闲、缓冲区、缓存 |
| 交换空间 | Swap总量、空闲、Cached |
| 内存详情 | Active、Inactive、AnonPages、Mapped、Shmem、Slab |
| HugePages | 大页总量、空闲 |

### Top (系统负载)

| 图表 | 说明 |
|------|------|
| 系统负载 | 1/5/15分钟负载 |
| 任务统计 | 总任务数、运行中、睡眠中 |
| CPU 使用率 | 用户态、系统态、空闲、IO等待 |
| 内存使用 | 内存和Swap使用情况 |
| Top 进程 | 按CPU排序的前15个进程 |

### MPstat (多处理器)

| 图表 | 说明 |
|------|------|
| CPU 使用率 | 整体使用率 |
| 用户态 | 用户进程CPU占用 |
| 系统态 | 内核CPU占用 |
| I/O 等待 | 等待IO的CPU时间 |
| 中断 | 硬件/软件中断 |
| 虚拟化偷取 | 虚拟化环境下的CPU偷取 |

### Ifconfig (网络接口)

| 图表 | 说明 |
|------|------|
| 吞吐量 | RX/TX MB/s |
| 数据包 | RX/TX 包/秒 |
| 错误 | RX/TX 错误/秒 |
| 丢包 | RX/TX 丢包/秒 |

## 目录结构

```
go-oswbb/
├── main.go                    # 程序入口
├── go.mod                     # Go 模块定义
├── go-oswbb.exe               # 编译后的可执行文件
├── internal/
│   ├── archive/
│   │   └── archive.go         # 归档文件解压处理
│   ├── config/
│   │   └── config.go          # 配置管理
│   ├── db/
│   │   ├── db.go              # 数据库操作
│   │   └── schema.go          # 数据库表结构
│   ├── model/
│   │   └── model.go           # 数据模型定义
│   ├── parser/
│   │   ├── common.go          # 通用解析函数
│   │   ├── vmstat.go          # VMstat 解析器
│   │   ├── iostat.go          # IOstat 解析器
│   │   ├── meminfo.go         # MemInfo 解析器
│   │   ├── top.go             # Top 解析器
│   │   ├── ifconfig.go        # Ifconfig 解析器
│   │   └── mpstat.go          # MPstat 解析器
│   └── web/
│       ├── handlers.go        # HTTP 处理函数
│       ├── server.go          # HTTP 服务器配置
│       └── template.go        # 模板管理
├── web/
│   ├── static/
│   │   ├── css/
│   │   │   ├── vendor/        # 第三方CSS (Bootstrap, Font Awesome)
│   │   │   ├── webfonts/      # Font Awesome 字体文件
│   │   │   └── style.css      # 自定义样式
│   │   └── js/
│   │       ├── vendor/        # 第三方JS (Bootstrap, ECharts)
│   │       ├── app.js         # 通用功能
│   │       ├── upload.js      # 上传页面逻辑
│   │       └── viewer.js      # 图表查看器逻辑
│   └── templates/
│       ├── layout.html        # 布局模板
│       ├── index.html         # 首页模板
│       ├── viewer.html        # 图表查看器模板
│       └── 404.html           # 404页面模板
├── data/                      # SQLite 数据库目录
├── uploads/                   # 上传文件目录
└── README.md                  # 本文件
```

## 数据清理机制

当使用 `-archive` 参数启动时，系统会自动清理旧数据：

- **启动时**：立即清理一次
- **每小时**：定期检查并清理
- **清理规则**：删除 `timestamp < (当前时间 - retention_days)` 的记录
- **空间回收**：清理后自动执行 `VACUUM` 回收空间

```bash
# 默认保留 3 天数据
go-oswbb.exe -archive /path/to/archive

# 自定义保留天数
go-oswbb.exe -archive /path/to/archive -retention-days 7

# 不清理数据
go-oswbb.exe -archive /path/to/archive -retention-days 0
```

## OSWatcher 归档目录结构

```
archive/
├── oswvmstat/          # VMstat 数据
│   ├── his_vmstat_21.12.31.0100.dat
│   └── his_vmstat_21.12.31.0100.dat.gz
├── oswiostat/          # IOstat 数据
├── oswmeminfo/         # MemInfo 数据
├── oswtop/             # Top 数据
├── oswmpstat/          # MPstat 数据
├── oswifconfig/        # Ifconfig 数据
├── oswps/              # PS 数据
├── oswslabinfo/        # SlabInfo 数据
└── ...
```

## 技术栈

- **后端**: Go 1.22+
- **数据库**: SQLite (modernc.org/sqlite, 纯Go实现)
- **前端**: Bootstrap 5.3, ECharts 5, Font Awesome 6
- **图表**: ECharts (支持缩放、筛选、导出)

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request。
