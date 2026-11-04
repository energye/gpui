#!/usr/bin/env python3
# 3e-2 反向鉴别：逐条把实现打断，钉必须变红；打完立刻还原。
# 锚点一律限定在 TagImageQuad 的 case 窗口内 / 用 nth 唯一化，避免打到同名语句的其他分支。
import subprocess, shutil, sys, os

ROOT = "/home/yanghy/app/projects/gogpu/gpui"
SCENE = os.path.join(ROOT, "render/scene")
FILES = ["encoding.go", "gpu_renderer.go", "scene.go", "renderer.go"]
BAK = "/tmp/p37_3e2_neg_bak"

PIN_TESTS = [
    "TestP373ESceneQuadEncodingAndMerge",
    "TestP373ESceneQuadReplayMatchesContext",
    "TestP373ESceneQuadBakesSceneTransform",
    "TestP373ESceneQuadAbsentFromLegacyStreams",
    "TestCPUTileQuadParallelogramEqualsAffineDraw",
    "TestCPUTileQuadTrapezoidRefusesAndCounts",
    "TestCPUTileQuadPerspectiveFlagRefusesAndCounts",
    "TestCPUTileAffineLiftProjectionAppliesToImagesAndQuads",
    "TestCPUTileProjectiveProjectionRefusesQuad",
    "TestCPUTileProjectionAppliesOutsideSceneMatrix",
]


def snapshot():
    shutil.rmtree(BAK, ignore_errors=True)
    os.makedirs(BAK)
    for f in FILES:
        shutil.copy(os.path.join(SCENE, f), os.path.join(BAK, f))


def restore():
    for f in FILES:
        shutil.copy(os.path.join(BAK, f), os.path.join(SCENE, f))


def run(test):
    return subprocess.run(["go", "test", "./render/scene", "-run", test, "-count=1"],
                          cwd=ROOT, capture_output=True, text=True)


def lines(fname):
    with open(os.path.join(SCENE, fname)) as fh:
        return fh.readlines()


def quad_windows(fname):
    """TagImageQuad 的 case 窗口：从 case 行到下一个 case/default 行。"""
    ls = lines(fname)
    wins = []
    for i, l in enumerate(ls):
        if "case TagImageQuad:" in l:
            j = i + 1
            while j < len(ls) and "case Tag" not in ls[j] and "default:" not in ls[j]:
                j += 1
            wins.append((i + 1, j))
    return wins


def find(fname, pat, wins=None, nth=0):
    hits = [i for i, l in enumerate(lines(fname), 1) if pat in l]
    if wins is not None:
        hits = [h for h in hits if any(a <= h < b for a, b in wins)]
    if nth >= len(hits):
        raise SystemExit(f"锚点不足：{fname} {pat!r} 命中 {hits}")
    return hits[nth]


ORDER = "TestCPUTileProjectionAppliesOutsideSceneMatrix"


def cases():
    ew = quad_windows("encoding.go")
    rw = quad_windows("renderer.go")
    gw = quad_windows("gpu_renderer.go")
    assert len(ew) == 2, f"encoding.go 应有两处 TagImageQuad 合并分支，实得 {len(ew)}"
    assert len(rw) == 1 and len(gw) == 1, (rw, gw)
    # (编号, 说明, 文件, 定位(行,旧,新), 应红测试, 应仍绿测试)
    return [
        ("N1", "编码不写权重（恒写 1，仍编译）", "encoding.go",
         lambda: (find("encoding.go", "float32(w))"), "float32(w)", "float32(1+w*0)"),
         "TestP373ESceneQuadEncodingAndMerge", None),
        ("N2", "位移合并把权重也当坐标平移", "encoding.go",
         lambda: (find("encoding.go", "i < 8", wins=[ew[1]]), "i < 8", "i < 12"),
         "TestP373ESceneQuadEncodingAndMerge", None),
        ("N3", "位移合并不抬图像索引", "encoding.go",
         lambda: (find("encoding.go", "e.drawData[drawDataStart+drawIdx] += imageOffset", wins=[ew[1]]),
                  "+= imageOffset", "+= 0"),
         "TestP373ESceneQuadEncodingAndMerge", None),
        ("N4", "无位移合并不抬图像索引", "encoding.go",
         lambda: (find("encoding.go", "e.drawData[drawDataStart+drawIdx] += imageOffset", wins=[ew[0]]),
                  "+= imageOffset", "+= 0"),
         "TestP373ESceneQuadEncodingAndMerge", None),
        ("N5", "GPU 回放丢掉透视位", "gpu_renderer.go",
         lambda: (find("gpu_renderer.go", "Perspective:   perspective,", wins=[gw[0]]), "perspective,", "perspective && false,"),
         "TestP373ESceneQuadReplayMatchesContext", None),
        ("N6", "GPU 回放取不到图（arm 空壳）", "gpu_renderer.go",
         lambda: (find("gpu_renderer.go", "buf := sceneImageBuf(scene, imageIndex)", wins=[gw[0]]),
                  "sceneImageBuf(scene, imageIndex)", "sceneImageBuf(scene, imageIndex+999)"),
         "TestP373ESceneQuadReplayMatchesContext", None),
        ("N7", "录制期不烘焙场景变换", "scene.go",
         lambda: (find("scene.go", "if !s.currentTransform.IsIdentity() {"),
                  "if !s.currentTransform.IsIdentity() {", "if false {"),
         "TestP373ESceneQuadBakesSceneTransform", None),
        ("N8", "平行四边形判据放松（梯形也放行）", "renderer.go",
         lambda: (find("renderer.go", "math.Abs(tr.X+bl.X-tl.X-br.X) > eps"), "> eps", "> 1e9"),
         "TestCPUTileQuadTrapezoidRefusesAndCounts", None),
        ("N9", "quad 拒画不计数", "renderer.go",
         lambda: (find("renderer.go", "r.projDegrades.Add(1)", wins=[rw[0]]), "Add(1)", "Add(0)"),
         "TestCPUTileQuadTrapezoidRefusesAndCounts", None),
        ("N10", "透视位被忽略（只判 projective）", "renderer.go",
         lambda: (find("renderer.go", "if !ok || perspective || projective {"),
                  "!ok || perspective || projective", "!ok || (perspective && false) || projective"),
         "TestCPUTileQuadPerspectiveFlagRefusesAndCounts", None),
        ("N11", "图像那条不折投影（退回 3e-1 的漏）", "renderer.go",
         lambda: (find("renderer.go", "blitImageToTile(img, projAffine.Multiply(imgTransform)"),
                  "projAffine.Multiply(imgTransform)", "imgTransform"),
         "TestCPUTileAffineLiftProjectionAppliesToImagesAndQuads", None),
        ("N12", "quad 角点不叠投影层", "renderer.go",
         lambda: (find("renderer.go", "if !projective && projAffine != IdentityAffine() {"),
                  "!projective && projAffine != IdentityAffine()", "false"),
         "TestCPUTileAffineLiftProjectionAppliesToImagesAndQuads", None),
        ("N13", "TagTransform 处把投影折回矩阵内侧", "renderer.go",
         lambda: (find("renderer.go", "currentTransform = projAffine.Multiply(rawTransform)", nth=1),
                  "projAffine.Multiply(rawTransform)", "rawTransform.Multiply(projAffine)"),
         ORDER + "/tag_transform_site", ORDER + "/tag_projection_site"),
        ("N14", "TagProjection 处把投影折回矩阵内侧", "renderer.go",
         lambda: (find("renderer.go", "currentTransform = projAffine.Multiply(rawTransform)", nth=0),
                  "projAffine.Multiply(rawTransform)", "rawTransform.Multiply(projAffine)"),
         ORDER + "/tag_projection_site", ORDER + "/tag_transform_site"),
    ]


def main():
    print("# P3.7 批 3e-2 反向鉴别（每条打断一处实现，跑对应钉，随后立刻还原）")
    print("# 跑法：python3 /tmp/p37_3e2_negatives.py")
    print()
    print("## 基线（未打断）")
    p = run("|".join(PIN_TESTS))
    print(f"      go test -run '<上列 10 条钉>' => {'GREEN' if p.returncode == 0 else 'RED(基线就红!)'}")
    if p.returncode != 0:
        print(p.stdout[-1500:])
        return 1
    print()
    print("## 打断")
    ok = True
    for cid, desc, fname, locator, red_test, green_test in cases():
        snapshot()
        ln, old, new = locator()
        ls = lines(fname)
        if old not in ls[ln - 1]:
            print(f"[{cid}] 锚点不符：{fname}:{ln} 里没有 {old!r}")
            ok = False
            restore()
            continue
        ls[ln - 1] = ls[ln - 1].replace(old, new, 1)
        with open(os.path.join(SCENE, fname), "w") as fh:
            fh.writelines(ls)
        r = run(red_test)
        out = r.stdout + r.stderr
        build_broken = "[build failed]" in out or "declared and not used" in out or "cannot use" in out
        red = r.returncode != 0 and not build_broken
        if not red:
            ok = False
        verdict = "RED" if red else ("RED(但是编译断了，不算鉴别!)" if build_broken else "GREEN(反例无效!)")
        msg = ""
        for l in out.splitlines():
            if "FAIL:" in l or "panic:" in l or ".go:" in l:
                msg = l.strip()
                break
        line = f"[{cid}] {desc}\n      打断：{fname}:{ln}: {old} -> {new}\n      跑：{red_test} => {verdict}"
        if green_test:
            g = run(green_test)
            still = g.returncode == 0
            if not still:
                ok = False
            line += f"\n      对照：{green_test} => {'GREEN' if still else 'RED(两条子钉没分开!)'}"
        print(line)
        if msg:
            print(f"      现象：{msg[:200]}")
        restore()
    print()
    print("## 还原后复跑")
    r = run("|".join(PIN_TESTS))
    print(f"      => {'GREEN' if r.returncode == 0 else 'RED(还原没生效!)'}")
    if r.returncode != 0:
        print(r.stdout[-1200:])
        ok = False
    print()
    print("结论：" + ("全部反例都打红、对照子钉各自独立" if ok else "有反例没打红或对照没分开，判据不成立"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
