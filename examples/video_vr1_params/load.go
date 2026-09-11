package main

import (
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

// loadParams parses parameters and cuts frames. A real MP4 path goes
// through mp4.Parse plus avcC plus bounded mdat sample reads; otherwise a
// synthetic Annex B stream is built in-memory. Numbers always come from
// the real parse, never hand-written.
func loadParams(path string) *paramsResult {
	t0 := time.Now()
	res := &paramsResult{hist: map[int]int{}, profilesHit: map[uint8]bool{}}
	defer func() {
		res.parseMs = float64(time.Since(t0).Microseconds()) / 1000.0
	}()
	// 基础/主档识别用合成单元同步证明，保证窗口内三档全认可验。
	for _, tc := range []struct {
		profile uint8
		wMBs    uint32
		hMap    uint32
		id      uint32
	}{{66, 53, 29, 11}, {77, 79, 44, 12}} {
		if s, err := h264.ParseSPS(buildSynSPS(tc.profile, 30, tc.id, tc.wMBs, tc.hMap, 0, nil)); err == nil && h264.IsBaselineMainHigh(s.ProfileIDC) {
			res.profilesHit[s.ProfileIDC] = true
		}
	}
	if path != "" {
		if err := loadReal(path, res); err != nil {
			res.err = err
		}
		return res
	}
	loadSynthetic(res)
	return res
}

func loadReal(path string, res *paramsResult) error {
	movie, err := mp4.ParseFile(path)
	if err != nil {
		res.source = "文件：" + path + "(盒子打不开)"
		return err
	}
	v := movie.Video
	if v == nil {
		res.source = "文件：" + path + "(无视频轨)"
		return fmt.Errorf("no video track")
	}
	res.source = fmt.Sprintf("文件：%s(%dx%d)", path, v.Width, v.Height)
	avcc, err := h264.ParseAVCC(v.AVCConfig)
	if err != nil {
		return err
	}
	res.packing = "AVCC长度前缀(盒内)+带外参数"
	ps := h264.NewParamSets()
	if err := ps.FromAVCC(avcc); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var ordered [][]byte
	var bytesRead int64
	const maxSamples = 8
	const maxBytes = int64(2 << 20)
	for i, s := range v.Samples {
		if i >= maxSamples || bytesRead >= maxBytes {
			break
		}
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			return fmt.Errorf("sample %d unreadable: %w", s.Number, err)
		}
		bytesRead += int64(s.Size)
		units, err := h264.SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			return err
		}
		for _, u := range units {
			if t, ok := h264.NALType(u); ok {
				res.hist[t]++
				res.nalTotal++
			}
			if isParam, err := ps.AddNALU(u); err != nil {
				return err
			} else if isParam {
				if t, _ := h264.NALType(u); t == h264.NALSPS {
					res.spsInband++
				} else {
					res.ppsInband++
				}
			}
			ordered = append(ordered, u)
		}
	}
	frames, err := h264.SplitFrames(ordered)
	if err != nil {
		return err
	}
	res.frames = frames
	res.spsTotal = len(ps.SPS)
	res.ppsTotal = len(ps.PPS)
	pickPrimary(ps, res)
	for _, s := range ps.SPS {
		if h264.IsBaselineMainHigh(s.ProfileIDC) {
			res.profilesHit[s.ProfileIDC] = true
		}
	}
	_ = avcc
	return nil
}

func pickPrimary(ps *h264.ParamSets, res *paramsResult) {
	var best *h264.SPS
	for _, s := range ps.SPS {
		if best == nil || s.ID < best.ID {
			best = s
		}
	}
	if best == nil {
		return
	}
	res.profile = best.Profile
	res.profileIDC = best.ProfileIDC
	res.level = best.Level
	res.levelIDC = best.LevelIDC
	res.levelOK = h264.LevelSupported(best.LevelIDC)
	res.width = best.Width
	res.height = best.Height
	if q, ok := ps.PPS[0]; ok {
		res.cabac = q.EntropyCABAC
	} else {
		for _, q := range ps.PPS {
			res.cabac = q.EntropyCABAC
			break
		}
	}
	if res.cabac {
		res.entropy = "高压缩那套"
	} else {
		res.entropy = "简单那套"
	}
	for _, f := range res.frames {
		if f.IsIDR {
			res.idrFrames++
		}
	}
}

func loadSynthetic(res *paramsResult) {
	res.source = "合成片：片头/图参数+关键帧+3个非关键帧(AnnexB起始码)"
	res.packing = "AnnexB起始码+带内参数"
	spsH := buildSynSPS(100, 31, 0, 79, 44, 0, nil)
	pps := buildSynPPS(0, 0, true, 0, false)
	stream := []byte{0, 0, 0, 1}
	stream = append(stream, spsH...)
	stream = append(stream, 0, 0, 1)
	stream = append(stream, pps...)
	for i, sl := range [][]byte{
		buildSynSlice(h264.NALSliceIDR, 3, 0),
		buildSynSlice(h264.NALSliceNonIDR, 2, 0),
		buildSynSlice(h264.NALSliceNonIDR, 2, 0),
		buildSynSlice(h264.NALSliceNonIDR, 2, 0),
	} {
		_ = i
		stream = append(stream, 0, 0, 1)
		stream = append(stream, sl...)
	}
	units, err := h264.SplitAnnexB(stream)
	if err != nil {
		res.err = err
		return
	}
	ps := h264.NewParamSets()
	var ordered [][]byte
	for _, u := range units {
		if t, ok := h264.NALType(u); ok {
			res.hist[t]++
			res.nalTotal++
		}
		if isParam, err := ps.AddNALU(u); err != nil {
			res.err = err
			return
		} else if isParam {
			if t, _ := h264.NALType(u); t == h264.NALSPS {
				res.spsInband++
			} else {
				res.ppsInband++
			}
		}
		ordered = append(ordered, u)
	}
	frames, err := h264.SplitFrames(ordered)
	if err != nil {
		res.err = err
		return
	}
	res.frames = frames
	res.spsTotal = len(ps.SPS)
	res.ppsTotal = len(ps.PPS)
	pickPrimary(ps, res)
	for _, s := range ps.SPS {
		if h264.IsBaselineMainHigh(s.ProfileIDC) {
			res.profilesHit[s.ProfileIDC] = true
		}
	}
}

func (res *paramsResult) infoLines() []string {
	profs := "基础/主/高"
	if !(res.profilesHit[66] && res.profilesHit[77] && res.profilesHit[100]) {
		profs += "(缺)"
	} else {
		profs += "(全认)"
	}
	errLine := "解析=成功"
	if res.err != nil {
		errLine = "解析=" + shortErr(translateH264Error(res.err.Error()), 48)
	}
	return []string{
		"来源：" + shortErr(res.source, 56),
		fmt.Sprintf("档位=%s(%s) 等级=%s", res.profile, profs, res.level),
		fmt.Sprintf("尺寸=%dx%d 熵编码=%s", res.width, res.height, res.entropy),
		fmt.Sprintf("片头参数=%d个(流内%d) 图参数=%d个(流内%d)", res.spsTotal, res.spsInband, res.ppsTotal, res.ppsInband),
		fmt.Sprintf("打包=%s", res.packing),
		fmt.Sprintf("NAL共%d个 切出%d帧(IDR%d帧)", res.nalTotal, len(res.frames), res.idrFrames),
		fmt.Sprintf("解析耗时=%.2f毫秒", res.parseMs),
		errLine,
	}
}

// Synthetic Annex B builders (test-data generation; parsing still goes
// through the engine).

type bitWriter struct {
	buf []byte
	cur uint8
	n   int
}

func (w *bitWriter) bit(b uint32) {
	w.cur = (w.cur << 1) | uint8(b&1)
	w.n++
	if w.n == 8 {
		w.buf = append(w.buf, w.cur)
		w.cur = 0
		w.n = 0
	}
}

func (w *bitWriter) bits(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		w.bit((v >> uint(i)) & 1)
	}
}

func (w *bitWriter) ue(v uint32) {
	n := uint32(0)
	t := v + 1
	for t >>= 1; t != 0; t >>= 1 {
		n++
	}
	for i := uint32(0); i < n; i++ {
		w.bit(0)
	}
	w.bit(1)
	if n > 0 {
		w.bits(v+1-(1<<n), int(n))
	}
}

func (w *bitWriter) flush() []byte {
	w.bit(1)
	for w.n != 0 {
		w.bit(0)
	}
	return append([]byte(nil), w.buf...)
}

func buildSynSPS(profile, level uint8, id, wMBs, hMap uint32, poc uint32, crop []uint32) []byte {
	w := &bitWriter{}
	w.bits(uint32(profile), 8)
	w.bits(0, 8)
	w.bits(uint32(level), 8)
	w.ue(id)
	if profile == 100 || profile == 110 || profile == 122 || profile == 244 {
		w.ue(1)
		w.ue(0)
		w.ue(0)
		w.bit(0)
		w.bit(0)
	}
	w.ue(0)
	w.ue(poc)
	if poc == 0 {
		w.ue(0)
	} else if poc == 1 {
		w.bit(0)
		w.ue(0)
		w.ue(0)
		w.ue(0)
	}
	w.ue(1)
	w.bit(0)
	w.ue(wMBs)
	w.ue(hMap)
	w.bit(1)
	w.bit(1)
	if len(crop) == 4 {
		w.bit(1)
		for _, c := range crop {
			w.ue(c)
		}
	} else {
		w.bit(0)
	}
	w.bit(0)
	return append([]byte{0x67}, w.flush()...)
}

func buildSynPPS(ppsID, spsID uint32, cabac bool, groups uint32, ext bool) []byte {
	w := &bitWriter{}
	w.ue(ppsID)
	w.ue(spsID)
	if cabac {
		w.bit(1)
	} else {
		w.bit(0)
	}
	w.bit(0)
	w.ue(groups)
	w.ue(0)
	w.ue(1)
	w.bit(0)
	w.bits(0, 2)
	w.ue(0)
	w.ue(0)
	w.ue(0)
	w.bit(1)
	w.bit(0)
	w.bit(0)
	if ext {
		w.bit(1)
		w.bit(0)
		w.ue(0)
	}
	return append([]byte{0x68}, w.flush()...)
}

func buildSynSlice(typ, refIDC int, firstMB uint32) []byte {
	w := &bitWriter{}
	w.ue(firstMB)
	return append([]byte{byte(refIDC<<5 | typ)}, w.flush()...)
}
