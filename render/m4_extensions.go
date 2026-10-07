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
//   - 老 DrawImageQuad 签名不动，内部改走正确梯形；退化四边形直接跳过不崩。
//   - 新 DrawImageQuadEx 带选项加显式报错：退化、自交、非有限数、
//     非 Normal 混合都返回哨兵错，调用方能判错不背锅。
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
