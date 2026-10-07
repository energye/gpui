"""T0 基线扫描脚本 1/2：全框架文件数、导出数、越层引用、后端漏线初扫、临时标记计数。

用法：在仓库根运行 `python3 tools/cleanup-audit/gpui_audit.py`。
输出：JSON（含各域 go 文件数、导出函数/类型/变量/方法数、ui 直调 gpu 名单、render 后端命中、TODO/TEMP 计数）。
口径说明：后端初扫为子串口径，精确口径见 gpui_audit2.py（词边界）。
基线快照日期：2026-10-07，见 docs/RENDER_CLEANUP_PLAN.md §7。
"""
import os, re, json, collections
root="/home/yanghy/app/projects/gogpu/gpui"
areas=["render","gpu","ui","engine","video","examples","scripts","tools"]
exp_func=re.compile(r'^func\s+([A-Z]\w*)', re.M)
exp_type=re.compile(r'^type\s+([A-Z]\w*)', re.M)
exp_varconst=re.compile(r'^(?:var|const)\s+([A-Z]\w*)', re.M)
method=re.compile(r'^func\s+\(\s*\w+\s+\*?(\w+)\)\s+([A-Z]\w*)', re.M)
todo=re.compile(r'TODO|FIXME|HACK|XXX|TEMP|Temporary|workaround', re.I)
result={"areas":{},"layer_ui_gpu":[],"render_backend":[],"todos":collections.Counter(),"files":0,"go_files":0}
for area in areas:
    base=os.path.join(root,area)
    if not os.path.isdir(base): continue
    gofiles=[]
    for dp, dn, fns in os.walk(base):
        for f in fns:
            result["files"]+=1
            if f.endswith(".go"):
                gofiles.append(os.path.join(dp, f))
    result["go_files"]+=len(gofiles)
    ef=et=ev=met=0
    for fp in gofiles:
        try:
            s=open(fp,encoding="utf-8",errors="ignore").read()
        except Exception:
            continue
        if fp.endswith("_test.go"):
            continue
        ef+=len(exp_func.findall(s)); et+=len(exp_type.findall(s)); ev+=len(exp_varconst.findall(s)); met+=len(method.findall(s))
        for m in todo.findall(s):
            result["todos"][m.upper()]+=1
        if area=="ui" and ("energye/gpui/gpu/" in s or '"github.com/energye/gpui/gpu"' in s):
            if "depcheck" not in fp and "gate_test" not in fp and "doc.go" not in fp:
                result["layer_ui_gpu"].append(os.path.relpath(fp,root))
        if area=="render" and not fp.endswith("_test.go"):
            hits=[]
            for pat in ["webgpu.","gwgpu/","gles."]:
                if pat in s: hits.append(pat)
            if hits and ("present_swapchain" not in fp and "CreateInstance" not in s and "present_target" not in fp and "backend.go" not in fp):
                result["render_backend"].append({"file":os.path.relpath(fp,root),"hits":hits})
    result["areas"][area]={"go_files":len(gofiles),"exp_func":ef,"exp_type":et,"exp_varconst":ev,"exp_methods":met}
print(json.dumps(result,indent=1)[:6000])
