package architecture_test

import (
        "go/parser"
        "go/token"
        "io/fs"
        "path/filepath"
        "strings"
        "testing"
)

func TestIdentityStoreHasNoIllegalImporters(t *testing.T) {
        root := filepath.Clean("../..")
        err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
                if walkErr != nil {
                        return walkErr
                }
                if entry.IsDir() {
                        name := entry.Name()
                        if name == ".git" || name == "_bmad" || name == ".agents" || name == ".codex" || name == "graphify-out" || name == "node_modules" || name == "vendor" {
                                return filepath.SkipDir
                        }
                        if name == "utils" || name == "helpers" || name == "common" {
                                t.Errorf("forbidden generic package: %s", path)
                        }
                        return nil
                }
                if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
                        return nil
                }
                parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
                if err != nil {
                        return err
                }
                for _, imported := range parsed.Imports {
                        name := strings.Trim(imported.Path.Value, `"`)
                        rel, _ := filepath.Rel(root, path)
                        if name == "github.com/artemiosu/staffy/internal/employee/store" && !inside(rel, filepath.Join("cmd", "staffy-core")) && !inside(rel, filepath.Join("internal", "employee")) {
                                t.Errorf("%s imports employee/store outside its owner or composition root", rel)
                        }
                        if name == "github.com/artemiosu/staffy/internal/connector/store" && !inside(rel, filepath.Join("cmd", "staffy-core")) && !inside(rel, filepath.Join("internal", "connector")) {
                                t.Errorf("%s imports connector/store outside its owner or composition root", rel)
                        }
                        if name == "github.com/artemiosu/staffy/internal/cases/store" && !inside(rel, filepath.Join("cmd", "staffy-core")) && !inside(rel, filepath.Join("internal", "cases")) {
                                t.Errorf("%s imports cases/store outside its owner or composition root", rel)
                        }
                        // DAG rule of the case contour (spec 2-1): privacy and
                        // entitlement are NOT in the case spine — their facts
                        // are relayed through the employee facade; a direct
                        // import would bypass the declared architecture.
                        if inside(rel, filepath.Join("internal", "cases")) && (strings.HasPrefix(name, "github.com/artemiosu/staffy/internal/privacy") || strings.HasPrefix(name, "github.com/artemiosu/staffy/internal/entitlement")) {
                                t.Errorf("%s imports %s: privacy/entitlement are not in the case DAG (relay via employee/api)", rel, name)
                        }
                        if name != "github.com/artemiosu/staffy/internal/identity/store" {
                                continue
                        }
                        if !inside(rel, filepath.Join("cmd", "staffy-core")) && !inside(rel, filepath.Join("internal", "identity")) {
                                t.Errorf("%s imports identity/store outside its owner or composition root", rel)
                        }
                }
                return nil
        })
        if err != nil {
                t.Fatal(err)
        }
}

func inside(path, directory string) bool {
        return path == directory || strings.HasPrefix(path, directory+string(filepath.Separator))
}
