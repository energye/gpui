"""T0 基线扫描脚本 2/2：render 后端漏线精确扫描（词边界）+ 大文件榜 + 根导出调用三态抽查。

用法：在仓库根运行 `python3 tools/cleanup-audit/gpui_audit2.py`（全仓 grep，约 2 分钟）。
输出：BACKEND_HITS（9 文件起点，重扫到零为止）、TOPFILES（render 根大文件榜）、
  RENDER_ROOT_EXPORTED_FUNCS（根导出 148）、UNUSED/TESTONLY/PROD 三态抽查。
注意：调用三态全量版在 T2 用 go doc 权威清单重出，本脚本为 T0 快照口径。
基线快照日期：2026-10-07，见 docs/RENDER_CLEANUP_PLAN.md §7。
"""
import os, re, json, collections
root="/home/yanghy/app/projects/gogpu/gpui"
# accurate backend check
pat_web=re.compile(r'webgpu\.')
pat_gwg=re.compile(r'gpu/gwgpu')
pat_gles=re.compile(r'\bgles\.')
hits=[]
for dp,dn,fns in os.walk(os.path.join(root,"render")):
    for f in fns:
        if not f.endswith(".go"): continue
        fp=os.path.join(dp,f)
        if fp.endswith("_test.go"): continue
        s=open(fp,encoding="utf-8",errors="ignore").read()
        h=[]
        if pat_web.search(s): h.append("webgpu.")
        if pat_gwg.search(s): h.append("gwgpu/")
        if pat_gles.search(s): h.append("gles.")
        if h and ("present_swapchain" not in fp and "present_target" not in fp and "backend.go" not in fp):
            hits.append({"file":os.path.relpath(fp,root),"hits":h})
print("BACKEND_HITS="+json.dumps(hits,indent=1)[:4000])
# big files in render top package
import pathlib
tops=[]
d=os.path.join(root,"render")
for f in os.listdir(d):
    if not f.endswith(".go") or f.endswith("_test.go"): continue
    fp=os.path.join(d,f)
    s=open(fp,encoding="utf-8",errors="ignore").read()
    nfunc=len(re.findall(r'^func\s+\(?',s,re.M))
    nline=s.count("\n")+1
    tops.append((f,nline,nfunc))
tops.sort(key=lambda x:-x[1])
print("TOPFILES="+json.dumps(tops[:20]))
# exported top-level funcs in render root + usage check (sample full but bounded)
defs=[]
for f in os.listdir(d):
    if not f.endswith(".go") or f.endswith("_test.go"): continue
    fp=os.path.join(d,f)
    s=open(fp,encoding="utf-8",errors="ignore").read()
    for m in re.finditer(r'^func\s+([A-Z]\w*)\s*\(',s,re.M):
        defs.append((m.group(1),f))
print("RENDER_ROOT_EXPORTED_FUNCS="+str(len(defs)))
# check each def usage outside render root non-test (full grep over repo, may take a while; cap 400)
import subprocess
testonly=[]; unused=[]; prod=[]
for name, deffile in defs[:400]:
    # word-boundary search excluding definition
    outs=[]
    for area in ["render","ui","engine","video","examples"]:
        base=os.path.join(root,area)
        for dp2,dn2,fns2 in os.walk(base):
            for f2 in fns2:
                if not f2.endswith(".go"): continue
                fp2=os.path.join(dp2,f2)
                if fp2.endswith(os.path.join("render",deffile)): continue
                try: t=open(fp2,encoding="utf-8",errors="ignore").read()
                except: continue
                if re.search(r'\b'+re.escape(name)+r'\b',t):
                    outs.append(os.path.relpath(fp2,root)+("|TEST" if fp2.endswith("_test.go") else ""))
                    if len(outs)>8: break
            if len(outs)>8: break
        if len(outs)>8: break
    if not outs: unused.append(name)
    elif all("|TEST" in o or o.startswith("examples/") for o in outs): testonly.append((name,outs[:3]))
    else: prod.append(name)
print("UNUSED_SAMPLE="+json.dumps(unused[:40]))
print("TESTONLY_SAMPLE="+json.dumps(testonly[:40])[:4000])
print("PROD_COUNT="+str(len(prod)))
