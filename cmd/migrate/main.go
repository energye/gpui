// GPUI 迁移工具
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

type Config struct {
	TargetModule   string            `json:"target_module"`
	TargetDir      string            `json:"target_dir"`
	Mappings       []Mapping         `json:"mappings"`
	ExternalDeps   []string          `json:"external_deps"`
	ReplaceImports map[string]string `json:"replace_imports"`
	ExcludePats    []string          `json:"exclude_patterns"`
}

type Mapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Module string `json:"module"`
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

	moduleMap := make(map[string]*Mapping)
	for i, m := range cfg.Mappings {
		moduleMap[m.Module] = &cfg.Mappings[i]
	}

	externalPrefixes := make([]string, len(cfg.ExternalDeps))
	copy(externalPrefixes, cfg.ExternalDeps)
	sort.Slice(externalPrefixes, func(i, j int) bool {
		return len(externalPrefixes[i]) > len(externalPrefixes[j])
	})

	replaceOld := make([]string, 0, len(cfg.ReplaceImports))
	for old := range cfg.ReplaceImports {
		replaceOld = append(replaceOld, old)
	}
	sort.Slice(replaceOld, func(i, j int) bool {
		return len(replaceOld[i]) > len(replaceOld[j])
	})

	targetDir := cfg.TargetDir
	excludeSet := makeSet(cfg.ExcludePats)

	// 步骤0: 清理各映射子目录中匹配排除模式的文件（不清理根目录）
	fmt.Println("🧹 步骤0: 清理各映射子目录中匹配排除模式的已有文件...")
	cleanCount := 0
	for _, m := range cfg.Mappings {
		subDir := filepath.Join(targetDir, m.Target)
		cleanCount += cleanExcludedFiles(subDir, excludeSet)
	}
	fmt.Printf("   ✅ 清理了 %d 个文件/目录\n", cleanCount)

	// 步骤1: 复制源文件到目标目录
	fmt.Println("📦 步骤1: 复制源文件到目标目录...")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 创建目标目录失败: %v\n", err)
		os.Exit(1)
	}
	copiedFiles, err := copySources(cfg.Mappings, targetDir, excludeSet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 复制文件失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   ✅ 复制了 %d 个文件\n", copiedFiles)

	// 步骤2: 扫描所有 .go 文件
	fmt.Println("\n🔍 步骤2: 扫描并解析导入依赖...")
	goFiles, err := scanGoFiles(targetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 扫描文件失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   📄 发现 %d 个 .go 文件\n", len(goFiles))

	// 步骤3: 分析导入依赖
	fmt.Println("\n🔎 步骤3: 分析导入依赖，识别不可用文件...")
	var deadFiles []string
	for _, gf := range goFiles {
		imports, err := parseImports(filepath.Join(targetDir, gf.path))
		if err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  解析 %s 失败: %v\n", gf.path, err)
			continue
		}

		var hasUnresolved bool
		for _, imp := range imports {
			switch {
			case isSelfImport(imp, moduleMap):
			case isMappedImport(imp, moduleMap):
			case isExternalDep(imp, externalPrefixes):
			case isReplaceImport(imp, replaceOld):
			default:
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

	// 步骤6: 删除各映射子目录中的 go.mod/go.sum（保留根 go.mod）
	fmt.Println("\n📄 步骤6: 删除各映射子目录中的 go.mod/go.sum...")
	modCount := 0
	for _, m := range cfg.Mappings {
		subDir := filepath.Join(targetDir, m.Target)
		filepath.WalkDir(subDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			if name == "go.mod" || name == "go.sum" {
				os.Remove(path)
				modCount++
			}
			return nil
		})
	}
	fmt.Printf("   ✅ 删除了 %d 个 go.mod/go.sum\n", modCount)

	// 步骤7: 生成根 go.mod
	fmt.Println("\n📝 步骤7: 生成根 go.mod...")
	allDeps := collectExternalDeps(cfg.Mappings, cfg.ExternalDeps)
	writeRootGoMod(targetDir, cfg.TargetModule, allDeps)
	fmt.Println("   ✅ 根 go.mod 已生成")

	// 步骤8: 重写导入路径
	fmt.Println("\n✏️  步骤8: 重写导入路径...")
	rewriteCount := 0
	for _, gf := range goFiles {
		fullPath := filepath.Join(targetDir, gf.path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			continue
		}
		changed, err := rewriteImports(fullPath, moduleMap, cfg.TargetModule, externalPrefixes, cfg.ReplaceImports)
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
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".wgsl" && ext != ".s" && ext != ".h" && ext != ".mod" {
			return nil
		}
		if err := copyFile(path, target); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func copyFile(src, dst string) error {
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

// cleanExcludedFiles 删除指定目录中匹配排除模式的文件和目录
func cleanExcludedFiles(root string, excludeSet map[string]bool) int {
	if root == "" {
		return 0
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return 0
	}
	count := 0
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}
		if shouldExclude(rel, excludeSet) {
			if d.IsDir() {
				os.RemoveAll(path)
				count++
				return filepath.SkipDir
			}
			os.Remove(path)
			count++
		}
		return nil
	})
	return count
}

func shouldExclude(rel string, excludeSet map[string]bool) bool {
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		if excludeSet[part] {
			return true
		}
	}
	name := parts[len(parts)-1]
	if excludeSet[name] {
		return true
	}
	for pat := range excludeSet {
		matched, _ := filepath.Match(pat, name)
		if matched {
			return true
		}
		for _, p := range parts {
			matched, _ := filepath.Match(pat, p)
			if matched {
				return true
			}
		}
	}
	return false
}

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
		files = append(files, fileInfo{path: rel, hasTest: strings.HasSuffix(rel, "_test.go")})
		return nil
	})
	return files, err
}

type fileInfo struct {
	path    string
	hasTest bool
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

func isSelfImport(imp string, moduleMap map[string]*Mapping) bool {
	for _, m := range moduleMap {
		if strings.HasPrefix(imp, m.Module) {
			return true
		}
	}
	return false
}

func isMappedImport(imp string, moduleMap map[string]*Mapping) bool {
	for _, m := range moduleMap {
		if imp == m.Module || strings.HasPrefix(imp, m.Module+"/") {
			return true
		}
	}
	return false
}

func isExternalDep(imp string, prefixes []string) bool {
	if !strings.Contains(imp, ".") {
		return true
	}
	for _, p := range prefixes {
		if imp == p || strings.HasPrefix(imp, p+"/") {
			return true
		}
	}
	return false
}

func isReplaceImport(imp string, replaceOld []string) bool {
	for _, old := range replaceOld {
		if imp == old || strings.HasPrefix(imp, old+"/") {
			return true
		}
	}
	return false
}

func rewriteImports(filePath string, moduleMap map[string]*Mapping, targetModule string, externalPrefixes []string, replaceImports map[string]string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	content := string(data)
	original := content

	// 替换模块路径（带 " 前缀，用于 import 语句）
	for _, m := range moduleMap {
		oldPrefix := "\"" + m.Module
		newPrefix := "\"" + targetModule + "/" + m.Target
		content = strings.ReplaceAll(content, oldPrefix, newPrefix)
	}

	// 替换模块路径（不带 " 前缀，用于注释、URL 等）
	sorted := make([]*Mapping, 0, len(moduleMap))
	for _, m := range moduleMap {
		sorted = append(sorted, m)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].Module) > len(sorted[j].Module)
	})
	for _, m := range sorted {
		content = strings.ReplaceAll(content, m.Module, targetModule+"/"+m.Target)
	}

	// 替换自定义导入（如 goffi → gpui/ffi）
	if len(replaceImports) > 0 {
		rOld := make([]string, 0, len(replaceImports))
		for old := range replaceImports {
			rOld = append(rOld, old)
		}
		sort.Slice(rOld, func(i, j int) bool {
			return len(rOld[i]) > len(rOld[j])
		})
		for _, old := range rOld {
			newPrefix := replaceImports[old]
			oldStr := "\"" + old
			newStr := "\"" + newPrefix
			content = strings.ReplaceAll(content, oldStr, newStr)
		}
	}

	if content != original {
		return true, os.WriteFile(filePath, []byte(content), 0644)
	}
	return false, nil
}

func cleanEmptyDirs(root string) {
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
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pkg := parts[0]
				ver := parts[1]
				if pkg == selfModule || strings.HasPrefix(pkg, selfModule+"/") {
					continue
				}
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

func writeRootGoMod(targetDir, moduleName string, deps map[string]string) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("module %s\n\n", moduleName))
	sb.WriteString("go 1.25.0\n\n")
	if len(deps) > 0 {
		sb.WriteString("require (\n")
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
	os.WriteFile(filepath.Join(targetDir, "go.mod"), []byte(sb.String()), 0644)
}
