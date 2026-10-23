package scanner

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/yv3000/siig/internal/version"
	"github.com/yv3000/siig/rules"
)

const maxDepth = 3

// noisyDetectNames are entrypoint-ish file names that live deep inside almost
// every repository. They only identify a stack when they sit at the project
// root — otherwise src/index.js would report an Express server that isn't there.
var noisyDetectNames = map[string]bool{
	"index.js": true, "index.ts": true, "main.ts": true, "main.js": true,
	"app.js": true, "app.ts": true, "server.js": true, "server.ts": true,
	"main.py": true, "app.py": true, "router.ts": true, "_app.ts": true,
}

// noisyDetectDirs are tool directories present in nearly every checkout, so
// their existence alone must not select a stack.
var noisyDetectDirs = map[string]bool{".vscode": true}

// selfAuthoredTargets are the files SIIG writes. Their content is a footprint,
// not a signal: without this guard a second run detects the stacks its own
// output created and appends more entries, breaking idempotency.
var selfAuthoredTargets = map[string]bool{
	".gitignore":    true,
	".vercelignore": true,
	".dockerignore": true,
	".npmignore":    true,
}

// DetectedStack holds info about a detected stack
type DetectedStack struct {
	Rule      rules.StackRule
	MatchedBy string // what triggered detection (e.g. "package.json")
}

// ScanResult holds everything the scanner found
type ScanResult struct {
	DetectedStacks []DetectedStack
	HasVSCodeDir   bool
	HasVercelJSON  bool
	HasDockerfile  bool
	HasNpmLib      bool // package.json with "main" or "exports" — it's a library
	ScanErrors     []string
}

// Scan walks the project directory (max depth 3), collects file/dir names,
// and matches them against detection rules.
func Scan(projectRoot string, stackRules []rules.StackRule) (*ScanResult, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return nil, fmt.Errorf("project root is required")
	}
	info, err := os.Stat(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("project root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project root is not a directory: %s", projectRoot)
	}

	return scanFS(os.DirFS(projectRoot), stackRules)
}

// scanFS is the walk itself, split out so tests can drive it with an in-memory
// or deliberately failing filesystem.
func scanFS(fsys fs.FS, stackRules []rules.StackRule) (*ScanResult, error) {
	idx := newIndex()
	var scanErrs []string

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Keep going, but remember what we could not read.
			scanErrs = append(scanErrs, fmt.Sprintf("%s: %v", path, err))
			return nil
		}

		// The walk root is the project itself — its own name is not a signal.
		if path == "." {
			return nil
		}

		if depth := strings.Count(path, "/"); depth > maxDepth {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			// node_modules contents are irrelevant — only its existence matters
			if d.Name() == "node_modules" {
				idx.dirs["node_modules"] = true
				return fs.SkipDir
			}
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			idx.dirs[d.Name()] = true
			idx.paths[path] = true
			return nil
		}

		idx.files[d.Name()] = true
		idx.paths[path] = true
		if !strings.Contains(path, "/") {
			idx.rootFiles[d.Name()] = true
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	result := &ScanResult{ScanErrors: scanErrs}
	authored := siigAuthoredFiles(fsys)

	result.HasVSCodeDir = idx.dirs[".vscode"]
	result.HasVercelJSON = idx.files["vercel.json"] || idx.files[".vercelignore"]
	result.HasDockerfile = idx.files["Dockerfile"] || idx.files["docker-compose.yml"] || idx.files["docker-compose.yaml"]
	result.HasNpmLib = detectNpmLib(fsys)

	for _, rule := range rules.GetDetectable(stackRules) {
		if rule.Detect == nil {
			continue
		}
		if matched, matchedBy := matchRule(rule, idx, authored); matched {
			result.DetectedStacks = append(result.DetectedStacks, DetectedStack{
				Rule:      rule,
				MatchedBy: matchedBy,
			})
		}
	}

	return result, nil
}

// nameIndex is what the walk collected, keyed so each detect pattern can be
// looked up at the scope it is trustworthy in.
type nameIndex struct {
	files     map[string]bool
	dirs      map[string]bool
	paths     map[string]bool
	rootFiles map[string]bool
}

func newIndex() *nameIndex {
	return &nameIndex{
		files:     make(map[string]bool),
		dirs:      make(map[string]bool),
		paths:     make(map[string]bool),
		rootFiles: make(map[string]bool),
	}
}

// matchRule checks if any detection file or dir identifies this rule
func matchRule(rule rules.StackRule, idx *nameIndex, authored map[string]bool) (bool, string) {
	for _, f := range rule.Detect.Files {
		lower := strings.ToLower(f)
		if selfAuthoredTargets[lower] && authored[lower] {
			continue
		}
		if matchedBy, ok := idx.findFile(f, lower); ok {
			return true, matchedBy
		}
	}

	for _, d := range rule.Detect.Dirs {
		if noisyDetectDirs[strings.ToLower(d)] {
			continue
		}
		if idx.dirs[d] || idx.paths[d] {
			return true, d + "/"
		}
	}

	return false, ""
}

// findFile resolves one detect pattern against the index and reports the name
// that triggered it.
func (idx *nameIndex) findFile(exact, lower string) (string, bool) {
	if noisyDetectNames[lower] {
		if idx.rootFiles[exact] {
			return exact, true
		}
		return "", false
	}
	if idx.files[exact] || idx.paths[exact] {
		return exact, true
	}
	if strings.HasPrefix(exact, "*") {
		suffix := exact[1:]
		for name := range idx.files {
			if strings.HasSuffix(name, suffix) {
				return name, true
			}
		}
	}
	return "", false
}

// siigAuthoredFiles reports which of SIIG's own output files already carry the
// SIIG marker line.
func siigAuthoredFiles(fsys fs.FS) map[string]bool {
	authored := make(map[string]bool, len(selfAuthoredTargets))
	for name := range selfAuthoredTargets {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			continue
		}
		authored[name] = strings.Contains(string(data), version.Marker)
	}
	return authored
}

// detectNpmLib is true when root package.json looks like a published library.
func detectNpmLib(fsys fs.FS) bool {
	data, err := fs.ReadFile(fsys, "package.json")
	if err != nil {
		return false
	}
	var pkg struct {
		Main    string          `json:"main"`
		Exports json.RawMessage `json:"exports"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return false
	}
	return pkg.Main != "" || len(pkg.Exports) > 0
}
