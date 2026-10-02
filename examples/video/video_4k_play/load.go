package main

import (
	"fmt"
	"os"

	govideo "github.com/energye/gpui/video"
)

// clipName is generated on the spot (big binaries stay out of git):
// bash video/testdata/gen_p3_2k4k.sh
const clipName = "p3_4k_3840_2160_30fps.mp4"

func resolveClip(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../../video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

// openLive opens the P3 clip for looping play. A missing clip is a hard
// error with the generation command, never a silent skip.
func openLive() (*govideo.Player, string, error) {
	path := resolveClip(clipName)
	if _, err := os.Stat(path); err != nil {
		return nil, path, fmt.Errorf("缺片 %s：先跑 bash video/testdata/gen_p3_2k4k.sh（大二进制不进仓库，本地与 CI 现场生成）", clipName)
	}
	p, err := govideo.OpenFile(path, govideo.Options{Loop: true})
	if err != nil {
		return nil, path, err
	}
	return p, path, nil
}
