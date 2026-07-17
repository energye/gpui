// Command standardtest generates CanvasKit standards, gpui test images,
// and side-by-side merge comparisons under standardtest/diff/.
//
//	go run ./standardtest/cmd/standardtest all
//	go run ./standardtest/cmd/standardtest all -id D01_ClipLayerText
//	go run ./standardtest/cmd/standardtest standard|test|merge
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/energye/gpui/standardtest"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	root := fs.String("root", ".", "module root or any subdir")
	id := fs.String("id", "", "single scene id (e.g. D01_ClipLayerText)")
	scenes := fs.String("scenes", "", "override scenes directory")
	_ = fs.Parse(os.Args[2:])

	p, err := standardtest.ResolvePaths(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *scenes != "" {
		p.Scenes = *scenes
	}

	switch cmd {
	case "all":
		err = standardtest.RunAll(p, *id)
	case "standard", "gen-standard":
		err = standardtest.GenStandard(p, *id)
	case "test", "gen-test":
		err = standardtest.GenTest(p, *id)
	case "merge":
		err = standardtest.GenMerge(p, *id)
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `standardtest — 标准图 / 测试图 / 合并对比

流程:
  1) CanvasKit → standardtest/diff/standard/
  2) gpui      → standardtest/diff/test/
  3) 合并      → standardtest/diff/merge/  (左标准 右待测 底栏预期说明)

用法:
  go run ./standardtest/cmd/standardtest all [-id D01_ClipLayerText]
  go run ./standardtest/cmd/standardtest standard [-id ...]
  go run ./standardtest/cmd/standardtest test [-id ...]
  go run ./standardtest/cmd/standardtest merge [-id ...]

环境 (gpui 出图):
  export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
  export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
  export DISPLAY=:1
`)
}
