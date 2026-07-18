/**
 * Word-Emoji 映射数据模块
 * ============================================================
 * 用于「拼字游戏」提示和「看图选词」游戏的图文关联
 *
 * 扩展方式：直接在 WORD_EMOJI 对象中追加 'word': 'emoji' 映射
 * 支持多词短语（以空格分隔，取首词匹配）
 *
 * 分类：动物 / 身体 / 颜色 / 食物 / 家庭 / 数字 / 衣物 /
 *       自然 / 学校 / 玩具 / 房屋 / 动词 / 交通 / 其他
 * ============================================================
 */
var WORD_EMOJI = {
  // Animals 动物
  'cat':'🐱','dog':'🐶','bird':'🐦','fish':'🐟','rabbit':'🐰','mouse':'🐭','cow':'🐮','horse':'🐎',
  'duck':'🦆','pig':'🐷','sheep':'🐑','chicken':'🐔','frog':'🐸','snake':'🐍','elephant':'🐘',
  'monkey':'🐵','tiger':'🐯','bear':'🐻','lion':'🦁','goat':'🐐','spider':'🕷️',

  // Body 身体
  'head':'😀','eye':'👁️','ear':'👂','nose':'👃','mouth':'👄','hand':'✋','foot':'🦶','hair':'💇',
  'arm':'💪','leg':'🦵','tooth':'🦷','face':'😊','finger':'👆','knee':'🦵',

  // Colors 颜色
  'red':'🔴','blue':'🔵','yellow':'🟡','green':'🟢','black':'⚫','white':'⚪','pink':'🩷','orange':'🟠',
  'purple':'🟣','brown':'🟤','grey':'🔘','gray':'🔘',

  // Food 食物
  'apple':'🍎','banana':'🍌','orange':'🍊','bread':'🍞','cake':'🍰','milk':'🥛','water':'💧',
  'egg':'🥚','rice':'🍚','meat':'🥩','ice cream':'🍦','juice':'🧃','tea':'🍵',
  'cheese':'🧀','burger':'🍔','cookie':'🍪','candy':'🍬','grape':'🍇','pear':'🍐',

  // Family 家庭
  'mother':'👩','father':'👨','sister':'👧','brother':'👦','baby':'👶','grandma':'👵','grandpa':'👴',
  'family':'👨‍👩‍👧‍👦','son':'👦','daughter':'👧',

  // Numbers 数字
  'one':'1️⃣','two':'2️⃣','three':'3️⃣','four':'4️⃣','five':'5️⃣','six':'6️⃣','seven':'7️⃣',
  'eight':'8️⃣','nine':'9️⃣','ten':'🔟',

  // Clothes 衣物
  'shirt':'👕','pants':'👖','shoes':'👟','hat':'🎩','dress':'👗','sock':'🧦','coat':'🧥','skirt':'👗',

  // Nature 自然
  'sun':'☀️','moon':'🌙','star':'⭐','tree':'🌳','flower':'🌸','grass':'🌱','rain':'🌧️','snow':'❄️',
  'cloud':'☁️','sky':'🌌','sea':'🌊','river':'🏞️','mountain':'⛰️','beach':'🏖️','wind':'💨','fire':'🔥',

  // School 学校
  'book':'📖','pen':'🖊️','pencil':'✏️','bag':'🎒','desk':'🪑','chair':'🪑','teacher':'👩‍🏫','school':'🏫',

  // Toys 玩具
  'ball':'⚽','doll':'🪆','kite':'🪁','toy':'🧸','bike':'🚲','car':'🚗','plane':'✈️','train':'🚂',

  // House 房屋
  'house':'🏠','door':'🚪','window':'🪟','bed':'🛏️','table':'🍽️','clock':'🕐','phone':'📱','key':'🔑',

  // Verbs 动词
  'run':'🏃','jump':'🤸','swim':'🏊','eat':'🍽️','drink':'🥤','read':'📖','write':'✍️','sing':'🎤',
  'dance':'💃','play':'🎮','sleep':'😴','sit':'🪑','stand':'🧍','walk':'🚶','fly':'🦅','drive':'🚗',

  // Transport 交通
  'bus':'🚌','boat':'⛵','ship':'🚢','truck':'🚚',

  // Misc 其他
  'tv':'📺','computer':'💻','umbrella':'☂️','glasses':'👓'
};

/**
 * 获取单词对应的 emoji
 * @param {string} word - 英文单词
 * @returns {string} emoji 字符，无匹配返回 📝
 */
function getWordEmoji(word) {
  if (!word) return '📝';
  var w = word.toLowerCase().trim();
  if (WORD_EMOJI[w]) return WORD_EMOJI[w];
  // 尝试匹配多词短语的首词
  var parts = w.split(' ');
  if (parts.length > 1 && WORD_EMOJI[parts[0]]) return WORD_EMOJI[parts[0]];
  return '📝';
}
