#!/usr/bin/env python3
"""P3.6 批 2 负向对照。

每路改坏一处，跑该路声明「必须红」的钉，逐条判定；然后还原并复跑，逐条要求全绿。
包级 rc 不作证据（可能因别的测试而红/绿），只看指定测试自己的 PASS/FAIL。

用法：python3 mutate.py <N1|N2|N3|N4> <仓库根目录>
"""
import re
import subprocess
import sys
import pathlib

NAMES = {
    "MipLevels": ("./render/internal/gpu/", "TestImageCache_MipLevelsAndExactBytes"),
    "ModeSplit": ("./render/internal/gpu/", "TestImageCache_MipModeSplitsEntries"),
    "R1Drive": ("./render/", "TestR1_ImageMipmapUsesGPU"),
    "R1Blend": ("./render/", "TestR1_MipLevelBlendSelect"),
}

ROUTES = {
    # 级数恒 1：当作没人要 mip —— 条目、字节、两条端到端钉都应红
    "N1": dict(
        rel="render/internal/gpu/image_cache.go",
        a="\tlevels = 1\n\tif !cmd.Mip.UsesChain() || cmd.Src == nil {",
        b="\tlevels = 1\n\tif true || !cmd.Mip.UsesChain() || cmd.Src == nil {",
        must=["MipLevels", "ModeSplit", "R1Drive", "R1Blend"],
    ),
    # 建了多级却只填 level0：摘逐级上传
    "N2": dict(
        rel="render/internal/gpu/image_cache.go",
        a="\ttotal := int64(need)\n\tfor i := uint32(1); i < levels; i++ {",
        b="\ttotal := int64(need)\n\tfor i := uint32(1); i < 0; i++ {",
        must=["MipLevels", "R1Drive"],
    ),
    # 摘按档绑采样器（回看 nearest 的历史采样器）：只有级间选择钉能判——
    # 这条正是第一轮暴露「mip 接上了但没人守档位选择」的地方
    "N3": dict(
        rel="render/internal/gpu/render_session.go",
        a="{Binding: 2, Sampler: s.imagePipeline.SamplerForFilter(slot.sampler)},",
        b="{Binding: 2, Sampler: s.imagePipeline.SamplerFor(slot.nearest)},",
        must=["R1Blend"],
    ),
    # 显存账退回只算 level0：预算对账钉应红
    "N4": dict(
        rel="gpu/hal/vram_ledger.go",
        a="\ttexels := MipChainTexels(size.Width, size.Height, mips)",
        b="\t_ = mips\n\ttexels := uint64(size.Width) * uint64(size.Height)",
        must=["MipLevels"],
    ),
}


def run(root, pkg, test):
    p = subprocess.run(["go", "test", pkg, "-count=1", "-v", "-timeout", "900s", "-run", f"^{test}$"],
                       cwd=root, capture_output=True, text=True)
    for line in (p.stdout + p.stderr).splitlines():
        m = re.search(r"--- (PASS|FAIL|SKIP): (\S+)", line)
        if m and m.group(2) == test:
            return m.group(1)
    return "未运行"


def main(which, root):
    cfg = ROUTES[which]
    path = root / cfg["rel"]
    original = path.read_text()
    if cfg["a"] not in original:
        print(f"[{which}] 锚点未找到：源码已变，这一路需要重找位置")
        return False
    ok = True
    try:
        path.write_text(original.replace(cfg["a"], cfg["b"], 1))
        print(f"[{which}] 已改坏 {cfg['rel']}")
        for key in cfg["must"]:
            pkg, test = NAMES[key]
            got = run(root, pkg, test)
            print(f"  改坏后 {test}: {got}（期望 FAIL）")
            if got != "FAIL":
                ok = False
    finally:
        path.write_text(original)
        for key in cfg["must"]:
            pkg, test = NAMES[key]
            got = run(root, pkg, test)
            print(f"  还原后 {test}: {got}（期望 PASS）")
            if got != "PASS":
                ok = False
    return ok


if __name__ == "__main__":
    sys.exit(0 if main(sys.argv[1].upper(), pathlib.Path(sys.argv[2])) else 1)
