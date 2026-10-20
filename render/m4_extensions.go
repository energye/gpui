//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import ()

//
// 一句话：显卡把四边形拆成 TL-TR-BL 与 TR-BR-BL 两个三角分别贴图，
// CPU 必须按同样拆法逐像素采样，不许再拉成方块，也不许静默画错。
//
// 约定：
//   - 统一入口 DrawImageQuad：零值为 Bilinear + 不透明 + Normal；
//     退化、自交、非有限数、非 Normal 混合都返回哨兵错，调用方能判错不背锅。
//   - 旧两参 DrawImageQuad 已删除；同名统一为带 opts 的入口（2026-10-21，接口全量切换制）。
//   - 源图恒取整张（u/v 0..1），与显卡 QueueImageDraw 一致；
//     采样按 Interpolation 选项走既有 Sample 实现。

// QuadKind 是四边形分类：只有 QuadOK 能画，其余画不得。
type QuadKind int

const (
	// QuadOK 可画（凸凹皆可，按显卡同拆法）。
	QuadOK QuadKind = iota
	// QuadDegenerate 面积为零（点线重合或被矩阵压扁）。
	QuadDegenerate
	// QuadBowTie 自交蝴蝶结（边与边交叉）。
	QuadBowTie
	// QuadNonFinite 角点有 NaN 或 Inf。
	QuadNonFinite
)
