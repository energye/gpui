package hint

// testset.go —— FT-light 移植的验证字集（回归测试样本）。
//
// 这不是"逐字人工校对的清单"，而是按**结构类型**抽样的自动化验证集：
// 规则（蓝线/捕捉/网格）对所有字统一生效，字集只用来量化进度
// （一致率/差异像素），全量跑一轮约 1 分钟。
//
// 覆盖原则（总计约 2000 字符，含扩展 3000 常用字见下）：
//   - 字母系语言 = 全字符集（拉丁/泰文/阿拉伯/西里尔/希腊本身小）
//   - CJK = 210 个结构代表字（覆盖全部常见笔画结构）+ 扩展 3000 常用字表
//     （脚本生成，见 docs/ENGINE_TEXT_HINT_LIGHT_PLAN.md §4.3）
//
// 每个分组有结构说明，供 fdiff 按组报告收敛进度。

// VerifierGroup 是字集的一个分组：同一结构类型的样本集合。
type VerifierGroup struct {
	// Name 分组名，如 "cjk-multi-horizontal"。
	Name string
	// Desc 结构特征说明（为什么代表这一类）。
	Desc string
	// Chars 该组的字符。
	Chars string
}

// VerifierSet 返回内置验证字集（按结构分组）。
func VerifierSet() []VerifierGroup {
	return structGroups
}

// structGroups 内置结构代表字集。
var structGroups = []VerifierGroup{
	// ── CJK（210 字，覆盖全部常见笔画结构）────────────────────────────
	{
		Name:  "cjk-horizontal-multi",
		Desc:  "多横主字（平行横笔，blue 对齐最敏感，最容易错位）",
		Chars: "三王五互世且直真其量理現等早直言章音竟童意義考者老重",
	},
	{
		Name:  "cjk-vertical-multi",
		Desc:  "多竖主字（竖笔密集，Y 不影响但竖距检验 X 不动守则）",
		Chars: "川州册山出业旧临师简单币巾中申由甲田电申鬼自白百身其鼻乐",
	},
	{
		Name:  "cjk-slash",
		Desc:  "撇捺主字（斜笔，检验网格拟合对斜向不误伤）",
		Chars: "人八入火木米来文父又义友发大天太夫失女处各备复金今令余食",
	},
	{
		Name:  "cjk-dot",
		Desc:  "点笔主字（小点着落，检验亚像素点定位）",
		Chars: "小少心必思志主永水江汉河海重点现总照相熊烈热然",
	},
	{
		Name:  "cjk-frame",
		Desc:  "方框/包围（外框横竖相接，检验角部对齐）",
		Chars: "口日目白回四图团园国因围圆固囚日直盾自囚囱回",
	},
	{
		Name:  "cjk-surround",
		Desc:  "半包围（走之/广/门，检验底部横笔对齐 + 上包下）",
		Chars: "这过还进迭边用月周同冈间问闲同用网风闪闻只因题起赵超",
	},
	{
		Name:  "cjk-leftright",
		Desc:  "左右结构（最常用体，检验左右组件的横笔同高对齐）",
		Chars: "的作使但化他你们说请问河清秋投机机械讲词该议论请信海",
	},
	{
		Name:  "cjk-topbottom",
		Desc:  "上下结构（顶横/底横 blue 双对齐）",
		Chars: "字安家室省会学党草茶音香早星香齿品坐安尽忠杰常堂荒笔牢",
	},
	{
		Name:  "cjk-repeat",
		Desc:  "重复部件（品/森，检验同字内同类横笔一致捕捉）",
		Chars: "品森众晶磊昌吕炎惷矗鑫淼羴毳譶鱻猋犇羴掱皛畾羶",
	},
	{
		Name:  "cjk-dense",
		Desc:  "高密笔画（小字号极限，笔画粘连风险最大）",
		Chars: "徽德囊罐灌赢警籍灌酱饕餮馨蠡繭羸藜曦羹髋鬯屭龘蘸黩龌",
	},
	{
		Name:  "cjk-punctuation",
		Desc:  "中文标点（头尾对齐，悬挂缩进相关）",
		Chars: "，。、；：？！「」『』（）《》〈〉——……·",
	},

	// ── Latin（ASCII 全套 + Latin-1 重音）───────────────────────────────
	{
		Name:  "latin-ascii",
		Desc:  "ASCII 可打印全集（95 字符，bytecode light 全量覆盖）",
		Chars: " !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~",
	},
	{
		Name:  "latin-accent",
		Desc:  "拉丁变音符（组合字形，Y 对齐影响 accent）",
		Chars: "éèêëàáâäïîíìöôòóüùúûñçåæœÿ€£¥",
	},
	{
		Name:  "latin-confusable",
		Desc:  "易混淆对（0/O 1/l/I，检验 discriminate）",
		Chars: "0O1lI2z5S8B6G9g3E",
	},

	// ── 泰文（56 主辅音 + 元音/音调）────────────────────────────────
	{
		Name:  "thai-consonants",
		Desc:  "泰文辅音（高位短字元 cue 在 baseline 附近，afcjk 变体）",
		Chars: "กขฃคฅฆงจฉชซฌญฎฏฐฑฒณดตถทธนบปผฝพฟภมยรลวศษสหฬอฮ",
	},
	{
		Name:  "thai-voweltone",
		Desc:  "泰文元音/音调（上下标高频）",
		Chars: "ะัาำิีึืุูเแโใไยฤฦ่้๊๋็์",
	},

	// ── 阿拉伯（28 字母 + 常见连字）────────────────────────────────
	{
		Name:  "arabic-letters",
		Desc:  "阿拉伯字母（右连，Y 高度差异大）",
		Chars: "ابتثجحخدذرزسشصضطظعغفقكلمنهوي",
	},
	{
		Name:  "arabic-ligature",
		Desc:  "阿拉伯常见连字/变体（Lam-Alef 等）",
		Chars: "لا أ إ آ ة ى ؤ ئ همزة",
	},

	// ── 西里尔 / 希腊（全字母表）───────────────────────────────────
	{
		Name:  "cyrillic",
		Desc:  "西里尔全字母（俄文，含高位元）",
		Chars: "АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯабвгдежзийклмнопрстуфхцчшщъыьэюяёЁ",
	},
	{
		Name:  "greek",
		Desc:  "希腊全字母（数学符号来源）",
		Chars: "ΑΒΓΔΕΖΗΘΙΚΛΜΝΞΟΠΡΣΤΥΦΧΨΩαβγδεζηθικλμνξοπρστυφχψω",
	},
}

// CJK 扩展 3000 常用字：内置 210 代表字之外的规模扩展。
//
// 3000 常用字表不内嵌（体积大），由脚本生成后以外部文件加载，
// 加载方式与字表来源在 fdiff 文档标注：
//
//	- 来源通用：《通用规范汉字表》(国标，8105 字) 前 3000 常用子集；
//	  或系统字体 Noto Sans CJK 按使用频度取前 3000。
//	- 运行时：fdiff --testset /path/to/cjk3000.txt
//
// 补充说明：3000 常用字由 214 部首的全部常见部件组合而成，
// 210 结构代表字已覆盖全部部件结构；扩展表的作用是回归覆盖
// （确保某条规则的收敛不破坏未采样的字），不是新的结构类型。
