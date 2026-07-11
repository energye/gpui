package examples

import (
	_ "embed"
	"github.com/energye/gpui/render/text"
	"log"
	"os"
)

//go:embed DroidSansFallbackFull.ttf
var Font []byte

func LoadTextSources() []*text.FontSource {
	var sources []*text.FontSource

	for _, candidate := range []struct {
		path string
		opts []text.SourceOption
	}{
		{path: "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"},
		//{path: "/usr/share/fonts/TTF/DejaVuSans.ttf"},
		//{path: "/usr/share/fonts/liberation/LiberationSans-Regular.ttf"},
		//{path: "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", opts: []text.SourceOption{text.WithCollectionIndex(1)}},
		//{path: "/System/Library/Fonts/Supplemental/Arial Unicode.ttf"},
		//{path: "/System/Library/Fonts/Supplemental/Arial.ttf"},
		//{path: "C:\\Windows\\Fonts\\arial.ttf"},
		//{path: "C:\\Windows\\Fonts\\malgun.ttf"},
	} {
		if _, err := os.Stat(candidate.path); err != nil {
			continue
		}
		src, err := text.NewFontSourceFromFile(candidate.path, candidate.opts...)
		if err != nil {
			log.Printf("Skipping font %s: %v", candidate.path, err)
			continue
		}
		sources = append(sources, src)
	}

	src, err := text.NewFontSource(Font)
	if err != nil {
		log.Fatalf("Embedded font load error: %v", err)
	}
	sources = append(sources, src)

	return sources
}

func MakeFallbackFace(sources []*text.FontSource, size float64) text.Face {
	faces := make([]text.Face, 0, len(sources))
	for _, src := range sources {
		faces = append(faces, src.Face(size))
	}
	if len(faces) == 1 {
		return faces[0]
	}
	mf, err := text.NewMultiFace(faces...)
	if err != nil {
		log.Fatalf("Font fallback error: %v", err)
	}
	return mf
}

var sources = LoadTextSources()

func Face(size float64) text.Face {
	return MakeFallbackFace(sources, size)
}
