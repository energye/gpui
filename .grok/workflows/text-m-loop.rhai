let meta = #{
    name: "text-m-loop",
    description: "M期自动链：M3起依次过M3.5M4，M4绿后M5待确认（2026-09-04：计时单跑+M4先建窗+M5人工闸）",
    phases: [
        #{ title: "Implement", detail: "按条目严谨实现本期能力" },
        #{ title: "Verify", detail: "五维逐项验证收原始证据" },
        #{ title: "Challenge", detail: "五路独立挑刺证伪" },
        #{ title: "FixLoop", detail: "红了就修再测，最多五轮" },
        #{ title: "Coverage", detail: "缺主测试就补再重测" },
        #{ title: "Converge", detail: "收敛优化性能可读性" },
        #{ title: "Retest", detail: "优化后全量重测" },
        #{ title: "Commit", detail: "只提交不推送" },
        #{ title: "Gate", detail: "链条 handoff" },
    ],
};
let verify_schema = #{
    "type": "object", "required": ["passed", "evidence", "failures"],
    "properties": #{
        "passed": #{ "type": "boolean" },
        "evidence": #{ "type": "string" },
        "failures": #{ "type": "string" },
    },
};
let challenge_schema = #{
    "type": "object", "required": ["passed", "reason", "evidence"],
    "properties": #{
        "passed": #{ "type": "boolean" },
        "reason": #{ "type": "string" },
        "evidence": #{ "type": "string" },
    },
};
let fix_schema = #{
    "type": "object", "required": ["fixed", "files", "retest"],
    "properties": #{
        "fixed": #{ "type": "boolean" },
        "files": #{ "type": "string" },
        "retest": #{ "type": "string" },
    },
};
let cover_schema = #{
    "type": "object", "required": ["added", "tests", "notes"],
    "properties": #{
        "added": #{ "type": "boolean" },
        "tests": #{ "type": "string" },
        "notes": #{ "type": "string" },
    },
};
let converge_schema = #{
    "type": "object", "required": ["changed", "files", "tests"],
    "properties": #{
        "changed": #{ "type": "boolean" },
        "files": #{ "type": "string" },
        "tests": #{ "type": "string" },
    },
};
let commit_schema = #{
    "type": "object", "required": ["committed", "hash", "files"],
    "properties": #{
        "committed": #{ "type": "boolean" },
        "hash": #{ "type": "string" },
        "files": #{ "type": "string" },
    },
};
let trigger_schema = #{
    "type": "object", "required": ["triggered", "ratio", "reason"],
    "properties": #{
        "triggered": #{ "type": "boolean" },
        "ratio": #{ "type": "string" },
        "reason": #{ "type": "string" },
    },
};
fn run_one_m(target, verify_schema, challenge_schema, fix_schema, cover_schema, converge_schema, commit_schema, trigger_schema) {
    let truewin = "本机是真实GPU真窗机，所有需要真窗或GPU运行时指标的验收都直接在本机跑真窗拿实测，不许以纯CPU测不了为由写没验证，不许估算冒充实测。";
    phase("Implement");
    if target == "M3.5" {
        let pt = "";
        pt += "你负责判定 M3.5 是否触发。先用 read_file 读 docs/ENGINE_TEXT_SCALE_PLAN.md 中 M3 与 M3.5 两节，找到 M3 产出的文本拷贝占比实测值。";
        pt += "触发条件满足任一才做：文本拷贝占击键耗时超过三成，或有明确需求支持百万字级编辑。缺数据就用 run_terminal_command 跑 BenchmarkKeystroke 拿实测值，贴原始输出。";
        pt += "触发就返回真并说明理由，不触发就返回假并贴出实测值。不许估算冒充实测，测不出来就如实说测不出。";
        pt += truewin;
        let tg = agent(pt, #{ label: "trigger", capability_mode: "all", output_schema: trigger_schema });
        if tg == () || tg.success == false {
            return #{ green: false, summary: "触发判定没产出，停下人工看" };
        }
        if tg.output.triggered == false {
            return #{ green: true, summary: "未触发跳过进下一期" };
        }
    }
    let p1 = "";
    p1 += "你负责严谨实现文本分期中的一期，期号是 ";
    p1 += target;
    p1 += "。先用 read_file 读 AGENTS.md 全文，再读 docs/ENGINE_TEXT_SESSION_BRIEF.md，再读 docs/ENGINE_TEXT_SCALE_PLAN.md 中本期的那一节，只读本期，外加跨期不变量表与验收清单。";
    p1 += "本次交付必须同时满足五条硬标准，缺一条都不算完：高可用，指不变量一条不违反，失败有降级退路且画面正确优先，回滚点真实可退；";
    p1 += "性能，指本期基准多档数据量实测并断言比值达标，不许单档数据冒充，不许口头宣称；";
    p1 += "可读性，指命名表意结构清晰，注释只写非显而易见的约束，不留占位与绕路代码；";
    p1 += "收敛，指改动只收在本期范围内，不夹带无关改动，不为过测试而降画质；";
    p1 += "覆盖，指验收项每一项都有测试对应，边界空值长串换行滚动等主路径一个不漏。";
    p1 += "若本期代码已完成，不要重写，逐项核对清单与五条标准，缺什么补什么，达标的不动。动手前先写失败单测确认它是红的再改代码。探针临时代码只放 tmp 目录并跑完清理。";
    p1 += "禁止改 13 个老裁判测试，禁止在 examples 目录实现引擎功能，禁止在 ui 目录新增业务文案写死，改了 render 公开面必须同步目录文档并跑 apidoc。";
    p1 += "计时类测试与基准一次只跑一个，绝不同其它测试命令并行（M3.8 教训：并行压测曾让 TestCaretBuild_NoQuadratic 误报红，烧掉修复轮次）。";
    if target == "M4" {
        p1 += "M4 的第一件事不是改代码，是新建真窗 ui_text_m4_virtual 并跑出改前全量基线（100 万行全量排版首屏时间与内存），基线一旦开始改代码就再也取不到；窗不存在就按§6.3.1先建窗。";
    }
    p1 += "改完用 grep 自查硬编码与禁区，用 read_file 和 grep 核实每一个引用行号再落笔。每改完一个文件跑一次相关测试。交回改了哪些文件和行号。";
    p1 += truewin;
    let impl = agent(p1, #{ label: "implement", capability_mode: "all" });
    phase("Verify");
    let p2 = "";
    p2 += "你负责按五条硬标准验证一期，期号是 ";
    p2 += target;
    p2 += "。用 read_file 和 grep 先确认本期验收清单：M3 看单测 TestUndoDelta 和基准 BenchmarkPushHistory 加 ui textinput 回归；";
    p2 += "M4 看单测 TestVirtualLines 和 TestLazyMeasure 加基准 BenchmarkOpenDocument；M5 看场景矩阵每项一个单测加基准 BenchmarkKeystroke 比值；";
    p2 += "M3.5 看单测 TestPieceTree 与 TestBufferLineAt 加基准 BenchmarkInsertAtOffset；M3 还要产出文本拷贝占比实测值，供 M3.5 判定触发用。";
    p2 += "逐项过：高可用看回归全绿加不变量未违反加回滚点真实可退；性能看基准多档比值达标；可读性看命名结构注释与目录文档同步；";
    p2 += "收敛看改动都在本期范围内且无降画质；覆盖看验收项与测试一一对应，缺一项就是没过。测试结构不对也判没过。";
    p2 += "跑测试必须按测试文件逐个用 run_terminal_command 跑，不许一次全量并发，每次贴原始输出。只认原始输出判分，有 FAIL 或 panic 就是没过。";
    p2 += "老裁判测试有一个红就是没过，证据缺了就是没过。返回是否通过加原始证据加失败清单。";
    p2 += "计时类测试与基准一次只跑一个，绝不同其它测试命令并行（M3.8 教训：并行压测曾让 TestCaretBuild_NoQuadratic 误报红，烧掉修复轮次）。";
    p2 += truewin;
    let v1 = agent(p2, #{ label: "verify", capability_mode: "all", output_schema: verify_schema });
    let green = false;
    if v1 != () && v1.success && v1.output.passed == true { green = true; }
    log("first verify done");
    phase("Challenge");
    if green == true {
        let s1 = "";
        s1 += "你是找茬员一号，只查跑法与裁判，期号是 ";
        s1 += target;
        s1 += "。用 run_terminal_command 跑 git status 和 git diff 看老裁判测试有没有被改动，用 grep 查证据里有没有全量并发跑法。";
        s1 += "发现老裁判被改或证据缺原始输出或用了禁止的全量跑法，就判没过并贴出证据。找不到漏洞才许判过。空口说过不算。";
        let s2 = "";
        s2 += "你是找茬员二号，只查分层与硬编码，期号是 ";
        s2 += target;
        s2 += "。用 grep 查 examples 目录有没有实现引擎功能，用 grep 查 ui 目录有没有新增业务文案写死，核对布局与绘制是否同源。";
        s2 += "改了 render 公开面必须有目录文档同步和 apidoc 记录，否则判没过。找不到漏洞才许判过。";
        let s3 = "";
        s3 += "你是找茬员三号，只查证据诚实，期号是 ";
        s3 += target;
        s3 += "。用 read_file 和 grep 核对每一句结论有没有原始输出撑腰，凡是只查了部分却说全部的都算漏洞。";
        s3 += "未验证的必须明说未验证，估算冒充实测一律判没过。找不到漏洞才许判过。";
        let s4 = "";
        s4 += "你是找茬员四号，只查高可用与性能，期号是 ";
        s4 += target;
        s4 += "。用 read_file 核对不变量表与降级链是否守住，用 grep 查基准是否多档数据量实测并断言比值。";
        s4 += "发现单档冒充多档或绝对值代替比值或失败无降级退路，就判没过并贴出证据。找不到漏洞才许判过。";
        let s5 = "";
        s5 += "你是找茬员五号，只查可读性与收敛，期号是 ";
        s5 += target;
        s5 += "。用 git diff 看改动是否收在本期范围内，有没有夹带无关改动，有没有为过测试而降画质，命名注释是否清晰。";
        s5 += "发现无关改动或降画质装绿或占位绕路代码，就判没过并贴出证据。找不到漏洞才许判过。";
        let jobs = [];
        jobs.push(#{ prompt: s1, label: "challenge:run", capability_mode: "all", output_schema: challenge_schema });
        jobs.push(#{ prompt: s2, label: "challenge:layer", capability_mode: "all", output_schema: challenge_schema });
        jobs.push(#{ prompt: s3, label: "challenge:proof", capability_mode: "all", output_schema: challenge_schema });
        jobs.push(#{ prompt: s4, label: "challenge:availperf", capability_mode: "all", output_schema: challenge_schema });
        jobs.push(#{ prompt: s5, label: "challenge:readconv", capability_mode: "all", output_schema: challenge_schema });
        let results = parallel(jobs);
        let ok_count = 0;
        for item in results {
            if item != () && item.success && item.output.passed == true { ok_count += 1; }
        }
        if ok_count != 5 { green = false; }
        log("challenge done");
    }
    phase("FixLoop");
    let rounds = [1, 2, 3, 4, 5];
    for r in rounds {
        if green == false {
            let pf = "";
            pf += "你负责修一期的失败或被挑刺的问题，期号是 ";
            pf += target;
            pf += "。先读失败清单和挑刺意见，只修引擎层该修的位置，不在 examples 绕路，不改老裁判测试迁就实现，不降画质。";
            pf += "修的时候同步守住五条硬标准：高可用不变量不破，性能不回退，可读性不下降，改动不扩散，缺的测试顺手补上。";
            pf += "每改完一个文件就跑一次相关测试。修完贴改了哪些文件和重测输出。";
            pf += "计时类测试与基准一次只跑一个，绝不同其它测试命令并行（M3.8 教训：并行压测曾让 TestCaretBuild_NoQuadratic 误报红，烧掉修复轮次）。";
            pf += truewin;
            let f = agent(pf, #{ label: "fix", capability_mode: "all", output_schema: fix_schema });
            let pv = "";
            pv += "你负责重测一期，期号是 ";
            pv += target;
            pv += "。按测试文件逐个重跑本期单测加基准加 13 个老裁判测试，贴原始输出，有 FAIL 就是没过。返回是否通过。";
            pv += "计时类测试与基准一次只跑一个，绝不同其它测试命令并行（M3.8 教训：并行压测曾让 TestCaretBuild_NoQuadratic 误报红，烧掉修复轮次）。";
            pv += truewin;
            let v = agent(pv, #{ label: "retest", capability_mode: "all", output_schema: verify_schema });
            if v != () && v.success && v.output.passed == true { green = true; } else { green = false; }
            log("fix round done");
        }
    }
    phase("Coverage");
    if green == true {
        let pc = "";
        pc += "你负责把一期的测试覆盖补到全面，期号是 ";
        pc += target;
        pc += "。用 grep 和 read_file 把验收项逐项列出，每一项必须有测试对应，边界空值长串换行滚动多档数据等主路径全查一遍。";
        pc += "缺一项就加新的主要测试含基准，加之前先确认它是红的再做绿。测试数据进 testdata 目录，不许在测试里硬编码生成字表。";
        pc += "只许加新测试，不许改老裁判测试。加完贴新增了哪些测试和跑出的原始输出。没有缺口就返回 added 为假并说明逐项看过哪些路径。";
        pc += truewin;
        let c = agent(pc, #{ label: "coverage", capability_mode: "all", output_schema: cover_schema });
        let need_recheck = false;
        if c != () && c.success && c.output.added == true { need_recheck = true; }
        if need_recheck == true {
            let pv2 = "";
            pv2 += "你负责在补测试后重测一期，期号是 ";
            pv2 += target;
            pv2 += "。按测试文件逐个跑新增测试加本期单测加基准加老裁判测试，贴原始输出，有 FAIL 就是没过。";
            pv2 += "计时类测试与基准一次只跑一个，绝不同其它测试命令并行（M3.8 教训：并行压测曾让 TestCaretBuild_NoQuadratic 误报红，烧掉修复轮次）。";
            pv2 += truewin;
            let v2 = agent(pv2, #{ label: "recheck", capability_mode: "all", output_schema: verify_schema });
            if v2 != () && v2.success && v2.output.passed == true { green = true; } else { green = false; }
        }
    }
    phase("Converge");
    if green == true {
        let pg = "";
        pg += "你负责对本期代码做收敛优化，期号是 ";
        pg += target;
        pg += "。方向三条：可读性，指命名表意结构清晰注释只写约束；性能，指热点消除重复开销且基准比值正向收敛；结构，指对齐成熟框架做法与复用既有能力。";
        pg += "红线：不许改行为，不许为过测试而降画质或绕路，不许扩散到本期之外。优化前后各跑一次相关测试与基准并贴输出对比，性能回退或行为不一致立刻停。";
        pg += "改了 render 公开面必须同步目录文档。交回改了哪些文件和前后对比输出。";
        pg += truewin;
        let g = agent(pg, #{ label: "converge", capability_mode: "all", output_schema: converge_schema });
        if g != () && g.success { log("converge done"); }
    }
    phase("Retest");
    if green == true {
        let pr = "";
        pr += "你负责在收敛优化后全量重测一期，期号是 ";
        pr += target;
        pr += "。按测试文件逐个重跑本期全部单测加基准加 13 个老裁判测试，贴原始输出，并与优化前数值对比确认性能正向收敛无回退。";
        pr += "计时类测试与基准一次只跑一个，绝不同其它测试命令并行（M3.8 教训：并行压测曾让 TestCaretBuild_NoQuadratic 误报红，烧掉修复轮次）。";
        pr += "任一 FAIL 或行为不一致或性能回退就是没过。";
        pr += truewin;
        let vr = agent(pr, #{ label: "final-retest", capability_mode: "all", output_schema: verify_schema });
        if vr != () && vr.success && vr.output.passed == true { green = true; } else { green = false; }
    }
    phase("Commit");
    if green == false {
        return #{ green: false, summary: "本期没绿，不提交不进下一期" };
    }
    let pk = "";
    pk += "你负责把本期改动做本地提交，期号是 ";
    pk += target;
    pk += "。用户已长期许可本工作流自动提交，无需再问，但范围锁死：先跑 git status 和 git diff 看清单，只加本次改动的文件，禁止加无关改动，禁止用 add 点或全量参数，禁止推送。";
    pk += "提交后用 git log 看一次提交记录确认落盘。交回提交哈希和完整文件清单与提交记录，留给用户事后检查。";
    let k = agent(pk, #{ label: "commit", capability_mode: "all", output_schema: commit_schema });
    if k != () && k.success && k.output.committed == true {
        return #{ green: true, summary: "本期已绿且已本地提交" };
    }
    return #{ green: false, summary: "提交没落盘，停下人工处理" };
}
fn extract_target(a) {
    if a == () { return (); }
    if type_of(a) == "string" {
        a.trim();
        if a == "M3" || a == "M3.5" || a == "M4" || a == "M5" { return a; }
        if a.contains("M3.5") { return "M3.5"; }
        if a.contains("M3") { return "M3"; }
        if a.contains("M4") { return "M4"; }
        if a.contains("M5") { return "M5"; }
        return a;
    }
    if type_of(a) == "array" && a.len() > 0 && type_of(a[0]) == "string" { let s = a[0]; s.trim(); return s; }
    if type_of(a) == "map" {
        if a.target != () && type_of(a.target) == "string" { let s = a.target; s.trim(); return s; }
        if a.objective != () && type_of(a.objective) == "string" { let s = a.objective; s.trim(); return s; }
        if a.phase != () && type_of(a.phase) == "string" { let s = a.phase; s.trim(); return s; }
        if a.arg != () && type_of(a.arg) == "string" { let s = a.arg; s.trim(); return s; }
        if a.query != () && type_of(a.query) == "string" { let s = a.query; s.trim(); return s; }
        if a.input != () && type_of(a.input) == "string" { let s = a.input; s.trim(); return s; }
    }
    return ();
}
let target = extract_target(args);
if target == () { pause("verification", "Pass M3 or M3.5 or M4 or M5 as args."); }
if target != "M3" && target != "M3.5" && target != "M4" && target != "M5" { pause("verification", "Unknown start phase."); }
let chain = ["M3", "M3.5", "M4", "M5"];
let start = 0;
if target == "M3.5" { start = 1; }
if target == "M4" { start = 2; }
if target == "M5" { start = 3; }
phase("Gate");
let notes = "";
for i in start..4 {
    let m = chain[i];
    log(m);
    let r = run_one_m(m, verify_schema, challenge_schema, fix_schema, cover_schema, converge_schema, commit_schema, trigger_schema);
    if r == () {
        complete(#{ green: false, next: "stop", summary: "链条中断，停下人工看" });
    }
    notes += m;
    notes += ": ";
    notes += r.summary;
    notes += ". ";
    if r.green == false {
        complete(#{ green: false, next: "stop", summary: notes });
    }
    if m == "M4" && r.green == true {
        complete(#{ green: true, next: "m5-pending", summary: notes + "M4已绿且已本地提交，M5待用户确认后另起，不自动进。" });
    }
}
complete(#{ green: true, next: "done", summary: notes });
