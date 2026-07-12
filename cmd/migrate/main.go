// GPUI 迁移工具
package main

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrate.json
var embeddedConfigFS embed.FS

const defaultConfigName = "migrate.json"

type Config struct {
	TargetModule   string            `json:"target_module"`
	TargetDir      string            `json:"target_dir"`
	Mappings       []Mapping         `json:"mappings"`
	ExternalDeps   []string          `json:"external_deps"`
	ReplaceImports map[string]string `json:"replace_imports"`
	ImportAliases  map[string]string `json:"import_aliases"`
	ExcludePats    []string          `json:"exclude_patterns"`
}

type Mapping struct {
	Source        string `json:"source"`
	Target        string `json:"target"`
	Module        string `json:"module"`
	RenamePackage string `json:"rename_package,omitempty"`
	Alias         string `json:"alias,omitempty"`
}

func main() {
	cfgPath := flag.String("config", "", "配置文件路径或目录（为空使用内嵌 migrate.json）")
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

	// 步骤2: 重写根包中的 package 声明（如 gg → render）
	fmt.Println("\n📛 步骤2: 重写根包中的 package 声明...")
	pkgRenameCount := 0
	for _, m := range cfg.Mappings {
		if m.RenamePackage == "" {
			continue
		}
		subDir := filepath.Join(targetDir, m.Target)
		// 从 module 路径提取旧包名（最后一段）
		oldPkg := m.Module
		if idx := strings.LastIndex(oldPkg, "/"); idx >= 0 {
			oldPkg = oldPkg[idx+1:]
		}
		n, err := renamePackageDeclarations(subDir, oldPkg, m.RenamePackage)
		if err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  重写 %s 包名失败: %v\n", m.Target, err)
			continue
		}
		pkgRenameCount += n
	}
	fmt.Printf("   ✅ 重写了 %d 个文件的 package 声明\n", pkgRenameCount)

	// 步骤3: 扫描各映射子目录中的 .go 文件（不扫描根目录）
	fmt.Println("\n🔍 步骤3: 扫描各映射子目录中的 .go 文件...")
	var goFiles []fileInfo
	for _, m := range cfg.Mappings {
		subDir := filepath.Join(targetDir, m.Target)
		files, err := scanGoFiles(subDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  扫描 %s 失败: %v\n", subDir, err)
			continue
		}
		// 将相对路径调整为相对于 targetDir 的路径
		for _, f := range files {
			f.path = m.Target + "/" + f.path
			goFiles = append(goFiles, f)
		}
	}
	fmt.Printf("   📄 发现 %d 个 .go 文件\n", len(goFiles))

	// 步骤5: 清理各映射子目录中的空目录（不清理根目录）
	fmt.Println("\n🧹 步骤5: 清理各映射子目录中的空目录...")
	for _, m := range cfg.Mappings {
		subDir := filepath.Join(targetDir, m.Target)
		cleanEmptyDirs(subDir)
	}
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
		changed, err := rewriteImports(fullPath, moduleMap, cfg.TargetModule, cfg.ReplaceImports, cfg.ImportAliases)
		if err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  重写 %s 失败: %v\n", gf.path, err)
			continue
		}
		if changed {
			rewriteCount++
		}
	}
	fmt.Printf("   ✅ 重写了 %d 个文件的导入路径\n", rewriteCount)

	// 步骤9: 检测 stdlib 冲突并添加导入别名
	// 例如：当包名改为 "context" 但文件同时导入 stdlib "context" 时，
	// 自动添加别名 gpucontext "github.com/energye/gpui/gpu/context"
	fmt.Println("\n🔀 步骤9: 检测 stdlib 冲突并添加导入别名...")
	aliasAddCount := 0
	mappedGoFiles := listMappedGoFiles(targetDir, cfg.Mappings)
	for _, m := range cfg.Mappings {
		if m.RenamePackage == "" {
			continue
		}
		oldPkg := m.Module
		if idx := strings.LastIndex(oldPkg, "/"); idx >= 0 {
			oldPkg = oldPkg[idx+1:]
		}
		newPath := cfg.TargetModule + "/" + m.Target
		aliasName := m.Alias
		if aliasName == "" {
			aliasName = oldPkg // 默认使用旧包名作为别名
		}
		for _, f := range mappedGoFiles {
			changed, err := addStdlibConflictAlias(f, aliasName, m.RenamePackage, newPath)
			if err != nil {
				continue
			}
			if changed {
				aliasAddCount++
			}
		}
	}
	fmt.Printf("   ✅ 添加了 %d 个导入别名\n", aliasAddCount)

	// 步骤10: 重写包限定符引用（如 gg.SomeType → render.SomeType）
	// 跳过新包名与文件内其他导入冲突的情况（如 context 与 stdlib 冲突）
	fmt.Println("\n🔀 步骤10: 重写包限定符引用...")
	qualRenameCount := 0
	for _, m := range cfg.Mappings {
		if m.RenamePackage == "" {
			continue
		}
		oldPkg := m.Module
		if idx := strings.LastIndex(oldPkg, "/"); idx >= 0 {
			oldPkg = oldPkg[idx+1:]
		}
		newQualifier := m.RenamePackage
		if m.Alias != "" {
			newQualifier = m.Alias
		}
		for _, f := range mappedGoFiles {
			changed, err := renamePackageQualifier(f, oldPkg, newQualifier)
			if err != nil {
				continue
			}
			if changed {
				qualRenameCount++
			}
		}
	}
	fmt.Printf("   ✅ 重写了 %d 个文件的包限定符引用\n", qualRenameCount)

	// 步骤11: 更新不再需要的导入别名（如 naga "gpu/shader" → "shader"）
	fmt.Println("\n🧹 步骤11: 更新不再需要的导入别名...")
	aliasUpdateCount := 0
	for _, m := range cfg.Mappings {
		if m.RenamePackage == "" {
			continue
		}
		if m.Alias != "" {
			continue
		}
		oldPkg := m.Module
		if idx := strings.LastIndex(oldPkg, "/"); idx >= 0 {
			oldPkg = oldPkg[idx+1:]
		}
		newPath := cfg.TargetModule + "/" + m.Target
		for _, f := range mappedGoFiles {
			changed, err := updateImportAlias(f, oldPkg, m.RenamePackage, newPath)
			if err != nil {
				continue
			}
			if changed {
				aliasUpdateCount++
			}
		}
	}
	fmt.Printf("   ✅ 更新了 %d 个文件的导入别名\n", aliasUpdateCount)

	// 步骤12: 根据最终导入图迭代删除不可解析的生产文件。
	// 测试文件保留，避免迁移时丢失上游单元测试；最终 go test 可暴露测试侧缺失依赖。
	fmt.Println("\n🗑️  步骤12: 删除不可解析的生产文件...")
	removedFiles, err := pruneUnresolvedProductionFiles(targetDir, cfg.Mappings, cfg.TargetModule, externalPrefixes, cfg.ReplaceImports)
	if err != nil {
		fmt.Fprintf(os.Stderr, "   ❌ 删除不可解析文件失败: %v\n", err)
		os.Exit(1)
	}
	for _, f := range removedFiles {
		fmt.Printf("   🗑️  删除 %s\n", f)
	}
	fmt.Printf("   ✅ 删除了 %d 个不可解析的生产文件\n", len(removedFiles))

	fmt.Println("\n🧹 步骤12b: 清理剔除后的空目录...")
	for _, m := range cfg.Mappings {
		subDir := filepath.Join(targetDir, m.Target)
		cleanEmptyDirs(subDir)
	}
	fmt.Println("   ✅ 空目录已清理")

	fmt.Println("\n🔎 步骤13: 验证迁移后的生产导入...")
	if err := validateMigrationOutput(targetDir, cfg.Mappings, cfg.TargetModule, externalPrefixes, cfg.ReplaceImports); err != nil {
		fmt.Fprintf(os.Stderr, "   ❌ 迁移结果验证失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   ✅ 生产导入无多余非迁移依赖")

	// 步骤14: go mod tidy
	fmt.Println("\n🧪 步骤14: go mod tidy...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = targetDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "   ⚠️  go mod tidy 警告: %v\n", err)
	} else {
		fmt.Println("   ✅ go mod tidy 完成")
	}

	// 步骤15: go fmt
	fmt.Println("\n✅ 步骤15: go fmt ...")
	fmtCmd := exec.Command("go", "fmt", "./...")
	fmtCmd.Dir = targetDir
	fmtCmd.Stdout = os.Stdout
	fmtCmd.Stderr = os.Stderr
	if err := fmtCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "   ❌ go fmt 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ go fmt 通过")

	// 步骤16: go build 验证
	fmt.Println("\n✅ 步骤16: go build 验证...")
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
	data, err := readConfigData(path)
	if err != nil {
		return nil, err
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

func readConfigData(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		data, err := embeddedConfigFS.ReadFile(defaultConfigName)
		if err != nil {
			return nil, fmt.Errorf("读取内嵌配置文件 %s: %w", defaultConfigName, err)
		}
		return data, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s: %w", path, err)
	}
	if info.IsDir() {
		path = filepath.Join(path, defaultConfigName)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s: %w", path, err)
	}
	return data, nil
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
			// 保留 testdata 目录中的所有文件（测试用二进制数据）
			if !strings.Contains(rel, "testdata"+string(filepath.Separator)) &&
				!strings.Contains(rel, "testdata/") {
				return nil
			}
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

func pruneUnresolvedProductionFiles(targetDir string, mappings []Mapping, targetModule string, externalPrefixes []string, replaceImports map[string]string) ([]string, error) {
	var removed []string
	moduleMap := make(map[string]*Mapping)
	for i := range mappings {
		moduleMap[mappings[i].Module] = &mappings[i]
	}

	for {
		available, err := availableMigratedPackages(targetDir, mappings, targetModule)
		if err != nil {
			return removed, err
		}

		var batch []string
		for _, m := range mappings {
			root := filepath.Join(targetDir, m.Target)
			if _, err := os.Stat(root); os.IsNotExist(err) {
				continue
			}
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				imports, err := parseImports(path)
				if err != nil {
					return err
				}
				for _, imp := range imports {
					if isAllowedMigratedImport(imp, available, moduleMap, targetModule, externalPrefixes, replaceImports) {
						continue
					}
					rel, _ := filepath.Rel(targetDir, path)
					batch = append(batch, filepath.ToSlash(rel))
					return nil
				}
				return nil
			})
			if err != nil {
				return removed, err
			}
		}

		if len(batch) == 0 {
			return removed, nil
		}
		sort.Strings(batch)
		for _, rel := range batch {
			if err := os.Remove(filepath.Join(targetDir, filepath.FromSlash(rel))); err != nil {
				return removed, err
			}
			removed = append(removed, rel)
		}
	}
}

func availableMigratedPackages(targetDir string, mappings []Mapping, targetModule string) (map[string]bool, error) {
	available := make(map[string]bool)
	for _, m := range mappings {
		root := filepath.Join(targetDir, m.Target)
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			dir := filepath.Dir(path)
			rel, err := filepath.Rel(targetDir, dir)
			if err != nil {
				return err
			}
			available[targetModule+"/"+filepath.ToSlash(rel)] = true
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return available, nil
}

func isAllowedMigratedImport(imp string, available map[string]bool, moduleMap map[string]*Mapping, targetModule string, externalPrefixes []string, replaceImports map[string]string) bool {
	if imp == "C" {
		return true
	}
	if isStdlibImport(imp) {
		return true
	}
	if isExternalDep(imp, externalPrefixes) {
		return true
	}
	for _, newPath := range replaceImports {
		if importPathMatches(imp, newPath) {
			return true
		}
	}
	if available[imp] {
		return true
	}
	rewritten := rewriteImportPath(imp, moduleMap, targetModule, replaceImports)
	return rewritten != imp && available[rewritten]
}

func validateMigrationOutput(targetDir string, mappings []Mapping, targetModule string, externalPrefixes []string, replaceImports map[string]string) error {
	available, err := availableMigratedPackages(targetDir, mappings, targetModule)
	if err != nil {
		return err
	}
	for _, m := range mappings {
		rootImport := targetModule + "/" + filepath.ToSlash(m.Target)
		hasPackage := false
		for imp := range available {
			if importPathMatches(imp, rootImport) {
				hasPackage = true
				break
			}
		}
		if !hasPackage {
			return fmt.Errorf("%s 没有保留可用的生产 Go 文件", m.Target)
		}
	}

	moduleMap := make(map[string]*Mapping)
	for i := range mappings {
		moduleMap[mappings[i].Module] = &mappings[i]
	}
	var unresolved []string
	for _, m := range mappings {
		root := filepath.Join(targetDir, m.Target)
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			imports, err := parseImports(path)
			if err != nil {
				return err
			}
			for _, imp := range imports {
				if isAllowedMigratedImport(imp, available, moduleMap, targetModule, externalPrefixes, replaceImports) {
					continue
				}
				rel, _ := filepath.Rel(targetDir, path)
				unresolved = append(unresolved, fmt.Sprintf("%s imports %s", filepath.ToSlash(rel), imp))
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		return fmt.Errorf("发现不可解析导入: %s", strings.Join(unresolved, "; "))
	}
	return nil
}

func isStdlibImport(imp string) bool {
	first := imp
	if i := strings.IndexByte(first, '/'); i >= 0 {
		first = first[:i]
	}
	return !strings.Contains(first, ".")
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

func rewriteImports(filePath string, moduleMap map[string]*Mapping, targetModule string, replaceImports map[string]string, importAliases map[string]string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, data, parser.ParseComments)
	if err != nil {
		return false, err
	}

	changed := false
	for _, imp := range f.Imports {
		oldPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		newPath := rewriteImportPath(oldPath, moduleMap, targetModule, replaceImports)
		if newPath != oldPath {
			imp.Path.Value = strconv.Quote(newPath)
			changed = true
		}
		if alias := configuredImportAlias(newPath, moduleMap, targetModule, importAliases); alias != "" {
			if imp.Name == nil || imp.Name.Name != alias {
				imp.Name = ast.NewIdent(alias)
				changed = true
			}
		}
	}
	if rewriteComments(f, moduleMap, targetModule, replaceImports) {
		changed = true
	}

	output := data
	if changed {
		var buf bytes.Buffer
		if err := format.Node(&buf, fset, f); err != nil {
			return false, err
		}
		output = buf.Bytes()
	}

	rewritten := []byte(rewriteSourceText(string(output), moduleMap, targetModule, replaceImports))
	if !bytes.Equal(output, rewritten) {
		output = rewritten
		changed = true
	}

	if !changed || bytes.Equal(data, output) {
		return false, nil
	}
	return true, os.WriteFile(filePath, output, 0644)
}

func rewriteComments(f *ast.File, moduleMap map[string]*Mapping, targetModule string, replaceImports map[string]string) bool {
	changed := false
	for _, group := range f.Comments {
		for _, comment := range group.List {
			text := rewriteCommentText(comment.Text, moduleMap, targetModule, replaceImports)
			if text != comment.Text {
				comment.Text = text
				changed = true
			}
		}
	}
	return changed
}

func rewriteCommentText(text string, moduleMap map[string]*Mapping, targetModule string, replaceImports map[string]string) string {
	return rewriteSourceText(text, moduleMap, targetModule, replaceImports)
}

func rewriteSourceText(text string, moduleMap map[string]*Mapping, targetModule string, replaceImports map[string]string) string {
	replacements := importPathReplacements(moduleMap, targetModule, replaceImports)
	for _, r := range replacements {
		text = strings.ReplaceAll(text, r.old, r.new)
	}
	return text
}

type importPathReplacement struct {
	old string
	new string
}

func importPathReplacements(moduleMap map[string]*Mapping, targetModule string, replaceImports map[string]string) []importPathReplacement {
	var replacements []importPathReplacement
	for _, m := range moduleMap {
		replacements = append(replacements, importPathReplacement{
			old: m.Module,
			new: targetModule + "/" + m.Target,
		})
	}
	for old, newPath := range replaceImports {
		replacements = append(replacements, importPathReplacement{old: old, new: newPath})
	}
	sort.Slice(replacements, func(i, j int) bool {
		return len(replacements[i].old) > len(replacements[j].old)
	})
	return replacements
}

func rewriteImportPath(path string, moduleMap map[string]*Mapping, targetModule string, replaceImports map[string]string) string {
	var bestOld, bestNew string
	for _, m := range moduleMap {
		if importPathMatches(path, m.Module) && len(m.Module) > len(bestOld) {
			bestOld = m.Module
			bestNew = targetModule + "/" + m.Target
		}
	}
	for old, newPath := range replaceImports {
		if importPathMatches(path, old) && len(old) > len(bestOld) {
			bestOld = old
			bestNew = newPath
		}
	}
	if bestOld == "" {
		return path
	}
	return bestNew + strings.TrimPrefix(path, bestOld)
}

func configuredImportAlias(path string, moduleMap map[string]*Mapping, targetModule string, importAliases map[string]string) string {
	if alias := importAliases[path]; alias != "" {
		return alias
	}
	for _, m := range moduleMap {
		if m.Alias == "" {
			continue
		}
		if path == targetModule+"/"+m.Target {
			return m.Alias
		}
	}
	return ""
}

func importPathMatches(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
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

// renamePackageDeclarations 重写指定目录（不含子目录）中 .go 文件的 package 声明。
// 例如：package gg → package render，package gg_test → package render_test
func renamePackageDeclarations(root, oldName, newName string) (int, error) {
	count := 0
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(root, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		// 找到 package 声明行并替换
		lines := strings.Split(content, "\n")
		changed := false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "package ") {
				fields := strings.Fields(trimmed)
				if len(fields) >= 2 {
					pkgName := strings.TrimSuffix(fields[1], "_test")
					if pkgName == oldName {
						oldPkg := fields[1]
						newPkg := newName
						if strings.HasSuffix(oldPkg, "_test") {
							newPkg = newName + "_test"
						}
						lines[i] = strings.Replace(line, "package "+oldPkg, "package "+newPkg, 1)
						changed = true
					}
				}
				break // package 声明只有一行
			}
		}
		if changed {
			if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
				return count, err
			}
			count++
			fmt.Printf("   📛 %s: package %s → %s\n", e.Name(), oldName, newName)
		}
	}
	return count, nil
}

// renamePackageQualifier 使用 AST 精确重写文件中包限定符引用。
// 例如：gg.SomeType → render.SomeType，只替换作为 SelectorExpr.X 的 Ident。
// 如果新包名与文件中的其他导入冲突（如 context 与 stdlib 冲突），则跳过重写。
func renamePackageQualifier(filePath, oldName, newName string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	content := string(data)
	original := content

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return false, nil // 忽略无法解析的文件
	}

	// 检查新包名是否与文件中的其他导入冲突
	// 如果新包名已经被其他导入（非当前包）使用，则跳过重写
	newNameInUse := false
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		// 跳过当前要处理的导入
		if strings.HasSuffix(path, "/"+oldName) || strings.HasSuffix(path, "/"+newName) {
			continue
		}
		// 检查其他导入的默认包名（无别名时使用路径最后一段）
		pkgName := path
		if idx := strings.LastIndex(pkgName, "/"); idx >= 0 {
			pkgName = pkgName[idx+1:]
		}
		// 或用别名
		if imp.Name != nil {
			pkgName = imp.Name.Name
		}
		if pkgName == newName {
			newNameInUse = true
			break
		}
	}
	if newNameInUse {
		return false, nil
	}

	// 查找所有 SelectorExpr 中 X 为 oldName 的 Ident
	var positions []token.Pos
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != oldName {
			return true
		}
		positions = append(positions, ident.NamePos)
		return true
	})

	if len(positions) == 0 {
		return false, nil
	}

	// 逆序替换，避免偏移量变化
	sort.Slice(positions, func(i, j int) bool {
		return positions[i] > positions[j]
	})

	for _, pos := range positions {
		p := fset.Position(pos)
		offset := p.Offset
		if offset+len(oldName) <= len(content) && content[offset:offset+len(oldName)] == oldName {
			content = content[:offset] + newName + content[offset+len(oldName):]
		}
	}

	if content != original {
		return true, os.WriteFile(filePath, []byte(content), 0644)
	}
	return false, nil
}

// listAllGoFiles 递归列出目录下所有 .go 文件的绝对路径。
func listAllGoFiles(root string) []string {
	var files []string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func listMappedGoFiles(targetDir string, mappings []Mapping) []string {
	var files []string
	for _, m := range mappings {
		files = append(files, listAllGoFiles(filepath.Join(targetDir, m.Target))...)
	}
	sort.Strings(files)
	return files
}

// addStdlibConflictAlias 检测 stdlib 冲突并添加导入别名。
// 当导入路径匹配 newPath 且新包名与文件中的其他导入冲突时（如 stdlib "context"），
// 添加别名：oldName "github.com/energye/gpui/gpu/context"
func addStdlibConflictAlias(filePath, oldName, newPkgName, newPath string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	content := string(data)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return false, nil
	}

	// 检查文件是否已经导入了新包名（作为其他包）
	hasStdlibConflict := false
	targetImported := false
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		if path == newPath || strings.HasPrefix(path, newPath+"/") {
			targetImported = true
			// 如果已经有别名且别名与旧名不同，说明已处理
			if imp.Name != nil && imp.Name.Name != newPkgName {
				return false, nil
			}
			continue
		}
		// 检查是否有其他导入使用了新包名（如 stdlib "context"）
		if imp.Name == nil {
			pkgName := path
			if idx := strings.LastIndex(pkgName, "/"); idx >= 0 {
				pkgName = pkgName[idx+1:]
			}
			if pkgName == newPkgName {
				hasStdlibConflict = true
			}
		} else if imp.Name.Name == newPkgName {
			hasStdlibConflict = true
		}
	}

	if !targetImported || !hasStdlibConflict {
		return false, nil
	}

	// 已有别名，不需要添加
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		if path == newPath || strings.HasPrefix(path, newPath+"/") {
			if imp.Name != nil {
				return false, nil
			}
		}
	}

	// 添加别名：oldName "newPath"
	// 找到目标 import 并添加别名
	oldImport := `"` + newPath + `"`
	newImport := oldName + ` "` + newPath + `"`
	if strings.Contains(content, oldImport) {
		content = strings.Replace(content, oldImport, newImport, 1)
		if content != string(data) {
			return true, os.WriteFile(filePath, []byte(content), 0644)
		}
	}
	return false, nil
}

// hasImportAlias 检查文件是否对指定导入路径使用了别名。
func hasImportAlias(filePath, oldName, newPath string) (bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return false, nil
	}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		if path == newPath || strings.HasPrefix(path, newPath+"/") {
			if imp.Name != nil && imp.Name.Name == oldName {
				return true, nil
			}
		}
	}
	return false, nil
}

// updateImportAlias 更新导入别名：当别名匹配 oldName 且导入路径匹配 newPath 时，
// 将别名从 oldName 更新为 newPkgName。但如果新包名与 stdlib 冲突，则保留旧别名。
func updateImportAlias(filePath, oldName, newPkgName, newPath string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	content := string(data)
	original := content

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return false, nil
	}

	type fix struct {
		start, end int
		text       string
	}
	var fixes []fix

	for _, imp := range f.Imports {
		if imp.Name == nil {
			continue
		}
		alias := imp.Name.Name
		if alias != oldName {
			continue
		}
		path := strings.Trim(imp.Path.Value, "\"")
		if !strings.HasPrefix(path, newPath) && !strings.HasPrefix(path, newPath+"/") {
			continue
		}
		// 如果新包名与旧别名相同，不需要修改
		if newPkgName == oldName {
			continue
		}
		// 检查新包名是否与 stdlib 冲突
		hasConflict := false
		for _, other := range f.Imports {
			if other == imp {
				continue
			}
			otherPath := strings.Trim(other.Path.Value, "\"")
			otherName := otherPath
			if idx := strings.LastIndex(otherName, "/"); idx >= 0 {
				otherName = otherName[idx+1:]
			}
			if other.Name != nil {
				otherName = other.Name.Name
			}
			if otherName == newPkgName {
				hasConflict = true
				break
			}
		}
		if hasConflict {
			continue // 保留旧别名
		}
		// 更新别名：oldName → newPkgName
		start := imp.Name.Pos()
		end := imp.Name.End()
		fixes = append(fixes, fix{
			start: int(start) - 1,
			end:   int(end) - 1,
			text:  newPkgName,
		})
	}

	if len(fixes) == 0 {
		return false, nil
	}

	sort.Slice(fixes, func(i, j int) bool {
		return fixes[i].start > fixes[j].start
	})

	for _, fx := range fixes {
		content = content[:fx.start] + fx.text + content[fx.end:]
	}

	if content != original {
		return true, os.WriteFile(filePath, []byte(content), 0644)
	}
	return false, nil
}
