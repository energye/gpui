//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package kit

// AnimatedImageProps 是动图组件的属性快照。
//
// 值语义（与 ButtonProps/IconProps 一致）：创建、传给 BuildAnimatedImage、用
// Update 换快照。组件不回写字段；节奏相关的一切决策只读这里。
//
// 循环与帧时长不进属性：它们是容器真值（AnimatedImage.RepetitionCount /
// FrameInfo.Duration），组件按容器原值播放，不偷偷改写（批 1b「节奏归消费者」
// 的口径——消费者可以选择怎么播，但不改数据）。
type AnimatedImageProps struct {
	// Width/Height 是布局尺寸（逻辑像素）。零值时用解码画布的尺寸。
	Width, Height float64
	// Speed 是播放倍速；<=0 按 1（原速）处理，是组件内部自洽的默认值。
	Speed float64
}

// DefaultAnimatedImageProps 返回按解码画布尺寸、原速播放的默认属性。
func DefaultAnimatedImageProps(w, h float64) AnimatedImageProps {
	return AnimatedImageProps{Width: w, Height: h, Speed: 1}
}
