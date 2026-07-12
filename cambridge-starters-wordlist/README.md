# Starters Learning Server (Gin)

剑桥 Starters 单词与语法第一阶段学习卡片 —— Go Gin 本地服务。

## 本地部署步骤（macOS / Go 1.23.3）

### 1. 复制项目到本地

将整个 `cambridge-starters-wordlist` 文件夹复制到本地电脑的任意目录，例如 `~/starters-learning/`。

### 2. 编译运行

打开终端，进入项目目录：

```bash
cd ~/starters-learning/cambridge-starters-wordlist
```

#### 首次运行（拉取依赖 + 编译）

```bash
/Users/jarick/sdk/go1.25.11/bin/go mod tidy
/Users/jarick/sdk/go1.25.11/bin/go build -o starters-server .
```

#### 之后直接运行

```bash
./starters-server
```

### 3. 访问

| 方式 | 地址 |
|------|------|
| 本机 | http://localhost:8080 |
| 局域网（手机/平板） | http://`你的电脑IP`:8080 |

查看本机 IP：
```bash
ifconfig | grep "inet "
```

### 4. 停止服务

在运行服务的终端中按 `Ctrl + C`。

### 5. 自定义端口（可选）

```bash
PORT=3000 ./starters-server
```

## 文件说明

| 文件 | 说明 |
|------|------|
| `cambridge-starters-wordlist.html` | 学习卡片主页面（含英式发音 + 学习进度） |
| `main.go` | Go Gin 服务器代码 |
| `go.mod` / `go.sum` | Go 依赖管理 |
| `assets/` | 封面图片 |
| `_shared/fonts/` | Outfit 字体文件 |

## 功能说明

- **英式发音**：点击单词卡片上的小喇叭，自动朗读英式发音
- **学习进度**：点击单词卡片可标记"已掌握"，进度自动保存到浏览器
- **分类导航**：顶部导航可快速跳转到各主题
- **进度总览**：顶部进度条显示各分类掌握情况

## 升级日志

- v2.0：补充完整官方 Starters 词汇表、新增 5 个语法点、嵌入英式发音、自动进度保存、性能优化、Go Gin 本地服务

## 需求

- 当前应用主要适用于 小学生学习剑桥英语，以及基本数学运算学习程序
- 要求增加进度跟踪，任务分配机制

### 参考

1. Oxford Phonics World <https://elt.oup.com/catalogue/items/global/young_learners/oxford_phonics_world/?cc=cn&selLanguage=zh>
2. Oxford Phonics Starter <https://elt.oup.com/catalogue/items/global/young_learners/cambridge_english_qualifications_young_learners_practice_tests/?cc=cn&selLanguage=zh&mode=hub>
3. 剑桥考试网站<https://www.cambridgeenglish.cn/exams-and-tests/young-learners-english/starters/>