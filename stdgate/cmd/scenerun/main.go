// Command scenerun renders scene JSON with gpui and writes PNG.
//
//	go run ./stdgate/cmd/scenerun -scene testdata/scenes/D01_ClipLayerText.json -out tmp/scenes
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/stdgate/scene"
)

func main() {
	scenePath := flag.String("scene", "", "path to one scene JSON")
	sceneDir := flag.String("dir", "", "directory of *.json scenes")
	outDir := flag.String("out", "tmp/scenes_gpui", "output PNG directory")
	fontRoot := flag.String("font-root", ".", "root for relative font paths in scenes")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal(err)
	}
	var files []string
	if *scenePath != "" {
		files = append(files, *scenePath)
	}
	if *sceneDir != "" {
		m, err := filepath.Glob(filepath.Join(*sceneDir, "*.json"))
		if err != nil {
			fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) == 0 {
		fatal(fmt.Errorf("provide -scene or -dir"))
	}
	for _, f := range files {
		s, err := scene.Load(f)
		if err != nil {
			fatal(err)
		}
		dc, err := scene.RunGPUI(s, *fontRoot)
		if err != nil {
			fatal(fmt.Errorf("%s: %w", f, err))
		}
		out := filepath.Join(*outDir, s.ID+".png")
		if err := dc.SavePNG(out); err != nil {
			dc.Close()
			fatal(err)
		}
		dc.Close()
		fmt.Println("wrote", out)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "scenerun:", err)
	if strings.Contains(err.Error(), "") {
		os.Exit(1)
	}
}
