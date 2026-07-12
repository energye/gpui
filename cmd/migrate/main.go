// GPUI 迁移工具 — 将 gogpu 生态的库迁移到 gpui，自动处理导入路径和依赖
//
// 用法:
//
//	migrate -config migrate.json
//
// 配置文件 migrate.json 示例:
//
//	{
//	  "target_module": "github.com/energye/gpui",
//	  "target_dir": "/home/yanghy/app/projects/gogpu/gpui",
//	  "mappings": [
//	    {"source": "/home/yanghy/app/projects/gogpu/gg", "target": "gg", "module": "github.com/gogpu/gg"},
//	    {"source": "/home/yanghy/app/projects/gogpu/wgpu", "target": "wgpu", "module": "github.com/gogpu/wgpu"},
//	    {"source": "/home/yanghy/app/projects/gogpu/naga", "target": "naga", "module": "github.com/gogpu/naga"},
//	    {"source": "/home/yanghy/app/projects/gogpu/gputypes", "target": "gputypes", "module": "github.com/gogpu/gputypes"},
//	    {"source": "/home/yanghy/app/projects/gogpu/gpucontext", "target": "gpucontext", "module": "github.com/gogpu/gpucontext"}
//	  ],
//	  "external_deps": [
//	    "github.com/go-webgpu/goffi",
//	    "golang.org/x/image",
//	    "golang.org/x/text",
//	    "golang.org/x/sys"
//	  ],
//	  "exclude_patterns": [
//	    ".git", ".github", "go.mod", "go.sum", ".gitignore",
//	    ".golangci.yml", "codecov.yml", "llms.txt", "AGENTS.md",
//	    "ROADMAP.md", "SECURITY.md", "CODE_OF_CONDUCT.md", "CONTRIBUTING.md",
//	    "UPSTREAM", "UPSTREAM.md", "STABILITY.md", "MIGRATION.md",
//	    "SPONSORS.md", "UPSTREAM", "assets", "docs", "scripts",
//	    "snapshot", "codecov.yml", ".codecov.yml"
//	  ]
//	}
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Config 迁移配置
type Config struct {
	TargetModule   string            `json:"target_module"`    // 目标模块名，如 github.com/energye/gpui
	TargetDir      string            `json:"target_dir"`       // 目标目录
	Mappings       []Mapping         `json:"mappings"`         // 源→目标映射
	ExternalDeps   []string          `json:"external_deps"`    // 保留的外部依赖前缀
	ReplaceImports map[string]string `json:"replace_imports"`  // 导入路径替换，旧前缀→新前缀
	ExcludePats    []string          `json:"exclude_patterns"` // 排除的文件/目录模式
}

// Mapping 单个库的映射
type Mapping struct {
	Source string `json:"source"` // 源路径
	Target string `json:"target"` // 目标目录名（如 gg → render）
	Module string `json:"module"` // 源模块名（如 github.com/gogpu/gg）
}

// fileInfo 扫描到的文件信息
type fileInfo struct {
	path    string   // 相对于 target_dir 的路径
	imports []string // 导入的包路径
	hasTest bool     // 是否 _test.go
}

// depVersion 依赖版本
type depVersion struct {
	path     string
	version  string
	indirect bool
}

func main() {
	cfgPath := flag.String("config", "migrate.json", "配置文件路径")
	flag.Parse()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 加载配置失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🚀 GPUI 迁移工具启动")
	fmt.Printf("   目标模块: %s\n", cfg.TargetModule)
	fmt.Printf("   目标目录: %s\n", cfg.TargetDir)
	fmt.Printf("   迁移库数: %d\n", len(cfg.Mappings))
	fmt.Println()

	// 建立模块名→映射的查找表
	moduleMap := make(map[string]*Mapping)
	for i, m := range cfg.Mappings {
		moduleMap[m.Module] = &cfg.Mappings[i]
	}

	// 建立外部依赖前缀查找表
	externalPrefixes := make([]string, len(cfg.ExternalDeps))
	copy(externalPrefixes, cfg.ExternalDeps)
	sort.Slice(externalPrefixes, func(i, j int) bool {
		return len(externalPrefixes[i]) > len(externalPrefixes[j]) // 长前缀优先
	})

	// 建立替换导入前缀查找表（按长度降序，长前缀优先匹配）
	replaceOld := make([]string, 0, len(cfg.ReplaceImports))
	for old := range cfg.ReplaceImports {
		replaceOld = append(replaceOld, old)
	}
	sort.Slice(replaceOld, func(i, j int) bool {
		return len(replaceOld[i]) > len(replaceOld[j])
	})

	// 步骤1: 复制源文件到目标目录
	fmt.Println("📦 步骤1: 复制源文件到目标目录...")
	targetDir := cfg.TargetDir
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 创建目标目录失败: %v\n", err)
		os.Exit(1)
	}
	excludeSet := makeSet(cfg.ExcludePats)
	copiedFiles, err := copySources(cfg.Mappings, targetDir, excludeSet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 复制文件失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   ✅ 复制了 %d 个文件\n", copiedFiles)

	// 步骤2: 扫描所有 .go 文件，解析导入
	fmt.Println("\n🔍 步骤2: 扫描并解析导入依赖...")
	goFiles, err := scanGoFiles(targetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 扫描文件失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   📄 发现 %d 个 .go 文件\n", len(goFiles))

	// 步骤3: 分析每个文件的导入
	fmt.Println("\n🔎 步骤3: 分析导入依赖，识别不可用文件...")
	targetModule := cfg.TargetModule
	var deadFiles []string // 需要删除的文件
	for _, gf := range goFiles {
		imports, err := parseImports(filepath.Join(targetDir, gf.path))
		if err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  解析 %s 失败: %v\n", gf.path, err)
			continue
		}

		var hasUnresolved bool
		for _, imp := range imports {
			switch {
			case isSelfImport(imp, moduleMap, targetModule, gf.path):
				// 自身引用，可解析

			case isMappedImport(imp, moduleMap):
				// 引用了其他被迁移的库，可解析

			case isExternalDep(imp, externalPrefixes):
				// 外部依赖，可解析

			case isReplaceImport(imp, replaceOld):
				// 需要替换的导入（如 goffi → gpui/ffi），可解析

			default:
				// 无法解析的导入 → 标记为死文件
				hasUnresolved = true
				fmt.Printf("   ⚠️  %s: 不可解析的导入 %s\n", gf.path, imp)
			}
		}

		if hasUnresolved {
			deadFiles = append(deadFiles, gf.path)
		}
	}

	// 步骤4: 删除不可用文件
	fmt.Println("\n🗑️  步骤4: 删除不可用文件...")
	deadCount := 0
	for _, f := range deadFiles {
		fullPath := filepath.Join(targetDir, f)
		if err := os.Remove(fullPath); err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  删除 %s 失败: %v\n", f, err)
			continue
		}
		deadCount++
		fmt.Printf("   🗑️  删除 %s\n", f)
	}
	fmt.Printf("   ✅ 删除了 %d 个不可用文件\n", deadCount)

	// 步骤5: 清理空目录
	fmt.Println("\n🧹 步骤5: 清理空目录...")
	cleanEmptyDirs(targetDir)
	fmt.Println("   ✅ 空目录已清理")

	// 步骤6: 删除子模块 go.mod/go.sum
	fmt.Println("\n📄 步骤6: 删除子模块 go.mod/go.sum...")
	for _, m := range cfg.Mappings {
		gm := filepath.Join(targetDir, m.Target, "go.mod")
		gs := filepath.Join(targetDir, m.Target, "go.sum")
		os.Remove(gm)
		os.Remove(gs)
	}
	fmt.Println("   ✅ 子模块 go.mod/go.sum 已删除")

	// 步骤7: 收集外部依赖版本，写入根 go.mod
	fmt.Println("\n📝 步骤7: 生成根 go.mod...")
	allDeps := collectExternalDeps(cfg.Mappings, cfg.ExternalDeps)
	writeRootGoMod(targetDir, cfg.TargetModule, allDeps)
	fmt.Println("   ✅ 根 go.mod 已生成")

	// 步骤8: 重写所有文件的导入路径
	fmt.Println("\n✏️  步骤8: 重写导入路径...")
	rewriteCount := 0
	for _, gf := range goFiles {
		fullPath := filepath.Join(targetDir, gf.path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			continue // 已被删除
		}
		changed, err := rewriteImports(fullPath, moduleMap, targetModule, externalPrefixes, cfg.ReplaceImports)
		if err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  重写 %s 失败: %v\n", gf.path, err)
			continue
		}
		if changed {
			rewriteCount++
		}
	}
	fmt.Printf("   ✅ 重写了 %d 个文件的导入路径\n", rewriteCount)

	// 步骤9: go mod tidy
	fmt.Println("\n🧪 步骤9: go mod tidy...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = targetDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "   ⚠️  go mod tidy 警告: %v\n", err)
	} else {
		fmt.Println("   ✅ go mod tidy 完成")
	}

	// 步骤10: go build 验证
	fmt.Println("\n✅ 步骤10: go build 验证...")
	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = targetDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "   ❌ go build 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   ✅ go build 通过")

	fmt.Println("\n🎉 迁移完成!")
}

// ─── 配置加载 ───────────────────────────────────────

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件: %w", err)
	}
	if cfg.TargetModule == "" {
		return nil, fmt.Errorf("target_module 不能为空")
	}
	if cfg.TargetDir == "" {
		return nil, fmt.Errorf("target_dir 不能为空")
	}
	if len(cfg.Mappings) == 0 {
		return nil, fmt.Errorf("mappings 不能为空")
	}
	return &cfg, nil
}

// ─── 文件复制 ───────────────────────────────────────

func makeSet(pats []string) map[string]bool {
	s := make(map[string]bool, len(pats))
	for _, p := range pats {
		s[p] = true
	}
	return s
}

func copySources(mappings []Mapping, targetDir string, excludeSet map[string]bool) (int, error) {
	total := 0
	for _, m := range mappings {
		src := m.Source
		dst := filepath.Join(targetDir, m.Target)
		count, err := copyDir(src, dst, excludeSet)
		if err != nil {
			return total, fmt.Errorf("复制 %s → %s: %w", src, dst, err)
		}
		total += count
	}
	return total, nil
}

func copyDir(src, dst string, excludeSet map[string]bool) (int, error) {
	count := 0
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}

		// 检查是否匹配排除模式
		if shouldExclude(rel, excludeSet) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		// 只复制 .go 文件和其他必要文件
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".wgsl" && ext != ".s" && ext != ".h" && ext != ".mod" {
			return nil
		}

		return copyFile(path, target)
	})
	return count, err
}

func copyFile(src, dst string) error {
	// 确保目标目录存在
	os.MkdirAll(filepath.Dir(dst), 0755)

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func shouldExclude(rel string, excludeSet map[string]bool) bool {
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		if excludeSet[part] {
			return true
		}
	}
	// 检查文件名是否匹配
	name := parts[len(parts)-1]
	if excludeSet[name] {
		return true
	}
	// 检查通配模式
	for pat := range excludeSet {
		matched, _ := filepath.Match(pat, name)
		if matched {
			return true
		}
		// 检查路径段匹配
		for _, p := range parts {
			matched, _ := filepath.Match(pat, p)
			if matched {
				return true
			}
		}
	}
	return false
}

// ─── 文件扫描 ───────────────────────────────────────

func scanGoFiles(root string) ([]fileInfo, error) {
	var files []fileInfo
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, fileInfo{
			path:    rel,
			hasTest: strings.HasSuffix(rel, "_test.go"),
		})
		return nil
	})
	return files, err
}

func parseImports(filePath string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var imports []string
	for _, imp := range f.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, "\""))
	}
	return imports, nil
}

// ─── 导入路径分析 ───────────────────────────────────

// isSelfImport 判断是否是自身模块的导入
func isSelfImport(imp string, moduleMap map[string]*Mapping, targetModule string, filePath string) bool {
	for _, m := range moduleMap {
		if strings.HasPrefix(imp, m.Module) {
			// 检查是否来自同一个源模块
			// 例如 github.com/gogpu/gg/text 来自 gg 模块
			return true
		}
	}
	return false
}

// rewriteSelfImport 重写自身导入路径
func rewriteSelfImport(imp string, moduleMap map[string]*Mapping, targetModule string, filePath string) string {
	for _, m := range moduleMap {
		prefix := m.Module
		if strings.HasPrefix(imp, prefix) {
			suffix := strings.TrimPrefix(imp, prefix)
			// 找到文件所在的目录，确定它属于哪个映射
			// 简单情况：直接替换前缀
			return targetModule + "/" + m.Target + suffix
		}
	}
	return imp
}

// isMappedImport 判断是否是其他被迁移的模块的导入
func isMappedImport(imp string, moduleMap map[string]*Mapping) bool {
	for _, m := range moduleMap {
		if imp == m.Module || strings.HasPrefix(imp, m.Module+"/") {
			return true
		}
	}
	return false
}

// rewriteMappedImport 重写跨模块导入路径
func rewriteMappedImport(imp string, moduleMap map[string]*Mapping, targetModule string) string {
	for _, m := range moduleMap {
		prefix := m.Module
		if strings.HasPrefix(imp, prefix) {
			suffix := strings.TrimPrefix(imp, prefix)
			return targetModule + "/" + m.Target + suffix
		}
		if imp == m.Module {
			return targetModule + "/" + m.Target
		}
	}
	return imp
}

// isExternalDep 判断是否是保留的外部依赖
func isExternalDep(imp string, prefixes []string) bool {
	// 标准库
	if !strings.Contains(imp, ".") {
		return true
	}
	// 外部依赖
	for _, p := range prefixes {
		if imp == p || strings.HasPrefix(imp, p+"/") {
			return true
		}
	}
	return false
}

// isReplaceImport 判断是否需要替换导入路径（如 goffi → gpui/ffi）
func isReplaceImport(imp string, replaceOld []string) bool {
	for _, old := range replaceOld {
		if imp == old || strings.HasPrefix(imp, old+"/") {
			return true
		}
	}
	return false
}

// ─── 文件重写 ───────────────────────────────────────

// rewriteImports 重写单个文件的导入路径，返回是否修改
func rewriteImports(filePath string, moduleMap map[string]*Mapping, targetModule string, externalPrefixes []string, replaceImports map[string]string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	content := string(data)
	original := content

	// 替换所有模块导入路径
	for _, m := range moduleMap {
		oldPrefix := "\"" + m.Module
		newPrefix := "\"" + targetModule + "/" + m.Target
		content = strings.ReplaceAll(content, oldPrefix, newPrefix)
	}

	// 替换需要替换的导入路径（如 goffi → gpui/ffi）
	// 按旧前缀长度降序替换，避免短前缀先替换导致长前缀失效
	replaceOld := make([]string, 0, len(replaceImports))
	for old := range replaceImports {
		replaceOld = append(replaceOld, old)
	}
	sort.Slice(replaceOld, func(i, j int) bool {
		return len(replaceOld[i]) > len(replaceOld[j])
	})
	for _, old := range replaceOld {
		newPrefix := replaceImports[old]
		oldStr := "\"" + old
		newStr := "\"" + newPrefix
		content = strings.ReplaceAll(content, oldStr, newStr)
	}

	if content != original {
		return true, os.WriteFile(filePath, []byte(content), 0644)
	}
	return false, nil
}

// ─── 空目录清理 ─────────────────────────────────────

func cleanEmptyDirs(root string) {
	// 多次遍历以处理嵌套空目录
	for i := 0; i < 10; i++ {
		changed := false
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() || path == root {
				return nil
			}
			entries, _ := os.ReadDir(path)
			if len(entries) == 0 {
				os.Remove(path)
				changed = true
				return filepath.SkipDir
			}
			return nil
		})
		if !changed {
			break
		}
	}
}

// ─── 外部依赖收集 ───────────────────────────────────

// collectExternalDeps 从源模块的 go.mod 中读取外部依赖版本
func collectExternalDeps(mappings []Mapping, keepDeps []string) map[string]string {
	deps := make(map[string]string)

	for _, m := range mappings {
		gmPath := filepath.Join(m.Source, "go.mod")
		versions := readGoModDeps(gmPath, keepDeps, m.Module)
		for pkg, ver := range versions {
			if _, exists := deps[pkg]; !exists {
				deps[pkg] = ver
			}
		}
	}

	return deps
}

// readGoModDeps 读取 go.mod 中的依赖版本
func readGoModDeps(gmPath string, keepDeps []string, selfModule string) map[string]string {
	deps := make(map[string]string)

	f, err := os.Open(gmPath)
	if err != nil {
		return deps
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inRequire := false
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if line == "require (" {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if inRequire {
			// 解析: github.com/xxx/xxx v1.2.3 // indirect
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pkg := parts[0]
				ver := parts[1]
				// 跳过自身模块和间接依赖
				if pkg == selfModule || strings.HasPrefix(pkg, selfModule+"/") {
					continue
				}
				// 只保留需要的外部依赖
				for _, keep := range keepDeps {
					if pkg == keep || strings.HasPrefix(pkg, keep+"/") {
						deps[pkg] = ver
						break
					}
				}
			}
		}
	}

	return deps
}

// ─── 根 go.mod 生成 ─────────────────────────────────

func writeRootGoMod(targetDir, moduleName string, deps map[string]string) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("module %s\n\n", moduleName))
	sb.WriteString("go 1.25.0\n\n")

	if len(deps) > 0 {
		sb.WriteString("require (\n")
		// 排序以保持稳定输出
		var pkgs []string
		for pkg := range deps {
			pkgs = append(pkgs, pkg)
		}
		sort.Strings(pkgs)
		for _, pkg := range pkgs {
			sb.WriteString(fmt.Sprintf("\t%s %s\n", pkg, deps[pkg]))
		}
		sb.WriteString(")\n")
	}

	path := filepath.Join(targetDir, "go.mod")
	os.WriteFile(path, []byte(sb.String()), 0644)
}
