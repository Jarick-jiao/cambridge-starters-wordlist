# 游戏扩展指南

本文档说明如何在本平台中新增一个学习游戏，以及如何复用平台提供的核心扩展模块。得益于 `GameRegistry` 插件化架构 + 六大解耦模块，新增游戏只需 3 步，且可声明式接入积分、事件、日志、配置体系。

## 架构概览

```
┌─────────────────────────────────────────────────────────┐
│  Core Extension Modules（核心扩展基础设施）              │
│  ├── EventBus         跨模块事件总线（pub/sub）          │
│  ├── Storage          命名空间化 localStorage 封装        │
│  ├── AppConfig        集中配置（游戏参数/难度/积分）     │
│  ├── Theme            运行时主题切换（CSS 变量驱动）      │
│  ├── WordDataProvider 词汇数据抽象层                     │
│  └── Logger           通过 EventBus 自动收集事件日志     │
└─────────────────────────────────────────────────────────┘
                         ▲
                         │ 订阅/调用
┌────────────────────────┴────────────────────────────────┐
│  GameRegistry（游戏插件化注册中心）                      │
│  ├── register(cfg)     注册游戏（支持生命周期钩子）      │
│  ├── get(id) / getAll() / has(id)                       │
│  ├── renderTabs()      动态渲染 tabs                    │
│  ├── switchTo(id)      统一切换 + onStart/onExit 钩子   │
│  └── complete(result)  通知一局结束 + onWin/onComplete  │
└─────────────────────────────────────────────────────────┘
                         ▲
                         │ 注册
┌────────────────────────┴────────────────────────────────┐
│  各游戏模块（browse/chapters/memory/listen/spell/picture）│
└─────────────────────────────────────────────────────────┘
```

## 游戏配置结构（含生命周期钩子）

```javascript
GameRegistry.register({
  id: 'unique-id',                  // 唯一标识
  label: '游戏名称',                 // tab 显示名
  icon: '🎮',                       // tab 图标
  type: 'play' | 'content',         // play=game-play-area, content=english-content
  start: function() {},             // 启动函数（必需）
  config: {                         // 游戏参数（自由扩展）
    total: 10,
    pointsPerCorrect: 3,
    description: '说明'
  },
  // —— 以下生命周期钩子均为可选 ——
  onStart:    function() {},         // 进入游戏时（start 之后）
  onExit:     function() {},         // 离开游戏时
  onComplete: function(result) {},   // 一局结束时（不论胜负）
  onWin:      function(result) {},   // 一局胜利时（stars >= 2）
  pointsRule: function(result) {     // 自定义积分计算
    return result.correct * 3;
  }
});
```

`result` 对象结构（传递给 `onComplete` / `onWin` / `pointsRule`）：

```javascript
{
  mode: 'mygame',          // 游戏 id
  stars: 3,                // 星数 0~3（≥2 视为胜利）
  points: 15,              // 积分（若提供 pointsRule 则用其返回值）
  correct: 9,              // 答对题数（可选）
  total: 10,               // 总题数（可选）
  time: 45,                // 用时秒（可选）
  moves: 8,                // 操作次数（可选）
  title: '太棒了！',        // 结果标题（可选）
  score: '答对 9 / 10 题'   // 结果描述（可选）
}
```

> 调用 `showGameResult({...})` 会自动触发 `GameRegistry.complete()`，无需手动调用。所有钩子执行后，对应事件会通过 `EventBus` 广播。

## 新增游戏步骤

### 第 1 步：编写游戏逻辑函数

在 `<script>` 的 IIFE 内部，添加你的游戏函数：

```javascript
function startMyGame() {
  // 推荐从 AppConfig 读取参数（支持运行时覆盖）
  var total = AppConfig.get('mygame', 'total', 10);
  myGameState = { round: 0, correct: 0, total: total };
  nextMyRound();
}

function nextMyRound() {
  // 推荐用 WordDataProvider 获取词汇（屏蔽数据来源细节）
  var words = WordDataProvider.getWordsByPhase();
  var picked = WordDataProvider.sample(words, 4);
  // ... 渲染题目
}

function finishMyGame() {
  var stars = myGameState.correct >= 9 ? 3 : (myGameState.correct >= 7 ? 2 : 1);
  var starStr = '';
  for (var s = 0; s < 3; s++) starStr += s < stars ? '⭐' : '☆';
  var points = myGameState.correct * 3;
  // showGameResult 会自动调用 GameRegistry.complete() 广播事件
  showGameResult({
    stars: starStr,
    title: stars === 3 ? '太棒了！' : '继续加油！',
    score: '答对 ' + myGameState.correct + ' / ' + myGameState.total + ' 题',
    points: '+' + points + ' 积分',
    onRetry: 'startMyGame()',
    onBack: 'switchGameMode(\'browse\')'
  });
  earnPoints('mygame_complete', points);
}
```

### 第 2 步：注册到 GameRegistry

在所有 `GameRegistry.register({...})` 调用之后追加：

```javascript
GameRegistry.register({
  id: 'mygame',
  label: '我的游戏',
  icon: '🎮',
  type: 'play',
  start: function() { startMyGame(); },
  config: { total: 10, pointsPerCorrect: 3, description: '游戏说明' },
  // 声明式接入积分规则（无需在 finishMyGame 里手动算）
  pointsRule: function(r) { return r.correct * AppConfig.get('mygame', 'pointsPerCorrect', 3); },
  // 胜利时额外奖励
  onWin: function(r) {
    if (r.stars === 3) earnPoints('mygame_perfect', 5);
    showToast('🎉 完美通关！');
  }
});
```

同时在 `AppConfig.games` 中追加默认配置（便于家长模式/难度切换统一调整）：

```javascript
AppConfig.games.mygame = { total: 10, pointsPerCorrect: 3, description: '我的游戏' };
```

### 第 3 步：添加 CSS（如需新样式）

在 `<style>` 块中添加游戏专用样式，**务必使用设计令牌**（不要硬编码颜色/圆角）：

```css
.mygame-area { padding: 1rem 1.5rem; text-align: center; }
.mygame-prompt {
  background: var(--surface);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  padding: 1.5rem;
}
```

并在 `@media (max-width: 600px)` 中添加移动端样式：

```css
@media (max-width: 600px) {
  .mygame-area { padding: 0.8rem 1rem; }
  .mygame-prompt { padding: 1.2rem 0.8rem; }
}
```

完成。tab 会自动出现，切换逻辑、事件广播、日志收集全部自动处理。

## 核心扩展模块 API 速查

### EventBus — 跨模块事件总线

```javascript
// 订阅事件（返回回调本身，便于配对 off）
var unsub = EventBus.on(EVT.GAME_WIN, function(result) {
  console.log('玩家胜利了', result);
});

// 取消订阅
EventBus.off(EVT.GAME_WIN, unsub);

// 只订阅一次
EventBus.once(EVT.GAME_COMPLETE, function(r) { ... });

// 触发事件（任何模块都可发）
EventBus.emit('mycustom:event', { foo: 'bar' });
```

**预置事件名（`EVT` 常量）：**

| 常量 | 事件名 | 触发时机 | data |
|------|--------|----------|------|
| `EVT.GAME_START` | `game:start` | 进入游戏 | `{mode, prev}` |
| `EVT.GAME_COMPLETE` | `game:complete` | 一局结束 | `result` |
| `EVT.GAME_WIN` | `game:win` | 一局胜利（≥2星） | `result` |
| `EVT.GAME_EXIT` | `game:exit` | 离开游戏 | `{mode}` |
| `EVT.POINTS_EARNED` | `points:earned` | 获得积分 | `{reason, count, x, y}` |
| `EVT.WORD_MASTERED` | `word:mastered` | 标记单词掌握 | `{word, group, phase}` |
| `EVT.PHASE_CHANGE` | `phase:change` | 切换阶段 | `{from, to}` |
| `EVT.CHAPTER_CHANGE` | `chapter:change` | 切换章节 | `{from, to, phase}` |
| `EVT.THEME_CHANGE` | `theme:change` | 切换主题 | `{name, tokens}` |
| `EVT.CONFIG_CHANGE` | `config:change` | 覆盖配置 | `{gameId, cfg}` |

### Storage — 命名空间化存储

所有 key 自动加 `cam_learn_` 前缀，避免与其他应用冲突。

```javascript
Storage.set('user_name', '樱桃');              // → localStorage['cam_learn_user_name']
Storage.get('user_name', '匿名');               // 读取，带默认值
Storage.setJSON('progress', { level: 5 });      // 自动 JSON.stringify
Storage.getJSON('progress', { level: 0 });      // 自动 JSON.parse
Storage.remove('user_name');                    // 删除
```

### AppConfig — 集中配置

```javascript
// 读取（带 override 优先级 + 默认值兜底）
AppConfig.get('memory', 'pairs', 6);            // → 6
AppConfig.get('spell', 'total', 8);             // → 8

// 运行时覆盖（不修改源对象，便于家长模式/难度切换）
AppConfig.override('spell', { total: 12, wordLenMax: 8 });

// 重置覆盖
AppConfig.reset('spell');
AppConfig.reset();                              // 全部重置
```

**已注册的游戏配置：**

| 游戏 | 参数 |
|------|------|
| `memory` | `pairs`, `pointsPerMatch`, `bonusPerStar`, `timeBonusMax` |
| `listen` | `total`, `pointsPerCorrect`, `pointsPerComplete` |
| `spell` | `total`, `wordLenMin`, `wordLenMax`, `pointsPerCorrect`, `pointsPerHint` |
| `picture` | `total`, `pointsPerCorrect`, `pointsPerComplete` |

### Theme — 运行时主题切换

```javascript
// 切换到深色主题
Theme.apply('cherry-night');

// 切换回樱桃暖阳风
Theme.apply('cherry-warm-sun');

// 注册新主题（如季节主题）
Theme.register('spring', {
  'accent': '#FF8AB3', 'accent-light': '#FFE8F0',
  'bg': '#FFFBF5', 'surface': '#FFFFFF',
  'radius': '12px', 'radius-lg': '16px'
  // ... 其他令牌
});
Theme.apply('spring');

// 查询当前主题
Theme.current();   // → 'cherry-warm-sun'
Theme.list();      // → ['cherry-warm-sun', 'cherry-night', 'spring']
```

> CSS 变量令牌列表见下方「设计令牌」表。

### WordDataProvider — 词汇数据抽象层

```javascript
// 获取当前阶段全部词汇
WordDataProvider.getWordsByPhase();             // 默认用 currentPhase

// 获取指定阶段词汇
WordDataProvider.getWordsByPhase('movers');

// 按章节获取
WordDataProvider.getWordsByChapter('animals');

// 按词长筛选（拼字游戏用）
WordDataProvider.getWordsByLength('starters', 3, 6);

// 只取有 emoji 映射的词（看图选词用）
WordDataProvider.getWordsWithEmoji('starters');

// 随机抽取 N 个
WordDataProvider.sample(words, 4);
```

> 后期若把词汇数据迁移到 API 或 IndexedDB，只需修改 `WordDataProvider` 内部实现，所有游戏代码无需改动。

### Logger — 自动日志收集

`Logger` 已通过 `EventBus` 自动订阅关键事件（`game:start` / `game:complete` / `game:win` / `points:earned` / `word:mastered`），无需手动调用。

```javascript
// 主动记录自定义事件
Logger.log('mycustom:event', { foo: 'bar' });

// 查看历史日志（最近 200 条）
Logger.history();

// 清空缓冲
Logger.clear();
```

## 可复用工具函数

| 函数 | 用途 | 示例 |
|------|------|------|
| `earnPoints(action, n, x?, y?)` | 发放积分 + 飞出动画 + 事件广播 | `earnPoints('mygame_correct', 3)` |
| `speak(word)` | TTS 发音（英式 en-GB） | `speak('apple')` |
| `showGameResult(data)` | 结果弹层 + 自动广播 `game:complete` | `showGameResult({stars, title, score, points, onRetry, onBack})` |
| `showToast(msg)` | 提示消息 | `showToast('答对了！')` |
| `shuffle(arr)` | 随机打乱数组 | `shuffle(candidates).slice(0, 4)` |
| `getWordEmoji(word)` | 获取 emoji（来自 `word-emoji-data.js`） | `getWordEmoji('cat')` → 🐱 |
| `getChapters(phase)` | 获取章节列表（底层，推荐优先用 `WordDataProvider`） | `getChapters('starters')` |

## 数据扩展：word-emoji 映射

`word-emoji-data.js` 是独立的外部数据模块，定义全局 `WORD_EMOJI` 对象和 `getWordEmoji(word)` 函数。

**扩展方式：** 直接编辑 `word-emoji-data.js`，在 `WORD_EMOJI` 对象中追加映射即可，无需改动 HTML 主文件。

```javascript
// word-emoji-data.js
var WORD_EMOJI = {
  // ... 已有映射
  'robot': '🤖',      // 新增
  'rocket': '🚀'      // 新增
};
```

## 设计令牌（CSS 变量）

所有令牌已注册到 `Theme` 模块，可通过 `Theme.apply()` 运行时切换。

| 令牌 | 默认值（樱桃暖阳风） | 用途 |
|------|---------------------|------|
| `--bg` | #FFF8F0 | 暖奶油背景 |
| `--bg2` | #FFF3E0 | 次级背景 |
| `--ink` | #2D2A26 | 主文字色 |
| `--muted` | #8A8477 | 次文字色 |
| `--rule` | #F0E6D8 | 边框色 |
| `--rule-light` | #F7F0E5 | 轻边框色 |
| `--accent` | #D64545 | 樱桃红主色 |
| `--accent-light` | #FDE8E8 | 主色浅底 |
| `--accent-dark` | #B83838 | 主色深色 |
| `--accent2` | #3DB5E6 | 天蓝点缀 |
| `--accent2-light` | #E5F4FF | 点缀浅底 |
| `--surface` | #FFFFFF | 卡片白底 |
| `--green` | #5CB85C | 成功色 |
| `--shadow-sm` | `0 1px 3px rgba(45,42,38,0.04)` | 轻阴影 |
| `--shadow-md` | `0 2px 8px rgba(45,42,38,0.06)` | 中阴影 |
| `--radius` | 12px | 标准圆角 |
| `--radius-lg` | 16px | 大圆角 |

## 现有游戏列表

| ID | 名称 | 类型 | 题数 | 积分/题 | AppConfig key |
|----|------|------|------|---------|---------------|
| browse | 浏览学习 | content | - | - | - |
| chapters | 章节闯关 | content | - | - | - |
| memory | 翻牌配对 | play | 6 对 | 2 | `memory` |
| listen | 听音选词 | play | 10 | 2 | `listen` |
| spell | 拼字游戏 | play | 8 | 3 | `spell` |
| picture | 看图选词 | play | 10 | 2 | `picture` |

## 完整示例：一个使用全部扩展能力的新游戏

```javascript
// 1. 注册配置
AppConfig.games.sentence = { total: 5, pointsPerCorrect: 4, description: '句子排序' };

// 2. 游戏逻辑
function startSentenceGame() {
  var total = AppConfig.get('sentence', 'total', 5);
  sentenceState = { round: 0, correct: 0, total: total };
  nextSentenceRound();
}

function finishSentenceGame() {
  var stars = sentenceState.correct >= 5 ? 3 : (sentenceState.correct >= 3 ? 2 : 1);
  var starStr = '';
  for (var s = 0; s < 3; s++) starStr += s < stars ? '⭐' : '☆';
  var points = sentenceState.correct * AppConfig.get('sentence', 'pointsPerCorrect', 4);
  showGameResult({
    stars: starStr,
    title: stars === 3 ? '句子大师！' : '继续加油！',
    score: '答对 ' + sentenceState.correct + ' / ' + sentenceState.total + ' 题',
    points: '+' + points + ' 积分',
    onRetry: 'startSentenceGame()',
    onBack: 'switchGameMode(\'browse\')'
  });
  earnPoints('sentence_complete', points);
}

// 3. 注册到 GameRegistry（带钩子）
GameRegistry.register({
  id: 'sentence',
  label: '句子排序',
  icon: '📝',
  type: 'play',
  start: function() { startSentenceGame(); },
  config: AppConfig.games.sentence,
  pointsRule: function(r) {
    return r.correct * AppConfig.get('sentence', 'pointsPerCorrect', 4);
  },
  onWin: function(r) {
    if (r.stars === 3) {
      earnPoints('sentence_perfect', 10);
      Storage.setJSON('sentence_best', { date: Date.now(), correct: r.correct });
    }
  }
});

// 4. 监听全局事件（如：任何游戏胜利都奖励经验）
EventBus.on(EVT.GAME_WIN, function(result) {
  if (result.mode === 'sentence') {
    showToast('🎉 句子排序通关！');
  }
});
```

完成。无需修改 `switchGameMode`、无需改 tabs HTML、无需改积分系统、无需改日志系统。
