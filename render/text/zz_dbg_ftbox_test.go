package text

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestDbgFTBox(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	cmd := exec.Command(ftexpLocal(t), "contour", fontPath, "合", "16", "l")
	out, _ := cmd.CombinedOutput()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var np, nc, adv int
	fmt.Sscanf(lines[0], "%d %d %d", &np, &nc, &adv)
	var ends []int
	for _, s := range strings.Fields(lines[np+1]) {
		v, _ := strconv.Atoi(s)
		ends = append(ends, v)
	}
	prev := 0
	for ci, e := range ends {
		fmt.Printf("轮廓%d [%d..%d]\n", ci, prev, e)
		for i := prev; i <= e; i++ {
			var x, y, tag int64
			fmt.Sscanf(lines[1+i], "%d %d %d", &x, &y, &tag)
			fmt.Printf("  %d (%d,%d) %s\n", i, x, y, map[int]string{0: "off", 1: "on"}[int(tag&1)])
		}
		prev = e + 1
	}
}
