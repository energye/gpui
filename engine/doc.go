//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package engine is the 2.5D game engine: pure game numbers, no pixels.
//
// S73 接口号：2.5D-IF-1.0（W15 终验拍板，2026-10-02，用户已评审）。
// 冻住的是能力清单，不是实现：子包只做加法，老调用方零改编过。
// 破坏性变更先升版本号（见 core.SchemaVersion），再搬数据。
//
// S73 联机预留（只冻口子，不写逻辑）：联机留定点数口（浮点不同步
// 问题以后走定点），脚本随包走（逻辑脚本与引擎同版本发货）。
// 窗与单测都不覆盖联机行为，覆盖的是"口子在、老调用能编过"。
//
// 能力清单（只读索引，详情见各包 doc.go）：
//   - core: Vec2/Vec3、Handle、Result、SchemaVersion（数与错的底座）。
//   - camera: 镜头、视差、透视投影、真 2.5D 三件套（基向量/Y 排序/影子）。
//   - sprite/anim: 精灵批、帧动画、时间轴、缓动、状态机、粒子。
//   - tilemap/world: 分区流送、世界开关、Y 排序盖序。
//   - physics/input/audio/save: 碰撞、输入、位置音、存盘、音乐分析。
//   - light/fx: 灯、柔边三档、色调三曲线（默认 Reinhard）、降级标记。
//   - tex/asset/preview: 压缩两格式、过滤两预设、图集工具、重载、录帧。
package engine
